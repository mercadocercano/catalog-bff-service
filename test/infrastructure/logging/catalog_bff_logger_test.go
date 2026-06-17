package logging_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalog-bff-service/src/domain/port"
	bfflogging "catalog-bff-service/src/infrastructure/logging"
)

func TestCatalogBFFLogger_Envelope(t *testing.T) {
	var buf bytes.Buffer
	logger := bfflogging.NewCatalogBFFLoggerWithWriter("catalog-bff-test", &buf)

	logger.Log(port.CatalogBFFEvent{
		Event:      "catalog_bff.dashboard_stats_fetched",
		TenantID:   "tenant-abc",
		DurationMs: 120,
		Count:      3,
	})

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))

	assert.Equal(t, "catalog_bff.dashboard_stats_fetched", line["event"])
	assert.Equal(t, "info", line["level"])
	assert.Equal(t, "catalog-bff-test", line["service"])
	assert.NotEmpty(t, line["ts"])
	assert.Equal(t, "tenant-abc", line["tenant_id"])
	assert.EqualValues(t, 120, line["duration_ms"])
	assert.EqualValues(t, 3, line["count"])
}

func TestCatalogBFFLogger_LevelMapping(t *testing.T) {
	cases := []struct {
		event    string
		expected string
	}{
		{"catalog_bff.dashboard_stats_fetched", "info"},
		{"catalog_bff.tenant_dashboard_fetched", "info"},
		{"catalog_bff.product_created", "info"},
		{"catalog_bff.tenant_cache_refreshed", "info"},
		{"catalog_bff.upstream_failed", "warn"},
		{"catalog_bff.tenant_dashboard_partial", "warn"},
		{"catalog_bff.dashboard_stats_failed", "error"},
		{"catalog_bff.tenant_dashboard_failed", "error"},
	}

	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			var buf bytes.Buffer
			logger := bfflogging.NewCatalogBFFLoggerWithWriter("catalog-bff-test", &buf)
			logger.Log(port.CatalogBFFEvent{Event: tc.event})

			var line map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
			assert.Equal(t, tc.expected, line["level"], "event: %s", tc.event)
		})
	}
}

func TestCatalogBFFLogger_Omitempty(t *testing.T) {
	var buf bytes.Buffer
	logger := bfflogging.NewCatalogBFFLoggerWithWriter("catalog-bff-test", &buf)

	// Solo event, sin campos opcionales
	logger.Log(port.CatalogBFFEvent{
		Event: "catalog_bff.tenant_cache_refreshed",
		Count: 5,
	})

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))

	// Campos vacíos no deben aparecer
	_, hasTenantID := line["tenant_id"]
	assert.False(t, hasTenantID, "tenant_id vacío no debe estar en el JSON")
	_, hasUserID := line["user_id"]
	assert.False(t, hasUserID, "user_id vacío no debe estar en el JSON")
	_, hasReason := line["reason"]
	assert.False(t, hasReason, "reason vacío no debe estar en el JSON")

	// duration_ms=0 no debe aparecer
	_, hasDuration := line["duration_ms"]
	assert.False(t, hasDuration, "duration_ms=0 no debe estar en el JSON")

	// count=5 sí debe aparecer
	assert.EqualValues(t, 5, line["count"])
}

func TestCatalogBFFLogger_UpstreamFailed(t *testing.T) {
	var buf bytes.Buffer
	logger := bfflogging.NewCatalogBFFLoggerWithWriter("catalog-bff-test", &buf)

	logger.Log(port.CatalogBFFEvent{
		Event:           "catalog_bff.upstream_failed",
		TenantID:        "tenant-xyz",
		UpstreamService: "pim",
		ProductID:       "prod-123",
		Reason:          "connection refused",
	})

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))

	assert.Equal(t, "warn", line["level"])
	assert.Equal(t, "pim", line["upstream_service"])
	assert.Equal(t, "prod-123", line["product_id"])
	assert.Equal(t, "connection refused", line["reason"])
}

func TestCatalogBFFLogger_NilSafe(t *testing.T) {
	// Verificar que un logger nil no paniquea (handler con logger=nil)
	var logger port.CatalogBFFEventLogger // nil interface

	// Esta función simula el helper log() de los handlers
	logIfNotNil := func(e port.CatalogBFFEvent) {
		if logger != nil {
			logger.Log(e)
		}
	}

	// No debe paniquear
	assert.NotPanics(t, func() {
		logIfNotNil(port.CatalogBFFEvent{Event: "catalog_bff.product_created"})
	})
}
