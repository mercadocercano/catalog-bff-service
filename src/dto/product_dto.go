package dto

import "time"

// ============================================================================
// DTOs de Producto para Backoffice
// Estos DTOs NO exponen detalles internos de PIM
// Son específicos para las necesidades del backoffice
// ============================================================================

// ProductListRequest - Request para listar productos
type ProductListRequest struct {
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Search     string `form:"search" binding:"omitempty"`
	CategoryID string `form:"category_id" binding:"omitempty,uuid"`
	BrandID    string `form:"brand_id" binding:"omitempty,uuid"`
	Status     string `form:"status" binding:"omitempty,oneof=active inactive draft archived"`
	SortBy     string `form:"sort_by" binding:"omitempty,oneof=name created_at updated_at"`
	SortDir    string `form:"sort_dir" binding:"omitempty,oneof=asc desc"`
}

// ProductListResponse - Response para listado de productos
type ProductListResponse struct {
	Items      []ProductSummary `json:"items"`
	TotalCount int              `json:"total_count"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

// ProductSummary - Resumen de producto para listados
type ProductSummary struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CategoryName   string    `json:"category_name,omitempty"`
	BrandName      string    `json:"brand_name,omitempty"`
	Status         string    `json:"status"`
	VariantsCount  int       `json:"variants_count"`
	HasActiveStock bool      `json:"has_active_stock"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProductDetailResponse - Detalle completo de un producto
type ProductDetailResponse struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	CategoryID   string              `json:"category_id,omitempty"`
	CategoryName string              `json:"category_name,omitempty"`
	BrandID      string              `json:"brand_id,omitempty"`
	BrandName    string              `json:"brand_name,omitempty"`
	Status       string              `json:"status"`
	Variants     []VariantSummary    `json:"variants"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// CreateProductRequest - Request para crear producto
type CreateProductRequest struct {
	Name        string   `json:"name" binding:"required,min=3,max=255"`
	Description string   `json:"description" binding:"omitempty,max=2000"`
	CategoryID  string   `json:"category_id" binding:"omitempty,uuid"`
	BrandID     string   `json:"brand_id" binding:"omitempty,uuid"`
	Status      string   `json:"status" binding:"omitempty,oneof=active inactive draft"`
	Variants    []CreateVariantRequest `json:"variants" binding:"omitempty,dive"`
}

// UpdateProductRequest - Request para actualizar producto
type UpdateProductRequest struct {
	Name        string                   `json:"name" binding:"omitempty,min=3,max=255"`
	Description string                   `json:"description" binding:"omitempty,max=2000"`
	CategoryID  string                   `json:"category_id" binding:"omitempty,uuid"`
	BrandID     string                   `json:"brand_id" binding:"omitempty,uuid"`
	Status      string                   `json:"status" binding:"omitempty,oneof=active inactive draft archived"`
	Variants    []UpdateVariantInRequest `json:"variants" binding:"omitempty,dive"`
}

type UpdateVariantInRequest struct {
	ID         string             `json:"id" binding:"required"`
	Name       string             `json:"name"`
	SKU        string             `json:"sku"`
	Price      float64            `json:"price"`
	Stock      int                `json:"stock"`
	IsActive   bool               `json:"is_active"`
	Attributes []VariantAttribute `json:"attributes"`
}

// ProductCreatedResponse - Response al crear producto
type ProductCreatedResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// DTOs de Variante para Backoffice
// ============================================================================

// VariantSummary - Resumen de variante en listados
type VariantSummary struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	SKU               string             `json:"sku"`
	Price             float64            `json:"price"`
	IsDefault         bool               `json:"is_default"`
	IsActive          bool               `json:"is_active"`
	Attributes        []VariantAttribute `json:"attributes,omitempty"`
	Stock             *StockInfo         `json:"stock,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// VariantAttribute - Atributo de variante
type VariantAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// StockInfo - Información de stock agregada
type StockInfo struct {
	Available    float64 `json:"available"`
	Reserved     float64 `json:"reserved"`
	Total        float64 `json:"total"`
	IsLowStock   bool    `json:"is_low_stock"`
	IsOutOfStock bool    `json:"is_out_of_stock"`
}

// CreateVariantRequest - Request para crear variante
type CreateVariantRequest struct {
	Name       string             `json:"name" binding:"required,min=1,max=255"`
	SKU        string             `json:"sku" binding:"required,min=1,max=100"`
	Price      float64            `json:"price" binding:"required,min=0"`
	IsDefault  bool               `json:"is_default" binding:"omitempty"`
	Attributes []VariantAttribute `json:"attributes" binding:"omitempty,dive"`
}

// UpdateVariantRequest - Request para actualizar variante
type UpdateVariantRequest struct {
	Name       string             `json:"name" binding:"omitempty,min=1,max=255"`
	SKU        string             `json:"sku" binding:"omitempty,min=1,max=100"`
	Price      float64            `json:"price" binding:"omitempty,min=0"`
	IsDefault  bool               `json:"is_default" binding:"omitempty"`
	Attributes []VariantAttribute `json:"attributes" binding:"omitempty,dive"`
}

// ToggleVariantStatusRequest - Request para activar/desactivar variante
type ToggleVariantStatusRequest struct {
	IsActive bool `json:"is_active"` // No usar binding:required con bool (false es válido)
}

// VariantCreatedResponse - Response al crear variante
type VariantCreatedResponse struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Name      string    `json:"name"`
	SKU       string    `json:"sku"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// DTOs internos para mapeo con PIM
// ============================================================================

// PIMProductResponse - Response de PIM (interno, no expuesto)
type PIMProductResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	CategoryID  string                `json:"category_id"`
	BrandID     string                `json:"brand_id"`
	Status      string                `json:"status"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// PIMProductListResponse - Response de listado de PIM (interno)
type PIMProductListResponse struct {
	Products   []PIMProductItem  `json:"products"`
	Pagination PIMPagination     `json:"pagination"`
}

type PIMPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// PIMProductItem - Item de producto en listado de PIM
type PIMProductItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CategoryID  string    `json:"category_id"`
	BrandID     string    `json:"brand_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PIMVariantResponse - Response de variante de PIM (interno)
type PIMVariantResponse struct {
	ID         string             `json:"id"`
	ProductID  string             `json:"product_id"`
	Name       string             `json:"name"`
	SKU        string             `json:"sku"`
	Price      float64            `json:"price"`
	Stock      int                `json:"stock"`
	IsDefault  bool               `json:"is_default"`
	Status     string             `json:"status"`
	Attributes []VariantAttribute `json:"attributes"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

// StockAvailabilityResponse - Response de Stock Service (interno)
type StockAvailabilityResponse struct {
	ProductSKU        string  `json:"product_sku"`
	AvailableQuantity float64 `json:"available_quantity"`
	ReservedQuantity  float64 `json:"reserved_quantity"`
	TotalQuantity     float64 `json:"total_quantity"`
	IsLowStock        bool    `json:"is_low_stock"`
	IsOutOfStock      bool    `json:"is_out_of_stock"`
}

// ============================================================================
// Error Response
// ============================================================================

// ErrorResponse - Response estándar de error
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}
