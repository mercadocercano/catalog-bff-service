package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"catalog-bff-service/src/infrastructure/cache"
)

// MockStockAvailabilityClient implementa StockAvailabilityClient para testing
type MockStockAvailabilityClient struct {
	GetAvailabilityFunc func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error)
	CallCount           int
}

func (m *MockStockAvailabilityClient) GetAvailability(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
	m.CallCount++
	if m.GetAvailabilityFunc != nil {
		return m.GetAvailabilityFunc(ctx, tenantID, sku)
	}
	return nil, nil
}

func TestCachedStockClient_CacheHit(t *testing.T) {
	// Arrange
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: 100,
				ReservedQuantity:  10,
				TotalQuantity:     110,
			}, nil
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada (cache miss)
	stock1, err1 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	
	// Segunda llamada (debería ser cache hit)
	stock2, err2 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")

	// Assert
	if err1 != nil {
		t.Errorf("Expected no error on first call, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("Expected no error on second call, got %v", err2)
	}
	if stock1 == nil || stock1.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available on first call")
	}
	if stock2 == nil || stock2.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available on second call")
	}
	
	// Verificar que solo se llamó una vez al servicio
	if mockClient.CallCount != 1 {
		t.Errorf("Expected 1 call to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_CacheMiss(t *testing.T) {
	// Arrange
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: 50,
			}, nil
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	stock, err := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if stock == nil || stock.AvailableQuantity != 50 {
		t.Error("Expected stock with 50 available")
	}
	if mockClient.CallCount != 1 {
		t.Errorf("Expected 1 call to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_ErrorNotCached(t *testing.T) {
	// Arrange
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			return nil, errors.New("stock service unavailable")
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada (error)
	_, err1 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	
	// Segunda llamada (debería volver a llamar al servicio)
	_, err2 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")

	// Assert
	if err1 == nil {
		t.Error("Expected error on first call")
	}
	if err2 == nil {
		t.Error("Expected error on second call")
	}
	
	// Verificar que se llamó dos veces (errores no se cachean)
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service (errors not cached), got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_NilNotCached(t *testing.T) {
	// Arrange - servicio retorna nil (404 - no existe stock)
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			return nil, nil // No existe stock
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada
	stock1, err1 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-NONEXISTENT")
	
	// Segunda llamada (debería volver a llamar al servicio, nil no se cachea)
	stock2, err2 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-NONEXISTENT")

	// Assert
	if err1 != nil {
		t.Errorf("Expected no error on first call, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("Expected no error on second call, got %v", err2)
	}
	if stock1 != nil {
		t.Error("Expected nil stock on first call")
	}
	if stock2 != nil {
		t.Error("Expected nil stock on second call")
	}
	
	// Verificar que se llamó dos veces (nil no se cachea)
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service (nil not cached), got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_TTLExpiration(t *testing.T) {
	// Arrange
	callCount := 0
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			callCount++
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: float64(callCount * 10),
			}, nil
		},
	}
	
	// Cache con TTL corto
	testCache := cache.NewInMemoryCache[*StockAvailability](50*time.Millisecond, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada
	stock1, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	
	// Segunda llamada inmediata (cache hit)
	stock2, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	
	// Esperar a que expire el cache
	time.Sleep(100 * time.Millisecond)
	
	// Tercera llamada (cache miss por expiración)
	stock3, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")

	// Assert
	if stock1 == nil || stock1.AvailableQuantity != 10 {
		t.Error("Expected stock with 10 available on first call")
	}
	if stock2 == nil || stock2.AvailableQuantity != 10 {
		t.Error("Expected stock with 10 available on second call (cache hit)")
	}
	if stock3 == nil || stock3.AvailableQuantity != 20 {
		t.Error("Expected stock with 20 available on third call (after expiration)")
	}
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_DifferentSKUs(t *testing.T) {
	// Arrange
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			qty := 100.0
			if sku == "SKU-002" {
				qty = 200.0
			}
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: qty,
			}, nil
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	stock1, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	stock2, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-002")
	
	// Segunda ronda (cache hits)
	stock1b, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	stock2b, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-002")

	// Assert
	if stock1 == nil || stock1.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available for SKU-001")
	}
	if stock2 == nil || stock2.AvailableQuantity != 200 {
		t.Error("Expected stock with 200 available for SKU-002")
	}
	if stock1b == nil || stock1b.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available for SKU-001 (cached)")
	}
	if stock2b == nil || stock2b.AvailableQuantity != 200 {
		t.Error("Expected stock with 200 available for SKU-002 (cached)")
	}
	
	// Cada SKU debe haber generado una llamada
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service (one per SKU), got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_DifferentTenants(t *testing.T) {
	// Arrange
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			qty := 100.0
			if tenantID == "tenant-2" {
				qty = 200.0
			}
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: qty,
			}, nil
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	stock1, _ := cachedClient.GetAvailability(ctx, "tenant-1", "SKU-001")
	stock2, _ := cachedClient.GetAvailability(ctx, "tenant-2", "SKU-001")
	
	// Segunda ronda (cache hits)
	stock1b, _ := cachedClient.GetAvailability(ctx, "tenant-1", "SKU-001")
	stock2b, _ := cachedClient.GetAvailability(ctx, "tenant-2", "SKU-001")

	// Assert
	if stock1 == nil || stock1.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available for tenant-1")
	}
	if stock2 == nil || stock2.AvailableQuantity != 200 {
		t.Error("Expected stock with 200 available for tenant-2")
	}
	if stock1b == nil || stock1b.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available for tenant-1 (cached)")
	}
	if stock2b == nil || stock2b.AvailableQuantity != 200 {
		t.Error("Expected stock with 200 available for tenant-2 (cached)")
	}
	
	// Cada tenant debe haber generado una llamada
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service (one per tenant), got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_InvalidateStock(t *testing.T) {
	// Arrange
	callCount := 0
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			callCount++
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: float64(callCount * 50),
			}, nil
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	stock1, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")
	
	// Invalidar cache (cast al tipo concreto para acceder al método)
	if c, ok := cachedClient.(*CachedStockAvailabilityClient); ok {
		c.InvalidateStock(ctx, "tenant-123", "SKU-001")
	}
	
	// Nueva llamada (debería consultar al servicio de nuevo)
	stock2, _ := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-001")

	// Assert
	if stock1 == nil || stock1.AvailableQuantity != 50 {
		t.Error("Expected stock with 50 available before invalidation")
	}
	if stock2 == nil || stock2.AvailableQuantity != 100 {
		t.Error("Expected stock with 100 available after invalidation")
	}
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedStockClient_ZeroQuantityCached(t *testing.T) {
	// Arrange - stock existe pero con cantidad 0
	mockClient := &MockStockAvailabilityClient{
		GetAvailabilityFunc: func(ctx context.Context, tenantID string, sku string) (*StockAvailability, error) {
			return &StockAvailability{
				ProductSKU:        sku,
				AvailableQuantity: 0,
				ReservedQuantity:  0,
				TotalQuantity:     0,
			}, nil
		},
	}
	
	testCache := cache.NewInMemoryCache[*StockAvailability](10*time.Second, 0)
	cachedClient := NewCachedStockAvailabilityClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada
	stock1, err1 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-ZERO")
	
	// Segunda llamada (debería ser cache hit)
	stock2, err2 := cachedClient.GetAvailability(ctx, "tenant-123", "SKU-ZERO")

	// Assert
	if err1 != nil {
		t.Errorf("Expected no error on first call, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("Expected no error on second call, got %v", err2)
	}
	if stock1 == nil || stock1.AvailableQuantity != 0 {
		t.Error("Expected stock with 0 available on first call")
	}
	if stock2 == nil || stock2.AvailableQuantity != 0 {
		t.Error("Expected stock with 0 available on second call (cached)")
	}
	
	// Verificar que solo se llamó una vez (stock con 0 se cachea)
	if mockClient.CallCount != 1 {
		t.Errorf("Expected 1 call to underlying service, got %d", mockClient.CallCount)
	}
}
