package domain

import (
	"context"
	"log"
	"os"
)

// TenantConfigClient define el contrato para obtener configuraciones de tenant
// (interfaz definida aquí para evitar dependencia circular)
type TenantConfigClient interface {
	GetConfig(ctx context.Context, tenantID string, key string) (string, error)
}

// TenantStockPolicyResolver resuelve la Stock Policy de un tenant
// consultando el tenant-service
type TenantStockPolicyResolver struct {
	client TenantConfigClient
}

// NewTenantStockPolicyResolver crea una nueva instancia del resolver
func NewTenantStockPolicyResolver(client TenantConfigClient) *TenantStockPolicyResolver {
	return &TenantStockPolicyResolver{
		client: client,
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
		log.Printf("[TenantStockPolicyResolver] Using forced policy from env: %s", forcedPolicy)
		switch forcedPolicy {
		case "IGNORE_STOCK":
			return IgnoreStock
		case "REQUIRE_STOCK":
			return RequireStock
		}
	}

	// Si no hay client configurado, fallback
	if r.client == nil {
		log.Printf("[TenantStockPolicyResolver] No tenant client configured, using default: REQUIRE_STOCK")
		return RequireStock
	}

	// Consultar tenant-service
	value, err := r.client.GetConfig(ctx, tenantID, "catalog.stock_policy")
	if err != nil {
		// Error al consultar: loggear y usar fallback
		log.Printf("[TenantStockPolicyResolver] Error fetching policy for tenant %s: %v. Using fallback: REQUIRE_STOCK", tenantID, err)
		return RequireStock
	}

	// Si no existe configuración (value vacío), usar fallback
	if value == "" {
		log.Printf("[TenantStockPolicyResolver] No policy configured for tenant %s. Using fallback: REQUIRE_STOCK", tenantID)
		return RequireStock
	}

	// Mapear valor a StockPolicy
	switch value {
	case "IGNORE_STOCK":
		log.Printf("[TenantStockPolicyResolver] Tenant %s policy: IGNORE_STOCK", tenantID)
		return IgnoreStock
	case "REQUIRE_STOCK", "VALIDATE_STOCK":
		log.Printf("[TenantStockPolicyResolver] Tenant %s policy: REQUIRE_STOCK", tenantID)
		return RequireStock
	default:
		// Valor desconocido: loggear warning y usar fallback
		log.Printf("[TenantStockPolicyResolver] Unknown policy value '%s' for tenant %s. Using fallback: REQUIRE_STOCK", value, tenantID)
		return RequireStock
	}
}
