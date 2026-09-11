package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistered(t *testing.T) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		metrics.HTTPRequestsTotal,
		metrics.HTTPRequestDuration,
		metrics.OrdersCreatedTotal,
		metrics.OrdersCancelledTotal,
		metrics.OrdersFailedTotal,
		metrics.InventoryReservationsTotal,
		metrics.InventoryReleasesTotal,
		metrics.InventoryUpdatesTotal,
		metrics.InventoryFailedTotal,
		metrics.WSConnectionsActive,
		metrics.WSMessagesSentTotal,
	)

	metrics.HTTPRequestsTotal.WithLabelValues("test-service", "GET", "/test", "200").Inc()
	metrics.OrdersCreatedTotal.WithLabelValues("test-service").Inc()
	metrics.WSConnectionsActive.WithLabelValues("test-service").Set(5)

	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.HTTPRequestsTotal.MustCurryWith(prometheus.Labels{
		"service": "test-service", "method": "GET", "path": "/test", "status": "200",
	})))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.OrdersCreatedTotal.WithLabelValues("test-service")))
	assert.Equal(t, float64(5), testutil.ToFloat64(metrics.WSConnectionsActive.WithLabelValues("test-service")))
}
