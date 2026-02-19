package admin

import "time"

// DashboardStatsResponse es la respuesta consolidada del dashboard
type DashboardStatsResponse struct {
	Curation CurationStats   `json:"curation"`
	Catalog  CatalogStats    `json:"catalog"`
	Tenants  TenantStats     `json:"tenants"`
	Services []ServiceHealth `json:"services"`
}

// CurationStats contiene estadísticas de curación de productos
type CurationStats struct {
	Pending       int `json:"pending"`
	ApprovedToday int `json:"approved_today"`
	RejectedToday int `json:"rejected_today"`
	TotalScraped  int `json:"total_scraped"`
}

// CatalogStats contiene estadísticas del catálogo de productos
type CatalogStats struct {
	TotalProducts   int             `json:"total_products"`
	TotalVariants   int             `json:"total_variants"`
	ActiveProducts  int             `json:"active_products"`
	CategoriesCount int             `json:"categories_count"`
	TopCategories   []CategoryCount `json:"top_categories"`
}

// CategoryCount representa el conteo de productos por categoría
type CategoryCount struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TenantStats contiene estadísticas de tenants
type TenantStats struct {
	Total        int          `json:"total"`
	Active       int          `json:"active"`
	NewThisMonth int          `json:"new_this_month"`
	Recent       []TenantInfo `json:"recent"`
}

// TenantInfo contiene información resumida de un tenant
type TenantInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Plan         string `json:"plan"`
	Status       string `json:"status"`
	LastActivity string `json:"last_activity"`
}

// ServiceHealth representa el estado de salud de un servicio
type ServiceHealth struct {
	Name          string    `json:"name"`
	Status        string    `json:"status"` // "up", "down", "degraded"
	LatencyMs     int64     `json:"latency_ms"`
	UptimePercent float64   `json:"uptime_percent"`
	LastCheck     time.Time `json:"last_check"`
}

// Estructuras internas para respuestas de servicios

// PIMStatsResponse respuesta de stats de PIM
type PIMStatsResponse struct {
	TotalCount int `json:"total_count"`
	Count      int `json:"count"`
}

// PIMProductsResponse respuesta de listado de productos de PIM
type PIMProductsResponse struct {
	Products   []PIMProduct   `json:"products"`
	Pagination PIMPagination  `json:"pagination"`
}

type PIMProduct struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	CategoryID string                 `json:"category_id"`
	Status     string                 `json:"status"`
	CreatedAt  string                 `json:"created_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type PIMPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// PIMCategoriesResponse respuesta de categorías de PIM
type PIMCategoriesResponse struct {
	Categories []PIMCategory  `json:"categories"`
	Pagination PIMPagination  `json:"pagination"`
}

type PIMCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProductsCount int  `json:"products_count,omitempty"`
}

// ScraperStatsResponse respuesta de stats de Scraper
type ScraperStatsResponse struct {
	TotalScraped int `json:"total_scraped"`
	RecentJobs   []ScraperJob `json:"recent_jobs,omitempty"`
}

type ScraperJob struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// TenantServiceResponse respuesta de Tenant Service
type TenantServiceResponse struct {
	Tenants    []Tenant       `json:"tenants"`
	Pagination TenantPagination `json:"pagination"`
}

type Tenant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TenantPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
}
