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

func TestUserReconciler_syncUserWithUserPool(t *testing.T) {
	tests := []struct {
		name       string
		user       *kcpv1alpha1.User
		setupMocks func(*mocks.MockUserPoolClient)
		expectErr  bool
	}{
		{
			name: "create new user successfully",
			user: &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			},
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test@example.com").Return(nil, errors.New("user not found"))
				mockUserPool.On("CreateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(&userpool.User{
					Username: "test-user",
					Email:    "test@example.com",
					Enabled:  true,
					Sub:      "test-sub-123",
				}, nil)
			},
			expectErr: false,
		},
		{
			name: "update existing user",
			user: &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "updated@example.com",
					Enabled: false,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			},
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
					Username: "test-user",
					Email:    "test@example.com",
					Enabled:  true,
					Sub:      "test-sub-123",
				}, nil)
				mockUserPool.On("UpdateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "user exists and up to date",
			user: &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			},
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
					Username: "test-user",
					Email:    "test@example.com",
					Enabled:  true,
					Sub:      "test-sub-123",
				}, nil)
			},
			expectErr: false,
		},
		{
			name: "create user fails",
			user: &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "test@example.com",
					Enabled: true,
				},
			},
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test@example.com").Return(nil, errors.New("user not found"))
				mockUserPool.On("CreateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(nil, errors.New("creation failed"))
			},
			expectErr: true,
		},
		{
			name: "update user fails",
			user: &kcpv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec: kcpv1alpha1.UserSpec{
					Email:   "updated@example.com",
					Enabled: false,
				},
				Status: kcpv1alpha1.UserStatus{
					Sub: "test-sub-123",
				},
			},
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
					Username: "test-user",
					Email:    "test@example.com",
					Enabled:  true,
					Sub:      "test-sub-123",
				}, nil)
				mockUserPool.On("UpdateUser", mock.Anything, mock.AnythingOfType("*userpool.User")).Return(errors.New("update failed"))
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			tt.setupMocks(mockUserPool)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()
			err := reconciler.syncUserWithUserPool(context.Background(), tt.user, log)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Check status updates for successful operations
				if !tt.expectErr {
					assert.NotNil(t, tt.user.Status.LastSyncTime)
					if tt.user.Status.Sub == "" {
						// For new user creation, sub should be populated
						if tt.name == "create new user successfully" {
							assert.Equal(t, "test-sub-123", tt.user.Status.Sub)
							assert.Equal(t, "CONFIRMED", tt.user.Status.UserPoolStatus)
						}
					}
				}
			}
		})
	}
}

func TestUserReconciler_deleteUserFromUserPool(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		sub        string
		setupMocks func(*mocks.MockUserPoolClient)
	}{
		{
			name:     "delete user with sub",
			username: "test-user",
			sub:      "test-sub-123",
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
					Username: "test-user",
					Sub:      "test-sub-123",
				}, nil)
				mockUserPool.On("DeleteUser", mock.Anything, "test-sub-123").Return(nil)
			},
		},
		{
			name:     "delete user with username fallback",
			username: "test-user",
			sub:      "",
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-user").Return(&userpool.User{
					Username: "test-user",
				}, nil)
				mockUserPool.On("DeleteUser", mock.Anything, "test-user").Return(nil)
			},
		},
		{
			name:     "user not found in pool",
			username: "test-user",
			sub:      "test-sub-123",
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(nil, errors.New("user not found"))
			},
		},
		{
			name:     "delete fails but continues",
			username: "test-user",
			sub:      "test-sub-123",
			setupMocks: func(mockUserPool *mocks.MockUserPoolClient) {
				mockUserPool.On("GetUser", mock.Anything, "test-sub-123").Return(&userpool.User{
					Username: "test-user",
					Sub:      "test-sub-123",
				}, nil)
				mockUserPool.On("DeleteUser", mock.Anything, "test-sub-123").Return(errors.New("delete failed"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserPool := mocks.NewMockUserPoolClient(t)
			tt.setupMocks(mockUserPool)

			reconciler := &UserReconciler{
				UserPoolClient: mockUserPool,
			}

			log := logr.Discard()

			// This method doesn't return an error, it just logs
			reconciler.deleteUserFromUserPool(context.Background(), tt.username, tt.sub, log)

			// Test passes if no panic occurs
		})
	}
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
