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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kcpv1alpha1 "piotrjanik.dev/users/api/v1alpha1"
	"piotrjanik.dev/users/pkg/userpool"
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
	client := cl.GetClient()
	if err := client.Get(ctx, req.NamespacedName, &user); err != nil {
		if errors.IsNotFound(err) {
			// User was deleted, remove from user pool
			if r.UserPoolClient != nil {
				if err := r.UserPoolClient.DeleteUser(ctx, req.NamespacedName.Name); err != nil {
					log.Error(err, "Failed to delete user from user pool", "username", req.NamespacedName.Name)
					// Continue with reconciliation even if user pool deletion fails
				} else {
					log.Info("User deleted from user pool", "username", req.NamespacedName.Name)
				}
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
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

	if err := client.Update(ctx, &user); err != nil {
		log.Error(err, "Failed to update User annotation")
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

	// Check if user exists in user pool
	existingUser, err := r.UserPoolClient.GetUser(ctx, user.Name)
	if err != nil {
		// User doesn't exist, create it
		log.Info("Creating user in user pool", "username", user.Name)
		if err := r.UserPoolClient.CreateUser(ctx, poolUser); err != nil {
			return fmt.Errorf("failed to create user in user pool: %w", err)
		}
		log.Info("User created in user pool", "username", user.Name)
	} else {
		// User exists, update if needed
		if existingUser.Email != poolUser.Email || existingUser.Enabled != poolUser.Enabled {
			log.Info("Updating user in user pool", "username", user.Name)
			if err := r.UserPoolClient.UpdateUser(ctx, poolUser); err != nil {
				return fmt.Errorf("failed to update user in user pool: %w", err)
			}
			log.Info("User updated in user pool", "username", user.Name)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&kcpv1alpha1.User{}).
		Named("user").
		Complete(mcreconcile.Func(r.Reconcile))
}
