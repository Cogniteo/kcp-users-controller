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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kcpv1alpha1 "github.com/cogniteo/kcp-users-controller/api/v1alpha1"
	"github.com/cogniteo/kcp-users-controller/pkg/userpool"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Manager        mcmanager.Manager
	UserPoolClient userpool.Client
}

// +kubebuilder:rbac:groups=kcp.cogniteo.io,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kcp.cogniteo.io,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kcp.cogniteo.io,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("cluster", req.ClusterName)
	log.Info("Reconciling User")

	// Fetch the User instance
	var user kcpv1alpha1.User
	cl, err := r.Manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get cluster: %w", err)
	}
	clusterClient := cl.GetClient()
	if err := clusterClient.Get(ctx, req.NamespacedName, &user); err != nil {
		if errors.IsNotFound(err) {
			// User was deleted, no action needed as finalizer should have handled cleanup
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle finalizer for cleanup before deletion
	finalizerName := "kcp.cogniteo.io/user-pool-cleanup"
	if user.ObjectMeta.DeletionTimestamp != nil {
		// User is being deleted, run cleanup
		if r.UserPoolClient != nil {
			r.deleteUserFromUserPool(ctx, user.Name, user.Status.Sub, log)
		}
		
		// Remove finalizer
		user.ObjectMeta.Finalizers = removeFinalizer(user.ObjectMeta.Finalizers, finalizerName)
		if err := clusterClient.Update(ctx, &user); err != nil {
			log.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !containsFinalizer(user.ObjectMeta.Finalizers, finalizerName) {
		user.ObjectMeta.Finalizers = append(user.ObjectMeta.Finalizers, finalizerName)
		if err := clusterClient.Update(ctx, &user); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Sync user with user pool
	if r.UserPoolClient != nil {
		if err := r.syncUserWithUserPool(ctx, &user, log); err != nil {
			log.Error(err, "Failed to sync user with user pool")
			return ctrl.Result{RequeueAfter: time.Minute * 5}, err
		}
	}

	// Add or update annotation
	if user.Annotations == nil {
		user.Annotations = make(map[string]string)
	}
	user.Annotations["kcp.cogniteo.io/lastReconciledAt"] = time.Now().Format(time.RFC3339)

	// Update the resource with both spec and status
	if err := clusterClient.Update(ctx, &user); err != nil {
		log.Error(err, "Failed to update User")
		return ctrl.Result{}, err
	}

	// Update the status subresource
	if err := clusterClient.Status().Update(ctx, &user); err != nil {
		log.Error(err, "Failed to update User status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// syncUserWithUserPool synchronizes a Kubernetes User with User Pool
func (r *UserReconciler) syncUserWithUserPool(ctx context.Context, user *kcpv1alpha1.User, log logr.Logger) error {
	poolUser := &userpool.User{
		Username: user.Name,
		Email:    user.Spec.Email,
		Enabled:  user.Spec.Enabled,
	}

	// Check if user exists in user pool (use sub from status if available)
	var existingUser *userpool.User
	var err error
	
	if user.Status.Sub != "" {
		// Use stored sub to check if user exists
		existingUser, err = r.UserPoolClient.GetUser(ctx, user.Status.Sub)
	} else {
		// Fallback to using email as identifier
		existingUser, err = r.UserPoolClient.GetUser(ctx, user.Spec.Email)
	}
	
	if err != nil {
		// User doesn't exist, create it
		log.Info("Creating user in user pool", "username", user.Name)
		createdUser, err := r.UserPoolClient.CreateUser(ctx, poolUser)
		if err != nil {
			return fmt.Errorf("failed to create user in user pool: %w", err)
		}
		log.Info("User created in user pool", "username", user.Name, "sub", createdUser.Sub)
		
		// Update the status with the sub
		user.Status.Sub = createdUser.Sub
		user.Status.UserPoolStatus = "CONFIRMED"
		now := metav1.Now()
		user.Status.LastSyncTime = &now
	} else {
		// User exists, check if update is needed
		if existingUser.Email != poolUser.Email || existingUser.Enabled != poolUser.Enabled {
			log.Info("Updating user in user pool", "username", user.Name)
			if err := r.UserPoolClient.UpdateUser(ctx, poolUser); err != nil {
				return fmt.Errorf("failed to update user in user pool: %w", err)
			}
			log.Info("User updated in user pool", "username", user.Name)
		} else {
			// User exists and is up to date
			log.Info("User already exists in user pool and is up to date", "username", user.Name)
		}
		
		// Update the status with sync time
		now := metav1.Now()
		user.Status.LastSyncTime = &now
		if user.Status.Sub == "" {
			user.Status.Sub = existingUser.Sub
		}
	}

	return nil
}

// deleteUserFromUserPool safely deletes a user from the user pool with appropriate logging
func (r *UserReconciler) deleteUserFromUserPool(ctx context.Context, username string, sub string, log logr.Logger) {
	// Determine what identifier to use for deletion
	identifier := sub
	if identifier == "" {
		// Fallback to username if sub is not available
		identifier = username
		log.Info("No sub available, using username for deletion", "username", username)
	}

	// Check if user exists first
	_, err := r.UserPoolClient.GetUser(ctx, identifier)
	if err != nil {
		// User doesn't exist in user pool
		log.Info("User not found in user pool, nothing to delete", "username", username, "identifier", identifier)
		return
	}

	// User exists, proceed with deletion
	if err := r.UserPoolClient.DeleteUser(ctx, identifier); err != nil {
		log.Error(err, "Failed to delete user from user pool", "username", username, "identifier", identifier)
		// Continue with reconciliation even if user pool deletion fails
	} else {
		log.Info("User deleted from user pool", "username", username, "identifier", identifier)
	}
}

// containsFinalizer checks if a finalizer is present in the list
func containsFinalizer(finalizers []string, finalizer string) bool {
	for _, f := range finalizers {
		if f == finalizer {
			return true
		}
	}
	return false
}

// removeFinalizer removes a finalizer from the list
func removeFinalizer(finalizers []string, finalizer string) []string {
	result := []string{}
	for _, f := range finalizers {
		if f != finalizer {
			result = append(result, f)
		}
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kcpv1alpha1.User{}).
		Named("user").
		Complete(mcreconcile.Func(r.Reconcile))
}
