package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"catalog-bff-service/src/domain"
)

// StockAvailability representa la disponibilidad de stock de un SKU
type StockAvailability struct {
	ProductSKU        string    `json:"product_sku"`
	ProductName       string    `json:"product_name"`
	AvailableQuantity float64   `json:"available_quantity"`
	ReservedQuantity  float64   `json:"reserved_quantity"`
	TotalQuantity     float64   `json:"total_quantity"`
	UnitOfMeasure     string    `json:"unit_of_measure"`
	AvgUnitCost       float64   `json:"avg_unit_cost"`
	TotalValue        float64   `json:"total_value"`
	IsLowStock        bool      `json:"is_low_stock"`
	IsOutOfStock      bool      `json:"is_out_of_stock"`
	LastEntryAt       time.Time `json:"last_entry_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// StockAvailabilityClient define el contrato para consultar stock
type StockAvailabilityClient interface {
	GetAvailability(ctx context.Context, tenantID string, sku string) (*StockAvailability, error)
}

// HTTPStockAvailabilityClient implementa el client HTTP para stock-service
type HTTPStockAvailabilityClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPStockAvailabilityClient crea un client HTTP sin cache
func NewHTTPStockAvailabilityClient(baseURL string) StockAvailabilityClient {
	return &HTTPStockAvailabilityClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 500 * time.Millisecond, // Timeout agresivo
		},
	}
}

// GetAvailability consulta la disponibilidad de un SKU
func (c *HTTPStockAvailabilityClient) GetAvailability(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
	url := fmt.Sprintf("%s/api/v1/availability?sku=%s", c.baseURL, sku)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call stock-service: %w", err)
	}
	defer resp.Body.Close()

	// Si es 404, no hay stock (retornar nil sin error)
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	// Si es error del servidor, propagar error
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("stock-service error: status %d", resp.StatusCode)
	}

	// Si no es 200, error inesperado
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from stock-service: %d", resp.StatusCode)
	}

	// Parsear respuesta
	var availability StockAvailability
	if err := json.NewDecoder(resp.Body).Decode(&availability); err != nil {
		return nil, fmt.Errorf("failed to parse stock-service response: %w", err)
	}

	return &availability, nil
}

// CachedStockAvailabilityClient envuelve un StockAvailabilityClient con cache
type CachedStockAvailabilityClient struct {
	underlying StockAvailabilityClient
	cache      domain.Cache[*StockAvailability]
}

// NewCachedStockAvailabilityClient crea un client con cache
func NewCachedStockAvailabilityClient(
	underlying StockAvailabilityClient,
	cache domain.Cache[*StockAvailability],
) StockAvailabilityClient {
	return &CachedStockAvailabilityClient{
		underlying: underlying,
		cache:      cache,
	}
}

// GetAvailability obtiene disponibilidad con cache
//
// Estrategia:
// 1. Intenta leer del cache
// 2. Si cache miss, consulta al servicio
// 3. Si el servicio responde OK, cachea el resultado
// 4. Si el servicio retorna error, NO cachea (para reintentar inmediatamente)
// 5. Si el servicio retorna nil (404), NO cachea (stock puede cambiar rápido)
func (c *CachedStockAvailabilityClient) GetAvailability(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
	// Generar cache key
	cacheKey := domain.CacheKey("stock", tenantID, sku)

	// 1. Intentar leer del cache
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		log.Printf("[CachedStockClient] Cache HIT for tenant=%s sku=%s", tenantID, sku)
		return cachedValue, nil
	}

	log.Printf("[CachedStockClient] Cache MISS for tenant=%s sku=%s", tenantID, sku)

	// 2. Cache miss: consultar al servicio
	availability, err := c.underlying.GetAvailability(ctx, tenantID, sku)

	// 3. Si hubo error, NO cachear
	if err != nil {
		log.Printf("[CachedStockClient] Error from underlying service: %v (not caching)", err)
		return nil, err
	}

	// 4. Si retornó nil (404 - no existe stock), NO cachear
	// Esto permite que el stock se actualice rápidamente cuando se crea
	if availability == nil {
		log.Printf("[CachedStockClient] No stock found for sku=%s (not caching nil)", sku)
		return nil, nil
	}

	// 5. Cachear el resultado exitoso
	c.cache.Set(ctx, cacheKey, availability, 0) // Usa TTL por defecto del cache
	log.Printf("[CachedStockClient] Cached availability for tenant=%s sku=%s", tenantID, sku)

	return availability, nil
}

// InvalidateStock permite invalidar manualmente el cache de un SKU
func (c *CachedStockAvailabilityClient) InvalidateStock(ctx context.Context, tenantID string, sku string) {
	cacheKey := domain.CacheKey("stock", tenantID, sku)
	c.cache.Delete(ctx, cacheKey)
	log.Printf("[CachedStockClient] Invalidated cache for tenant=%s sku=%s", tenantID, sku)
}
