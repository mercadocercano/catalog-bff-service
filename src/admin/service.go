package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// DashboardService orquesta la obtención de métricas de múltiples servicios
type DashboardService struct {
	pimServiceURL     string
	scraperServiceURL string
	iamServiceURL     string
	tenantServiceURL  string
	httpClient        *http.Client
}

// NewDashboardService crea una nueva instancia del servicio de dashboard
func NewDashboardService(pimURL, scraperURL, iamURL, tenantURL string) *DashboardService {
	return &DashboardService{
		pimServiceURL:     pimURL,
		scraperServiceURL: scraperURL,
		iamServiceURL:     iamURL,
		tenantServiceURL:  tenantURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // Timeout de 2 segundos por servicio
		},
	}
}

// GetDashboardStats obtiene todas las estadísticas del dashboard de forma paralela
func (s *DashboardService) GetDashboardStats(ctx context.Context) (*DashboardStatsResponse, error) {
	var (
		curationStats CurationStats
		catalogStats  CatalogStats
		tenantStats   TenantStats
		serviceHealth []ServiceHealth
		wg            sync.WaitGroup
		mu            sync.Mutex
	)

	// Curación stats (de PIM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats := s.getCurationStats(ctx)
		mu.Lock()
		curationStats = stats
		mu.Unlock()
	}()

	// Catálogo stats (de PIM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats := s.getCatalogStats(ctx)
		mu.Lock()
		catalogStats = stats
		mu.Unlock()
	}()

	// Tenant stats (de Tenant Service)
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats := s.getTenantStats(ctx)
		mu.Lock()
		tenantStats = stats
		mu.Unlock()
	}()

	// Services health
	wg.Add(1)
	go func() {
		defer wg.Done()
		health := s.getServicesHealth(ctx)
		mu.Lock()
		serviceHealth = health
		mu.Unlock()
	}()

	wg.Wait()

	return &DashboardStatsResponse{
		Curation: curationStats,
		Catalog:  catalogStats,
		Tenants:  tenantStats,
		Services: serviceHealth,
	}, nil
}

// getCurationStats obtiene estadísticas de curación desde PIM
func (s *DashboardService) getCurationStats(ctx context.Context) CurationStats {
	stats := CurationStats{}

	// Obtener productos pendientes
	pendingURL := fmt.Sprintf("%s/api/v1/products?status=pending&page=1&page_size=1", s.pimServiceURL)
	if resp := s.makeRequest(ctx, pendingURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			stats.Pending = pimResp.Pagination.TotalItems
		}
	}

	// Obtener productos aprobados hoy
	today := time.Now().Format("2006-01-02")
	approvedURL := fmt.Sprintf("%s/api/v1/products?status=approved&date_from=%s&page=1&page_size=1", s.pimServiceURL, today)
	if resp := s.makeRequest(ctx, approvedURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			stats.ApprovedToday = pimResp.Pagination.TotalItems
		}
	}

	// Obtener productos rechazados hoy
	rejectedURL := fmt.Sprintf("%s/api/v1/products?status=rejected&date_from=%s&page=1&page_size=1", s.pimServiceURL, today)
	if resp := s.makeRequest(ctx, rejectedURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			stats.RejectedToday = pimResp.Pagination.TotalItems
		}
	}

	// Obtener total de productos scrapeados (source=scraper)
	scrapedURL := fmt.Sprintf("%s/api/v1/products?page=1&page_size=1", s.pimServiceURL)
	if resp := s.makeRequest(ctx, scrapedURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			// Contar productos cuyo metadata contiene "source": "scraper"
			// Por ahora usar el total, luego se puede filtrar mejor
			stats.TotalScraped = s.countScrapedProducts(ctx)
		}
	}

	return stats
}

// getCatalogStats obtiene estadísticas del catálogo desde PIM
func (s *DashboardService) getCatalogStats(ctx context.Context) CatalogStats {
	stats := CatalogStats{
		TopCategories: []CategoryCount{},
	}

	// Total de productos
	productsURL := fmt.Sprintf("%s/api/v1/products?page=1&page_size=1", s.pimServiceURL)
	if resp := s.makeRequest(ctx, productsURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			stats.TotalProducts = pimResp.Pagination.TotalItems
		}
	}

	// Productos activos
	activeURL := fmt.Sprintf("%s/api/v1/products?is_active=true&page=1&page_size=1", s.pimServiceURL)
	if resp := s.makeRequest(ctx, activeURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			stats.ActiveProducts = pimResp.Pagination.TotalItems
		}
	}

	// Total de variantes (endpoint puede no existir, usar fallback)
	variantsURL := fmt.Sprintf("%s/api/v1/product-variants?page=1&page_size=1", s.pimServiceURL)
	if resp := s.makeRequest(ctx, variantsURL); resp != nil {
		var variantsResp struct {
			Pagination PIMPagination `json:"pagination"`
		}
		if err := json.Unmarshal(resp, &variantsResp); err == nil {
			stats.TotalVariants = variantsResp.Pagination.TotalItems
		}
	}

	// Total de categorías
	categoriesURL := fmt.Sprintf("%s/api/v1/categories?page=1&page_size=1", s.pimServiceURL)
	if resp := s.makeRequest(ctx, categoriesURL); resp != nil {
		var catResp PIMCategoriesResponse
		if err := json.Unmarshal(resp, &catResp); err == nil {
			stats.CategoriesCount = catResp.Pagination.TotalItems
		}
	}

	// Top 5 categorías (si el endpoint existe)
	topCategoriesURL := fmt.Sprintf("%s/api/v1/categories?page=1&page_size=5&sort_by=products_count&sort_dir=desc", s.pimServiceURL)
	if resp := s.makeRequest(ctx, topCategoriesURL); resp != nil {
		var catResp PIMCategoriesResponse
		if err := json.Unmarshal(resp, &catResp); err == nil {
			for _, cat := range catResp.Categories {
				stats.TopCategories = append(stats.TopCategories, CategoryCount{
					ID:    cat.ID,
					Name:  cat.Name,
					Count: cat.ProductsCount,
				})
			}
		}
	}

	return stats
}

// getTenantStats obtiene estadísticas de tenants desde Tenant Service
func (s *DashboardService) getTenantStats(ctx context.Context) TenantStats {
	stats := TenantStats{
		Recent: []TenantInfo{},
	}

	if s.tenantServiceURL == "" {
		log.Println("⚠️ Tenant Service URL no configurado, retornando stats vacíos")
		return stats
	}

	// Obtener lista de tenants
	tenantsURL := fmt.Sprintf("%s/api/v1/tenants?page=1&page_size=100", s.tenantServiceURL)
	if resp := s.makeRequest(ctx, tenantsURL); resp != nil {
		var tenantResp TenantServiceResponse
		if err := json.Unmarshal(resp, &tenantResp); err == nil {
			stats.Total = tenantResp.Pagination.TotalItems
			
			// Contar activos
			for _, tenant := range tenantResp.Tenants {
				if tenant.Status == "active" {
					stats.Active++
				}
				
				// Contar nuevos este mes
				if t, err := time.Parse(time.RFC3339, tenant.CreatedAt); err == nil {
					now := time.Now()
					if t.Year() == now.Year() && t.Month() == now.Month() {
						stats.NewThisMonth++
					}
				}
			}
			
			// Obtener los 5 más recientes
			limit := 5
			if len(tenantResp.Tenants) < limit {
				limit = len(tenantResp.Tenants)
			}
			
			for i := 0; i < limit; i++ {
				tenant := tenantResp.Tenants[i]
				stats.Recent = append(stats.Recent, TenantInfo{
					ID:           tenant.ID,
					Name:         tenant.Name,
					Plan:         tenant.Plan,
					Status:       tenant.Status,
					LastActivity: tenant.UpdatedAt,
				})
			}
		}
	}

	return stats
}

// getServicesHealth verifica el estado de salud de todos los servicios
func (s *DashboardService) getServicesHealth(ctx context.Context) []ServiceHealth {
	services := []struct {
		name string
		url  string
	}{
		{"pim-service", s.pimServiceURL},
		{"scraper-service", s.scraperServiceURL},
		{"iam-service", s.iamServiceURL},
		{"tenant-service", s.tenantServiceURL},
	}

	var health []ServiceHealth
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, service := range services {
		if service.url == "" {
			continue
		}

		wg.Add(1)
		go func(name, baseURL string) {
			defer wg.Done()

			start := time.Now()
			healthURL := fmt.Sprintf("%s/health", baseURL)
			
			status := "down"
			latency := int64(0)

			req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			if err != nil {
				log.Printf("❌ Error creando request para %s: %v", name, err)
			} else {
				resp, err := s.httpClient.Do(req)
				latency = time.Since(start).Milliseconds()
				
				if err == nil && resp != nil {
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						status = "up"
					} else {
						status = "degraded"
					}
				}
			}

			mu.Lock()
			health = append(health, ServiceHealth{
				Name:          name,
				Status:        status,
				LatencyMs:     latency,
				UptimePercent: 99.5, // TODO: Calcular de métricas reales
				LastCheck:     time.Now(),
			})
			mu.Unlock()
		}(service.name, service.url)
	}

	wg.Wait()
	return health
}

// makeRequest hace una petición HTTP y retorna el body como []byte
func (s *DashboardService) makeRequest(ctx context.Context, url string) []byte {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("❌ Error creando request a %s: %v", url, err)
		return nil
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("❌ Error haciendo request a %s: %v", url, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ Request a %s retornó status %d", url, resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error leyendo response de %s: %v", url, err)
		return nil
	}

	return body
}

// countScrapedProducts cuenta productos que vienen de scraper
// En producción esto debería ser un endpoint específico en PIM
func (s *DashboardService) countScrapedProducts(ctx context.Context) int {
	// Por ahora retornar un valor estimado
	// TODO: Implementar endpoint en PIM que filtre por source=scraper
	productsURL := fmt.Sprintf("%s/api/v1/products?page=1&page_size=100", s.pimServiceURL)
	if resp := s.makeRequest(ctx, productsURL); resp != nil {
		var pimResp PIMProductsResponse
		if err := json.Unmarshal(resp, &pimResp); err == nil {
			count := 0
			for _, product := range pimResp.Products {
				if metadata, ok := product.Metadata["source"]; ok {
					if source, ok := metadata.(string); ok && source == "scraper" {
						count++
					}
				}
			}
			// Si encontramos productos scrapeados en la primera página,
			// estimar el total proporcionalmente
			if count > 0 && pimResp.Pagination.TotalItems > 0 {
				ratio := float64(count) / float64(len(pimResp.Products))
				return int(ratio * float64(pimResp.Pagination.TotalItems))
			}
		}
	}
	
	return 0
}
