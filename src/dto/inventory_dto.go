package dto

import "time"

// InventoryItem represents a single inventory row (stock + PIM enrichment)
type InventoryItem struct {
	VariantSKU        string    `json:"variant_sku"`
	ProductName       string    `json:"product_name"`
	CategoryID        *string   `json:"category_id,omitempty"`
	CategoryName      string    `json:"category_name"`
	AvailableQuantity float64   `json:"available_quantity"`
	ReservedQuantity  float64   `json:"reserved_quantity"`
	LastEntryAt       *time.Time `json:"last_entry_at,omitempty"`
	SalePrice         float64   `json:"sale_price"`
	StockValue        float64   `json:"stock_value"`
}

type InventoryListResponse struct {
	Items      []InventoryItem `json:"items"`
	TotalCount int             `json:"total_count"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

type InventoryListRequest struct {
	Search            string `form:"search"`
	CategoryID        string `form:"category_id"`
	MinAvailable      *float64 `form:"min_available"`
	MaxAvailable      *float64 `form:"max_available"`
	MinPrice          *float64 `form:"min_price"`
	MaxPrice          *float64 `form:"max_price"`
	LastMovementFrom  string `form:"last_movement_from"`
	LastMovementTo    string `form:"last_movement_to"`
	SortBy            string `form:"sort_by"`
	SortDir           string `form:"sort_dir"`
	Page              int    `form:"page"`
	PageSize          int    `form:"page_size"`
}

// InventorySummaryResponse dashboard totals
type InventorySummaryResponse struct {
	Totals     InventoryTotals          `json:"totals"`
	ByCategory []InventoryCategoryTotal `json:"by_category"`
}

type InventoryTotals struct {
	TotalSKUs      int     `json:"total_skus"`
	TotalAvailable float64 `json:"total_available"`
	TotalReserved  float64 `json:"total_reserved"`
	TotalStockValue float64 `json:"total_stock_value"`
}

type InventoryCategoryTotal struct {
	CategoryID        *string `json:"category_id"`
	CategoryName      string  `json:"category_name"`
	SKUCount          int     `json:"sku_count"`
	AvailableQuantity float64 `json:"available_quantity"`
	ReservedQuantity  float64 `json:"reserved_quantity"`
	StockValue        float64 `json:"stock_value"`
}

// Internal DTOs for stock-service responses

type StockAvailabilityListResponse struct {
	Items      []StockAvailabilityItem `json:"items"`
	TotalCount int                     `json:"total_count"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalPages int                     `json:"total_pages"`
}

type StockAvailabilityItem struct {
	ProductSKU        string     `json:"product_sku"`
	ProductID         *string    `json:"product_id,omitempty"`
	ProductName       string     `json:"product_name,omitempty"`
	AvailableQuantity float64    `json:"available_quantity"`
	ReservedQuantity  float64    `json:"reserved_quantity"`
	TotalQuantity     float64    `json:"total_quantity"`
	AvgUnitCost       *float64   `json:"avg_unit_cost,omitempty"`
	TotalValue        *float64   `json:"total_value,omitempty"`
	IsLowStock        bool       `json:"is_low_stock"`
	IsOutOfStock      bool       `json:"is_out_of_stock"`
	LastEntryAt       *time.Time `json:"last_entry_at,omitempty"`
}

// Internal DTOs for PIM batch variant response

type PIMVariantsBySKUsResponse struct {
	Variants []PIMEnrichedVariant `json:"variants"`
}

type PIMEnrichedVariant struct {
	VariantID    string  `json:"variant_id"`
	ProductID    string  `json:"product_id"`
	SKU          string  `json:"sku"`
	VariantName  string  `json:"variant_name"`
	ProductName  string  `json:"product_name"`
	CategoryID   *string `json:"category_id,omitempty"`
	CategoryName string  `json:"category_name"`
	Price        float64 `json:"price"`
}
