package port

// CatalogBFFEvent es el payload canónico para eventos de composición/agregación del
// catalog-bff (ADR-001). Este BFF no tiene dominio de negocio propio: sus eventos son
// outcomes de orquestación (composición exitosa, fallo de upstream, degradación).
//
// Campos flat, named. Los nombres comunes (tenant_id, user_id) son idénticos al resto
// de la flota para que el LogQL cross-service funcione.
type CatalogBFFEvent struct {
	// Event sigue la convención <domain>.<action>_<result>, ej: "catalog_bff.dashboard_stats_fetched"
	Event string

	// Campos de contexto — idénticos a la flota
	TenantID string
	UserID   string

	// upstream_service identifica qué servicio upstream fue invocado (pim, stock, iam, tenant)
	UpstreamService string

	// duration_ms latencia de la composición o del upstream call (0 = no aplica)
	DurationMs int64

	// Reason describe el error en eventos de fallo; vacío en success
	Reason string

	// product_id para eventos de creación/actualización de producto vía backoffice
	ProductID string

	// count es un entero de propósito general (ej: variantes creadas, tenants en cache)
	Count int

	// policy es la stock policy resuelta para el tenant (REQUIRE_STOCK / IGNORE_STOCK)
	Policy string
}

// CatalogBFFEventLogger es el puerto para emitir eventos canónicos del catalog-bff.
// Los handlers y servicios dependen de esta interfaz; el adapter (JSON a stdout) la implementa.
type CatalogBFFEventLogger interface {
	Log(e CatalogBFFEvent)
}
