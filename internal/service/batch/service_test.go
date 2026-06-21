package batch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
	"github.com/leyl1ne/ProductService/internal/service"
	batchservice "github.com/leyl1ne/ProductService/internal/service/batch"
	"github.com/leyl1ne/ProductService/internal/service/batch/mocks"
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

// --- CreateBatch ---

func Test_Service_CreateBatch(t *testing.T) {
	productID := uuid.New()
	now := time.Now()
	companyID := testSellerUser.CompanyID

	type createInput struct {
		warehouseCompanyID *uuid.UUID
		quantityTotal      float64
		productionDate     *time.Time
		expirationDate     *time.Time
		batchNumber        string
	}

	type mockSetup struct {
		user           auth.User
		userInCtx      bool
		product        *productmodel.Product
		getProductErr  error
		createdBatch   *batchmodel.ProductBatch
		createBatchErr error
	}

	cases := []struct {
		name      string
		productID uuid.UUID
		input     createInput
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:      "success — seller owner",
			productID: productID,
			input: createInput{
				quantityTotal: 100.5,
				batchNumber:   "BATCH-001",
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
					Name:      "Apple",
				},
				createdBatch: &batchmodel.ProductBatch{
					ID:               uuid.New(),
					ProductID:        productID,
					QuantityTotal:    100.5,
					QuantityReserved: 0,
					BatchNumber:      "BATCH-001",
					CreatedAt:        now,
				},
			},
			expectErr: nil,
		},
		{
			name:      "success — admin from different company",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: uuid.New(), // different company
					Name:      "Apple",
				},
				createdBatch: &batchmodel.ProductBatch{
					ID:               uuid.New(),
					ProductID:        productID,
					QuantityTotal:    50.0,
					QuantityReserved: 0,
					CreatedAt:        now,
				},
			},
			expectErr: nil,
		},
		{
			name:      "forbidden — buyer",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				user:      testBuyerUser,
				userInCtx: true,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:      "forbidden — no user in context",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:      "product not found",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name:      "forbidden — seller from different company",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: uuid.New(), // different company
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:      "get product unexpected error",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:      "create batch unexpected error",
			productID: productID,
			input: createInput{
				quantityTotal: 50.0,
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				createBatchErr: errors.New("db write failed"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx && auth.CanCreateBatch(tc.mockSetup.user) {
				mockRepo.On("GetProductByID", mock.Anything, tc.productID).
					Return(tc.mockSetup.product, tc.mockSetup.getProductErr).
					Once()

				if tc.mockSetup.getProductErr == nil &&
					auth.CanManageCompanyResource(tc.mockSetup.user, tc.mockSetup.product.CompanyID) {
					mockRepo.On("CreateBatch", mock.Anything, mock.AnythingOfType("batch.ProductBatch")).
						Return(tc.mockSetup.createdBatch, tc.mockSetup.createBatchErr).
						Once()
				}
			}

			svc := batchservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.CreateBatch(ctx, tc.productID, batchservice.CreateBatchInput{
				WarehouseCompanyID: tc.input.warehouseCompanyID,
				QuantityTotal:      tc.input.quantityTotal,
				ProductionDate:     tc.input.productionDate,
				ExpirationDate:     tc.input.expirationDate,
				BatchNumber:        tc.input.batchNumber,
			})

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getProductErr != nil || tc.mockSetup.createBatchErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.createdBatch.ID, result.ID)
				require.Equal(t, tc.mockSetup.createdBatch.QuantityTotal, result.QuantityTotal)
			}
		})
	}
}

// --- GetBatchByID ---

func Test_Service_GetBatchByID(t *testing.T) {
	batchID := uuid.New()
	productID := uuid.New()
	now := time.Now()
	companyID := testSellerUser.CompanyID
	otherCompanyID := uuid.New()

	type mockSetup struct {
		user          auth.User
		userInCtx     bool
		batch         *batchmodel.ProductBatch
		getBatchErr   error
		product       *productmodel.Product
		getProductErr error
	}

	cases := []struct {
		name      string
		batchID   uuid.UUID
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:    "success — owner",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:               batchID,
					ProductID:        productID,
					QuantityTotal:    100.0,
					QuantityReserved: 20.0,
					CreatedAt:        now,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
			},
			expectErr: nil,
		},
		{
			name:    "success — admin from different company",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: nil,
		},
		{
			name:    "forbidden — seller from different company",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:    "forbidden — no user in context",
			batchID: batchID,
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:    "batch not found",
			batchID: batchID,
			mockSetup: mockSetup{
				user:        testSellerUser,
				userInCtx:   true,
				getBatchErr: batchmodel.ErrBatchNotFound,
			},
			expectErr: service.ErrBatchNotFound,
		},
		{
			name:    "get batch unexpected error",
			batchID: batchID,
			mockSetup: mockSetup{
				user:        testSellerUser,
				userInCtx:   true,
				getBatchErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:    "product not found during ownership check",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name:    "get product unexpected error during ownership check",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx {
				mockRepo.On("GetBatchByID", mock.Anything, tc.batchID).
					Return(tc.mockSetup.batch, tc.mockSetup.getBatchErr).
					Once()

				if tc.mockSetup.batch != nil && tc.mockSetup.getBatchErr == nil {
					mockRepo.On("GetProductByID", mock.Anything, tc.mockSetup.batch.ProductID).
						Return(tc.mockSetup.product, tc.mockSetup.getProductErr).
						Once()
				}
			}

			svc := batchservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.GetBatchByID(ctx, tc.batchID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getBatchErr != nil || tc.mockSetup.getProductErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.batch.ID, result.ID)
				require.Equal(t, tc.mockSetup.batch.QuantityTotal-tc.mockSetup.batch.QuantityReserved, result.AvailableQuantity)
			}
		})
	}
}

// --- ListBatchesByProduct ---

func Test_Service_ListBatchesByProduct(t *testing.T) {
	productID := uuid.New()
	now := time.Now()
	companyID := testSellerUser.CompanyID
	otherCompanyID := uuid.New()

	type mockSetup struct {
		user           auth.User
		userInCtx      bool
		product        *productmodel.Product
		getProductErr  error
		batches        []batchmodel.ProductBatch
		listBatchesErr error
	}

	cases := []struct {
		name      string
		productID uuid.UUID
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:      "success — owner",
			productID: productID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				batches: []batchmodel.ProductBatch{
					{
						ID:               uuid.New(),
						ProductID:        productID,
						QuantityTotal:    100.0,
						QuantityReserved: 20.0,
						CreatedAt:        now,
					},
					{
						ID:               uuid.New(),
						ProductID:        productID,
						QuantityTotal:    50.0,
						QuantityReserved: 0.0,
						CreatedAt:        now,
					},
				},
			},
			expectErr: nil,
		},
		{
			name:      "success — empty list",
			productID: productID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				batches: []batchmodel.ProductBatch{},
			},
			expectErr: nil,
		},
		{
			name:      "forbidden — seller from different company",
			productID: productID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:      "forbidden — no user in context",
			productID: productID,
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:      "product not found",
			productID: productID,
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: productmodel.ErrProductNotFound,
			},
			expectErr: service.ErrProductNotFound,
		},
		{
			name:      "get product unexpected error",
			productID: productID,
			mockSetup: mockSetup{
				user:          testSellerUser,
				userInCtx:     true,
				getProductErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:      "list batches unexpected error",
			productID: productID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				listBatchesErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx {
				mockRepo.On("GetProductByID", mock.Anything, tc.productID).
					Return(tc.mockSetup.product, tc.mockSetup.getProductErr).
					Once()

				if tc.mockSetup.getProductErr == nil &&
					auth.CanManageCompanyResource(tc.mockSetup.user, tc.mockSetup.product.CompanyID) {
					mockRepo.On("ListBatchesByProduct", mock.Anything, tc.productID).
						Return(tc.mockSetup.batches, tc.mockSetup.listBatchesErr).
						Once()
				}
			}

			svc := batchservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.ListBatchesByProduct(ctx, tc.productID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getProductErr != nil || tc.mockSetup.listBatchesErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, result, len(tc.mockSetup.batches))
			}
		})
	}
}

// --- UpdateBatch ---

func Test_Service_UpdateBatch(t *testing.T) {
	batchID := uuid.New()
	productID := uuid.New()
	now := time.Now()
	companyID := testSellerUser.CompanyID
	otherCompanyID := uuid.New()

	type mockSetup struct {
		user           auth.User
		userInCtx      bool
		batch          *batchmodel.ProductBatch
		getBatchErr    error
		product        *productmodel.Product
		getProductErr  error
		updatedBatch   *batchmodel.ProductBatch
		updateBatchErr error
	}

	cases := []struct {
		name      string
		batchID   uuid.UUID
		input     batchservice.UpdateBatchInput
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:    "success — owner updates batch number",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("UPDATED-001"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				updatedBatch: &batchmodel.ProductBatch{
					ID:          batchID,
					ProductID:   productID,
					BatchNumber: "UPDATED-001",
					CreatedAt:   now,
				},
			},
			expectErr: nil,
		},
		{
			name:    "success — admin from different company",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("ADMIN-UPDATE"),
			},
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
				updatedBatch: &batchmodel.ProductBatch{
					ID:          batchID,
					ProductID:   productID,
					BatchNumber: "ADMIN-UPDATE",
				},
			},
			expectErr: nil,
		},
		{
			name:    "forbidden — seller from different company",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("HACKED"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:    "forbidden — no user in context",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("HACKED"),
			},
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:    "batch not found",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("X"),
			},
			mockSetup: mockSetup{
				user:        testSellerUser,
				userInCtx:   true,
				getBatchErr: batchmodel.ErrBatchNotFound,
			},
			expectErr: service.ErrBatchNotFound,
		},
		{
			name:    "get batch unexpected error",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("X"),
			},
			mockSetup: mockSetup{
				user:        testSellerUser,
				userInCtx:   true,
				getBatchErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:    "update batch fails — not found (race condition)",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("X"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				updateBatchErr: batchmodel.ErrBatchNotFound,
			},
			expectErr: service.ErrBatchNotFound,
		},
		{
			name:    "update batch unexpected error",
			batchID: batchID,
			input: batchservice.UpdateBatchInput{
				BatchNumber: strPtr("X"),
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				updateBatchErr: errors.New("db write failed"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx {
				mockRepo.On("GetBatchByID", mock.Anything, tc.batchID).
					Return(tc.mockSetup.batch, tc.mockSetup.getBatchErr).
					Once()

				if tc.mockSetup.batch != nil && tc.mockSetup.getBatchErr == nil {
					mockRepo.On("GetProductByID", mock.Anything, tc.mockSetup.batch.ProductID).
						Return(tc.mockSetup.product, tc.mockSetup.getProductErr).
						Once()

					if tc.mockSetup.getProductErr == nil &&
						auth.CanManageCompanyResource(tc.mockSetup.user, tc.mockSetup.product.CompanyID) {
						mockRepo.On("UpdateBatch", mock.Anything, tc.batchID, mock.AnythingOfType("UpdateBatchParams")).
							Return(tc.mockSetup.updatedBatch, tc.mockSetup.updateBatchErr).
							Once()
					}
				}
			}

			svc := batchservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.UpdateBatch(ctx, tc.batchID, tc.input)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.getBatchErr != nil || tc.mockSetup.getProductErr != nil || tc.mockSetup.updateBatchErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.updatedBatch.ID, result.ID)
			}
		})
	}
}

// --- DeleteBatch ---

func Test_Service_DeleteBatch(t *testing.T) {
	batchID := uuid.New()
	productID := uuid.New()
	companyID := testSellerUser.CompanyID
	otherCompanyID := uuid.New()

	type mockSetup struct {
		user           auth.User
		userInCtx      bool
		batch          *batchmodel.ProductBatch
		getBatchErr    error
		product        *productmodel.Product
		getProductErr  error
		deleteBatchErr error
	}

	cases := []struct {
		name      string
		batchID   uuid.UUID
		mockSetup mockSetup
		expectErr error
	}{
		{
			name:    "success — owner",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
			},
			expectErr: nil,
		},
		{
			name:    "success — admin from different company",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: nil,
		},
		{
			name:    "forbidden — seller from different company",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: otherCompanyID,
				},
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:    "forbidden — no user in context",
			batchID: batchID,
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:    "batch not found",
			batchID: batchID,
			mockSetup: mockSetup{
				user:        testSellerUser,
				userInCtx:   true,
				getBatchErr: batchmodel.ErrBatchNotFound,
			},
			expectErr: service.ErrBatchNotFound,
		},
		{
			name:    "get batch unexpected error",
			batchID: batchID,
			mockSetup: mockSetup{
				user:        testSellerUser,
				userInCtx:   true,
				getBatchErr: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
		{
			name:    "delete batch fails — not found (race condition)",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				deleteBatchErr: batchmodel.ErrBatchNotFound,
			},
			expectErr: service.ErrBatchNotFound,
		},
		{
			name:    "delete batch unexpected error",
			batchID: batchID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
				batch: &batchmodel.ProductBatch{
					ID:        batchID,
					ProductID: productID,
				},
				product: &productmodel.Product{
					ID:        productID,
					CompanyID: companyID,
				},
				deleteBatchErr: errors.New("db write failed"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx {
				mockRepo.On("GetBatchByID", mock.Anything, tc.batchID).
					Return(tc.mockSetup.batch, tc.mockSetup.getBatchErr).
					Once()

				if tc.mockSetup.batch != nil && tc.mockSetup.getBatchErr == nil {
					mockRepo.On("GetProductByID", mock.Anything, tc.mockSetup.batch.ProductID).
						Return(tc.mockSetup.product, tc.mockSetup.getProductErr).
						Once()

					if tc.mockSetup.getProductErr == nil &&
						auth.CanManageCompanyResource(tc.mockSetup.user, tc.mockSetup.product.CompanyID) {
						mockRepo.On("DeleteBatch", mock.Anything, tc.batchID).
							Return(tc.mockSetup.deleteBatchErr).
							Once()
					}
				}
			}

			svc := batchservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			err := svc.DeleteBatch(ctx, tc.batchID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
			} else if tc.mockSetup.getBatchErr != nil || tc.mockSetup.getProductErr != nil || tc.mockSetup.deleteBatchErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- helpers ---

func strPtr(s string) *string {
	return &s
}
