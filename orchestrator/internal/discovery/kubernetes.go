package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
)

// KubernetesDiscovery implements WorkerDiscovery for K8s worker pods
type KubernetesDiscovery struct {
	client    kubernetes.Interface // Use interface for testability
	namespace string
	service   string
	port      int32
	log       *logrus.Logger
}

// NewKubernetesDiscovery creates K8s worker discovery
func NewKubernetesDiscovery(namespace, service string, port int32, log *logrus.Logger) (*KubernetesDiscovery, error) {
	// Get in-cluster K8s config
	config, err := rest.InClusterConfig()
	if err != nil {
		// CRITICAL: Provide helpful error for Docker deployments
		if strings.Contains(err.Error(), "unable to load in-cluster configuration") {
			return nil, fmt.Errorf(
				"kubernetes discovery requires running inside a Kubernetes cluster. "+
					"For Docker Compose deployments, use WORKER_DISCOVERY=localhost (or omit, it's the default). "+
					"Original error: %w", err)
		}
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	// Create K8s clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s client: %w", err)
	}

	log.Info("Kubernetes discovery initialized successfully (running in K8s cluster)")

	return &KubernetesDiscovery{
		client:    clientset,
		namespace: namespace,
		service:   service,
		port:      port,
		log:       log,
	}, nil
}

// GetWorkers discovers all worker pods via K8s Endpoints API
func (d *KubernetesDiscovery) GetWorkers(ctx context.Context) ([]Worker, error) {
	// Get Endpoints object for worker service
	endpoints, err := d.client.CoreV1().Endpoints(d.namespace).Get(
		ctx, d.service, metav1.GetOptions{},
	)
	if err != nil {
		// Handle errors gracefully
		if k8sErrors.IsNotFound(err) {
			d.log.WithFields(logrus.Fields{
				"namespace": d.namespace,
				"service":   d.service,
			}).Warn("Worker service endpoints not found - workers may not be deployed yet")
			return []Worker{}, nil // Return empty slice, not error
		}

		if k8sErrors.IsForbidden(err) {
			d.log.WithFields(logrus.Fields{
				"namespace": d.namespace,
				"service":   d.service,
				"error":     err,
			}).Error("RBAC permission denied - check ServiceAccount, Role, and RoleBinding")
			return []Worker{}, fmt.Errorf("RBAC permission denied (apply deploy/rbac.yaml): %w", err)
		}

		d.log.WithError(err).Error("Failed to get endpoints from K8s API")
		return []Worker{}, fmt.Errorf("failed to get endpoints: %w", err)
	}

	// Check if any pods are ready
	if len(endpoints.Subsets) == 0 {
		d.log.WithFields(logrus.Fields{
			"namespace": d.namespace,
			"service":   d.service,
		}).Debug("Endpoints exist but no ready pods yet")
		return []Worker{}, nil
	}

	// Parse worker IPs from endpoint subsets
	var workers []Worker
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			workerAddr := fmt.Sprintf("%s:%d", addr.IP, d.port)

			// Perform health check via gRPC
			healthy, activeJobs := d.checkWorkerHealth(ctx, workerAddr)

			worker := Worker{
				ID:       addr.TargetRef.Name, // Pod name
				Address:  workerAddr,
				Healthy:  healthy,
				Active:   activeJobs,
				LastSeen: time.Now(),
			}

			workers = append(workers, worker)

			d.log.WithFields(logrus.Fields{
				"worker_id": worker.ID,
				"address":   worker.Address,
				"healthy":   worker.Healthy,
				"active":    worker.Active,
			}).Debug("Discovered worker from K8s")
		}
	}

	d.log.WithFields(logrus.Fields{
		"count":     len(workers),
		"namespace": d.namespace,
		"service":   d.service,
	}).Info("Discovered workers from K8s Endpoints API")

	return workers, nil
}

// checkWorkerHealth performs gRPC health check
func (d *KubernetesDiscovery) checkWorkerHealth(ctx context.Context, address string) (bool, int32) {
	// Create timeout context for health check (5 seconds)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to connect to worker
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		// Connection failed - worker unhealthy
		d.log.WithFields(logrus.Fields{
			"address": address,
			"error":   err,
		}).Debug("Worker health check failed (connection error)")
		return false, 0
	}
	defer conn.Close()

	// Call HealthCheck RPC
	client := pb.NewTranscriptionServiceClient(conn)
	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		// Health check RPC failed
		d.log.WithFields(logrus.Fields{
			"address": address,
			"error":   err,
		}).Debug("Worker health check failed (RPC error)")
		return false, 0
	}

	// Check response status
	healthy := resp.Status == pb.HealthCheckResponse_HEALTHY
	activeJobs := resp.JobsActive

	d.log.WithFields(logrus.Fields{
		"address":     address,
		"status":      resp.Status,
		"healthy":     healthy,
		"active_jobs": activeJobs,
	}).Debug("Worker health check completed")

	return healthy, activeJobs
}

// Watch monitors K8s endpoints for worker changes
// Returns a channel that emits WorkerEvent when workers are added/removed/updated
// The watch runs in a background goroutine and automatically handles reconnection
func (d *KubernetesDiscovery) Watch(ctx context.Context) (<-chan WorkerEvent, error) {
	d.log.Info("Starting Kubernetes endpoints watch for dynamic worker discovery")

	// Create K8s watch on Endpoints resource
	watcher, err := d.client.CoreV1().Endpoints(d.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", d.service),
	})
	if err != nil {
		d.log.WithError(err).Error("Failed to create Kubernetes watch")
		return nil, fmt.Errorf("failed to watch endpoints: %w", err)
	}

	// Create output channel for worker events
	// Buffer size of 100 to handle rapid scaling (e.g., 50 workers added at once)
	ch := make(chan WorkerEvent, 100)

	// Start background goroutine to process watch events
	go func() {
		defer close(ch)
		defer watcher.Stop()

		d.log.Info("Kubernetes watch established successfully")

		// Keep track of known workers to detect additions/removals
		knownWorkers := make(map[string]Worker) // Map of ID -> Worker

		for {
			select {
			case <-ctx.Done():
				d.log.Info("Watch context cancelled, stopping Kubernetes watch")
				return

			case event, ok := <-watcher.ResultChan():
				if !ok {
					// Watch channel closed (disconnection)
					d.log.Warn("Kubernetes watch channel closed, will reconnect")
					// Return to trigger reconnection in pool.go watchLoop()
					return
				}

				// Process the watch event
				d.handleEndpointEvent(ctx, event, ch, knownWorkers)
			}
		}
	}()

	return ch, nil
}

// handleEndpointEvent processes a single Kubernetes watch event
func (d *KubernetesDiscovery) handleEndpointEvent(
	ctx context.Context,
	event watch.Event,
	ch chan<- WorkerEvent,
	knownWorkers map[string]Worker,
) {
	switch event.Type {
	case watch.Added, watch.Modified:
		// Parse endpoint object
		endpoints, ok := event.Object.(*corev1.Endpoints)
		if !ok {
			d.log.Error("Unexpected object type in watch event (expected Endpoints)")
			return
		}

		d.log.WithFields(logrus.Fields{
			"event_type": event.Type,
			"subsets":    len(endpoints.Subsets),
		}).Debug("Processing endpoint watch event")

		// Parse current workers from endpoints
		currentWorkers := d.parseWorkers(ctx, endpoints)

		// Build set of current worker IDs for comparison
		currentIDs := make(map[string]bool)
		for _, w := range currentWorkers {
			currentIDs[w.ID] = true
		}

		// Detect new or updated workers
		for _, worker := range currentWorkers {
			if existing, found := knownWorkers[worker.ID]; found {
				// Worker exists - check if health status changed
				if existing.Healthy != worker.Healthy {
					d.log.WithFields(logrus.Fields{
						"worker_id": worker.ID,
						"address":   worker.Address,
						"healthy":   worker.Healthy,
					}).Info("Worker health status changed")

					WorkerWatchEventsTotal.WithLabelValues("updated").Inc()
					ch <- WorkerEvent{
						Type:   EventTypeUpdated,
						Worker: worker,
					}
				}
			} else {
				// New worker discovered
				d.log.WithFields(logrus.Fields{
					"worker_id": worker.ID,
					"address":   worker.Address,
				}).Info("New worker discovered")

				WorkerWatchEventsTotal.WithLabelValues("added").Inc()
				ch <- WorkerEvent{
					Type:   EventTypeAdded,
					Worker: worker,
				}
			}

			// Update known workers map
			knownWorkers[worker.ID] = worker
		}

		// Detect removed workers
		for id, worker := range knownWorkers {
			if !currentIDs[id] {
				// Worker no longer in endpoints
				d.log.WithFields(logrus.Fields{
					"worker_id": id,
					"address":   worker.Address,
				}).Info("Worker removed from endpoints")

				WorkerWatchEventsTotal.WithLabelValues("removed").Inc()
				ch <- WorkerEvent{
					Type:   EventTypeRemoved,
					Worker: worker,
				}

				// Remove from known workers
				delete(knownWorkers, id)
			}
		}

	case watch.Deleted:
		// Entire Endpoints object deleted (all workers removed)
		d.log.Warn("Endpoints object deleted - all workers removed")

		// Send remove events for all known workers
		for id, worker := range knownWorkers {
			WorkerWatchEventsTotal.WithLabelValues("removed").Inc()
			ch <- WorkerEvent{
				Type:   EventTypeRemoved,
				Worker: worker,
			}
			delete(knownWorkers, id)
		}

	case watch.Error:
		// Error event from Kubernetes API
		WorkerWatchErrorsTotal.Inc()
		WorkerWatchEventsTotal.WithLabelValues("error").Inc()

		status, ok := event.Object.(*metav1.Status)
		if ok {
			d.log.WithFields(logrus.Fields{
				"reason":  status.Reason,
				"message": status.Message,
				"code":    status.Code,
			}).Error("Kubernetes watch error event")
		} else {
			d.log.WithField("object", event.Object).Error("Kubernetes watch error event (unknown type)")
		}

	default:
		d.log.WithField("event_type", event.Type).Warn("Unknown Kubernetes watch event type")
	}
}

// parseWorkers extracts worker information from Endpoints object
// This is used by Watch events and skips health checks for performance
// Health checks are done separately by the periodic refresh loop
func (d *KubernetesDiscovery) parseWorkers(ctx context.Context, endpoints *corev1.Endpoints) []Worker {
	var workers []Worker

	// Check if any pods are ready
	if len(endpoints.Subsets) == 0 {
		return workers // Empty slice
	}

	// Parse worker IPs from endpoint subsets
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			workerAddr := fmt.Sprintf("%s:%d", addr.IP, d.port)

			// Use pod name as worker ID (from TargetRef)
			workerID := workerAddr // Default to address
			if addr.TargetRef != nil && addr.TargetRef.Name != "" {
				workerID = addr.TargetRef.Name
			}

			worker := Worker{
				ID:       workerID,
				Address:  workerAddr,
				Healthy:  true, // Assume healthy - health check done by periodic refresh
				Active:   0,    // Active job count updated by periodic refresh
				LastSeen: time.Now(),
			}

			workers = append(workers, worker)
		}
	}

	return workers
}

// NewKubernetesDiscoveryWithClient creates K8s discovery with custom client (for testing)
// This allows injecting fake clientsets in unit tests without calling rest.InClusterConfig()
func NewKubernetesDiscoveryWithClient(client kubernetes.Interface, namespace, service string, port int32, log *logrus.Logger) *KubernetesDiscovery {
	return &KubernetesDiscovery{
		client:    client,
		namespace: namespace,
		service:   service,
		port:      port,
		log:       log,
	}
}
