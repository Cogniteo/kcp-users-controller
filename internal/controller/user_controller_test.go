/*
Copyright 2025 Piotr Janik.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kcpv1alpha1 "github.com/cogniteo/kcp-users-controller/api/v1alpha1"
	"github.com/cogniteo/kcp-users-controller/internal/controller/mocks"
	"github.com/cogniteo/kcp-users-controller/pkg/userpool"
)

func TestUserReconciler(t *testing.T) {
	t.Run("Reconcile", func(t *testing.T) {
		t.Run("nil user pool client handling", func(t *testing.T) {
			reconciler := &UserReconciler{
				UserPoolClient: nil,
			}

			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			}

			log := logr.Discard()

			err := reconciler.syncUserWithUserPool(context.Background(), user, log)
			require.NoError(t, err, "syncUserWithUserPool should handle nil UserPoolClient gracefully")
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "test-sub-123", log)
		})

		t.Run("finalizer management", func(t *testing.T) {
			finalizers := []string{"other.io/finalizer"}
			finalizerName := "kcp.cogniteo.io/user-pool-cleanup"

			assert.False(t, containsFinalizer(finalizers, finalizerName))

			finalizersWithAdded := append(finalizers, finalizerName)
			assert.True(t, containsFinalizer(finalizersWithAdded, finalizerName))

			finalizersAfterRemoval := removeFinalizer(finalizersWithAdded, finalizerName)
			assert.False(t, containsFinalizer(finalizersAfterRemoval, finalizerName))
			assert.Equal(t, finalizers, finalizersAfterRemoval)
		})

		t.Run("user pool sync integration", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("CreateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(&userpool.User{
				Username: "test-user",
				Email:    "test@example.com",
				Enabled:  true,
				Sub:      "test-sub-123",
			}, nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.NoError(t, err)
			assert.Equal(t, "test-sub-123", user.Status.Sub)
			assert.Equal(t, "CONFIRMED", user.Status.UserPoolStatus)
			assert.NotNil(t, user.Status.LastSyncTime)
			mockUserPool.AssertExpectations(t)
		})

		t.Run("user pool deletion integration", func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Sub:      "test-sub-123",
			}, nil)
			mockUserPool.On("DeleteUser", mock.Anything, "test-sub-123").Return(nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "test-sub-123", log)

			mockUserPool.AssertExpectations(t)
		})

		t.Run("error handling in sync operations", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("CreateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(nil, errors.New("user pool service unavailable"))

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create user in user pool")
			mockUserPool.AssertExpectations(t)
		})

		t.Run("sub-only user fetching", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "updated@example.com",
					Enabled: false,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Email:    "test@example.com",
				Enabled:  true,
				Sub:      "test-sub-123",
			}, nil)
			mockUserPool.On("UpdateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.NoError(t, err)
			assert.NotNil(t, user.Status.LastSyncTime)
			mockUserPool.AssertExpectations(t)
		})
	})
	t.Run("deleteUserFromUserPool", func(t *testing.T) {
		t.Run("without user pool client", func(t *testing.T) {
			reconciler := &UserReconciler{
				UserPoolClient: nil, // No user pool client
			}

			log := logr.Discard()

			// Should not panic when UserPoolClient is nil
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "test-sub-123", log)

			// Test passes if no panic occurs
		})

		t.Run("delete user with sub", func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Sub:      "test-sub-123",
			}, nil)
			mockUserPool.On("DeleteUser", mock.Anything, "test-sub-123").Return(nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()

			// This method doesn't return an error, it just logs
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "test-sub-123", log)

			// Test passes if no panic occurs and mocks are satisfied
			mockUserPool.AssertExpectations(t)
		})

		t.Run("delete user with username fallback", func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-user").Return(&userpool.User{
				Username: "test-user",
			}, nil)
			mockUserPool.On("DeleteUser", mock.Anything, "test-user").Return(nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()

			// This method doesn't return an error, it just logs
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "", log)

			// Test passes if no panic occurs and mocks are satisfied
			mockUserPool.AssertExpectations(t)
		})

		t.Run("user not found in pool", func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(nil, errors.New("user not found"))

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()

			// This method doesn't return an error, it just logs
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "test-sub-123", log)

			// Test passes if no panic occurs and mocks are satisfied
			mockUserPool.AssertExpectations(t)
		})

		t.Run("delete fails but continues", func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Sub:      "test-sub-123",
			}, nil)
			mockUserPool.On("DeleteUser", mock.Anything, "test-sub-123").Return(errors.New("delete failed"))

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()

			// This method doesn't return an error, it just logs
			reconciler.deleteUserFromUserPool(context.Background(), "test-user", "test-sub-123", log)

			// Test passes if no panic occurs and mocks are satisfied
			mockUserPool.AssertExpectations(t)
		})
	})

	t.Run("syncUserWithUserPool", func(t *testing.T) {
		t.Run("create new user successfully", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("CreateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(&userpool.User{
				Username: "test-user",
				Email:    "test@example.com",
				Enabled:  true,
				Sub:      "test-sub-123",
			}, nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.NoError(t, err)
			assert.NotNil(t, user.Status.LastSyncTime)
			assert.Equal(t, "test-sub-123", user.Status.Sub)
			assert.Equal(t, "CONFIRMED", user.Status.UserPoolStatus)
		})

		t.Run("update existing user", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "updated@example.com",
					Enabled: false,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Email:    "test@example.com",
				Enabled:  true,
				Sub:      "test-sub-123",
			}, nil)
			mockUserPool.On("UpdateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.NoError(t, err)
			assert.NotNil(t, user.Status.LastSyncTime)
		})

		t.Run("user exists and up to date", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Email:    "test@example.com",
				Enabled:  true,
				Sub:      "test-sub-123",
			}, nil)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.NoError(t, err)
			assert.NotNil(t, user.Status.LastSyncTime)
		})

		t.Run("create user fails", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("CreateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(nil, errors.New("creation failed"))

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create user in user pool")
		})

		t.Run("update user fails", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "updated@example.com",
					Enabled: false,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			}

			mockUserPool := mocks.NewMockUserPoolClient(t)
			mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
				Username: "test-user",
				Email:    "test@example.com",
				Enabled:  true,
				Sub:      "test-sub-123",
			}, nil)
			mockUserPool.On("UpdateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(errors.New("update failed"))

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update user in user pool")
		})

		t.Run("without user pool client", func(t *testing.T) {
			user := &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			}

			reconciler := &UserReconciler{
				UserPoolClient: nil, // No user pool client
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), user, log)

			// Should not error when UserPoolClient is nil (graceful handling)
			require.NoError(t, err)
		})
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("containsFinalizer", func(t *testing.T) {
		tests := []struct {
			name       string
			finalizers []string
			finalizer  string
			expected   bool
		}{
			{
				name:       "finalizer exists",
				finalizers: []string{"test.io/finalizer", "other.io/finalizer"},
				finalizer:  "test.io/finalizer",
				expected:   true,
			},
			{
				name:       "finalizer does not exist",
				finalizers: []string{"test.io/finalizer", "other.io/finalizer"},
				finalizer:  "missing.io/finalizer",
				expected:   false,
			},
			{
				name:       "empty finalizers list",
				finalizers: []string{},
				finalizer:  "test.io/finalizer",
				expected:   false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := containsFinalizer(tt.finalizers, tt.finalizer)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("removeFinalizer", func(t *testing.T) {
		tests := []struct {
			name       string
			finalizers []string
			finalizer  string
			expected   []string
		}{
			{
				name:       "remove existing finalizer",
				finalizers: []string{"test.io/finalizer", "other.io/finalizer"},
				finalizer:  "test.io/finalizer",
				expected:   []string{"other.io/finalizer"},
			},
			{
				name:       "remove non-existing finalizer",
				finalizers: []string{"test.io/finalizer", "other.io/finalizer"},
				finalizer:  "missing.io/finalizer",
				expected:   []string{"test.io/finalizer", "other.io/finalizer"},
			},
			{
				name:       "remove from empty list",
				finalizers: []string{},
				finalizer:  "test.io/finalizer",
				expected:   []string{},
			},
			{
				name:       "remove all occurrences",
				finalizers: []string{"test.io/finalizer", "other.io/finalizer", "test.io/finalizer"},
				finalizer:  "test.io/finalizer",
				expected:   []string{"other.io/finalizer"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := removeFinalizer(tt.finalizers, tt.finalizer)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}
