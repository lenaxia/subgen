package discovery_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/discovery"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// TestKubernetesDiscovery_GetWorkers_Success tests successful worker discovery from endpoints
func TestKubernetesDiscovery_GetWorkers_Success(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create fake K8s clientset with test endpoints
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.1",
						TargetRef: &corev1.ObjectReference{
							Name: "worker-pod-1",
						},
					},
					{
						IP: "10.0.0.2",
						TargetRef: &corev1.ObjectReference{
							Name: "worker-pod-2",
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port: 50051,
					},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(endpoints)

	// Create discovery instance with mocked client
	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	require.NoError(t, err)
	assert.NotNil(t, workers)
	assert.Equal(t, 2, len(workers), "Should discover 2 workers")

	// Verify worker addresses
	assert.Contains(t, []string{"10.0.0.1:50051", "10.0.0.2:50051"},
		workers[0].Address, "Worker 0 should have valid address")
	assert.Contains(t, []string{"10.0.0.1:50051", "10.0.0.2:50051"},
		workers[1].Address, "Worker 1 should have valid address")

	// Verify pod names are set as IDs
	assert.Contains(t, []string{"worker-pod-1", "worker-pod-2"},
		workers[0].ID, "Worker ID should be pod name")
}

// TestKubernetesDiscovery_GetWorkers_NotFound tests graceful handling when endpoints don't exist
func TestKubernetesDiscovery_GetWorkers_NotFound(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create empty clientset (no endpoints)
	clientset := fake.NewSimpleClientset()

	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	// Should NOT return error - empty slice is valid when no workers deployed
	require.NoError(t, err, "GetWorkers should not error when endpoints not found")
	assert.NotNil(t, workers, "Should return non-nil slice")
	assert.Equal(t, 0, len(workers), "Should return empty slice when no endpoints")
}

// TestKubernetesDiscovery_GetWorkers_EmptySubsets tests when endpoints exist but no ready pods
func TestKubernetesDiscovery_GetWorkers_EmptySubsets(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create endpoints with empty subsets (no ready pods)
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{}, // Empty subsets
	}

	clientset := fake.NewSimpleClientset(endpoints)
	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	require.NoError(t, err)
	assert.NotNil(t, workers)
	assert.Equal(t, 0, len(workers), "Should return empty slice when no ready pods")
}

// TestKubernetesDiscovery_GetWorkers_MultipleSubsets tests multiple endpoint subsets
func TestKubernetesDiscovery_GetWorkers_MultipleSubsets(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create endpoints with multiple subsets (happens with rolling updates)
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.1",
						TargetRef: &corev1.ObjectReference{
							Name: "worker-pod-1",
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{Port: 50051},
				},
			},
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.2",
						TargetRef: &corev1.ObjectReference{
							Name: "worker-pod-2",
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{Port: 50051},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(endpoints)
	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, len(workers), "Should discover workers from both subsets")
}

// TestKubernetesDiscovery_GetWorkers_RBACForbidden tests RBAC permission denied
func TestKubernetesDiscovery_GetWorkers_RBACForbidden(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create clientset that returns Forbidden error
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("get", "endpoints", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, k8serrors.NewForbidden(
			schema.GroupResource{Group: "", Resource: "endpoints"},
			"worker",
			nil,
		)
	})

	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	// RBAC errors SHOULD return error (unlike NotFound)
	require.Error(t, err, "Should return error for RBAC permission denied")
	assert.Contains(t, err.Error(), "RBAC", "Error message should mention RBAC")
	assert.Contains(t, err.Error(), "rbac.yaml", "Error message should hint at solution")
	assert.NotNil(t, workers)
	assert.Equal(t, 0, len(workers), "Should return empty slice on RBAC error")
}

// TestKubernetesDiscovery_GetWorkers_DifferentNamespace tests namespace isolation
func TestKubernetesDiscovery_GetWorkers_DifferentNamespace(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create endpoints in "production" namespace
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker",
			Namespace: "production",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.1",
						TargetRef: &corev1.ObjectReference{
							Name: "worker-pod-1",
						},
					},
				},
				Ports: []corev1.EndpointPort{
					{Port: 50051},
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(endpoints)

	// Discovery is configured for "production" namespace
	disc := createTestKubernetesDiscovery(clientset, "production", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workers, err := disc.GetWorkers(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, len(workers), "Should discover worker in correct namespace")
}

// TestNewKubernetesDiscovery_OutsideCluster tests error handling when not in K8s cluster
func TestNewKubernetesDiscovery_OutsideCluster(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// This test should be skipped in CI since we can't mock InClusterConfig failure
	// We're testing the actual NewKubernetesDiscovery, not the test helper
	t.Skip("Requires running outside K8s cluster - manual test only")

	// If run manually outside K8s:
	// disc, err := discovery.NewKubernetesDiscovery("default", "worker", 50051, log)
	//
	// require.Error(t, err, "Should error when running outside K8s cluster")
	// assert.Contains(t, err.Error(), "kubernetes discovery requires running inside", "Error should be helpful")
	// assert.Contains(t, err.Error(), "WORKER_DISCOVERY=localhost", "Error should mention localhost mode")
	// assert.Nil(t, disc)
}

// Helper function to create KubernetesDiscovery with fake clientset for testing
// This bypasses NewKubernetesDiscovery() which would call rest.InClusterConfig()
func createTestKubernetesDiscovery(clientset *fake.Clientset, namespace, service string, port int32, log *logrus.Logger) *discovery.KubernetesDiscovery {
	// Cast to kubernetes.Interface since fake.Clientset implements it
	return discovery.NewKubernetesDiscoveryWithClient(clientset, namespace, service, port, log)
}

// ============================================================================
// Watch API Tests (STORY_03)
// ============================================================================

// TestKubernetesDiscovery_Watch_Success tests successful watch establishment
func TestKubernetesDiscovery_Watch_Success(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	// Create fake clientset
	clientset := fake.NewSimpleClientset()

	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	eventCh, err := disc.Watch(ctx)

	require.NoError(t, err, "Watch should not return error")
	require.NotNil(t, eventCh, "Event channel should not be nil")

	// Watch starts in background goroutine
	// Channel should stay open until context cancelled
	select {
	case _, ok := <-eventCh:
		if ok {
			// Got an event (fine for real K8s, unexpected for fake)
			t.Log("Received event from watch (OK)")
		} else {
			// Channel closed (expected when watch ends)
			t.Log("Watch channel closed (OK)")
		}
	case <-time.After(100 * time.Millisecond):
		// No events yet (expected - no changes)
		t.Log("No events received yet (OK)")
	}
}

// TestKubernetesDiscovery_Watch_ContextCancelled tests watch stops on context cancellation
func TestKubernetesDiscovery_Watch_ContextCancelled(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	clientset := fake.NewSimpleClientset()
	disc := createTestKubernetesDiscovery(clientset, "default", "worker", 50051, log)

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, err := disc.Watch(ctx)
	require.NoError(t, err)
	require.NotNil(t, eventCh)

	// Cancel context immediately
	cancel()

	// Channel should close within reasonable time
	select {
	case _, ok := <-eventCh:
		assert.False(t, ok, "Channel should be closed after context cancelled")
	case <-time.After(1 * time.Second):
		t.Fatal("Channel did not close after context cancellation")
	}
}
