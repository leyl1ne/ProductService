package product_test

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
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
	postgresrepo "github.com/leyl1ne/ProductService/internal/repository/postgres"
	"github.com/leyl1ne/ProductService/internal/service/product"
	"github.com/leyl1ne/ProductService/internal/testutils"
	producthandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/product"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupProductHandler builds a real database-backed handler stack for
// integration tests. It returns:
//   - *producthandler.Handler — the handler under test
//   - *postgresrepo.Repository  — the underlying repository (used by helpers
//     that pre-seed categories / products before the test request)
//
// Each call spins up a fresh Postgres testcontainer via testutils.SetupPostgres,
// so tests are fully isolated.
func setupProductHandler(t *testing.T) (*producthandler.Handler, *postgresrepo.Repository) {
	t.Helper()

	db := testutils.SetupPostgres(t)

	repo := postgresrepo.NewPostgresRepository(db.Pool())
	productSvc := product.NewService(repo)

	// Discard logs in tests — we don't assert on log output.
	log := testutils.NewDiscardLogger(t)

	handler := producthandler.NewHandler(log, productSvc)
	return handler, repo
}

// setupRouter wires the product handler onto a fresh gin engine with the
// test auth middleware already installed.
func setupRouter(h *producthandler.Handler) *gin.Engine {
	r := testutils.SetupGin()
	r.Use(testutils.AuthMiddleware())

	r.POST("/products", h.CreateProduct())
	r.GET("/products", h.ListProducts())
	r.GET("/products/:id", h.GetProduct())
	r.PATCH("/products/:id", h.UpdateProduct())
	r.DELETE("/products/:id", h.DeleteProduct())
	r.GET("/products/:id/availability", h.GetProductAvailability())
	return r
}

// seedCategory inserts a category directly via the repository and returns
// the inserted row. Used for tests that need a valid category_id.
func seedCategory(t *testing.T, repo *postgresrepo.Repository, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	created, err := repo.CreateCategory(ctx, categorymodel.Category{ID: id, Name: name})
	require.NoError(t, err)
	return created.ID
}

// seedProduct inserts a product directly via the repository and returns
// the inserted row. companyID is required, categoryID may be nil.
func seedProduct(
	t *testing.T,
	repo *postgresrepo.Repository,
	companyID uuid.UUID,
	categoryID *uuid.UUID,
	name string,
	price float64,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	product := productmodel.Product{
		ID:         uuid.New(),
		CompanyID:  companyID,
		CategoryID: categoryID,
		Name:       name,
		Price:      price,
	}
	created, err := repo.CreateProduct(ctx, product)
	require.NoError(t, err)
	return created.ID
}

// ============================================
// CREATE PRODUCT TESTS
// ============================================

func TestProductHandler_CreateProduct_Integration(t *testing.T) {
	handler, repo := setupProductHandler(t)

	// Pre-existing category used by happy-path tests.
	categoryID := seedCategory(t, repo, "Fruits")

	// A seller from company A — used for the success cases.
	companyA := uuid.New()
	sellerA := auth.User{
		ID:        uuid.New(),
		CompanyID: companyA,
		Role:      auth.RoleSeller,
	}
	// An admin without a company — also allowed by policy.
	admin := auth.User{
		ID:        uuid.New(),
		CompanyID: uuid.Nil,
		Role:      auth.RoleAdmin,
	}
	// A buyer from company A — not allowed to create products.
	buyerA := auth.User{
		ID:        uuid.New(),
		CompanyID: companyA,
		Role:      auth.RoleBuyer,
	}

	tests := []struct {
		name           string
		user           auth.User
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name: "success - SELLER with category",
			user: sellerA,
			body: map[string]interface{}{
				"name":        "Apple",
				"description": "Fresh apple",
				"price":       10.5,
				"unit":        "kg",
				"category_id": categoryID.String(),
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.NotEqual(t, uuid.Nil, resp.ID)
				assert.Equal(t, "Apple", resp.Name)
				assert.Equal(t, "Fresh apple", resp.Description)
				assert.Equal(t, 10.5, resp.Price)
				assert.Equal(t, "kg", resp.Unit)
				assert.True(t, resp.IsActive)
				assert.Equal(t, companyA, resp.CompanyID)
				require.NotNil(t, resp.Category)
				assert.Equal(t, categoryID, resp.Category.ID)
				assert.Equal(t, "Fruits", resp.Category.Name)
			},
		},
		{
			name: "success - ADMIN without category",
			user: admin,
			body: map[string]interface{}{
				"name":  "Plain Item",
				"price": 5.0,
				"unit":  "pcs",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.NotEqual(t, uuid.Nil, resp.ID)
				assert.Equal(t, "Plain Item", resp.Name)
				assert.Equal(t, 5.0, resp.Price)
				assert.Nil(t, resp.Category)
			},
		},
		{
			name: "forbidden - BUYER cannot create product",
			user: buyerA,
			body: map[string]interface{}{
				"name":  "Forbidden",
				"price": 1.0,
				"unit":  "kg",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name: "unauthorized - missing X-User-ID header",
			user: auth.User{}, // zero value → no headers applied
			body: map[string]interface{}{
				"name":  "Anonymous",
				"price": 1.0,
				"unit":  "kg",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name: "category not found",
			user: sellerA,
			body: map[string]interface{}{
				"name":        "Ghost",
				"price":       1.0,
				"unit":        "kg",
				"category_id": uuid.NewString(), // non-existent
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "category not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name: "validation error - missing name",
			user: sellerA,
			body: map[string]interface{}{
				"price": 10.5,
				"unit":  "kg",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "field is required", fields["name"])
			},
		},
		{
			name: "validation error - empty name",
			user: sellerA,
			body: map[string]interface{}{
				"name":  "",
				"price": 10.5,
				"unit":  "kg",
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
			name: "validation error - price missing",
			user: sellerA,
			body: map[string]interface{}{
				"name": "Apple",
				"unit": "kg",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "field is required", fields["price"])
			},
		},
		{
			name: "validation error - zero price",
			user: sellerA,
			body: map[string]interface{}{
				"name":  "Apple",
				"price": 0,
				"unit":  "kg",
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
			name: "validation error - negative price",
			user: sellerA,
			body: map[string]interface{}{
				"name":  "Apple",
				"price": -1,
				"unit":  "kg",
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
			name: "validation error - missing unit",
			user: sellerA,
			body: map[string]interface{}{
				"name":  "Apple",
				"price": 10.5,
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "field is required", fields["unit"])
			},
		},
		{
			name: "validation error - invalid category_id format",
			user: sellerA,
			body: map[string]interface{}{
				"name":        "Apple",
				"price":       10.5,
				"unit":        "kg",
				"category_id": "not-a-uuid",
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
			name: "validation error - description too long",
			user: sellerA,
			body: map[string]interface{}{
				"name":        "Apple",
				"description": strings.Repeat("x", 2001),
				"price":       10.5,
				"unit":        "kg",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "too long", fields["description"])
			},
		},
		{
			name:           "validation error - empty body",
			user:           sellerA,
			body:           nil,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "request body is empty", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name: "validation error - malformed JSON",
			user: sellerA,
			body: map[string]interface{}{
				"__malformed__": nil, // sentinel — see test runner below
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				msg, _ := errResp["error"].(map[string]interface{})["message"].(string)
				assert.Contains(t, msg, "invalid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			var reqBody []byte
			if tt.body != nil {
				if _, ok := tt.body["__malformed__"]; ok {
					reqBody = []byte(`{not valid json`)
				} else {
					var err error
					reqBody, err = json.Marshal(tt.body)
					require.NoError(t, err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(reqBody))
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
// GET PRODUCT TESTS
// ============================================

func TestProductHandler_GetProduct_Integration(t *testing.T) {
	handler, repo := setupProductHandler(t)

	companyID := uuid.New()
	categoryID := seedCategory(t, repo, "Drinks")
	productID := seedProduct(t, repo, companyID, &categoryID, "Juice", 2.5)

	tests := []struct {
		name           string
		productID      string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "success",
			productID:      productID.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, productID, resp.ID)
				assert.Equal(t, "Juice", resp.Name)
				assert.Equal(t, 2.5, resp.Price)
				assert.Equal(t, companyID, resp.CompanyID)
				require.NotNil(t, resp.Category)
				assert.Equal(t, categoryID, resp.Category.ID)
			},
		},
		{
			name:           "not found - non-existent UUID",
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

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%s", tt.productID), nil)
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
// LIST PRODUCTS TESTS
// ============================================

func TestProductHandler_ListProducts_Integration(t *testing.T) {
	handler, repo := setupProductHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	categoryFruits := seedCategory(t, repo, "Fruits")
	categoryDrinks := seedCategory(t, repo, "Drinks")

	// Seed a handful of products across companies and categories.
	seedProduct(t, repo, companyA, &categoryFruits, "Apple", 1.5)
	seedProduct(t, repo, companyA, &categoryFruits, "Pear", 2.0)
	seedProduct(t, repo, companyA, &categoryDrinks, "Juice", 3.0)
	seedProduct(t, repo, companyB, &categoryFruits, "Banana", 0.5)
	seedProduct(t, repo, companyB, nil, "Generic Item", 100.0)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "success - list all (default pagination)",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, 5, resp.Total)
				assert.Len(t, resp.Products, 5)
				assert.Equal(t, 1, resp.Page)
				assert.Equal(t, 20, resp.Limit)
			},
		},
		{
			name:           "success - filter by company_id",
			query:          fmt.Sprintf("company_id=%s", companyA),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, 3, resp.Total)
				for _, p := range resp.Products {
					assert.Equal(t, companyA, p.CompanyID)
				}
			},
		},
		{
			name:           "success - filter by category_id",
			query:          fmt.Sprintf("category_id=%s", categoryFruits),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, 3, resp.Total)
				for _, p := range resp.Products {
					require.NotNil(t, p.Category)
					assert.Equal(t, categoryFruits, p.Category.ID)
				}
			},
		},
		{
			name:           "success - pagination page=1 limit=2",
			query:          "page=1&limit=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, 5, resp.Total)
				assert.Len(t, resp.Products, 2)
				assert.Equal(t, 1, resp.Page)
				assert.Equal(t, 2, resp.Limit)
			},
		},
		{
			name:           "success - pagination page=2 limit=2",
			query:          "page=2&limit=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, 5, resp.Total)
				assert.Len(t, resp.Products, 2)
				assert.Equal(t, 2, resp.Page)
				assert.Equal(t, 2, resp.Limit)
			},
		},
		{
			name:           "success - filter by min_price and max_price",
			query:          "min_price=1.0&max_price=3.0",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				// Apple(1.5), Pear(2.0), Juice(3.0), Banana would be 0.5 (excluded)
				assert.Equal(t, 3, resp.Total)
				for _, p := range resp.Products {
					assert.GreaterOrEqual(t, p.Price, 1.0)
					assert.LessOrEqual(t, p.Price, 3.0)
				}
			},
		},
		{
			name:           "success - empty result with very high min_price",
			query:          "min_price=1000",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, 0, resp.Total)
				assert.Empty(t, resp.Products)
			},
		},
		{
			name:           "validation error - invalid category_id format",
			query:          "category_id=not-a-uuid",
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:           "validation error - invalid company_id format",
			query:          "company_id=not-a-uuid",
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:           "validation error - page less than 1",
			query:          "page=-1",
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:           "validation error - limit greater than 100",
			query:          "limit=200",
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
		{
			name:           "validation error - limit less than 1",
			query:          "limit=-1",
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			url := "/products"
			if tt.query != "" {
				url += "?" + tt.query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
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
// UPDATE PRODUCT TESTS
// ============================================

func TestProductHandler_UpdateProduct_Integration(t *testing.T) {
	handler, repo := setupProductHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()
	categoryFruits := seedCategory(t, repo, "Fruits")
	categoryDrinks := seedCategory(t, repo, "Drinks")
	productA := seedProduct(t, repo, companyA, &categoryFruits, "Apple", 1.5)

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

	newName := "Green Apple"
	newPrice := 2.0
	newDesc := "Updated description"

	tests := []struct {
		name           string
		user           auth.User
		productID      string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:      "success - owner updates name and price",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"name":  newName,
				"price": newPrice,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, productA, resp.ID)
				assert.Equal(t, newName, resp.Name)
				assert.Equal(t, newPrice, resp.Price)
				// description unchanged because we didn't pass it
				assert.Equal(t, "", resp.Description)
			},
		},
		{
			name:      "success - admin updates description",
			user:      admin,
			productID: productA.String(),
			body: map[string]interface{}{
				"description": newDesc,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, newDesc, resp.Description)
			},
		},
		{
			name:      "success - owner changes category",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"category_id": categoryDrinks.String(),
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				require.NotNil(t, resp.Category)
				assert.Equal(t, categoryDrinks, resp.Category.ID)
				assert.Equal(t, "Drinks", resp.Category.Name)
			},
		},
		{
			name:      "success - owner deactivates product",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"is_active": false,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.False(t, resp.IsActive)
			},
		},
		{
			name:      "forbidden - seller from different company",
			user:      sellerB,
			productID: productA.String(),
			body: map[string]interface{}{
				"name": "Hacked",
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
				"name": "Anonymous",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "not found",
			user:      sellerA,
			productID: uuid.NewString(),
			body: map[string]interface{}{
				"name": "Updated",
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "product not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:      "bad request - invalid UUID",
			user:      sellerA,
			productID: "not-a-uuid",
			body: map[string]interface{}{
				"name": "Updated",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "invalid product id", errResp["error"].(map[string]interface{})["message"])
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
		{
			name:      "validation error - name too long",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"name": strings.Repeat("x", 256),
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
				fields := resp["fields"].(map[string]interface{})
				assert.Equal(t, "too long", fields["name"])
			},
		},
		{
			name:      "validation error - negative price",
			user:      sellerA,
			productID: productA.String(),
			body: map[string]interface{}{
				"price": -1.0,
			},
			expectedStatus: http.StatusUnprocessableEntity,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errObj, _ := resp["error"].(map[string]interface{})
				assert.Equal(t, "validation failed", errObj["message"])
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

			req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/products/%s", tt.productID), bytes.NewReader(reqBody))
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
// DELETE PRODUCT TESTS
// ============================================

func TestProductHandler_DeleteProduct_Integration(t *testing.T) {
	handler, repo := setupProductHandler(t)

	companyA := uuid.New()
	companyB := uuid.New()

	sellerA := auth.User{ID: uuid.New(), CompanyID: companyA, Role: auth.RoleSeller}
	sellerB := auth.User{ID: uuid.New(), CompanyID: companyB, Role: auth.RoleSeller}
	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

	tests := []struct {
		name           string
		user           auth.User
		seedProduct    bool   // if true, seed a new product owned by companyA
		productID      string // override; if empty, use the seeded product id
		expectedStatus int
	}{
		{
			name:           "success - owner deletes own product",
			user:           sellerA,
			seedProduct:    true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "success - admin deletes any product",
			user:           admin,
			seedProduct:    true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "forbidden - seller from different company",
			user:           sellerB,
			seedProduct:    true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "forbidden - no user in context",
			user:           auth.User{},
			seedProduct:    true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not found",
			user:           sellerA,
			seedProduct:    false,
			productID:      uuid.NewString(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "bad request - invalid UUID",
			user:           sellerA,
			seedProduct:    false,
			productID:      "not-a-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			productID := tt.productID
			if tt.seedProduct {
				productID = seedProduct(t, repo, companyA, nil, "To Delete", 1.0).String()
			}

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/products/%s", productID), nil)
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			// For successful deletes, verify the product is actually gone.
			if tt.expectedStatus == http.StatusNoContent {
				_, err := repo.GetProductByID(context.Background(), uuid.MustParse(productID))
				require.Error(t, err, "product should be deleted from DB")
			}
		})
	}
}

// ============================================
// GET PRODUCT AVAILABILITY TESTS
// ============================================

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

func TestProductHandler_GetProductAvailability_Integration(t *testing.T) {
	handler, repo := setupProductHandler(t)

	companyA := uuid.New()
	productWithBatches := seedProduct(t, repo, companyA, nil, "Stocked Item", 5.0)
	productWithoutBatches := seedProduct(t, repo, companyA, nil, "Empty Item", 5.0)

	// Seed two batches for the first product. The availability endpoint
	// should return only batches with available_quantity > 0, ordered by
	// expiration_date ASC (FIFO).
	seedBatch(t, repo, productWithBatches, 100, 0, "2026-12-31")
	seedBatch(t, repo, productWithBatches, 50, 0, "2026-06-30") // earlier expiry

	tests := []struct {
		name           string
		productID      string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "success - product with batches (FIFO order)",
			productID:      productWithBatches.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductAvailabilityResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, productWithBatches, resp.ProductID)
				require.Len(t, resp.Batches, 2)

				// FIFO: earlier expiration_date comes first.
				require.NotNil(t, resp.Batches[0].ExpirationDate)
				require.NotNil(t, resp.Batches[1].ExpirationDate)
				assert.Equal(t, "2026-06-30", *resp.Batches[0].ExpirationDate)
				assert.Equal(t, "2026-12-31", *resp.Batches[1].ExpirationDate)
				assert.Equal(t, float64(50), resp.Batches[0].AvailableQuantity)
				assert.Equal(t, float64(100), resp.Batches[1].AvailableQuantity)
			},
		},
		{
			name:           "success - product without batches",
			productID:      productWithoutBatches.String(),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp producthandler.ProductAvailabilityResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, productWithoutBatches, resp.ProductID)
				assert.Empty(t, resp.Batches)
			},
		},
		{
			name:           "not found - non-existent product",
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

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/products/%s/availability", tt.productID), nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec.Body.Bytes())
			}
		})
	}
}
