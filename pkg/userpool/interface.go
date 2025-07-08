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

package userpool

import (
	"context"
)

// User represents a user in a user pool
type User struct {
	Username string
	Email    string
	Enabled  bool
}

// Client defines the interface for managing users in a user pool
type Client interface {
	// CreateUser creates a new user in the user pool
	CreateUser(ctx context.Context, user *User) error

	// GetUser retrieves a user from the user pool by username
	GetUser(ctx context.Context, username string) (*User, error)

	// UpdateUser updates an existing user in the user pool
	UpdateUser(ctx context.Context, user *User) error

	// DeleteUser removes a user from the user pool
	DeleteUser(ctx context.Context, username string) error

	// ListUsers lists all users in the user pool
	ListUsers(ctx context.Context) ([]*User, error)
}
