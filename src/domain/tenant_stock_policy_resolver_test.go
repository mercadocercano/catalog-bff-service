package domain

import (
	"context"
	"errors"
	"testing"
)

// MockTenantConfigClient implementa TenantConfigClient para testing
type MockTenantConfigClient struct {
	GetConfigFunc func(ctx context.Context, tenantID string, key string) (string, error)
}

func (m *MockTenantConfigClient) GetConfig(ctx context.Context, tenantID string, key string) (string, error) {
	if m.GetConfigFunc != nil {
		return m.GetConfigFunc(ctx, tenantID, key)
	}
	return "", nil
}

func TestTenantStockPolicyResolver_Resolve_IgnoreStock(t *testing.T) {
	// Arrange
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "IGNORE_STOCK", nil
		},
	}
	resolver := NewTenantStockPolicyResolver(mockClient)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert
	if policy != IgnoreStock {
		t.Errorf("Expected IgnoreStock, got %v", policy)
	}
}

func TestTenantStockPolicyResolver_Resolve_RequireStock(t *testing.T) {
	// Arrange
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "REQUIRE_STOCK", nil
		},
	}
	resolver := NewTenantStockPolicyResolver(mockClient)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert
	if policy != RequireStock {
		t.Errorf("Expected RequireStock, got %v", policy)
	}
}

func TestTenantStockPolicyResolver_Resolve_ValidateStock(t *testing.T) {
	// Arrange: VALIDATE_STOCK debe mapearse a REQUIRE_STOCK
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "VALIDATE_STOCK", nil
		},
	}
	resolver := NewTenantStockPolicyResolver(mockClient)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert
	if policy != RequireStock {
		t.Errorf("Expected RequireStock for VALIDATE_STOCK, got %v", policy)
	}
}

func TestTenantStockPolicyResolver_Resolve_ConfigNotFound(t *testing.T) {
	// Arrange: tenant-service devuelve empty string (404)
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "", nil // No existe configuración
		},
	}
	resolver := NewTenantStockPolicyResolver(mockClient)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert: debe usar fallback
	if policy != RequireStock {
		t.Errorf("Expected RequireStock (fallback), got %v", policy)
	}
}

func TestTenantStockPolicyResolver_Resolve_ServiceError(t *testing.T) {
	// Arrange: tenant-service falla
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "", errors.New("service unavailable")
		},
	}
	resolver := NewTenantStockPolicyResolver(mockClient)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert: debe usar fallback
	if policy != RequireStock {
		t.Errorf("Expected RequireStock (fallback), got %v", policy)
	}
}

func TestTenantStockPolicyResolver_Resolve_UnknownValue(t *testing.T) {
	// Arrange: valor desconocido
	mockClient := &MockTenantConfigClient{
		GetConfigFunc: func(ctx context.Context, tenantID string, key string) (string, error) {
			return "UNKNOWN_POLICY", nil
		},
	}
	resolver := NewTenantStockPolicyResolver(mockClient)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert: debe usar fallback
	if policy != RequireStock {
		t.Errorf("Expected RequireStock (fallback), got %v", policy)
	}
}

func TestTenantStockPolicyResolver_Resolve_NoClient(t *testing.T) {
	// Arrange: resolver sin client
	resolver := NewTenantStockPolicyResolver(nil)

	// Act
	policy := resolver.Resolve(context.Background(), "tenant-123")

	// Assert: debe usar fallback
	if policy != RequireStock {
		t.Errorf("Expected RequireStock (fallback), got %v", policy)
	}
}
