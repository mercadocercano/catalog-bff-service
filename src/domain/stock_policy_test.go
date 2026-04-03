package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSellable_RequireStock_WithPositiveQuantity(t *testing.T) {
	result := IsSellable(RequireStock, 10)
	assert.True(t, result)
}

func TestIsSellable_RequireStock_WithZeroQuantity(t *testing.T) {
	result := IsSellable(RequireStock, 0)
	assert.False(t, result)
}

func TestIsSellable_RequireStock_WithNegativeQuantity(t *testing.T) {
	result := IsSellable(RequireStock, -5)
	assert.False(t, result)
}

func TestIsSellable_IgnoreStock_WithPositiveQuantity(t *testing.T) {
	result := IsSellable(IgnoreStock, 10)
	assert.True(t, result)
}

func TestIsSellable_IgnoreStock_WithZeroQuantity(t *testing.T) {
	result := IsSellable(IgnoreStock, 0)
	assert.True(t, result)
}

func TestIsSellable_IgnoreStock_WithNegativeQuantity(t *testing.T) {
	result := IsSellable(IgnoreStock, -5)
	assert.True(t, result)
}

func TestIsSellable_UnknownPolicy_FallsBackToRequireStock(t *testing.T) {
	unknownPolicy := StockPolicy("UNKNOWN")

	assert.True(t, IsSellable(unknownPolicy, 10))
	assert.False(t, IsSellable(unknownPolicy, 0))
	assert.False(t, IsSellable(unknownPolicy, -1))
}

func TestIsSellable_EmptyPolicy_FallsBackToRequireStock(t *testing.T) {
	emptyPolicy := StockPolicy("")

	assert.True(t, IsSellable(emptyPolicy, 1))
	assert.False(t, IsSellable(emptyPolicy, 0))
}

func TestIsSellable_RequireStock_FractionalQuantity(t *testing.T) {
	assert.True(t, IsSellable(RequireStock, 0.001))
	assert.False(t, IsSellable(RequireStock, 0.0))
}

func TestStockPolicy_Constants(t *testing.T) {
	assert.Equal(t, StockPolicy("REQUIRE_STOCK"), RequireStock)
	assert.Equal(t, StockPolicy("IGNORE_STOCK"), IgnoreStock)
}
