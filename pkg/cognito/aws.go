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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"

	"github.com/cogniteo/kcp-users-controller/pkg/userpool"
)

// AWSClient implements the userpool.Client interface for AWS Cognito
type AWSClient struct {
	cognito    CognitoAPI
	userPoolID string
}

// NewAWSClient creates a new AWS Cognito client with Pod Identity authentication
func NewAWSClient(ctx context.Context, userPoolID string) (*AWSClient, error) {
	if userPoolID == "" {
		return nil, fmt.Errorf("userPoolID cannot be empty")
	}

	// Load AWS configuration with Pod Identity (IRSA)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSClient{
		cognito:    cognitoidentityprovider.NewFromConfig(cfg),
		userPoolID: userPoolID,
	}, nil
}

// NewAWSClientByName creates a new AWS Cognito client by finding user pool ID from name
func NewAWSClientByName(ctx context.Context, userPoolName string) (*AWSClient, error) {
	if userPoolName == "" {
		return nil, fmt.Errorf("userPoolName cannot be empty")
	}

	// Load AWS configuration with Pod Identity (IRSA)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	cognito := cognitoidentityprovider.NewFromConfig(cfg)

	// Find user pool ID by name
	userPoolID, err := findUserPoolIDByName(ctx, cognito, userPoolName)
	if err != nil {
		return nil, fmt.Errorf("failed to find user pool by name %s: %w", userPoolName, err)
	}

	return &AWSClient{
		cognito:    cognito,
		userPoolID: userPoolID,
	}, nil
}

// findUserPoolIDByName finds a user pool ID by its name
func findUserPoolIDByName(ctx context.Context, cognito CognitoAPI,
	userPoolName string) (string, error) {
	var nextToken *string

	for {
		input := &cognitoidentityprovider.ListUserPoolsInput{
			MaxResults: aws.Int32(60), // Max allowed by AWS
			NextToken:  nextToken,
		}

		output, err := cognito.ListUserPools(ctx, input)
		if err != nil {
			return "", fmt.Errorf("failed to list user pools: %w", err)
		}

		for _, userPool := range output.UserPools {
			if userPool.Name != nil && strings.EqualFold(*userPool.Name, userPoolName) {
				if userPool.Id != nil {
					return *userPool.Id, nil
				}
			}
		}

		nextToken = output.NextToken
		if nextToken == nil {
			break
		}
	}

	return "", fmt.Errorf("user pool with name %s not found", userPoolName)
}

// CreateUser creates a new user in the Cognito user pool
func (c *AWSClient) CreateUser(ctx context.Context, user *userpool.User) (*userpool.User, error) {
	if user == nil {
		return nil, fmt.Errorf("user cannot be nil")
	}
	if user.Email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	attributes := []types.AttributeType{
		{
			Name:  aws.String("email"),
			Value: aws.String(user.Email),
		},
		{
			Name:  aws.String("email_verified"),
			Value: aws.String("true"),
		},
	}

	input := &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:     aws.String(c.userPoolID),
		Username:       aws.String(user.Email),
		UserAttributes: attributes,
		MessageAction:  types.MessageActionTypeSuppress, // Don't send welcome email
	}

	resp, err := c.cognito.AdminCreateUser(ctx, input)
	if err != nil {
		// Check if the error is due to user already existing
		var userExistsErr *types.UsernameExistsException
		if errors.As(err, &userExistsErr) {
			// User already exists, get the existing user info
			return c.GetUser(ctx, user.Email)
		}
		return nil, fmt.Errorf("failed to create user %s: %w", user.Email, err)
	}

	// Extract the sub from the response
	createdUser := &userpool.User{
		Username: user.Username,
		Email:    user.Email,
		Enabled:  user.Enabled,
		Sub:      user.Email, // Default fallback
	}

	// If the response contains user attributes, look for the sub attribute
	if resp.User != nil && len(resp.User.Attributes) > 0 {
		for _, attr := range resp.User.Attributes {
			if attr.Name != nil && *attr.Name == "sub" && attr.Value != nil {
				createdUser.Sub = *attr.Value
				break
			}
		}
	}

	return createdUser, nil
}

// GetUser retrieves a user from the Cognito user pool by username
func (c *AWSClient) GetUser(ctx context.Context, username string) (*userpool.User, error) {
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	input := &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(username),
	}

	output, err := c.cognito.AdminGetUser(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s: %w", username, err)
	}

	user := &userpool.User{
		Username: username,
		Enabled:  output.Enabled,
		Sub:      username, // The username is the sub in Cognito
	}

	// Extract email from user attributes
	for _, attr := range output.UserAttributes {
		if attr.Name != nil && *attr.Name == "email" && attr.Value != nil {
			user.Email = *attr.Value
			break
		}
	}

	return user, nil
}

// UpdateUser updates an existing user in the Cognito user pool
func (c *AWSClient) UpdateUser(ctx context.Context, user *userpool.User) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}
	if user.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Update user attributes
	attributes := []types.AttributeType{
		{
			Name:  aws.String("email"),
			Value: aws.String(user.Email),
		},
	}

	updateInput := &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserPoolId:     aws.String(c.userPoolID),
		Username:       aws.String(user.Username),
		UserAttributes: attributes,
	}

	_, err := c.cognito.AdminUpdateUserAttributes(ctx, updateInput)
	if err != nil {
		return fmt.Errorf("failed to update user attributes for %s: %w", user.Username, err)
	}

	// Update user status if needed
	if user.Enabled {
		enableInput := &cognitoidentityprovider.AdminEnableUserInput{
			UserPoolId: aws.String(c.userPoolID),
			Username:   aws.String(user.Username),
		}
		_, err = c.cognito.AdminEnableUser(ctx, enableInput)
		if err != nil {
			return fmt.Errorf("failed to enable user %s: %w", user.Username, err)
		}
	} else {
		disableInput := &cognitoidentityprovider.AdminDisableUserInput{
			UserPoolId: aws.String(c.userPoolID),
			Username:   aws.String(user.Username),
		}
		_, err = c.cognito.AdminDisableUser(ctx, disableInput)
		if err != nil {
			return fmt.Errorf("failed to disable user %s: %w", user.Username, err)
		}
	}

	return nil
}

// DeleteUser removes a user from the Cognito user pool
func (c *AWSClient) DeleteUser(ctx context.Context, username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	input := &cognitoidentityprovider.AdminDeleteUserInput{
		UserPoolId: aws.String(c.userPoolID),
		Username:   aws.String(username),
	}

	_, err := c.cognito.AdminDeleteUser(ctx, input)
	if err != nil {
		// Check if the error is due to user not existing
		var userNotFoundErr *types.UserNotFoundException
		if errors.As(err, &userNotFoundErr) {
			// User doesn't exist, this is not an error for deletion
			return nil
		}
		return fmt.Errorf("failed to delete user %s: %w", username, err)
	}

	return nil
}

// ListUsers lists all users in the Cognito user pool
func (c *AWSClient) ListUsers(ctx context.Context) ([]*userpool.User, error) {
	var users []*userpool.User
	var nextToken *string

	for {
		input := &cognitoidentityprovider.ListUsersInput{
			UserPoolId:      aws.String(c.userPoolID),
			PaginationToken: nextToken,
		}

		output, err := c.cognito.ListUsers(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list users: %w", err)
		}

		for _, cognitoUser := range output.Users {
			if cognitoUser.Username == nil {
				continue
			}

			user := &userpool.User{
				Username: *cognitoUser.Username,
				Enabled:  cognitoUser.Enabled,
			}

			// Extract email from user attributes
			for _, attr := range cognitoUser.Attributes {
				if attr.Name != nil && *attr.Name == "email" && attr.Value != nil {
					user.Email = *attr.Value
					break
				}
			}

			users = append(users, user)
		}

		nextToken = output.PaginationToken
		if nextToken == nil {
			break
		}
	}

	return users, nil
}

// NewClient creates a new Cognito client with Pod Identity authentication
// This is a convenience function that returns the AWS implementation
func NewClient(ctx context.Context, userPoolID string) (userpool.Client, error) {
	return NewAWSClient(ctx, userPoolID)
}

// NewClientByName creates a new Cognito client by finding user pool ID from name
// This is a convenience function that returns the AWS implementation
func NewClientByName(ctx context.Context, userPoolName string) (userpool.Client, error) {
	return NewAWSClientByName(ctx, userPoolName)
}
