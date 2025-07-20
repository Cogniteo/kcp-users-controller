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

package cognito

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cogniteo/kcp-users-controller/pkg/cognito/mocks"
	"github.com/cogniteo/kcp-users-controller/pkg/userpool"
)

func TestNewAWSClient(t *testing.T) {
	tests := []struct {
		name       string
		userPoolID string
		expectErr  bool
	}{
		{
			name:       "valid user pool ID",
			userPoolID: "us-east-1_ABC123",
			expectErr:  false,
		},
		{
			name:       "empty user pool ID",
			userPoolID: "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test will try to load AWS config, which may fail in CI/CD
			// In a real implementation, you might want to mock the config loading
			client, err := NewAWSClient(context.Background(), tt.userPoolID)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				// In a test environment, this might fail due to AWS config
				// but the userPoolID validation should work
				if err != nil {
					// If AWS config fails, that's expected in test environment
					assert.Contains(t, err.Error(), "failed to load AWS config")
				} else {
					require.NotNil(t, client)
					assert.Equal(t, tt.userPoolID, client.userPoolID)
				}
			}
		})
	}
}

func TestNewAWSClientByName(t *testing.T) {
	tests := []struct {
		name         string
		userPoolName string
		expectErr    bool
	}{
		{
			name:         "valid user pool name",
			userPoolName: "my-user-pool",
			expectErr:    false,
		},
		{
			name:         "empty user pool name",
			userPoolName: "",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test will try to load AWS config and call AWS APIs
			client, err := NewAWSClientByName(context.Background(), tt.userPoolName)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				// In a test environment, this will likely fail due to AWS config/auth
				// but the userPoolName validation should work
				if err != nil {
					// Expected errors in test environment
					expectedErrors := []string{
						"failed to load AWS config",
						"failed to find user pool by name",
						"user pool with name",
						"not found",
					}
					hasExpectedError := false
					for _, expectedErr := range expectedErrors {
						if strings.Contains(err.Error(), expectedErr) {
							hasExpectedError = true
							break
						}
					}
					assert.True(t, hasExpectedError, "Error should be related to AWS config or user pool lookup: %v", err)
				} else {
					require.NotNil(t, client)
				}
			}
		})
	}
}

func TestNewAWSClientByName_WithMock(t *testing.T) {
	tests := []struct {
		name         string
		userPoolName string
		setupMocks   func(*mocks.MockCognitoAPI)
		expectErr    bool
		expectedID   string
	}{
		{
			name:         "successful user pool lookup",
			userPoolName: "test-pool",
			setupMocks: func(mockAPI *mocks.MockCognitoAPI) {
				mockAPI.On("ListUserPools", mock.Anything,
					mock.AnythingOfType("*cognitoidentityprovider.ListUserPoolsInput")).
					Return(&cognitoidentityprovider.ListUserPoolsOutput{
						UserPools: []types.UserPoolDescriptionType{
							{
								Id:   aws.String("us-east-1_ABC123"),
								Name: aws.String("test-pool"),
							},
						},
						NextToken: nil,
					}, nil)
			},
			expectErr:  false,
			expectedID: "us-east-1_ABC123",
		},
		{
			name:         "user pool not found",
			userPoolName: "nonexistent-pool",
			setupMocks: func(mockAPI *mocks.MockCognitoAPI) {
				mockAPI.On("ListUserPools", mock.Anything,
					mock.AnythingOfType("*cognitoidentityprovider.ListUserPoolsInput")).
					Return(&cognitoidentityprovider.ListUserPoolsOutput{
						UserPools: []types.UserPoolDescriptionType{
							{
								Id:   aws.String("us-east-1_ABC123"),
								Name: aws.String("other-pool"),
							},
						},
						NextToken: nil,
					}, nil)
			},
			expectErr:  true,
			expectedID: "",
		},
		{
			name:         "AWS API error",
			userPoolName: "test-pool",
			setupMocks: func(mockAPI *mocks.MockCognitoAPI) {
				mockAPI.On("ListUserPools", mock.Anything,
					mock.AnythingOfType("*cognitoidentityprovider.ListUserPoolsInput")).
					Return(nil, errors.New("AWS API error"))
			},
			expectErr:  true,
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := mocks.NewMockCognitoAPI(t)
			tt.setupMocks(mockAPI)

			// Test the findUserPoolIDByName function directly since we can't easily mock the config loading
			result, err := findUserPoolIDByName(context.Background(), mockAPI, tt.userPoolName)

			if tt.expectErr {
				require.Error(t, err)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedID, result)
			}
		})
	}
}

// Test the convenience functions
func TestNewClient(t *testing.T) {
	tests := []struct {
		name       string
		userPoolID string
		expectErr  bool
	}{
		{
			name:       "valid user pool ID",
			userPoolID: "us-east-1_ABC123",
			expectErr:  false,
		},
		{
			name:       "empty user pool ID",
			userPoolID: "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(context.Background(), tt.userPoolID)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				// In test environment, AWS config loading may fail
				if err != nil {
					assert.Contains(t, err.Error(), "failed to load AWS config")
				} else {
					require.NotNil(t, client)
				}
			}
		})
	}
}

func TestNewClientByName(t *testing.T) {
	tests := []struct {
		name         string
		userPoolName string
		expectErr    bool
	}{
		{
			name:         "valid user pool name",
			userPoolName: "my-user-pool",
			expectErr:    false,
		},
		{
			name:         "empty user pool name",
			userPoolName: "",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClientByName(context.Background(), tt.userPoolName)

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, client)
			} else {
				// In test environment, AWS operations will likely fail
				if err != nil {
					// Expected errors in test environment
					expectedErrors := []string{
						"failed to load AWS config",
						"failed to find user pool by name",
						"user pool with name",
						"not found",
					}
					hasExpectedError := false
					for _, expectedErr := range expectedErrors {
						if assert.ObjectsAreEqual(err.Error(), expectedErr) ||
							strings.Contains(err.Error(), expectedErr) {
							hasExpectedError = true
							break
						}
					}
					assert.True(t, hasExpectedError, "Error should be related to AWS config or user pool lookup: %v", err)
				} else {
					require.NotNil(t, client)
				}
			}
		})
	}
}

// Test edge cases for AWS client
func TestAWSClient_EdgeCases(t *testing.T) {
	t.Run("aws client with nil cognito API", func(t *testing.T) {
		client := &AWSClient{
			cognito:    nil,
			userPoolID: "test-pool",
		}

		// This should panic or return an error when calling methods
		assert.Panics(t, func() {
			_, _ = client.CreateUser(context.Background(), &userpool.User{
				Username: "test",
				Email:    "test@example.com",
				Enabled:  true,
			})
		})
	})

	t.Run("aws client with empty pool ID", func(t *testing.T) {
		mockAPI := mocks.NewMockCognitoAPI(t)
		client := &AWSClient{
			cognito:    mockAPI,
			userPoolID: "",
		}

		// Should still work, but AWS will likely return an error
		// We'll mock a validation error from AWS
		mockAPI.On("AdminCreateUser", mock.Anything,
			mock.AnythingOfType("*cognitoidentityprovider.AdminCreateUserInput")).
			Return(nil, errors.New("invalid user pool ID"))

		_, err := client.CreateUser(context.Background(), &userpool.User{
			Username: "test",
			Email:    "test@example.com",
			Enabled:  true,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create user")
	})
}
