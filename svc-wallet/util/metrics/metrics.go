package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts every HTTP request by method, path, and status code.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks the latency distribution of HTTP requests.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency distribution",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// HTTPRequestBytesRead tracks the size of incoming request bodies.
	HTTPRequestBytesRead = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_bytes_read",
			Help:    "Size of HTTP request bodies in bytes",
			Buckets: []float64{0, 100, 500, 1000, 5000, 10000, 50000},
		},
		[]string{"method", "path"},
	)

	// HTTPResponseBytesWritten tracks the size of HTTP response bodies.
	HTTPResponseBytesWritten = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_bytes_written",
			Help:    "Size of HTTP response bodies in bytes",
			Buckets: []float64{0, 100, 500, 1000, 5000, 10000, 50000},
		},
		[]string{"method", "path"},
	)

	// MongoOperationDuration tracks MongoDB query latency by operation and collection.
	MongoOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mongodb_operation_duration_seconds",
			Help:    "MongoDB operation latency",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "collection"},
	)

	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "grpc_requests_total", Help: "Total number of gRPC requests"},
		[]string{"method", "code"},
	)

	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "grpc_request_duration_seconds", Help: "gRPC request latency", Buckets: prometheus.DefBuckets},
		[]string{"method", "code"},
	)

	RedisOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "redis_operations_total", Help: "Redis operations by operation and outcome"},
		[]string{"operation", "outcome"},
	)

	KafkaMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{Name: "kafka_messages_total", Help: "Kafka messages by processing outcome"},
		[]string{"outcome"},
	)

	KafkaRetriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{Name: "kafka_retries_total", Help: "Total Kafka message processing retries"},
	)

	KafkaProcessDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{Name: "kafka_process_duration_seconds", Help: "Kafka message processing latency", Buckets: prometheus.DefBuckets},
		[]string{"outcome"},
	)
)
