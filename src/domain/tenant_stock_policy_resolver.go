package domain

import (
	"context"
	"os"

	"catalog-bff-service/src/domain/port"
)

// TenantConfigClient define el contrato para obtener configuraciones de tenant
// (interfaz definida aquí para evitar dependencia circular)
type TenantConfigClient interface {
	GetConfig(ctx context.Context, tenantID string, key string) (string, error)
}

// TenantStockPolicyResolver resuelve la Stock Policy de un tenant
// consultando el tenant-service
type TenantStockPolicyResolver struct {
	client    TenantConfigClient
	eventLog  port.CatalogBFFEventLogger
}

// NewTenantStockPolicyResolver crea una nueva instancia del resolver
func NewTenantStockPolicyResolver(client TenantConfigClient) *TenantStockPolicyResolver {
	return &TenantStockPolicyResolver{
		client: client,
	}
}

// NewTenantStockPolicyResolverWithLogger crea una instancia del resolver con logger canónico (ADR-001)
func NewTenantStockPolicyResolverWithLogger(client TenantConfigClient, eventLog port.CatalogBFFEventLogger) *TenantStockPolicyResolver {
	return &TenantStockPolicyResolver{
		client:   client,
		eventLog: eventLog,
	}
}

// logEvent emite un evento canónico si el logger está configurado; no-op en caso contrario.
func (r *TenantStockPolicyResolver) logEvent(e port.CatalogBFFEvent) {
	if r.eventLog != nil {
		r.eventLog.Log(e)
	}
}

// Resolve obtiene la Stock Policy del tenant desde tenant-service
//
// Comportamiento:
// - Consulta tenant-service por la key "catalog.stock_policy"
// - Si responde con valor válido, lo convierte a StockPolicy
// - Si falla o no existe, devuelve RequireStock (fallback seguro)
// - Nunca falla: siempre devuelve una policy válida
func (r *TenantStockPolicyResolver) Resolve(ctx context.Context, tenantID string) StockPolicy {
	// Permitir override por env var para testing/desarrollo
	if forcedPolicy := os.Getenv("FORCE_STOCK_POLICY"); forcedPolicy != "" {
		switch forcedPolicy {
		case "IGNORE_STOCK":
			r.logEvent(port.CatalogBFFEvent{
				Event:    "catalog_bff.stock_policy_resolved",
				TenantID: tenantID,
				Policy:   "IGNORE_STOCK",
				Reason:   "forced_by_env",
			})
			return IgnoreStock
		case "REQUIRE_STOCK":
			r.logEvent(port.CatalogBFFEvent{
				Event:    "catalog_bff.stock_policy_resolved",
				TenantID: tenantID,
				Policy:   "REQUIRE_STOCK",
				Reason:   "forced_by_env",
			})
			return RequireStock
		}
	}

	// Si no hay client configurado, fallback
	if r.client == nil {
		r.logEvent(port.CatalogBFFEvent{
			Event:    "catalog_bff.stock_policy_resolved",
			TenantID: tenantID,
			Policy:   "REQUIRE_STOCK",
			Reason:   "no_client_configured",
		})
		return RequireStock
	}

	// Consultar tenant-service
	value, err := r.client.GetConfig(ctx, tenantID, "catalog.stock_policy")
	if err != nil {
		// Error al consultar: loggear y usar fallback
		r.logEvent(port.CatalogBFFEvent{
			Event:    "catalog_bff.stock_policy_fetch_failed",
			TenantID: tenantID,
			Policy:   "REQUIRE_STOCK",
			Reason:   err.Error(),
		})
		return RequireStock
	}

	// Si no existe configuración (value vacío), usar fallback
	if value == "" {
		r.logEvent(port.CatalogBFFEvent{
			Event:    "catalog_bff.stock_policy_resolved",
			TenantID: tenantID,
			Policy:   "REQUIRE_STOCK",
			Reason:   "no_policy_configured",
		})
		return RequireStock
	}

	// Mapear valor a StockPolicy
	switch value {
	case "IGNORE_STOCK":
		r.logEvent(port.CatalogBFFEvent{
			Event:    "catalog_bff.stock_policy_resolved",
			TenantID: tenantID,
			Policy:   "IGNORE_STOCK",
			Reason:   "tenant_config",
		})
		return IgnoreStock
	case "REQUIRE_STOCK", "VALIDATE_STOCK":
		r.logEvent(port.CatalogBFFEvent{
			Event:    "catalog_bff.stock_policy_resolved",
			TenantID: tenantID,
			Policy:   "REQUIRE_STOCK",
			Reason:   "tenant_config",
		})
		return RequireStock
	default:
		// Valor desconocido: loggear warning y usar fallback
		r.logEvent(port.CatalogBFFEvent{
			Event:    "catalog_bff.stock_policy_resolved",
			TenantID: tenantID,
			Policy:   "REQUIRE_STOCK",
			Reason:   "unknown_value:" + value,
		})
		return RequireStock
	}
}
