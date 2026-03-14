package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"catalog-bff-service/src/dto"
)

type InventoryHandler struct {
	stockServiceURL string
	pimServiceURL   string
	httpClient      *http.Client
}

func NewInventoryHandler(stockServiceURL, pimServiceURL string) *InventoryHandler {
	return &InventoryHandler{
		stockServiceURL: stockServiceURL,
		pimServiceURL:   pimServiceURL,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

// ListInventory GET /api/v1/inventory
func (h *InventoryHandler) ListInventory(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "missing_tenant", Message: "X-Tenant-ID header es requerido"})
		return
	}

	var req dto.InventoryListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid_params", Message: err.Error()})
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	items, err := h.fetchAndMerge(c, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "orchestration_error", Message: err.Error()})
		return
	}

	filtered := h.applyFilters(items, &req)
	h.applySort(filtered, req.SortBy, req.SortDir)

	totalCount := len(filtered)
	totalPages := int(math.Ceil(float64(totalCount) / float64(req.PageSize)))
	offset := (req.Page - 1) * req.PageSize
	end := offset + req.PageSize
	if offset > totalCount {
		offset = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	c.JSON(http.StatusOK, dto.InventoryListResponse{
		Items:      filtered[offset:end],
		TotalCount: totalCount,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	})
}

// GetInventorySummary GET /api/v1/inventory/summary
func (h *InventoryHandler) GetInventorySummary(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "missing_tenant", Message: "X-Tenant-ID header es requerido"})
		return
	}

	items, err := h.fetchAndMerge(c, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "orchestration_error", Message: err.Error()})
		return
	}

	summary := h.buildSummary(items)
	c.JSON(http.StatusOK, summary)
}

// fetchAndMerge orchestrates Stock + PIM calls and merges into InventoryItems
func (h *InventoryHandler) fetchAndMerge(c *gin.Context, tenantID string) ([]dto.InventoryItem, error) {
	stockItems, err := h.fetchAllStock(c, tenantID)
	if err != nil {
		return nil, fmt.Errorf("stock-service: %w", err)
	}

	if len(stockItems) == 0 {
		return []dto.InventoryItem{}, nil
	}

	skus := make([]string, 0, len(stockItems))
	for _, item := range stockItems {
		skus = append(skus, item.ProductSKU)
	}

	// Expandir SKUs: el quickstart crea stock con product.sku (ej: ALMACEN-001)
	// pero PIM guarda variantes con sku = product.sku + "-DEF" (ej: ALMACEN-001-DEF)
	skusForPIM := expandSKUsForPIMLookup(skus)

	pimData, err := h.fetchPIMVariantsBySKUs(c, tenantID, skusForPIM)
	if err != nil {
		return nil, fmt.Errorf("pim-service: %w", err)
	}

	// pimMap: clave = SKU de stock para lookup. Incluir tanto SKU exacto como base (sin -DEF)
	// para que stock con "ALMACEN-001" encuentre variante "ALMACEN-001-DEF"
	pimMap := buildPIMLookupMap(pimData, skus)

	items := make([]dto.InventoryItem, 0, len(stockItems))
	for _, stock := range stockItems {
		item := dto.InventoryItem{
			VariantSKU:        stock.ProductSKU,
			AvailableQuantity: stock.AvailableQuantity,
			ReservedQuantity:  stock.ReservedQuantity,
			LastEntryAt:       stock.LastEntryAt,
		}

		if pim, ok := pimMap[stock.ProductSKU]; ok {
			item.ProductName = pim.ProductName
			item.CategoryID = pim.CategoryID
			item.CategoryName = pim.CategoryName
			item.SalePrice = pim.Price
			item.StockValue = pim.Price * stock.AvailableQuantity
		} else {
			// Fallback: usar product_name de stock si está disponible (ej: bulk create con product_name)
			item.ProductName = stock.ProductName
			if item.ProductName == "" {
				item.ProductName = "Producto desconocido"
			}
			item.CategoryName = "-"
		}

		items = append(items, item)
	}

	return items, nil
}

// expandSKUsForPIMLookup agrega variantes -DEF para lookup en PIM.
// El quickstart crea stock con product.sku (ALMACEN-001) pero PIM guarda variantes con sku+'-DEF'.
func expandSKUsForPIMLookup(skus []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(skus)*2)
	for _, s := range skus {
		if s == "" {
			continue
		}
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
		defSku := s + "-DEF"
		if !seen[defSku] {
			seen[defSku] = true
			result = append(result, defSku)
		}
	}
	return result
}

// buildPIMLookupMap construye un mapa para lookup: stock SKU -> variante PIM.
// Permite que stock con "ALMACEN-001" encuentre variante "ALMACEN-001-DEF".
func buildPIMLookupMap(pimData []dto.PIMEnrichedVariant, stockSKUs []string) map[string]*dto.PIMEnrichedVariant {
	stockSet := make(map[string]bool)
	for _, s := range stockSKUs {
		stockSet[s] = true
	}
	result := make(map[string]*dto.PIMEnrichedVariant, len(pimData)*2)
	for i := range pimData {
		v := &pimData[i]
		result[v.SKU] = v
		// Si la variante tiene sufijo -DEF, mapear también la base (sin -DEF) para stock del quickstart
		if strings.HasSuffix(v.SKU, "-DEF") {
			base := strings.TrimSuffix(v.SKU, "-DEF")
			if stockSet[base] {
				result[base] = v
			}
		}
	}
	return result
}

// fetchAllStock gets all stock availability for the tenant (paginated internally, returns all)
func (h *InventoryHandler) fetchAllStock(c *gin.Context, tenantID string) ([]dto.StockAvailabilityItem, error) {
	var allItems []dto.StockAvailabilityItem
	page := 1
	pageSize := 500

	for {
		url := fmt.Sprintf("%s/api/v1/availability?page=%d&page_size=%d", h.stockServiceURL, page, pageSize)
		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Tenant-ID", tenantID)
		if auth := c.GetHeader("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}

		resp, err := h.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
		}

		var result dto.StockAvailabilityListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		allItems = append(allItems, result.Items...)

		if page >= result.TotalPages || len(result.Items) == 0 {
			break
		}
		page++
	}

	return allItems, nil
}

// fetchPIMVariantsBySKUs batch-fetches variant+product+category data from PIM
func (h *InventoryHandler) fetchPIMVariantsBySKUs(c *gin.Context, tenantID string, skus []string) ([]dto.PIMEnrichedVariant, error) {
	body, err := json.Marshal(map[string][]string{"skus": skus})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/variants/by-skus", h.pimServiceURL)
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result dto.PIMVariantsBySKUsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Variants, nil
}

func (h *InventoryHandler) applyFilters(items []dto.InventoryItem, req *dto.InventoryListRequest) []dto.InventoryItem {
	filtered := make([]dto.InventoryItem, 0, len(items))
	searchLower := strings.ToLower(req.Search)

	var lastMovementFrom, lastMovementTo time.Time
	if req.LastMovementFrom != "" {
		lastMovementFrom, _ = time.Parse("2006-01-02", req.LastMovementFrom)
	}
	if req.LastMovementTo != "" {
		lastMovementTo, _ = time.Parse("2006-01-02", req.LastMovementTo)
		lastMovementTo = lastMovementTo.Add(24*time.Hour - time.Nanosecond)
	}

	for _, item := range items {
		if searchLower != "" {
			if !strings.Contains(strings.ToLower(item.ProductName), searchLower) &&
				!strings.Contains(strings.ToLower(item.VariantSKU), searchLower) {
				continue
			}
		}

		if req.CategoryID != "" {
			if item.CategoryID == nil || *item.CategoryID != req.CategoryID {
				continue
			}
		}

		if req.MinAvailable != nil && item.AvailableQuantity < *req.MinAvailable {
			continue
		}
		if req.MaxAvailable != nil && item.AvailableQuantity > *req.MaxAvailable {
			continue
		}

		if req.MinPrice != nil && item.SalePrice < *req.MinPrice {
			continue
		}
		if req.MaxPrice != nil && item.SalePrice > *req.MaxPrice {
			continue
		}

		if !lastMovementFrom.IsZero() && item.LastEntryAt != nil && item.LastEntryAt.Before(lastMovementFrom) {
			continue
		}
		if !lastMovementTo.IsZero() && item.LastEntryAt != nil && item.LastEntryAt.After(lastMovementTo) {
			continue
		}

		filtered = append(filtered, item)
	}

	return filtered
}

func (h *InventoryHandler) applySort(items []dto.InventoryItem, sortBy, sortDir string) {
	if sortBy == "" {
		sortBy = "variant_sku"
	}
	asc := strings.ToLower(sortDir) != "desc"

	sort.SliceStable(items, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "variant_sku":
			less = items[i].VariantSKU < items[j].VariantSKU
		case "product_name":
			less = strings.ToLower(items[i].ProductName) < strings.ToLower(items[j].ProductName)
		case "category_name":
			less = strings.ToLower(items[i].CategoryName) < strings.ToLower(items[j].CategoryName)
		case "available_quantity":
			less = items[i].AvailableQuantity < items[j].AvailableQuantity
		case "reserved_quantity":
			less = items[i].ReservedQuantity < items[j].ReservedQuantity
		case "last_entry_at":
			ti := time.Time{}
			tj := time.Time{}
			if items[i].LastEntryAt != nil {
				ti = *items[i].LastEntryAt
			}
			if items[j].LastEntryAt != nil {
				tj = *items[j].LastEntryAt
			}
			less = ti.Before(tj)
		case "sale_price":
			less = items[i].SalePrice < items[j].SalePrice
		case "stock_value":
			less = items[i].StockValue < items[j].StockValue
		default:
			less = items[i].VariantSKU < items[j].VariantSKU
		}
		if !asc {
			return !less
		}
		return less
	})
}

func (h *InventoryHandler) buildSummary(items []dto.InventoryItem) dto.InventorySummaryResponse {
	totals := dto.InventoryTotals{TotalSKUs: len(items)}
	catMap := make(map[string]*dto.InventoryCategoryTotal)

	for _, item := range items {
		totals.TotalAvailable += item.AvailableQuantity
		totals.TotalReserved += item.ReservedQuantity
		totals.TotalStockValue += item.StockValue

		catKey := ""
		if item.CategoryID != nil {
			catKey = *item.CategoryID
		}

		cat, exists := catMap[catKey]
		if !exists {
			catName := "Sin categoría"
			if item.CategoryName != "" && item.CategoryName != "-" {
				catName = item.CategoryName
			}
			cat = &dto.InventoryCategoryTotal{
				CategoryID:   item.CategoryID,
				CategoryName: catName,
			}
			catMap[catKey] = cat
		}

		cat.SKUCount++
		cat.AvailableQuantity += item.AvailableQuantity
		cat.ReservedQuantity += item.ReservedQuantity
		cat.StockValue += item.StockValue
	}

	byCategory := make([]dto.InventoryCategoryTotal, 0, len(catMap))
	for _, cat := range catMap {
		byCategory = append(byCategory, *cat)
	}
	sort.Slice(byCategory, func(i, j int) bool {
		return byCategory[i].StockValue > byCategory[j].StockValue
	})

	return dto.InventorySummaryResponse{
		Totals:     totals,
		ByCategory: byCategory,
	}
}
