package logging

import (
	"io"

	"catalog-bff-service/src/domain/port"

	sharedlog "github.com/hornosg/go-shared/infrastructure/logging"
)

// CatalogBFFLogger implementa port.CatalogBFFEventLogger emitiendo una línea JSON canónica
// (ADR-001) por evento, delegando el envelope (ts/level/service/event + campos flat omitempty)
// en go-shared CanonicalLogger (>= v0.8.0). El mapeo struct→fields y las reglas de nivel por
// evento viven acá; el formato canónico es compartido por la flota.
type CatalogBFFLogger struct {
	canonical *sharedlog.CanonicalLogger
}

// NewCatalogBFFLogger crea el adapter escribiendo a stdout. El service se fija acá, nunca por-call.
func NewCatalogBFFLogger(service string) *CatalogBFFLogger {
	return &CatalogBFFLogger{canonical: sharedlog.NewCanonicalLogger(service)}
}

// NewCatalogBFFLoggerWithWriter permite inyectar un io.Writer (tests).
func NewCatalogBFFLoggerWithWriter(service string, w io.Writer) *CatalogBFFLogger {
	return &CatalogBFFLogger{canonical: sharedlog.NewCanonicalLoggerWithWriter(service, w)}
}

// levelFor aplica las reglas de nivel del ADR-001 por tipo de evento del catalog-bff.
// info  → flujo normal / composición completada
// warn  → degradación recuperable / upstream parcialmente falla
// error → fallo de composición que impide respuesta al cliente
func levelFor(event string) string {
	switch event {
	case "catalog_bff.dashboard_stats_fetched",
		"catalog_bff.tenant_dashboard_fetched",
		"catalog_bff.product_created",
		"catalog_bff.tenant_cache_refreshed":
		return "info"
	case "catalog_bff.upstream_failed",
		"catalog_bff.tenant_dashboard_partial":
		return "warn"
	case "catalog_bff.dashboard_stats_failed",
		"catalog_bff.tenant_dashboard_failed":
		return "error"
	default:
		return "info"
	}
}

// Log emite una línea JSON canónica a stdout (o writer inyectado).
func (l *CatalogBFFLogger) Log(e port.CatalogBFFEvent) {
	fields := map[string]any{
		"tenant_id":        e.TenantID,
		"user_id":          e.UserID,
		"upstream_service": e.UpstreamService,
		"reason":           e.Reason,
		"product_id":       e.ProductID,
	}
	if e.DurationMs > 0 {
		fields["duration_ms"] = e.DurationMs
	}
	if e.Count > 0 {
		fields["count"] = e.Count
	}
	l.canonical.Emit(levelFor(e.Event), e.Event, fields)
}
