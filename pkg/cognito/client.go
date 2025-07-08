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
	"piotrjanik.dev/users/pkg/userpool"
)

// NewClient creates a new Cognito client with Pod Identity authentication
// This is a convenience function that returns the AWS implementation
func NewClient(ctx context.Context, userPoolID string) (userpool.Client, error) {
	return NewAWSClient(ctx, userPoolID)
}
