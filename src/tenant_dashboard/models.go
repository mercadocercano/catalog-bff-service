package tenant_dashboard

import "catalog-bff-service/src/dto"

type TenantDashboardResponse struct {
	Catalog   CatalogStats              `json:"catalog"`
	Inventory dto.InventorySummaryResponse `json:"inventory"`
}

type CatalogStats struct {
	TotalProducts    int `json:"total_products"`
	ActiveProducts   int `json:"active_products"`
	InactiveProducts int `json:"inactive_products"`
	TotalVariants    int `json:"total_variants"`
	BrandsCount      int `json:"brands_count"`
	CategoriesCount  int `json:"categories_count"`
}

type PIMProductsResponse struct {
	Products   []interface{}  `json:"products"`
	Pagination PIMPagination  `json:"pagination"`
}

type PIMPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type PIMListResponse struct {
	Items      []interface{} `json:"items"`
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}
