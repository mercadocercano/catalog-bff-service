package domain

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTenantStockPolicy_Default_ReturnsRequireStock(t *testing.T) {
	// Limpiar env var si existe
	os.Unsetenv("FORCE_STOCK_POLICY")

	policy := GetTenantStockPolicy("any-tenant")
	assert.Equal(t, RequireStock, policy)
}

func TestGetTenantStockPolicy_ForcedIgnoreStock(t *testing.T) {
	os.Setenv("FORCE_STOCK_POLICY", "IGNORE_STOCK")
	defer os.Unsetenv("FORCE_STOCK_POLICY")

	policy := GetTenantStockPolicy("any-tenant")
	assert.Equal(t, IgnoreStock, policy)
}

func TestGetTenantStockPolicy_ForcedRequireStock(t *testing.T) {
	os.Setenv("FORCE_STOCK_POLICY", "REQUIRE_STOCK")
	defer os.Unsetenv("FORCE_STOCK_POLICY")

	policy := GetTenantStockPolicy("any-tenant")
	assert.Equal(t, RequireStock, policy)
}

func TestGetTenantStockPolicy_InvalidForcedValue_FallsBackToDefault(t *testing.T) {
	os.Setenv("FORCE_STOCK_POLICY", "INVALID_VALUE")
	defer os.Unsetenv("FORCE_STOCK_POLICY")

	policy := GetTenantStockPolicy("any-tenant")
	assert.Equal(t, RequireStock, policy)
}

func TestGetTenantStockPolicy_DifferentTenants_SameDefault(t *testing.T) {
	os.Unsetenv("FORCE_STOCK_POLICY")

	policy1 := GetTenantStockPolicy("tenant-1")
	policy2 := GetTenantStockPolicy("tenant-2")
	assert.Equal(t, policy1, policy2)
}
