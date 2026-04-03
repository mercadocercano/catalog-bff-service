package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheKey_StandardFormat(t *testing.T) {
	key := CacheKey("tenant_config", "tenant-123", "catalog.stock_policy")
	assert.Equal(t, "tenant_config:tenant-123:catalog.stock_policy", key)
}

func TestCacheKey_StockFormat(t *testing.T) {
	key := CacheKey("stock", "tenant-abc", "SKU-001")
	assert.Equal(t, "stock:tenant-abc:SKU-001", key)
}

func TestCacheKey_EmptyValues(t *testing.T) {
	key := CacheKey("", "", "")
	assert.Equal(t, "::", key)
}

func TestCacheKey_DifferentTenants_ProduceDifferentKeys(t *testing.T) {
	key1 := CacheKey("config", "tenant-1", "key")
	key2 := CacheKey("config", "tenant-2", "key")
	assert.NotEqual(t, key1, key2)
}

func TestCacheKey_DifferentSuffixes_ProduceDifferentKeys(t *testing.T) {
	key1 := CacheKey("stock", "tenant-1", "SKU-001")
	key2 := CacheKey("stock", "tenant-1", "SKU-002")
	assert.NotEqual(t, key1, key2)
}

func TestCacheKey_DifferentPrefixes_ProduceDifferentKeys(t *testing.T) {
	key1 := CacheKey("stock", "tenant-1", "item")
	key2 := CacheKey("config", "tenant-1", "item")
	assert.NotEqual(t, key1, key2)
}
