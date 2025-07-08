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
	"net/http"
	"testing"
	"time"

	logr "github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kcpv1alpha1 "piotrjanik.dev/users/api/v1alpha1"
	"piotrjanik.dev/users/pkg/cognito"
)

// Test helper types
type fakeCluster struct {
	client client.Client
}

func (f *fakeCluster) GetClient() client.Client                             { return f.client }
func (f *fakeCluster) GetAPIReader() client.Reader                          { return f.client }
func (f *fakeCluster) GetHTTPClient() *http.Client                          { return nil }
func (f *fakeCluster) GetConfig() *rest.Config                              { return nil }
func (f *fakeCluster) GetCache() cache.Cache                                { return nil }
func (f *fakeCluster) GetScheme() *runtime.Scheme                           { return nil }
func (f *fakeCluster) GetFieldIndexer() client.FieldIndexer                 { return nil }
func (f *fakeCluster) GetEventRecorderFor(name string) record.EventRecorder { return nil }
func (f *fakeCluster) GetRESTMapper() meta.RESTMapper                       { return nil }
func (f *fakeCluster) Start(ctx context.Context) error                      { return nil }

type fakeManager struct {
	cluster cluster.Cluster
	err     error
}

func (f *fakeManager) GetCluster(ctx context.Context, name string) (cluster.Cluster, error) {
	return f.cluster, f.err
}

// Minimal Manager interface implementation
func (f *fakeManager) Add(runnable mcmanager.Runnable) error { return nil }
func (f *fakeManager) Elected() <-chan struct{}              { return nil }
func (f *fakeManager) AddMetricsServerExtraHandler(path string, handler http.Handler) error {
	return nil
}
func (f *fakeManager) AddHealthzCheck(name string, check healthz.Checker) error { return nil }
func (f *fakeManager) AddReadyzCheck(name string, check healthz.Checker) error  { return nil }
func (f *fakeManager) Start(ctx context.Context) error                          { return nil }
func (f *fakeManager) GetWebhookServer() webhook.Server                         { return nil }
func (f *fakeManager) GetLogger() logr.Logger                                   { return logr.Logger{} }
func (f *fakeManager) GetControllerOptions() config.Controller                  { return config.Controller{} }
func (f *fakeManager) ClusterFromContext(ctx context.Context) (cluster.Cluster, error) {
	return nil, nil
}
func (f *fakeManager) GetManager(ctx context.Context, clusterName string) (manager.Manager, error) {
	return nil, nil
}
func (f *fakeManager) GetLocalManager() manager.Manager     { return nil }
func (f *fakeManager) GetProvider() multicluster.Provider   { return nil }
func (f *fakeManager) GetFieldIndexer() client.FieldIndexer { return nil }
func (f *fakeManager) Engage(ctx context.Context, clusterName string, cluster cluster.Cluster) error {
	return nil
}

// Use the mock client from the cognito package for testing

var _ = Describe("User Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		user := &kcpv1alpha1.User{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind User")
			err := k8sClient.Get(ctx, typeNamespacedName, user)
			if err != nil && errors.IsNotFound(err) {
				resource := &kcpv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kcpv1alpha1.UserSpec{
						Email:   "test@example.com",
						Enabled: true,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &kcpv1alpha1.User{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance User")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			// Create a mock manager that returns the test client
			mockManager := &fakeManager{
				cluster: &fakeCluster{client: k8sClient},
				err:     nil,
			}

			// Create a mock Cognito client
			mockCognitoClient := cognito.NewMockClient()

			controllerReconciler := &UserReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				Manager:        mockManager,
				UserPoolClient: mockCognitoClient,
			}

			_, err := controllerReconciler.Reconcile(ctx, mcreconcile.Request{
				ClusterName: "platform",
				Request:     reconcile.Request{NamespacedName: typeNamespacedName},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify user was created in Cognito
			cognitoUser, err := mockCognitoClient.GetUser(ctx, resourceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(cognitoUser.Username).To(Equal(resourceName))
		})
	})
})

// Unit tests using standard testing framework
func TestUserReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	if err := kcpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add kcpv1alpha1 scheme: %v", err)
	}

	userName := "test-user"
	userNamespace := "default"
	namespacedName := types.NamespacedName{Name: userName, Namespace: userNamespace}

	t.Run("cluster retrieval failure", func(t *testing.T) {
		mgr := &fakeManager{err: fmt.Errorf("cluster not found")}
		r := &UserReconciler{Scheme: scheme, Manager: mgr}
		_, err := r.Reconcile(context.Background(), mcreconcile.Request{
			ClusterName: "cluster1",
			Request:     reconcile.Request{NamespacedName: namespacedName},
		})
		if err == nil || err.Error() != "failed to get cluster: cluster not found" {
			t.Errorf("expected error from GetCluster, got %v", err)
		}
	})

	t.Run("resource not found", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		mgr := &fakeManager{cluster: &fakeCluster{client: fakeClient}, err: nil}
		r := &UserReconciler{Scheme: scheme, Manager: mgr}
		result, err := r.Reconcile(context.Background(), mcreconcile.Request{
			ClusterName: "cluster1",
			Request:     reconcile.Request{NamespacedName: namespacedName},
		})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != (ctrl.Result{}) {
			t.Errorf("expected empty result, got %v", result)
		}
	})

	t.Run("successful reconciliation", func(t *testing.T) {
		initialUser := &kcpv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userName,
				Namespace: userNamespace,
			},
			Spec: kcpv1alpha1.UserSpec{
				Email:   "test@example.com",
				Enabled: true,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initialUser).Build()
		mgr := &fakeManager{cluster: &fakeCluster{client: fakeClient}, err: nil}
		mockCognitoClient := cognito.NewMockClient()
		r := &UserReconciler{Scheme: scheme, Manager: mgr, UserPoolClient: mockCognitoClient}
		_, err := r.Reconcile(context.Background(), mcreconcile.Request{
			ClusterName: "cluster1",
			Request:     reconcile.Request{NamespacedName: namespacedName},
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		updatedUser := &kcpv1alpha1.User{}
		if err := fakeClient.Get(context.Background(), namespacedName, updatedUser); err != nil {
			t.Fatalf("failed to get updated user: %v", err)
		}
		ts, ok := updatedUser.Annotations["kcp.cogniteo.io/lastReconciledAt"]
		if !ok {
			t.Errorf("expected annotation lastReconciledAt, got %v", updatedUser.Annotations)
		} else {
			if _, err := time.Parse(time.RFC3339, ts); err != nil {
				t.Errorf("annotation lastReconciledAt not RFC3339: %v", err)
			}
		}

		// Verify user was created in Cognito
		cognitoUser, err := mockCognitoClient.GetUser(context.Background(), userName)
		if err != nil {
			t.Errorf("expected user to be created in Cognito, got error: %v", err)
		} else {
			if cognitoUser.Username != userName {
				t.Errorf("expected username %s, got %s", userName, cognitoUser.Username)
			}
			if cognitoUser.Email != "test@example.com" {
				t.Errorf("expected email test@example.com, got %s", cognitoUser.Email)
			}
			if !cognitoUser.Enabled {
				t.Errorf("expected user to be enabled")
			}
		}
	})
}
