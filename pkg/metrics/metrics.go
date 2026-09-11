package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)

	OrdersCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total number of orders created",
		},
		[]string{"service"},
	)

	OrdersCancelledTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_cancelled_total",
			Help: "Total number of orders cancelled",
		},
		[]string{"service"},
	)

	OrdersFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_failed_total",
			Help: "Total number of failed order operations",
		},
		[]string{"service", "reason"},
	)

	InventoryReservationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_reservations_total",
			Help: "Total number of inventory reservations",
		},
		[]string{"service", "status"},
	)

	InventoryReleasesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_releases_total",
			Help: "Total number of inventory reservation releases",
		},
		[]string{"service"},
	)

	InventoryUpdatesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_updates_total",
			Help: "Total number of inventory stock updates",
		},
		[]string{"service"},
	)

	InventoryFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inventory_failed_total",
			Help: "Total number of failed inventory operations",
		},
		[]string{"service", "reason"},
	)

	WSConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ws_connections_active",
			Help: "Current number of active WebSocket connections",
		},
		[]string{"service"},
	)

	WSConnectionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_connections_total",
			Help: "Total number of WebSocket connections established",
		},
		[]string{"service"},
	)

	WSMessagesSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ws_messages_sent_total",
			Help: "Total number of WebSocket messages sent",
		},
		[]string{"service", "channel"},
	)

	KafkaMessagesProducedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_produced_total",
			Help: "Total number of Kafka messages produced",
		},
		[]string{"service", "topic"},
	)

	KafkaMessagesConsumedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_consumed_total",
			Help: "Total number of Kafka messages consumed",
		},
		[]string{"service", "topic"},
	)

	KafkaProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kafka_processing_duration_seconds",
			Help:    "Kafka message processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "topic"},
	)
)
