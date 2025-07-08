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
	"net/http"

	logr "github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
}

func (f *fakeManager) GetCluster(ctx context.Context, name string) (cluster.Cluster, error) {
	return f.cluster, nil
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
					// TODO(user): Specify other spec details if needed.
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
			}

			controllerReconciler := &UserReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Manager: mockManager,
			}

			_, err := controllerReconciler.Reconcile(ctx, mcreconcile.Request{
				ClusterName: "platform",
				Request:     reconcile.Request{NamespacedName: typeNamespacedName},
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
