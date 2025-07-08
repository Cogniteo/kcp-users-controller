package controller

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	logr "github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	kcpv1alpha1 "piotrjanik.dev/users/api/v1alpha1"
)

type testCluster struct {
	client client.Client
}

func (f *testCluster) GetClient() client.Client                             { return f.client }
func (f *testCluster) GetAPIReader() client.Reader                          { return f.client }
func (f *testCluster) GetHTTPClient() *http.Client                          { return nil }
func (f *testCluster) GetConfig() *rest.Config                              { return nil }
func (f *testCluster) GetCache() cache.Cache                                { return nil }
func (f *testCluster) GetScheme() *runtime.Scheme                           { return nil }
func (f *testCluster) GetFieldIndexer() client.FieldIndexer                 { return nil }
func (f *testCluster) GetEventRecorderFor(name string) record.EventRecorder { return nil }
func (f *testCluster) GetRESTMapper() meta.RESTMapper                       { return nil }
func (f *testCluster) Start(ctx context.Context) error                      { return nil }

type testManager struct {
	cluster cluster.Cluster
	err     error
}

func (f *testManager) GetCluster(ctx context.Context, name string) (cluster.Cluster, error) {
	return f.cluster, f.err
}

// Minimal Manager interface implementation
func (f *testManager) Add(runnable mcmanager.Runnable) error { return nil }
func (f *testManager) Elected() <-chan struct{}              { return nil }
func (f *testManager) AddMetricsServerExtraHandler(path string, handler http.Handler) error {
	return nil
}
func (f *testManager) AddHealthzCheck(name string, check healthz.Checker) error { return nil }
func (f *testManager) AddReadyzCheck(name string, check healthz.Checker) error  { return nil }
func (f *testManager) Start(ctx context.Context) error                          { return nil }
func (f *testManager) GetWebhookServer() webhook.Server                         { return nil }
func (f *testManager) GetLogger() logr.Logger                                   { return logr.Logger{} }
func (f *testManager) GetControllerOptions() config.Controller                  { return config.Controller{} }
func (f *testManager) ClusterFromContext(ctx context.Context) (cluster.Cluster, error) {
	return nil, nil
}
func (f *testManager) GetManager(ctx context.Context, clusterName string) (manager.Manager, error) {
	return nil, nil
}
func (f *testManager) GetLocalManager() manager.Manager     { return nil }
func (f *testManager) GetProvider() multicluster.Provider   { return nil }
func (f *testManager) GetFieldIndexer() client.FieldIndexer { return nil }
func (f *testManager) Engage(ctx context.Context, clusterName string, cluster cluster.Cluster) error {
	return nil
}

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
		mgr := &testManager{err: fmt.Errorf("cluster not found")}
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
		mgr := &testManager{cluster: &testCluster{client: fakeClient}, err: nil}
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
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initialUser).Build()
		mgr := &testManager{cluster: &testCluster{client: fakeClient}, err: nil}
		r := &UserReconciler{Scheme: scheme, Manager: mgr}
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
	})
}
