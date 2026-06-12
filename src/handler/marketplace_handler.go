package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hornosg/go-shared/infrastructure/response"
)

// MarketplaceHandler orquesta PIM + IAM Service para endpoints del marketplace
type MarketplaceHandler struct {
	pimServiceURL string
	iamServiceURL string
	s2sAPIKey     string
	httpClient    *http.Client

	// Cache de tenants (se refresca cada 5 minutos)
	tenantCache   map[string]string // tenant_id → name
	tenantCacheMu sync.RWMutex
	cacheExpiry   time.Time
}

// NewMarketplaceHandler crea una nueva instancia del handler
func NewMarketplaceHandler(pimServiceURL, iamServiceURL, s2sAPIKey string) *MarketplaceHandler {
	return &MarketplaceHandler{
		pimServiceURL: pimServiceURL,
		iamServiceURL: iamServiceURL,
		s2sAPIKey:     s2sAPIKey,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		tenantCache:   make(map[string]string),
	}
}

// ListStoreTypes lista los tipos de comercio con conteos
func (h *MarketplaceHandler) ListStoreTypes(c *gin.Context) {
	h.proxyToPIM(c, "/api/v1/marketplace/store-types")
}

// ListProducts lista todos los productos del marketplace, enriquecidos con tenant name
func (h *MarketplaceHandler) ListProducts(c *gin.Context) {
	queryString := ""
	if raw := c.Request.URL.RawQuery; raw != "" {
		queryString = "?" + raw
	}
	h.proxyAndEnrich(c, "/api/v1/marketplace/products"+queryString)
}

// ListProductsByStoreType lista productos por tipo de comercio, enriquecidos con tenant name
func (h *MarketplaceHandler) ListProductsByStoreType(c *gin.Context) {
	code := c.Param("code")
	queryString := ""
	if raw := c.Request.URL.RawQuery; raw != "" {
		queryString = "?" + raw
	}
	h.proxyAndEnrich(c, "/api/v1/marketplace/products/by-store-type/"+code+queryString)
}

// GetProduct obtiene un producto por ID, enriquecido con tenant name
func (h *MarketplaceHandler) GetProduct(c *gin.Context) {
	productID := c.Param("id")

	body, statusCode, err := h.fetchFromPIM(c, "/api/v1/marketplace/products/"+productID)
	if err != nil {
		response.JSON(c, http.StatusBadGateway, err.Error())
		return
	}
	if statusCode != http.StatusOK {
		c.Data(statusCode, "application/json", body)
		return
	}

	var product pimProduct
	if err := json.Unmarshal(body, &product); err != nil {
		c.Data(statusCode, "application/json", body)
		return
	}

	tenantNames := h.getTenantNames(c)

	result := map[string]interface{}{
		"id":            product.ID,
		"tenant_id":     product.TenantID,
		"name":          product.Name,
		"description":   product.Description,
		"category_name": product.CategoryName,
		"brand_name":    product.BrandName,
		"image_url":     product.ImageURL,
		"store_type":    product.StoreType,
		"variant":       product.Variant,
		"tenant_name":   resolveTenantName(tenantNames, product.TenantID),
	}

	c.JSON(http.StatusOK, result)
}

// ListStores lista comercios del marketplace con info y conteo de productos
func (h *MarketplaceHandler) ListStores(c *gin.Context) {
	tenantNames := h.getTenantNames(c)

	// Fetch product counts por tenant desde PIM
	body, statusCode, err := h.fetchFromPIM(c, "/api/v1/marketplace/products?page=1&page_size=1000")
	if err != nil || statusCode != http.StatusOK {
		// Fallback: solo devolver tenants sin product counts
		stores := make([]map[string]interface{}, 0)
		for id := range tenantNames {
			stores = append(stores, map[string]interface{}{
				"id":            id,
				"name":          resolveTenantName(tenantNames, id),
				"product_count": 0,
			})
		}
		c.JSON(http.StatusOK, gin.H{"stores": stores, "total": len(stores)})
		return
	}

	var pimResp pimProductListResponse
	if err := json.Unmarshal(body, &pimResp); err != nil {
		c.JSON(http.StatusOK, gin.H{"stores": []interface{}{}, "total": 0})
		return
	}

	// Contar productos y detectar store_type por tenant
	type storeInfo struct {
		ProductCount int
		StoreType    interface{}
	}
	tenantProducts := make(map[string]*storeInfo)
	for _, p := range pimResp.Products {
		info, ok := tenantProducts[p.TenantID]
		if !ok {
			info = &storeInfo{}
			tenantProducts[p.TenantID] = info
		}
		info.ProductCount++
		if info.StoreType == nil && p.StoreType != nil {
			info.StoreType = p.StoreType
		}
	}

	stores := make([]map[string]interface{}, 0)
	for tenantID, info := range tenantProducts {
		stores = append(stores, map[string]interface{}{
			"id":            tenantID,
			"name":          resolveTenantName(tenantNames, tenantID),
			"product_count": info.ProductCount,
			"store_type":    info.StoreType,
		})
	}

	c.JSON(http.StatusOK, gin.H{"stores": stores, "total": len(stores)})
}

// GetStore devuelve info de un comercio + sus productos
func (h *MarketplaceHandler) GetStore(c *gin.Context) {
	storeID := c.Param("id")
	tenantNames := h.getTenantNames(c)

	queryString := ""
	if raw := c.Request.URL.RawQuery; raw != "" {
		queryString = "?" + raw
	}

	body, statusCode, err := h.fetchFromPIM(c, "/api/v1/marketplace/products/by-tenant/"+storeID+queryString)
	if err != nil {
		response.JSON(c, http.StatusBadGateway, err.Error())
		return
	}
	if statusCode != http.StatusOK {
		c.Data(statusCode, "application/json", body)
		return
	}

	var pimResp pimProductListResponse
	if err := json.Unmarshal(body, &pimResp); err != nil {
		c.Data(statusCode, "application/json", body)
		return
	}

	// Enriquecer productos con tenant_name
	enriched := make([]map[string]interface{}, 0, len(pimResp.Products))
	for _, p := range pimResp.Products {
		enriched = append(enriched, map[string]interface{}{
			"id":            p.ID,
			"tenant_id":     p.TenantID,
			"name":          p.Name,
			"description":   p.Description,
			"category_name": p.CategoryName,
			"brand_name":    p.BrandName,
			"image_url":     p.ImageURL,
			"store_type":    p.StoreType,
			"variant":       p.Variant,
			"tenant_name":   resolveTenantName(tenantNames, p.TenantID),
		})
	}

	// Detectar store_type del primer producto
	var storeType interface{}
	if len(pimResp.Products) > 0 {
		storeType = pimResp.Products[0].StoreType
	}

	c.JSON(http.StatusOK, gin.H{
		"store": gin.H{
			"id":         storeID,
			"name":       resolveTenantName(tenantNames, storeID),
			"store_type": storeType,
		},
		"products":    enriched,
		"total":       pimResp.Total,
		"page":        pimResp.Page,
		"page_size":   pimResp.PageSize,
		"total_pages": pimResp.TotalPages,
	})
}

// ListCategories lista las categorías globales del marketplace
func (h *MarketplaceHandler) ListCategories(c *gin.Context) {
	queryString := ""
	if raw := c.Request.URL.RawQuery; raw != "" {
		queryString = "?" + raw
	}
	h.proxyToPIM(c, "/api/v1/marketplace/categories"+queryString)
}

// ListCategoriesTree devuelve el árbol de categorías
func (h *MarketplaceHandler) ListCategoriesTree(c *gin.Context) {
	h.proxyToPIM(c, "/api/v1/marketplace/categories/tree")
}

// --- Product response with tenant enrichment ---

type pimProductListResponse struct {
	Products   []pimProduct `json:"products"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	TotalPages int          `json:"total_pages"`
}

type pimProduct struct {
	ID           string      `json:"id"`
	TenantID     string      `json:"tenant_id"`
	Name         string      `json:"name"`
	Description  *string     `json:"description"`
	CategoryName *string     `json:"category_name"`
	BrandName    *string     `json:"brand_name"`
	ImageURL     *string     `json:"image_url"`
	StoreType    interface{} `json:"store_type"`
	Variant      interface{} `json:"variant"`
}

type tenantServiceResponse struct {
	Tenants []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"tenants"`
}

// proxyAndEnrich fetches products from PIM and enriches with tenant names
func (h *MarketplaceHandler) proxyAndEnrich(c *gin.Context, path string) {
	// Fetch products from PIM
	body, statusCode, err := h.fetchFromPIM(c, path)
	if err != nil {
		response.JSON(c, http.StatusBadGateway, err.Error())
		return
	}
	if statusCode != http.StatusOK {
		c.Data(statusCode, "application/json", body)
		return
	}

	// Parse response
	var pimResp pimProductListResponse
	if err := json.Unmarshal(body, &pimResp); err != nil {
		// Can't parse, return raw
		c.Data(statusCode, "application/json", body)
		return
	}

	// Load tenant names
	tenantNames := h.getTenantNames(c)

	// Enrich products with tenant_name
	enriched := make([]map[string]interface{}, 0, len(pimResp.Products))
	for _, p := range pimResp.Products {
		tenantName := resolveTenantName(tenantNames, p.TenantID)
		item := map[string]interface{}{
			"id":            p.ID,
			"tenant_id":     p.TenantID,
			"name":          p.Name,
			"description":   p.Description,
			"category_name": p.CategoryName,
			"brand_name":    p.BrandName,
			"image_url":     p.ImageURL,
			"store_type":    p.StoreType,
			"variant":       p.Variant,
			"tenant_name":   tenantName,
		}
		enriched = append(enriched, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"products":    enriched,
		"total":       pimResp.Total,
		"page":        pimResp.Page,
		"page_size":   pimResp.PageSize,
		"total_pages": pimResp.TotalPages,
	})
}

// getTenantNames returns a map of tenant_id → name, cached for 5 minutes
func (h *MarketplaceHandler) getTenantNames(c *gin.Context) map[string]string {
	h.tenantCacheMu.RLock()
	if time.Now().Before(h.cacheExpiry) && len(h.tenantCache) > 0 {
		cache := h.tenantCache
		h.tenantCacheMu.RUnlock()
		return cache
	}
	h.tenantCacheMu.RUnlock()

	// Refresh cache
	names := make(map[string]string)

	if h.iamServiceURL == "" {
		log.Println("IAM Service URL no configurada, tenant_name no disponible")
		return names
	}

	url := fmt.Sprintf("%s/api/v1/tenants?page=1&page_size=200", h.iamServiceURL)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Error creando request a tenant-service: %v", err)
		return names
	}
	req.Header.Set("X-API-Key", h.s2sAPIKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("Error llamando a tenant-service: %v", err)
		return names
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return names
	}

	var tenantResp tenantServiceResponse
	if err := json.Unmarshal(body, &tenantResp); err != nil {
		log.Printf("Error parseando respuesta de tenant-service: %v", err)
		return names
	}

	for _, t := range tenantResp.Tenants {
		names[t.ID] = t.Name
	}

	// Update cache
	h.tenantCacheMu.Lock()
	h.tenantCache = names
	h.cacheExpiry = time.Now().Add(5 * time.Minute)
	h.tenantCacheMu.Unlock()

	log.Printf("Tenant cache refreshed: %d tenants", len(names))
	return names
}

// resolveTenantName devuelve el nombre del tenant o "Comercio" como fallback
func resolveTenantName(tenantNames map[string]string, tenantID string) string {
	if name, ok := tenantNames[tenantID]; ok && name != "" {
		return name
	}
	return "Comercio"
}

// --- Helpers ---

func (h *MarketplaceHandler) fetchFromPIM(c *gin.Context, path string) ([]byte, int, error) {
	url := h.pimServiceURL + path

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("error creando request: %w", err)
	}

	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if role := c.GetHeader("X-User-Role"); role != "" {
		req.Header.Set("X-User-Role", role)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error conectando con PIM: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("error leyendo respuesta de PIM: %w", err)
	}

	return body, resp.StatusCode, nil
}

func (h *MarketplaceHandler) proxyToPIM(c *gin.Context, path string) {
	body, statusCode, err := h.fetchFromPIM(c, path)
	if err != nil {
		log.Printf("Error proxy PIM %s: %v", path, err)
		response.JSON(c, http.StatusBadGateway, err.Error())
		return
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		c.Data(statusCode, "application/json", body)
		return
	}

	c.JSON(statusCode, result)
}
