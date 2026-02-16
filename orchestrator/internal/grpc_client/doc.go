// Package grpc_client provides a gRPC client for communicating with Python workers.
//
// The client implements all 3 RPC methods defined in transcription.proto:
// - Transcribe: Send transcription tasks to workers
// - DetectLanguage: Detect language from audio files
// - HealthCheck: Monitor worker health
//
// Features:
// - Connection pooling for efficient reuse
// - Retry logic with exponential backoff (3 retries max)
// - Configurable timeouts (5hr for transcribe, 5s for health)
// - Prometheus metrics for monitoring
// - Structured logging with logrus
//
// Usage:
//
//	metrics := grpc_client.NewClientMetrics()
//	client := grpc_client.NewClient(
//		5*time.Hour,  // transcribe timeout
//		5*time.Second, // health timeout
//		3,             // max retries
//		1*time.Second, // initial retry delay
//		metrics,
//		log,
//	)
//
//	// Transcribe a task
//	resp, err := client.Transcribe(ctx, "localhost:50051", task)
package grpc_client
