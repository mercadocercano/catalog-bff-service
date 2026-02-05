package domain

// StockPolicy define cómo se determina la vendibilidad de una variante
type StockPolicy string

const (
	// RequireStock: solo vendible si available_quantity > 0
	RequireStock StockPolicy = "REQUIRE_STOCK"
	
	// IgnoreStock: vendible siempre, aunque no tenga stock
	// Útil para tenants que venden bajo pedido o sin inventario real
	IgnoreStock StockPolicy = "IGNORE_STOCK"
)

// IsSellable determina si una variante es vendible según la policy y el stock disponible
func IsSellable(policy StockPolicy, availableQuantity float64) bool {
	switch policy {
	case RequireStock:
		return availableQuantity > 0
	case IgnoreStock:
		return true
	default:
		// Por seguridad, default a REQUIRE_STOCK
		return availableQuantity > 0
	}
}
