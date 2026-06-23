package batch_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
	postgresrepo "github.com/leyl1ne/ProductService/internal/repository/postgres"
	"github.com/leyl1ne/ProductService/internal/service/batch"
	"github.com/leyl1ne/ProductService/internal/testutils"
	batchhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/batch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupBatchHandler builds a real database-backed handler stack for
// integration tests. It returns:
//   - *batchhandler.Handler    — the handler under test
//   - *postgresrepo.Repository  — the underlying repository (used by helpers
//     that pre-seed products / batches before the test request)
//
// Each call spins up a fresh Postgres testcontainer via testutils.SetupPostgres.
func setupBatchHandler(t *testing.T) (*batchhandler.Handler, *postgresrepo.Repository) {
	t.Helper()

	db := testutils.SetupPostgres(t)
	repo := postgresrepo.NewPostgresRepository(db.Pool())
	batchSvc := batch.NewService(repo)
	log := testutils.NewDiscardLogger(t)

	handler := batchhandler.NewHandler(log, batchSvc)
	return handler, repo
}

// setupRouter wires the batch handler onto a fresh gin engine with the
// test auth middleware already installed.
func setupRouter(h *batchhandler.Handler) *gin.Engine {
	r := testutils.SetupGin()
	r.Use(testutils.AuthMiddleware())

	r.POST("/products/:id/batches", h.CreateBatch())
	r.GET("/products/:id/batches", h.ListBatchesByProduct())
	r.GET("/batches/:id", h.GetBatch())
	r.PATCH("/batches/:id", h.UpdateBatch())
	r.DELETE("/batches/:id", h.DeleteBatch())
	return r
}

// seedProduct inserts a product directly via the repository and returns
// the inserted row's ID. Batches must reference an existing product.
func seedProduct(
	t *testing.T,
	repo *postgresrepo.Repository,
	companyID uuid.UUID,
	name string,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	p := productmodel.Product{
		ID:        uuid.New(),
		CompanyID: companyID,
		Name:      name,
		Price:     1.0,
		Unit:      "kg",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	created, err := repo.CreateProduct(ctx, p)
	require.NoError(t, err)
	return created.ID
}

// seedBatch inserts a batch directly via the repository and returns the
// inserted row's ID. Used for tests that need an existing batch to fetch /
// update / delete.
func seedBatch(
	t *testing.T,
	repo *postgresrepo.Repository,
	productID uuid.UUID,
	quantityTotal float64,
	quantityReserved float64,
	expirationDateStr string,
) uuid.UUID {
	t.Helper()

	var expDate *time.Time
	if expirationDateStr != "" {
		parsed, err := time.Parse("2006-01-02", expirationDateStr)
		require.NoError(t, err)
		expDate = &parsed
	}

	b := batchmodel.ProductBatch{
		ID:               uuid.New(),
		ProductID:        productID,
		QuantityTotal:    quantityTotal,
		QuantityReserved: quantityReserved,
		ExpirationDate:   expDate,
		BatchNumber:      "BATCH-" + uuid.NewString()[:8],
		CreatedAt:        time.Now(),
	}
	created, err := repo.CreateBatch(context.Background(), b)
	require.NoError(t, err)
	return created.ID
}

// ============================================
// CREATE BATCH TESTS
// ============================================

func TestBatchHandler_CreateBatch_Integration(t *testing.T) {
	handler, repo := setupBatchHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	productA := seedProduct(t, repo, companyA, "Apple")

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}
	buyerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleBuyer}

	tests := []struct {
		name           string
		user           auth.User
		productID      string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:      "success - SELLER creates batch on own product",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": 100.0,
				"batch_number":   "BATCH-001",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.NotEqual(t, uuid.Nil, resp.ID)
				assert.Equal(t, productA, resp.ProductID)
				assert.Equal(t, float64(100), resp.QuantityTotal)
				assert.Equal(t, float64(0), resp.QuantityReserved)
				assert.Equal(t, float64(100), resp.AvailableQuantity)
				assert.Equal(t, "BATCH-001", resp.BatchNumber)
				assert.NotEmpty(t, resp.CreatedAt)
			},
		},
		{
			name:      "success - ADMIN creates batch on any product",
			user:      admin,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total":  50.0,
				"expiration_date": "2026-12-31",
				"production_date": "2026-01-01",
				"batch_number":    "ADM-001",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, float64(50), resp.QuantityTotal)
				require.NotNil(t, resp.ExpirationDate)
				assert.Equal(t, "2026-12-31", *resp.ExpirationDate)
				require.NotNil(t, resp.ProductionDate)
				assert.Equal(t, "2026-01-01", *resp.ProductionDate)
			},
		},
		{
			name:      "success - with warehouse_company_id",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total":       25.0,
				"warehouse_company_id": uuid.NewString(),
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				require.NotNil(t, resp.WarehouseCompanyID)
			},
		},
		{
			name:      "forbidden - BUYER cannot create batch",
			user:      buyerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": 100.0,
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "forbidden - seller from different company",
			user:      sellerB,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": 100.0,
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "forbidden - no user in context",
			user:      auth.User{},
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": 100.0,
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "product not found",
			user:      sellerA,
			productID: uuid.NewString(),
			body: map[string]interface{}{
				"quantity_total": 100.0,
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "product not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "bad request - invalid product UUID",
			user:      sellerA,
			productID: "not-a-uuid",
			body: map[string]interface{}{
				"quantity_total": 100.0,
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "invalid product id", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "validation error - missing quantity_total",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"batch_number": "BATCH-X",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "field is required", fields["quantitytotal"])
			},
		},
		{
			name:      "validation error - zero quantity_total",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": 0,
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:      "validation error - negative quantity_total",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": -5,
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:      "validation error - invalid warehouse_company_id format",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total":       100,
				"warehouse_company_id": "not-a-uuid",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:      "validation error - invalid production_date format",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total":  100,
				"production_date": "2026/01/01",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:      "validation error - invalid expiration_date format",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total":  100,
				"expiration_date": "31-12-2026",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:      "validation error - batch_number too long",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"quantity_total": 100,
				"batch_number":   strings.Repeat("x", 101),
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "too long", fields["batchnumber"])
			},
		},
		{
			name:           "validation error - empty body",
			user:           sellerA,
			productID:      productA.String(),
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "request body is empty", errResp["error"].(map[string]interface{})["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			var reqBody []byte
			if tt.body != nil {
				var err error
				reqBody, err = json.Marshal(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/products/%s/batches", tt.productID), bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.Bytes())
			}
		})
	}
}

// ============================================
// LIST BATCHES BY PRODUCT TESTS
// ============================================

func TestBatchHandler_ListBatchesByProduct_Integration(t *testing.T) {
	handler, repo := setupBatchHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	productA := seedProduct(t, repo, companyA, "Apple")
	productB := seedProduct(t, repo, companyB, "Banana")

	// Seed multiple batches for productA
	seedBatch(t, repo, productA, 100, 0, "2026-12-31")
	seedBatch(t, repo, productA, 50, 20, "2026-06-30")
	seedBatch(t, repo, productA, 75, 75, "2026-09-15")

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

	tests := []struct {
		name           string
		user           auth.User
		productID      string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "success - owner lists batches",
			user:           sellerA,
			productID:      productA.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Len(t, resp.Batches, 3)
				// Verify computed available_quantity for each batch
				for _, b := range resp.Batches {
					assert.Equal(t, productA, b.ProductID)
					assert.GreaterOrEqual(t, b.AvailableQuantity, float64(0))
				}
			},
		},
		{
			name:           "success - admin lists batches",
			user:           admin,
			productID:      productA.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Len(t, resp.Batches, 3)
			},
		},
		{
			name:           "success - product without batches returns empty list",
			user:           sellerB,
			productID:      productB.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Empty(t, resp.Batches)
			},
		},
		{
			name:           "forbidden - seller from different company",
			user:           sellerB,
			productID:      productA.String(),
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "forbidden - no user in context",
			user:           auth.User{},
			productID:      productA.String(),
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "product not found",
			user:           sellerA,
			productID:      uuid.NewString(),
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "product not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "bad request - invalid UUID",
			user:           sellerA,
			productID:      "not-a-uuid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "invalid product id", errResp["error"].(map[string]interface{})["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%s/batches", tt.productID), nil)
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.Bytes())
			}
		})
	}
}

// ============================================
// GET BATCH TESTS
// ============================================

func TestBatchHandler_GetBatch_Integration(t *testing.T) {
	handler, repo := setupBatchHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	productA := seedProduct(t, repo, companyA, "Apple")
	productB := seedProduct(t, repo, companyB, "Banana")
	batchA := seedBatch(t, repo, productA, 100, 30, "2026-12-31")
	batchB := seedBatch(t, repo, productB, 50, 0, "2026-06-30")

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

	tests := []struct {
		name           string
		user           auth.User
		batchID        string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "success - owner fetches own batch",
			user:           sellerA,
			batchID:        batchA.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, batchA, resp.ID)
				assert.Equal(t, productA, resp.ProductID)
				assert.Equal(t, float64(100), resp.QuantityTotal)
				assert.Equal(t, float64(30), resp.QuantityReserved)
				assert.Equal(t, float64(70), resp.AvailableQuantity)
				require.NotNil(t, resp.ExpirationDate)
				assert.Equal(t, "2026-12-31", *resp.ExpirationDate)
			},
		},
		{
			name:           "success - admin fetches any batch",
			user:           admin,
			batchID:        batchB.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, batchB, resp.ID)
				assert.Equal(t, productB, resp.ProductID)
			},
		},
		{
			name:           "forbidden - seller from different company",
			user:           sellerB,
			batchID:        batchA.String(),
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "forbidden - no user in context",
			user:           auth.User{},
			batchID:        batchA.String(),
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "not found - non-existent UUID",
			user:           sellerA,
			batchID:        uuid.NewString(),
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "batch not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "bad request - invalid UUID",
			user:           sellerA,
			batchID:        "not-a-uuid",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "invalid batch id", errResp["error"].(map[string]interface{})["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/batches/%s", tt.batchID), nil)
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.Bytes())
			}
		})
	}
}

// ============================================
// UPDATE BATCH TESTS
// ============================================

func TestBatchHandler_UpdateBatch_Integration(t *testing.T) {
	handler, repo := setupBatchHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	productA := seedProduct(t, repo, companyA, "Apple")
	productB := seedProduct(t, repo, companyB, "Banana")
	batchA := seedBatch(t, repo, productA, 100, 0, "2026-12-31")
	batchB := seedBatch(t, repo, productB, 50, 0, "")

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

	newBatchNumber := "UPDATED-001"
	newExpiration := "2027-06-30"

	tests := []struct {
		name           string
		user           auth.User
		batchID        string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:    "success - owner updates batch_number",
			user:    sellerA,
			batchID: batchA.String(),
			body: map[string]interface{}{
				"batch_number": newBatchNumber,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, batchA, resp.ID)
				assert.Equal(t, newBatchNumber, resp.BatchNumber)
				// Other fields unchanged
				assert.Equal(t, float64(100), resp.QuantityTotal)
			},
		},
		{
			name:    "success - owner updates expiration_date",
			user:    sellerA,
			batchID: batchA.String(),
			body: map[string]interface{}{
				"expiration_date": newExpiration,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				require.NotNil(t, resp.ExpirationDate)
				assert.Equal(t, newExpiration, *resp.ExpirationDate)
			},
		},
		{
			name:    "success - admin updates any batch",
			user:    admin,
			batchID: batchB.String(),
			body: map[string]interface{}{
				"production_date": "2026-01-15",
				"batch_number":    "ADM-UPDATED",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp batchhandler.BatchResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				require.NotNil(t, resp.ProductionDate)
				assert.Equal(t, "2026-01-15", *resp.ProductionDate)
				assert.Equal(t, "ADM-UPDATED", resp.BatchNumber)
			},
		},
		{
			name:    "forbidden - seller from different company",
			user:    sellerB,
			batchID: batchA.String(),
			body: map[string]interface{}{
				"batch_number": "HACKED",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:    "forbidden - no user in context",
			user:    auth.User{},
			batchID: batchA.String(),
			body: map[string]interface{}{
				"batch_number": "Anonymous",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:    "not found - non-existent UUID",
			user:    sellerA,
			batchID: uuid.NewString(),
			body: map[string]interface{}{
				"batch_number": "Updated",
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "batch not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:    "bad request - invalid UUID",
			user:    sellerA,
			batchID: "not-a-uuid",
			body: map[string]interface{}{
				"batch_number": "Updated",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "invalid batch id", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "validation error - empty body",
			user:           sellerA,
			batchID:        batchA.String(),
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "request body is empty", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:    "validation error - invalid expiration_date format",
			user:    sellerA,
			batchID: batchA.String(),
			body: map[string]interface{}{
				"expiration_date": "2026/12/31",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:    "validation error - batch_number too long",
			user:    sellerA,
			batchID: batchA.String(),
			body: map[string]interface{}{
				"batch_number": strings.Repeat("x", 101),
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "too long", fields["batchnumber"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			var reqBody []byte
			if tt.body != nil {
				var err error
				reqBody, err = json.Marshal(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/batches/%s", tt.batchID), bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.Bytes())
			}
		})
	}
}

// ============================================
// DELETE BATCH TESTS
// ============================================

func TestBatchHandler_DeleteBatch_Integration(t *testing.T) {
	handler, repo := setupBatchHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	productA := seedProduct(t, repo, companyA, "Apple")
	productB := seedProduct(t, repo, companyB, "Banana")

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

	tests := []struct {
		name           string
		user           auth.User
		seedBatch      bool   // if true, seed a fresh batch on productA
		batchID        string // override; if empty use the seeded batch id
		expectedStatus int
	}{
		{
			name:           "success - owner deletes own batch",
			user:           sellerA,
			seedBatch:      true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "success - admin deletes any batch",
			user:           admin,
			seedBatch:      true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "forbidden - seller from different company",
			user:           sellerB,
			seedBatch:      true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "forbidden - no user in context",
			user:           auth.User{},
			seedBatch:      true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not found",
			user:           sellerA,
			seedBatch:      false,
			batchID:        uuid.NewString(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "bad request - invalid UUID",
			user:           sellerA,
			seedBatch:      false,
			batchID:        "not-a-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			batchID := tt.batchID
			if tt.seedBatch {
				batchID = seedBatch(t, repo, productA, 100, 0, "2026-12-31").String()
			}
			// also seed a batch on productB so admin test case has something different to delete
			if tt.name == "success - admin deletes any batch" {
				batchID = seedBatch(t, repo, productB, 50, 0, "").String()
			}

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/batches/%s", batchID), nil)
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			// For successful deletes, verify the batch is actually gone.
			if tt.expectedStatus == http.StatusNoContent {
				_, err := repo.GetBatchByID(context.Background(), uuid.MustParse(batchID))
				require.Error(t, err, "batch should be deleted from DB")
			}
		})
	}
}
