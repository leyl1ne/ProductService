package category_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	postgresrepo "github.com/leyl1ne/ProductService/internal/repository/postgres"
	categoryservice "github.com/leyl1ne/ProductService/internal/service/category"
	"github.com/leyl1ne/ProductService/internal/testutils"
	categoryhandler "github.com/leyl1ne/ProductService/internal/transport/http/handler/category"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCategoryHandler builds a real database-backed handler stack for
// integration tests. It returns:
//   - *categoryhandler.Handler — the handler under test
//   - *postgresrepo.Repository   — the underlying repository (used by helpers
//     that pre-seed categories / products before the test request)
func setupCategoryHandler(t *testing.T) (*categoryhandler.Handler, *postgresrepo.Repository) {
	t.Helper()

	pool := testutils.SetupPostgres(t)
	repo := postgresrepo.NewPostgresRepository(pool.Pool())
	categorySvc := categoryservice.NewService(repo)
	log := testutils.NewDiscardLogger(t)

	handler := categoryhandler.NewHandler(log, categorySvc)
	return handler, repo
}

// setupRouter wires the category handler onto a fresh gin engine with the
// test auth middleware already installed.
func setupRouter(h *categoryhandler.Handler) *gin.Engine {
	r := testutils.SetupGin()
	r.Use(testutils.AuthMiddleware())

	r.GET("/categories", h.ListCategories())
	r.POST("/categories", h.CreateCategory())
	r.PATCH("/categories/:id", h.UpdateCategory())
	r.DELETE("/categories/:id", h.DeleteCategory())
	return r
}

// seedCategory inserts a category directly via the repository and returns
// the inserted row's ID. Used by tests that need an existing category to
// fetch / update / delete.
func seedCategory(
	t *testing.T,
	repo *postgresrepo.Repository,
	name string,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	c := categorymodel.Category{
		ID:   uuid.New(),
		Name: name,
	}
	created, err := repo.CreateCategory(ctx, c)
	require.NoError(t, err)
	return created.ID
}

// ============================================
// LIST CATEGORIES TESTS
// ============================================

func TestCategoryHandler_ListCategories_Integration(t *testing.T) {
	handler, repo := setupCategoryHandler(t)

	// Seed a few categories
	seedCategory(t, repo, "Fruits")
	seedCategory(t, repo, "Vegetables")
	seedCategory(t, repo, "Drinks")

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "success - returns all categories (no auth required)",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp categoryhandler.CategoryListResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Len(t, resp.Categories, 3)
				// Verify all categories are returned
				names := make(map[string]bool, len(resp.Categories))
				for _, c := range resp.Categories {
					names[c.Name] = true
				}
				assert.True(t, names["Fruits"])
				assert.True(t, names["Vegetables"])
				assert.True(t, names["Drinks"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			req := httptest.NewRequest(http.MethodGet, "/categories", nil)
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
// CREATE CATEGORY TESTS
// ============================================

func TestCategoryHandler_CreateCategory_Integration(t *testing.T) {
	handler, repo := setupCategoryHandler(t)

	// Pre-existing category used to test the "duplicate name" case.
	existingCategoryID := seedCategory(t, repo, "Existing")

	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}
	seller := auth.User{ID: uuid.New(), CompanyID: uuid.New(), Role: auth.RoleSeller}
	buyer := auth.User{ID: uuid.New(), CompanyID: uuid.New(), Role: auth.RoleBuyer}

	tests := []struct {
		name           string
		user           auth.User
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name: "success - ADMIN creates category",
			user: admin,
			body: map[string]interface{}{
				"name": "Fruits",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body []byte) {
				var resp categoryhandler.CategoryResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.NotEqual(t, uuid.Nil, resp.ID)
				assert.NotEqual(t, existingCategoryID, resp.ID)
				assert.Equal(t, "Fruits", resp.Name)
			},
		},
		{
			name: "forbidden - SELLER cannot create category",
			user: seller,
			body: map[string]interface{}{
				"name": "Vegetables",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name: "forbidden - BUYER cannot create category",
			user: buyer,
			body: map[string]interface{}{
				"name": "Drinks",
			},
			expectedStatus: http.StatusForbidden,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "forbidden", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name: "forbidden - no user in context",
			user: auth.User{},
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
			name: "conflict - duplicate name",
			user: admin,
			body: map[string]interface{}{
				"name": "Existing",
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "category already exists", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "validation error - missing name",
			user:           admin,
			body:           map[string]interface{}{},
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
			user: admin,
			body: map[string]interface{}{
				"name": "",
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
			name: "validation error - name too long",
			user: admin,
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
			name:           "validation error - empty body",
			user:           admin,
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
			user: admin,
			body: map[string]interface{}{
				"__malformed__": nil,
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

			req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewReader(reqBody))
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
// UPDATE CATEGORY TESTS
// ============================================

func TestCategoryHandler_UpdateCategory_Integration(t *testing.T) {
	handler, repo := setupCategoryHandler(t)

	// Pre-existing categories
	fruitsID := seedCategory(t, repo, "Fruits")
	// "Drinks" is seeded to trigger the duplicate-name conflict case below.
	_ = seedCategory(t, repo, "Drinks")

	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}
	seller := auth.User{ID: uuid.New(), CompanyID: uuid.New(), Role: auth.RoleSeller}

	tests := []struct {
		name           string
		user           auth.User
		categoryID     string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:       "success - ADMIN renames category",
			user:       admin,
			categoryID: fruitsID.String(),
			body: map[string]interface{}{
				"name": "Fresh Fruits",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp categoryhandler.CategoryResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, fruitsID, resp.ID)
				assert.Equal(t, "Fresh Fruits", resp.Name)
			},
		},
		{
			name:       "forbidden - SELLER cannot update category",
			user:       seller,
			categoryID: fruitsID.String(),
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
			name:       "forbidden - no user in context",
			user:       auth.User{},
			categoryID: fruitsID.String(),
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
			name:       "not found - non-existent UUID",
			user:       admin,
			categoryID: uuid.NewString(),
			body: map[string]interface{}{
				"name": "Updated",
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "category not found", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:       "conflict - rename to an existing name",
			user:       admin,
			categoryID: fruitsID.String(),
			body: map[string]interface{}{
				"name": "Drinks",
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "category already exists", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:       "bad request - invalid UUID",
			user:       admin,
			categoryID: "not-a-uuid",
			body: map[string]interface{}{
				"name": "Updated",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				var errResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &errResp))
				assert.Equal(t, "invalid category id", errResp["error"].(map[string]interface{})["message"])
			},
		},
		{
			name:           "validation error - missing name",
			user:           admin,
			categoryID:     fruitsID.String(),
			body:           map[string]interface{}{},
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
			name:       "validation error - empty name",
			user:       admin,
			categoryID: fruitsID.String(),
			body: map[string]interface{}{
				"name": "",
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
			name:       "validation error - name too long",
			user:       admin,
			categoryID: fruitsID.String(),
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
			name:           "validation error - empty body",
			user:           admin,
			categoryID:     fruitsID.String(),
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

			req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/categories/%s", tt.categoryID), bytes.NewReader(reqBody))
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
// DELETE CATEGORY TESTS
// ============================================

func TestCategoryHandler_DeleteCategory_Integration(t *testing.T) {
	handler, repo := setupCategoryHandler(t)

	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}
	seller := auth.User{ID: uuid.New(), CompanyID: uuid.New(), Role: auth.RoleSeller}

	tests := []struct {
		name           string
		user           auth.User
		seedCategory   bool   // if true, seed a fresh category
		categoryID     string // override; if empty use the seeded category id
		expectedStatus int
	}{
		{
			name:           "success - ADMIN deletes category",
			user:           admin,
			seedCategory:   true,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "forbidden - SELLER cannot delete category",
			user:           seller,
			seedCategory:   true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "forbidden - no user in context",
			user:           auth.User{},
			seedCategory:   true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not found - non-existent UUID",
			user:           admin,
			seedCategory:   false,
			categoryID:     uuid.NewString(),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "bad request - invalid UUID",
			user:           admin,
			seedCategory:   false,
			categoryID:     "not-a-uuid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRouter(handler)

			categoryID := tt.categoryID
			if tt.seedCategory {
				categoryID = seedCategory(t, repo, "To Delete "+uuid.NewString()[:8]).String()
			}

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/categories/%s", categoryID), nil)
			testutils.ApplyUserHeaders(req.Header, tt.user)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			// For successful deletes, verify the category is actually gone.
			if tt.expectedStatus == http.StatusNoContent {
				_, err := repo.GetCategoryByID(context.Background(), uuid.MustParse(categoryID))
				require.Error(t, err, "category should be deleted from DB")
			}
		})
	}
}

// ============================================
// DELETE CATEGORY IN USE TESTS
// ============================================

// TestCategoryHandler_DeleteCategory_InUse_Integration verifies that a
// category cannot be deleted when it is referenced by at least one product.
// The repository returns ErrCategoryInUse which the handler maps to 409.
//
// Note: this test requires a separate database setup because the foreign
// key constraint on products.category_id triggers the error.
// func TestCategoryHandler_DeleteCategory_InUse_Integration(t *testing.T) {
// 	handler, repo := setupCategoryHandler(t)

// 	admin := auth.User{ID: uuid.New(), CompanyID: uuid.Nil, Role: auth.RoleAdmin}

// 	// Seed a category and then a product that references it.
// 	categoryID := seedCategory(t, repo, "UsedCategory")
// 	seedProductWithCategory(t, repo, categoryID)

// 	r := setupRouter(handler)
// 	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/categories/%s", categoryID), nil)
// 	testutils.ApplyUserHeaders(req.Header, admin)

// 	rec := httptest.NewRecorder()
// 	r.ServeHTTP(rec, req)

// 	// Should return 409 Conflict
// 	assert.Equal(t, http.StatusConflict, rec.Code)

// 	var errResp map[string]interface{}
// 	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
// 	assert.Equal(t, "category is in use by products", errResp["error"].(map[string]interface{})["message"])
// }

// // seedProductWithCategory inserts a product referencing the given category.
// // Used by the "category in use" test to violate the foreign key constraint
// // on delete.
// func seedProductWithCategory(
// 	t *testing.T,
// 	repo *postgresrepo.Repository,
// 	categoryID uuid.UUID,
// ) {
// 	t.Helper()
// 	ctx := context.Background()

// 	// Insert a minimal product. We use raw SQL via the pool because the
// 	// product model requires a company_id and other fields.
// 	_, err := repo.CreateProduct(ctx, productModelForCategoryTest(uuid.New(), categoryID))
// 	require.NoError(t, err)
// }
