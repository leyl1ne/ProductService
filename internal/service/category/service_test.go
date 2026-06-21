package category_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	"github.com/leyl1ne/ProductService/internal/service"
	categoryservice "github.com/leyl1ne/ProductService/internal/service/category"
	"github.com/leyl1ne/ProductService/internal/service/category/mocks"
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
	testNoUserCtx = context.Background()
)

func ctxWithUser(u auth.User) context.Context {
	return auth.ContextWithUser(context.Background(), u)
}

// --- ListCategories ---

func Test_Service_ListCategories(t *testing.T) {
	type mockSetup struct {
		categories []categorymodel.Category
		err        error
	}

	cases := []struct {
		name      string
		mockSetup mockSetup
		expectErr error
	}{
		{
			name: "success — with categories",
			mockSetup: mockSetup{
				categories: []categorymodel.Category{
					{ID: uuid.New(), Name: "Fruits"},
					{ID: uuid.New(), Name: "Vegetables"},
					{ID: uuid.New(), Name: "Dairy"},
				},
			},
			expectErr: nil,
		},
		{
			name: "success — empty list",
			mockSetup: mockSetup{
				categories: []categorymodel.Category{},
			},
			expectErr: nil,
		},
		{
			name: "list categories unexpected error",
			mockSetup: mockSetup{
				err: errors.New("db connection lost"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			mockRepo.On("ListCategories", mock.Anything).
				Return(tc.mockSetup.categories, tc.mockSetup.err).
				Once()

			svc := categoryservice.NewService(mockRepo)

			result, err := svc.ListCategories(context.Background())

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.err != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, result, len(tc.mockSetup.categories))
				if len(tc.mockSetup.categories) > 0 {
					require.Equal(t, tc.mockSetup.categories[0].Name, result[0].Name)
				}
			}
		})
	}
}

// --- CreateCategory ---

func Test_Service_CreateCategory(t *testing.T) {
	type createInput struct {
		name string
	}

	type mockSetup struct {
		user              auth.User
		userInCtx         bool
		createdCategory   *categorymodel.Category
		createCategoryErr error
	}

	cases := []struct {
		name      string
		input     createInput
		mockSetup mockSetup
		expectErr error
	}{
		{
			name: "success — admin",
			input: createInput{
				name: "Fruits",
			},
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				createdCategory: &categorymodel.Category{
					ID:   uuid.New(),
					Name: "Fruits",
				},
			},
			expectErr: nil,
		},
		{
			name: "forbidden — seller",
			input: createInput{
				name: "Fruits",
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "forbidden — no user in context",
			input: createInput{
				name: "Fruits",
			},
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name: "category already exists",
			input: createInput{
				name: "Fruits",
			},
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				createCategoryErr: categorymodel.ErrCategoryAlreadyExists,
			},
			expectErr: service.ErrCategoryAlreadyExists,
		},
		{
			name: "create category unexpected error",
			input: createInput{
				name: "Fruits",
			},
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				createCategoryErr: errors.New("db write failed"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx && auth.CanManageCategory(tc.mockSetup.user) {
				mockRepo.On("CreateCategory", mock.Anything, mock.AnythingOfType("category.Category")).
					Return(tc.mockSetup.createdCategory, tc.mockSetup.createCategoryErr).
					Once()
			}

			svc := categoryservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.CreateCategory(ctx, categoryservice.CreateCategoryInput{
				Name: tc.input.name,
			})

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.createCategoryErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.createdCategory.ID, result.ID)
				require.Equal(t, tc.mockSetup.createdCategory.Name, result.Name)
			}
		})
	}
}

// --- UpdateCategory ---

func Test_Service_UpdateCategory(t *testing.T) {
	categoryID := uuid.New()

	type updateInput struct {
		name string
	}

	type mockSetup struct {
		user              auth.User
		userInCtx         bool
		updatedCategory   *categorymodel.Category
		updateCategoryErr error
	}

	cases := []struct {
		name       string
		categoryID uuid.UUID
		input      updateInput
		mockSetup  mockSetup
		expectErr  error
	}{
		{
			name:       "success — admin",
			categoryID: categoryID,
			input: updateInput{
				name: "Updated Fruits",
			},
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
				updatedCategory: &categorymodel.Category{
					ID:   categoryID,
					Name: "Updated Fruits",
				},
			},
			expectErr: nil,
		},
		{
			name:       "forbidden — seller",
			categoryID: categoryID,
			input: updateInput{
				name: "Hacked",
			},
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:       "forbidden — no user in context",
			categoryID: categoryID,
			input: updateInput{
				name: "Hacked",
			},
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:       "category not found",
			categoryID: categoryID,
			input: updateInput{
				name: "Updated",
			},
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				updateCategoryErr: categorymodel.ErrCategoryNotFound,
			},
			expectErr: service.ErrCategoryNotFound,
		},
		{
			name:       "category already exists (name conflict)",
			categoryID: categoryID,
			input: updateInput{
				name: "Existing Name",
			},
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				updateCategoryErr: categorymodel.ErrCategoryAlreadyExists,
			},
			expectErr: service.ErrCategoryAlreadyExists,
		},
		{
			name:       "update category unexpected error",
			categoryID: categoryID,
			input: updateInput{
				name: "Updated",
			},
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				updateCategoryErr: errors.New("db write failed"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx && auth.CanManageCategory(tc.mockSetup.user) {
				mockRepo.On("UpdateCategory", mock.Anything, tc.categoryID, tc.input.name).
					Return(tc.mockSetup.updatedCategory, tc.mockSetup.updateCategoryErr).
					Once()
			}

			svc := categoryservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			result, err := svc.UpdateCategory(ctx, tc.categoryID, categoryservice.UpdateCategoryInput{
				Name: tc.input.name,
			})

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, result)
			} else if tc.mockSetup.updateCategoryErr != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.mockSetup.updatedCategory.ID, result.ID)
				require.Equal(t, tc.mockSetup.updatedCategory.Name, result.Name)
			}
		})
	}
}

// --- DeleteCategory ---

func Test_Service_DeleteCategory(t *testing.T) {
	categoryID := uuid.New()

	type mockSetup struct {
		user              auth.User
		userInCtx         bool
		deleteCategoryErr error
	}

	cases := []struct {
		name       string
		categoryID uuid.UUID
		mockSetup  mockSetup
		expectErr  error
	}{
		{
			name:       "success — admin",
			categoryID: categoryID,
			mockSetup: mockSetup{
				user:      testAdminUser,
				userInCtx: true,
			},
			expectErr: nil,
		},
		{
			name:       "forbidden — seller",
			categoryID: categoryID,
			mockSetup: mockSetup{
				user:      testSellerUser,
				userInCtx: true,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:       "forbidden — no user in context",
			categoryID: categoryID,
			mockSetup: mockSetup{
				userInCtx: false,
			},
			expectErr: service.ErrForbidden,
		},
		{
			name:       "category not found",
			categoryID: categoryID,
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				deleteCategoryErr: categorymodel.ErrCategoryNotFound,
			},
			expectErr: service.ErrCategoryNotFound,
		},
		{
			name:       "category in use by products",
			categoryID: categoryID,
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				deleteCategoryErr: categorymodel.ErrCategoryInUse,
			},
			expectErr: service.ErrCategoryInUse,
		},
		{
			name:       "delete category unexpected error",
			categoryID: categoryID,
			mockSetup: mockSetup{
				user:              testAdminUser,
				userInCtx:         true,
				deleteCategoryErr: errors.New("db write failed"),
			},
			expectErr: nil, // wrapped
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := mocks.NewRepository(t)

			if tc.mockSetup.userInCtx && auth.CanManageCategory(tc.mockSetup.user) {
				mockRepo.On("DeleteCategory", mock.Anything, tc.categoryID).
					Return(tc.mockSetup.deleteCategoryErr).
					Once()
			}

			svc := categoryservice.NewService(mockRepo)

			ctx := testNoUserCtx
			if tc.mockSetup.userInCtx {
				ctx = ctxWithUser(tc.mockSetup.user)
			}

			err := svc.DeleteCategory(ctx, tc.categoryID)

			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
			} else if tc.mockSetup.deleteCategoryErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
