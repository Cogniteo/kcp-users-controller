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
	"fmt"

	"github.com/cogniteo/kcp-users-controller/pkg/userpool"
)

// MockClient implements the userpool.Client interface for testing
type MockClient struct {
	users map[string]*userpool.User
}

// NewMockClient creates a new mock client for testing
func NewMockClient() *MockClient {
	return &MockClient{
		users: make(map[string]*userpool.User),
	}
}

// CreateUser creates a new user in the mock store
func (m *MockClient) CreateUser(ctx context.Context, user *userpool.User) (*userpool.User, error) {
	if user == nil {
		return nil, fmt.Errorf("user cannot be nil")
	}
	if user.Username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	// Check if user already exists
	if existingUser, exists := m.users[user.Username]; exists {
		return existingUser, nil
	}

	// Create a copy to avoid reference issues
	createdUser := &userpool.User{
		Username: user.Username,
		Email:    user.Email,
		Enabled:  user.Enabled,
		Sub:      user.Username, // Mock uses username as sub
	}
	m.users[user.Username] = createdUser

	return createdUser, nil
}

// GetUser retrieves a user from the mock store
func (m *MockClient) GetUser(ctx context.Context, username string) (*userpool.User, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	user, exists := m.users[username]
	if !exists {
		return nil, fmt.Errorf("user %s not found", username)
	}

	// Return a copy to avoid reference issues
	return &userpool.User{
		Username: user.Username,
		Email:    user.Email,
		Enabled:  user.Enabled,
		Sub:      user.Sub,
	}, nil
}

// UpdateUser updates an existing user in the mock store
func (m *MockClient) UpdateUser(ctx context.Context, user *userpool.User) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}
	if user.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Check if user exists
	if _, exists := m.users[user.Username]; !exists {
		return fmt.Errorf("user %s not found", user.Username)
	}

	// Update the user
	m.users[user.Username] = &userpool.User{
		Username: user.Username,
		Email:    user.Email,
		Enabled:  user.Enabled,
	}

	return nil
}

// DeleteUser removes a user from the mock store
func (m *MockClient) DeleteUser(ctx context.Context, username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Check if user exists
	if _, exists := m.users[username]; !exists {
		return fmt.Errorf("user %s not found", username)
	}

	delete(m.users, username)
	return nil
}

// ListUsers lists all users in the mock store
func (m *MockClient) ListUsers(ctx context.Context) ([]*userpool.User, error) {
	users := make([]*userpool.User, 0, len(m.users))
	for _, user := range m.users {
		// Return copies to avoid reference issues
		users = append(users, &userpool.User{
			Username: user.Username,
			Email:    user.Email,
			Enabled:  user.Enabled,
		})
	}
	return users, nil
}
