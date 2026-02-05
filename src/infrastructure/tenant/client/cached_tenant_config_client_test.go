package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"catalog-bff-service/src/infrastructure/cache"
)

// MockTenantConfigClient implementa TenantConfigClient para testing
type MockTenantConfigClient struct {
	GetConfigFunc func(ctx context.Context, tenantID string, key string) (string, error)
	CallCount     int
}

func (m *MockTenantConfigClient) GetConfig(ctx context.Context, tenantID string, key string) (string, error) {
	m.CallCount++
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(ctx, tenantID, key)
	}
	return "", nil
}

func TestCachedTenantConfigClient_CacheHit(t *testing.T) {
	// Arrange
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "IGNORE_STOCK", nil
		},
	}
	
	testCache := cache.NewInMemoryCache[string](10*time.Second, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada (cache miss)
	value1, err1 := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")
	
	// Segunda llamada (debería ser cache hit)
	value2, err2 := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")

	// Assert
	if err1 != nil {
		t.Errorf("Expected no error on first call, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("Expected no error on second call, got %v", err2)
	}
	if value1 != "IGNORE_STOCK" {
		t.Errorf("Expected IGNORE_STOCK on first call, got %s", value1)
	}
	if value2 != "IGNORE_STOCK" {
		t.Errorf("Expected IGNORE_STOCK on second call, got %s", value2)
	}
	
	// Verificar que solo se llamó una vez al servicio (segunda fue cache hit)
	if mockClient.CallCount != 1 {
		t.Errorf("Expected 1 call to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedTenantConfigClient_CacheMiss(t *testing.T) {
	// Arrange
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "REQUIRE_STOCK", nil
		},
	}
	
	testCache := cache.NewInMemoryCache[string](10*time.Second, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	value, err := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")

	// Assert
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != "REQUIRE_STOCK" {
		t.Errorf("Expected REQUIRE_STOCK, got %s", value)
	}
	if mockClient.CallCount != 1 {
		t.Errorf("Expected 1 call to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedTenantConfigClient_ErrorNotCached(t *testing.T) {
	// Arrange
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "", errors.New("service unavailable")
		},
	}
	
	testCache := cache.NewInMemoryCache[string](10*time.Second, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada (error)
	_, err1 := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")
	
	// Segunda llamada (debería volver a llamar al servicio, no usar cache)
	_, err2 := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")

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

func TestCachedTenantConfigClient_EmptyValueCached(t *testing.T) {
	// Arrange - servicio retorna empty string (config no existe)
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "", nil // No existe, pero no es error
		},
	}
	
	testCache := cache.NewInMemoryCache[string](10*time.Second, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada
	value1, err1 := cachedClient.GetConfig(ctx, "tenant-123", "nonexistent.key")
	
	// Segunda llamada (debería ser cache hit)
	value2, err2 := cachedClient.GetConfig(ctx, "tenant-123", "nonexistent.key")

	// Assert
	if err1 != nil {
		t.Errorf("Expected no error on first call, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("Expected no error on second call, got %v", err2)
	}
	if value1 != "" {
		t.Errorf("Expected empty string on first call, got %s", value1)
	}
	if value2 != "" {
		t.Errorf("Expected empty string on second call, got %s", value2)
	}
	
	// Verificar que solo se llamó una vez (empty string se cachea)
	if mockClient.CallCount != 1 {
		t.Errorf("Expected 1 call to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedTenantConfigClient_TTLExpiration(t *testing.T) {
	// Arrange
	callCount := 0
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			callCount++
			if callCount == 1 {
				return "FIRST_VALUE", nil
			}
			return "SECOND_VALUE", nil
		},
	}
	
	// Cache con TTL corto
	testCache := cache.NewInMemoryCache[string](50*time.Millisecond, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act - primera llamada
	value1, _ := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")
	
	// Segunda llamada inmediata (cache hit)
	value2, _ := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")
	
	// Esperar a que expire el cache
	time.Sleep(100 * time.Millisecond)
	
	// Tercera llamada (cache miss por expiración)
	value3, _ := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")

	// Assert
	if value1 != "FIRST_VALUE" {
		t.Errorf("Expected FIRST_VALUE on first call, got %s", value1)
	}
	if value2 != "FIRST_VALUE" {
		t.Errorf("Expected FIRST_VALUE on second call (cache hit), got %s", value2)
	}
	if value3 != "SECOND_VALUE" {
		t.Errorf("Expected SECOND_VALUE on third call (after expiration), got %s", value3)
	}
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service, got %d", mockClient.CallCount)
	}
}

func TestCachedTenantConfigClient_DifferentTenants(t *testing.T) {
	// Arrange
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			if tenantID == "tenant-1" {
				return "IGNORE_STOCK", nil
			}
			return "REQUIRE_STOCK", nil
		},
	}
	
	testCache := cache.NewInMemoryCache[string](10*time.Second, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	value1, _ := cachedClient.GetConfig(ctx, "tenant-1", "catalog.stock_policy")
	value2, _ := cachedClient.GetConfig(ctx, "tenant-2", "catalog.stock_policy")
	
	// Segunda ronda (cache hits)
	value1b, _ := cachedClient.GetConfig(ctx, "tenant-1", "catalog.stock_policy")
	value2b, _ := cachedClient.GetConfig(ctx, "tenant-2", "catalog.stock_policy")

	// Assert
	if value1 != "IGNORE_STOCK" {
		t.Errorf("Expected IGNORE_STOCK for tenant-1, got %s", value1)
	}
	if value2 != "REQUIRE_STOCK" {
		t.Errorf("Expected REQUIRE_STOCK for tenant-2, got %s", value2)
	}
	if value1b != "IGNORE_STOCK" {
		t.Errorf("Expected IGNORE_STOCK for tenant-1 (cached), got %s", value1b)
	}
	if value2b != "REQUIRE_STOCK" {
		t.Errorf("Expected REQUIRE_STOCK for tenant-2 (cached), got %s", value2b)
	}
	
	// Cada tenant debe haber generado una llamada
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service (one per tenant), got %d", mockClient.CallCount)
	}
}

func TestCachedTenantConfigClient_InvalidateConfig(t *testing.T) {
	// Arrange
	callCount := 0
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			callCount++
			if callCount == 1 {
				return "OLD_VALUE", nil
			}
			return "NEW_VALUE", nil
		},
	}
	
	testCache := cache.NewInMemoryCache[string](10*time.Second, 0)
	cachedClient := NewCachedTenantConfigClient(mockClient, testCache)
	ctx := context.Background()

	// Act
	value1, _ := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")
	
	// Invalidar cache (cast al tipo concreto para acceder al método)
	if c, ok := cachedClient.(*CachedTenantConfigClient); ok {
		c.InvalidateConfig(ctx, "tenant-123", "catalog.stock_policy")
	}
	
	// Nueva llamada (debería consultar al servicio de nuevo)
	value2, _ := cachedClient.GetConfig(ctx, "tenant-123", "catalog.stock_policy")

	// Assert
	if value1 != "OLD_VALUE" {
		t.Errorf("Expected OLD_VALUE before invalidation, got %s", value1)
	}
	if value2 != "NEW_VALUE" {
		t.Errorf("Expected NEW_VALUE after invalidation, got %s", value2)
	}
	if mockClient.CallCount != 2 {
		t.Errorf("Expected 2 calls to underlying service, got %d", mockClient.CallCount)
	}
}
