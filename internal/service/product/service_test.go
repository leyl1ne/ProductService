package product_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
	"github.com/leyl1ne/ProductService/internal/service"
	productservice "github.com/leyl1ne/ProductService/internal/service/product"
	"github.com/leyl1ne/ProductService/internal/service/product/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testAdminUser = auth.User{
		ID:        uuid.New(),
		CompanyID: uuid.New(),
		Role:      auth.RoleAdmin,
	}
	testSellerUser = auth.User{
		ID:        uuid.New(),
		CompanyID: uuid.New(),
		Role:      auth.RoleSeller,
	}
	testBuyerUser = auth.User{
		ID:        uuid.New(),
		CompanyID: uuid.New(),
		Role:      auth.RoleBuyer,
	}
	testNoUserCtx = context.Background()
)

func ctxWithUser(u auth.User) context.Context {
	return auth.ContextWithUser(context.Background(), u)
}

// --- CreateProduct ---

func Test_Service_CreateProduct(t *testing.T) {
	categoryID := uuid.New()
	now := time.Now()

	type createInput struct {
		categoryID *uuid.UUID
		name       string
		desc       string
		price      float64
		unit       string
	}

	type mockSetup struct {
		user             auth.User
		userInCtx        bool
		createProduct    *productmodel.Product
		createProductErr error
		getCategory      *categorymodel.Category
		getCategoryErr   error
	}

	cases := []struct {
		name      string
		input     createInput
		mockSetup mockSetup
		expectErr error
	}{
		{
			name: "success — seller",
			input: createInput{
				categoryID: &categoryID,
				name:       "Apple",
				desc:       "Fresh apple",
				price:      10.5,
				unit:       "kg",
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				createProduct: &productmodel.Product{
					ID:          uuid.New(),
					CompanyID:   testSellerUser.CompanyID,
					CategoryID:  &categoryID,
					Name:        "Apple",
					Description: "Fresh apple",
					Price:       10.5,
					Unit:        "kg",
					IsActive:    true,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
				getCategory: &categorymodel.Category{
					ID:   categoryID,
					Name: "Fruits",
				},
			},
			expectErr: nil,
		},
		{
			name: "success — admin without category",
			input: createInput{
				categoryID: nil,
				name:       "Apple",
				desc:       "Fresh apple",
				price:      10.5,
				unit:       "kg",
			},
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				createProduct: &productmodel.Product{
					ID:          uuid.New(),
					CompanyID:   testAdminUser.CompanyID,
					CategoryID:  nil,
					Name:        "Apple",
					Description: "Fresh apple",
					Price:       10.5,
					Unit:        "kg",
					IsActive:    true,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			},
			expectErr: nil,
		},
		{
			name: "forbidden — buyer",
			input: createInput{
				name:  "Apple",
				price: 10.5,
				unit:  "kg",
			},
			mockSetup: mockSetup{
				user:      testBuyerUser,
				userInCtx: true,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "forbidden — no user in context",
			input: createInput{
				name:  "Apple",
				price: 10.5,
				unit:  "kg",
			},
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "category not found",
			input: createInput{
				categoryID: &categoryID,
				name:       "Apple",
				price:      10.5,
				unit:       "kg",
			},
			mockSetup: mockSetup{
				user:             testSellerUser,
				userInCtx:        true,
				createProductErr: categorymodel.ErrCategoryNotFound,
			},
			expectErr: service.ErrCategoryNotFound,
		},
		{
			name: "create product unexpected error",
			input: createInput{
				name:  "Apple",
				price: 10.5,
				unit:  "kg",
			},
			mockSetup: mockSetup{
				user:             testSellerUser,
				userInCtx:        true,
				createProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx && auth.CanCreateProduct(tc.mockSetup.user) {
				mockRepo.On("CreateProduct", mock.Anything, mock.AnythingOfType("product.Product")).
					Return(tc.mockSetup.createProduct, tc.mockSetup.createProductErr).
					Once()

				if tc.mockSetup.createProduct != nil && tc.mockSetup.createProduct.CategoryID != nil && tc.mockSetup.createProductErr == nil {
					if tc.mockSetup.getCategory != nil {
						mockRepo.On("GetCategoryByID", mock.Anything, *tc.mockSetup.createProduct.CategoryID).
							Return(tc.mockSetup.getCategory, tc.mockSetup.getCategoryErr).
							Once()
					}
				}
			}

			svc := productservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.CreateProduct(ctx, productservice.CreateProductInput{
				CategoryID: tc.input.categoryID,
				Name:       tc.input.name,
				Desc:       tc.input.desc,
				Price:      tc.input.price,
				Unit:       tc.input.unit,
			})

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.createProductErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.createProduct.ID, result.ID)
				require.Equal(t, tc.mockSetup.createProduct.Name, result.Name)
			}
		})
	}
}

// --- GetProductByID ---

func Test_Service_GetProductByID(t *testing.T) {
	productID := uuid.New()
	categoryID := uuid.New()
	now := time.Now()

	type mockSetup struct {
		getProduct     *productmodel.Product
		getProductErr  error
		getCategory    *categorymodel.Category
		getCategoryErr error
	}

	cases := []struct {
		name      string
		productID uuid.UUID
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:      "success — with category",
			productID: productID,
			mockSetup: mockSetup{
				getProduct: &productmodel.Product{
					ID:         productID,
					CompanyID:  uuid.New(),
					CategoryID: &categoryID,
					Name:       "Apple",
					Price:      10.5,
					Unit:       "kg",
					IsActive:   true,
					CreatedAt:  now,
					UpdatedAt:  now,
				},
				getCategory: &categorymodel.Category{
					ID:   categoryID,
					Name: "Fruits",
				},
			},
			expectErr: nil,
		},
		{
			name:      "success — without category",
			productID: productID,
			mockSetup: mockSetup{
				getProduct: &productmodel.Product{
					ID:         productID,
					CompanyID:  uuid.New(),
					CategoryID: nil,
					Name:       "Apple",
					Price:      10.5,
					Unit:       "kg",
					IsActive:   true,
					CreatedAt:  now,
					UpdatedAt:  now,
				},
			},
			expectErr: nil,
		},
		{
			name:      "product not found",
			productID: productID,
			mockSetup: mockSetup{
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name:      "get product unexpected error",
			productID: productID,
			mockSetup: mockSetup{
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:      "category fetch fails — still success (category enriched only on success)",
			productID: productID,
			mockSetup: mockSetup{
				getProduct: &productmodel.Product{
					ID:         productID,
					CompanyID:  uuid.New(),
					CategoryID: &categoryID,
					Name:       "Apple",
					IsActive:   true,
				},
				getCategoryErr: errors.New("category fetch failed"),
			},
			expectErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			mockRepo.On("GetProductByID", mock.Anything, tc.productID).
				Return(tc.mockSetup.getProduct, tc.mockSetup.getProductErr).
				Once()

			if tc.mockSetup.getProduct != nil &&
				tc.mockSetup.getProductErr == nil &&
				tc.mockSetup.getProduct.CategoryID != nil {
				mockRepo.On("GetCategoryByID", mock.Anything, *tc.mockSetup.getProduct.CategoryID).
					Return(tc.mockSetup.getCategory, tc.mockSetup.getCategoryErr).
					Once()
			}

			svc := productservice.NewService(mockRepo)

			result, err := svc.GetProductByID(context.Background(), tc.productID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getProductErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.getProduct.ID, result.ID)
			}
		})
	}
}

// --- ListProducts ---

func Test_Service_ListProducts(t *testing.T) {
	categoryID := uuid.New()
	now := time.Now()

	type mockSetup struct {
		listProducts      productservice.ProductListResult
		listProductsErr   error
		listCategories    []categorymodel.Category
		listCategoriesErr error
	}

	cases := []struct {
		name      string
		filter    productservice.ProductFilter
		page      int
		limit     int
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:   "success — with categories enrichment",
			filter: productservice.ProductFilter{},
			page:   1,
			limit:  20,
			mockSetup: mockSetup{
				listProducts: productservice.ProductListResult{
					Products: []productmodel.Product{
						{
							ID:         uuid.New(),
							CompanyID:  uuid.New(),
							CategoryID: &categoryID,
							Name:       "Apple",
							IsActive:   true,
							CreatedAt:  now,
						},
						{
							ID:         uuid.New(),
							CompanyID:  uuid.New(),
							CategoryID: nil,
							Name:       "Banana",
							IsActive:   true,
							CreatedAt:  now,
						},
					},
					Total: 2,
				},
				listCategories: []categorymodel.Category{
					{ID: categoryID, Name: "Fruits"},
					{ID: uuid.New(), Name: "Vegetables"},
				},
			},
			expectErr: nil,
		},
		{
			name:   "success — empty list",
			filter: productservice.ProductFilter{},
			page:   1,
			limit:  20,
			mockSetup: mockSetup{
				listProducts: productservice.ProductListResult{
					Products: []productmodel.Product{},
					Total:    0,
				},
				listCategories: []categorymodel.Category{},
			},
			expectErr: nil,
		},
		{
			name:   "list products fails",
			filter: productservice.ProductFilter{},
			page:   1,
			limit:  20,
			mockSetup: mockSetup{
				listProductsErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:   "list categories fails",
			filter: productservice.ProductFilter{},
			page:   1,
			limit:  20,
			mockSetup: mockSetup{
				listProducts: productservice.ProductListResult{
					Products: []productmodel.Product{},
					Total:    0,
				},
				listCategoriesErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			mockRepo.On("ListProducts", mock.Anything, tc.filter, tc.page, tc.limit).
				Return(tc.mockSetup.listProducts, tc.mockSetup.listProductsErr).
				Once()

			if tc.mockSetup.listProductsErr == nil {
				mockRepo.On("ListCategories", mock.Anything).
					Return(tc.mockSetup.listCategories, tc.mockSetup.listCategoriesErr).
					Once()
			}

			svc := productservice.NewService(mockRepo)

			result, err := svc.ListProducts(context.Background(), tc.filter, tc.page, tc.limit)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.listProductsErr != nil || tc.mockSetup.listCategoriesErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, result.Products, len(tc.mockSetup.listProducts.Products))
				require.Equal(t, tc.mockSetup.listProducts.Total, result.Total)
				// Verify category enrichment for first product if it has CategoryID
				if len(result.Products) > 0 && result.Products[0].CategoryID != nil {
					require.NotNil(t, result.Products[0].Category)
					require.Equal(t, "Fruits", result.Products[0].Category.Name)
				}
			}
		})
	}
}

// --- UpdateProduct ---

func Test_Service_UpdateProduct(t *testing.T) {
	productID := uuid.New()
	companyID := testSellerUser.CompanyID
	categoryID := uuid.New()
	otherCompanyID := uuid.New()
	now := time.Now()

	type updateInput struct {
		name        *string
		description *string
		price       *float64
		unit        *string
		categoryID  *uuid.UUID
		isActive    *bool
	}

	type mockSetup struct {
		user             auth.User
		userInCtx        bool
		existingProduct  *productmodel.Product
		getProductErr    error
		updatedProduct   *productmodel.Product
		updateProductErr error
		getCategory      *categorymodel.Category
		getCategoryErr   error
	}

	cases := []struct {
		name      string
		input     updateInput
		mockSetup mockSetup
		expectErr error
	}{
		{
			name: "success — owner updates name",
			input: updateInput{
				name: strPtr("Updated Apple"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
					Name:      "Apple",
					IsActive:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
				updatedProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
					Name:      "Updated Apple",
					IsActive:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
			expectErr: nil,
		},
		{
			name: "success — admin updates from different company",
			input: updateInput{
				name: strPtr("Admin Updated"),
			},
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
					Name:      "Apple",
					IsActive:  true,
				},
				updatedProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
					Name:      "Admin Updated",
					IsActive:  true,
				},
			},
			expectErr: nil,
		},
		{
			name: "forbidden — seller from different company",
			input: updateInput{
				name: strPtr("Hacked"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
					Name:      "Apple",
					IsActive:  true,
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "forbidden — no user in context",
			input: updateInput{
				name: strPtr("Hacked"),
			},
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "product not found",
			input: updateInput{
				name: strPtr("Updated"),
			},
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name: "get product unexpected error",
			input: updateInput{
				name: strPtr("Updated"),
			},
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name: "update product fails — product not found (race condition)",
			input: updateInput{
				name: strPtr("Updated"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
					Name:      "Apple",
				},
				updateProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name: "update product fails — category not found",
			input: updateInput{
				categoryID: &categoryID,
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				updateProductErr: categorymodel.ErrCategoryNotFound,
			},
			expectErr: service.ErrCategoryNotFound,
		},
		{
			name: "update product unexpected error",
			input: updateInput{
				name: strPtr("Updated"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				updateProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx {
				mockRepo.On("GetProductByID", mock.Anything, productID).
					Return(tc.mockSetup.existingProduct, tc.mockSetup.getProductErr).
					Once()
			}

			// Update is called only if ownership check passes
			if tc.mockSetup.userInCtx && tc.mockSetup.getProductErr == nil &&
				auth.CanManageCompanyResource(tc.mockSetup.user, tc.mockSetup.existingProduct.CompanyID) {
				mockRepo.On("UpdateProduct", mock.Anything, productID, mock.AnythingOfType("UpdateProductParams")).
					Return(tc.mockSetup.updatedProduct, tc.mockSetup.updateProductErr).
					Once()

				if tc.mockSetup.updatedProduct != nil &&
					tc.mockSetup.updatedProduct.CategoryID != nil &&
					tc.mockSetup.updateProductErr == nil {
					mockRepo.On("GetCategoryByID", mock.Anything, *tc.mockSetup.updatedProduct.CategoryID).
						Return(tc.mockSetup.getCategory, tc.mockSetup.getCategoryErr).
						Once()
				}
			}

			svc := productservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.UpdateProduct(ctx, productID, productservice.UpdateProductInput{
				Name:        tc.input.name,
				Description: tc.input.description,
				Price:       tc.input.price,
				Unit:        tc.input.unit,
				CategoryID:  tc.input.categoryID,
				IsActive:    tc.input.isActive,
			})

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getProductErr != nil || tc.mockSetup.updateProductErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.updatedProduct.ID, result.ID)
			}
		})
	}
}

// --- DeleteProduct ---

func Test_Service_DeleteProduct(t *testing.T) {
	productID := uuid.New()
	companyID := testSellerUser.CompanyID
	otherCompanyID := uuid.New()

	type mockSetup struct {
		user             auth.User
		userInCtx        bool
		existingProduct  *productmodel.Product
		getProductErr    error
		deleteProductErr error
	}

	cases := []struct {
		name      string
		mockSetup mockSetup
		expectErr error
	}{
		{
			name: "success — owner",
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
			},
			expectErr: nil,
		},
		{
			name: "success — admin from different company",
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: nil,
		},
		{
			name: "forbidden — seller from different company",
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "forbidden — no user in context",
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "product not found",
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name: "get product unexpected error",
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name: "delete product fails — not found",
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				deleteProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name: "delete product unexpected error",
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				existingProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				deleteProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx {
				mockRepo.On("GetProductByID", mock.Anything, productID).
					Return(tc.mockSetup.existingProduct, tc.mockSetup.getProductErr).
					Once()
			}

			if tc.mockSetup.userInCtx && tc.mockSetup.getProductErr == nil &&
				auth.CanManageCompanyResource(tc.mockSetup.user, tc.mockSetup.existingProduct.CompanyID) {
				mockRepo.On("DeleteProduct", mock.Anything, productID).
					Return(tc.mockSetup.deleteProductErr).
					Once()
			}

			svc := productservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			err := svc.DeleteProduct(ctx, productID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
			} else if tc.mockSetup.getProductErr != nil || tc.mockSetup.deleteProductErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- GetProductAvailability ---

func Test_Service_GetProductAvailability(t *testing.T) {
	productID := uuid.New()
	batchID := uuid.New()
	expDate := time.Now().Add(48 * time.Hour)

	type mockSetup struct {
		getProduct       *productmodel.Product
		getProductErr    error
		availableBatches []productservice.AvailableBatch
		availabilityErr  error
	}

	cases := []struct {
		name      string
		productID uuid.UUID
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:      "success — with available batches",
			productID: productID,
			mockSetup: mockSetup{
				getProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: uuid.New(),
					Name:      "Apple",
				},
				availableBatches: []productservice.AvailableBatch{
					{
						BatchID:           batchID,
						AvailableQuantity: 50.5,
						ExpirationDate:    &expDate,
					},
				},
			},
			expectErr: nil,
		},
		{
			name:      "success — empty batches",
			productID: productID,
			mockSetup: mockSetup{
				getProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: uuid.New(),
				},
				availableBatches: []productservice.AvailableBatch{},
			},
			expectErr: nil,
		},
		{
			name:      "product not found",
			productID: productID,
			mockSetup: mockSetup{
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name:      "get product unexpected error",
			productID: productID,
			mockSetup: mockSetup{
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:      "get availability unexpected error",
			productID: productID,
			mockSetup: mockSetup{
				getProduct: &productmodel.Product{
					ID:        productID,
					CompanyID: uuid.New(),
				},
				availabilityErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			mockRepo.On("GetProductByID", mock.Anything, tc.productID).
				Return(tc.mockSetup.getProduct, tc.mockSetup.getProductErr).
				Once()

			if tc.mockSetup.getProductErr == nil {
				mockRepo.On("GetProductAvailability", mock.Anything, tc.productID).
					Return(tc.mockSetup.availableBatches, tc.mockSetup.availabilityErr).
					Once()
			}

			svc := productservice.NewService(mockRepo)

			result, err := svc.GetProductAvailability(context.Background(), tc.productID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getProductErr != nil || tc.mockSetup.availabilityErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.productID, result.ProductID)
				require.Len(t, result.Batches, len(tc.mockSetup.availableBatches))
			}
		})
	}
}

// --- helpers ---

func strPtr(s string) *string {
	return &s
}
