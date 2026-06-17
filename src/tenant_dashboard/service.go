package tenant_dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"catalog-bff-service/src/dto"
)

type Service struct {
	pimServiceURL   string
	stockServiceURL string
	httpClient      *http.Client
}

func NewService(pimURL, stockURL string) *Service {
	return &Service{
		pimServiceURL:   pimURL,
		stockServiceURL: stockURL,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

func (s *Service) GetDashboard(ctx context.Context, tenantID, authHeader string) (*TenantDashboardResponse, error) {
	var (
		catalog   CatalogStats
		inventory dto.InventorySummaryResponse
		wg        sync.WaitGroup
		mu        sync.Mutex
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		stats := s.getCatalogStats(ctx, tenantID, authHeader)
		mu.Lock()
		catalog = stats
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		inv, err := s.getInventorySummary(ctx, tenantID, authHeader)
		mu.Lock()
		if err == nil {
			inventory = *inv
		}
		mu.Unlock()
	}()

	wg.Wait()

	return &TenantDashboardResponse{
		Catalog:   catalog,
		Inventory: inventory,
	}, nil
}

func (s *Service) getCatalogStats(ctx context.Context, tenantID, authHeader string) CatalogStats {
	stats := CatalogStats{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	type countResult struct {
		field string
		value int
	}

	calls := []struct {
		field string
		url   string
		parse func([]byte) int
	}{
		{
			field: "total_products",
			url:   fmt.Sprintf("%s/api/v1/products?page=1&page_size=1", s.pimServiceURL),
			parse: func(body []byte) int {
				var r PIMProductsResponse
				if json.Unmarshal(body, &r) == nil {
					return r.Pagination.TotalItems
				}
				return 0
			},
		},
		{
			field: "active_products",
			url:   fmt.Sprintf("%s/api/v1/products?status=active&page=1&page_size=1", s.pimServiceURL),
			parse: func(body []byte) int {
				var r PIMProductsResponse
				if json.Unmarshal(body, &r) == nil {
					return r.Pagination.TotalItems
				}
				return 0
			},
		},
		{
			field: "inactive_products",
			url:   fmt.Sprintf("%s/api/v1/products?status=inactive&page=1&page_size=1", s.pimServiceURL),
			parse: func(body []byte) int {
				var r PIMProductsResponse
				if json.Unmarshal(body, &r) == nil {
					return r.Pagination.TotalItems
				}
				return 0
			},
		},
		{
			field: "total_variants",
			url:   fmt.Sprintf("%s/api/v1/product-variants?page=1&page_size=1", s.pimServiceURL),
			parse: func(body []byte) int {
				var r PIMPagination
				type wrapper struct {
					Pagination PIMPagination `json:"pagination"`
				}
				var w wrapper
				if json.Unmarshal(body, &w) == nil {
					r = w.Pagination
					return r.TotalItems
				}
				return 0
			},
		},
		{
			field: "brands_count",
			url:   fmt.Sprintf("%s/api/v1/brands?page=1&page_size=1", s.pimServiceURL),
			parse: func(body []byte) int {
				var r PIMListResponse
				if json.Unmarshal(body, &r) == nil {
					return r.TotalCount
				}
				return 0
			},
		},
		{
			field: "categories_count",
			url:   fmt.Sprintf("%s/api/v1/categories?page=1&page_size=1", s.pimServiceURL),
			parse: func(body []byte) int {
				var r PIMListResponse
				if json.Unmarshal(body, &r) == nil {
					return r.TotalCount
				}
				return 0
			},
		},
	}

	results := make([]countResult, len(calls))

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, field, url string, parse func([]byte) int) {
			defer wg.Done()
			body := s.makeRequest(ctx, url, tenantID, authHeader)
			value := 0
			if body != nil {
				value = parse(body)
			}
			mu.Lock()
			results[idx] = countResult{field: field, value: value}
			mu.Unlock()
		}(i, call.field, call.url, call.parse)
	}

	wg.Wait()

	for _, r := range results {
		switch r.field {
		case "total_products":
			stats.TotalProducts = r.value
		case "active_products":
			stats.ActiveProducts = r.value
		case "inactive_products":
			stats.InactiveProducts = r.value
		case "total_variants":
			stats.TotalVariants = r.value
		case "brands_count":
			stats.BrandsCount = r.value
		case "categories_count":
			stats.CategoriesCount = r.value
		}
	}

	return stats
}

func (s *Service) getInventorySummary(ctx context.Context, tenantID, authHeader string) (*dto.InventorySummaryResponse, error) {
	url := fmt.Sprintf("%s/api/v1/availability?page=1&page_size=500", s.stockServiceURL)
	body := s.makeRequest(ctx, url, tenantID, authHeader)
	if body == nil {
		empty := dto.InventorySummaryResponse{}
		return &empty, fmt.Errorf("stock-service no respondió")
	}

	var stockResp dto.StockAvailabilityListResponse
	if err := json.Unmarshal(body, &stockResp); err != nil {
		empty := dto.InventorySummaryResponse{}
		return &empty, fmt.Errorf("error parseando stock: %w", err)
	}

	items := make([]dto.InventoryItem, 0, len(stockResp.Items))
	for _, stock := range stockResp.Items {
		items = append(items, dto.InventoryItem{
			VariantSKU:        stock.ProductSKU,
			AvailableQuantity: stock.AvailableQuantity,
			ReservedQuantity:  stock.ReservedQuantity,
			ProductName:       stock.ProductName,
			CategoryName:      "-",
		})
	}

	totals := dto.InventoryTotals{TotalSKUs: len(items)}
	for _, item := range items {
		totals.TotalAvailable += item.AvailableQuantity
		totals.TotalReserved += item.ReservedQuantity
		totals.TotalStockValue += item.StockValue
	}

	summary := &dto.InventorySummaryResponse{
		Totals:     totals,
		ByCategory: []dto.InventoryCategoryTotal{},
	}

	return summary, nil
}

func (s *Service) makeRequest(ctx context.Context, url, tenantID, authHeader string) []byte {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return body
}
