package domain

import "os"

// GetTenantStockPolicy resuelve la política de stock para un tenant específico
// 
// IMPLEMENTACIÓN ACTUAL: Mock/hardcoded
// TODO: En el futuro, leer desde:
//   - Config Service
//   - Base de datos de configuración por tenant
//   - Feature flags
//
// Para testing, se puede forzar la policy con variable de entorno:
//   FORCE_STOCK_POLICY=IGNORE_STOCK
func GetTenantStockPolicy(tenantID string) StockPolicy {
	// Permitir override por env var para testing
	if forcedPolicy := os.Getenv("FORCE_STOCK_POLICY"); forcedPolicy != "" {
		switch forcedPolicy {
		case "IGNORE_STOCK":
			return IgnoreStock
		case "REQUIRE_STOCK":
			return RequireStock
		}
	}

	// TODO: Implementar lógica real de resolución por tenant
	// Por ahora, default conservador: REQUIRE_STOCK
	return RequireStock
}
