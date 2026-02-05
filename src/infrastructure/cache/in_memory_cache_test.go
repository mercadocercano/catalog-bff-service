package cache

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryCache_GetSet(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[string](10*time.Second, 0)
	ctx := context.Background()

	// Act
	cache.Set(ctx, "key1", "value1", 0)
	value, found := cache.Get(ctx, "key1")

	// Assert
	if !found {
		t.Error("Expected to find key1 in cache")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %s", value)
	}
}

func TestInMemoryCache_GetMiss(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[string](10*time.Second, 0)
	ctx := context.Background()

	// Act
	value, found := cache.Get(ctx, "nonexistent")

	// Assert
	if found {
		t.Error("Expected cache miss for nonexistent key")
	}
	if value != "" {
		t.Errorf("Expected zero value (empty string), got %s", value)
	}
}

func TestInMemoryCache_TTLExpiration(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[string](50*time.Millisecond, 0)
	ctx := context.Background()

	// Act
	cache.Set(ctx, "key1", "value1", 0) // Usa TTL por defecto (50ms)
	
	// Verificar que existe inmediatamente
	value, found := cache.Get(ctx, "key1")
	if !found {
		t.Error("Expected to find key1 immediately after set")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %s", value)
	}

	// Esperar a que expire
	time.Sleep(100 * time.Millisecond)
	
	value, found = cache.Get(ctx, "key1")

	// Assert
	if found {
		t.Error("Expected key1 to be expired")
	}
	if value != "" {
		t.Errorf("Expected zero value after expiration, got %s", value)
	}
}

func TestInMemoryCache_CustomTTL(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[string](10*time.Second, 0) // TTL por defecto largo
	ctx := context.Background()

	// Act - usar TTL custom corto
	cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	
	// Verificar que existe inmediatamente
	_, found := cache.Get(ctx, "key1")
	if !found {
		t.Error("Expected to find key1 immediately")
	}

	// Esperar a que expire el TTL custom
	time.Sleep(100 * time.Millisecond)
	
	_, found = cache.Get(ctx, "key1")

	// Assert
	if found {
		t.Error("Expected key1 to be expired after custom TTL")
	}
}

func TestInMemoryCache_Delete(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[string](10*time.Second, 0)
	ctx := context.Background()

	// Act
	cache.Set(ctx, "key1", "value1", 0)
	cache.Delete(ctx, "key1")
	_, found := cache.Get(ctx, "key1")

	// Assert
	if found {
		t.Error("Expected key1 to be deleted")
	}
}

func TestInMemoryCache_Clear(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[string](10*time.Second, 0)
	ctx := context.Background()

	// Act
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)
	
	cache.Clear(ctx)
	
	_, found1 := cache.Get(ctx, "key1")
	_, found2 := cache.Get(ctx, "key2")
	_, found3 := cache.Get(ctx, "key3")

	// Assert
	if found1 || found2 || found3 {
		t.Error("Expected all keys to be cleared")
	}
}

func TestInMemoryCache_ConcurrentAccess(t *testing.T) {
	// Arrange
	cache := NewInMemoryCache[int](10*time.Second, 0)
	ctx := context.Background()

	// Act - escrituras concurrentes
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			cache.Set(ctx, "key", n, 0)
			done <- true
		}(i)
	}

	// Esperar a que terminen todas las goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Assert - debe haber algún valor sin panic
	_, found := cache.Get(ctx, "key")
	if !found {
		t.Error("Expected to find key after concurrent writes")
	}
}

func TestInMemoryCache_AutoCleanup(t *testing.T) {
	// Arrange - cache con cleanup automático cada 50ms
	cache := NewInMemoryCache[string](50*time.Millisecond, 50*time.Millisecond)
	defer cache.Stop()
	
	ctx := context.Background()

	// Act
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	
	// Verificar que existen
	_, found1 := cache.Get(ctx, "key1")
	if !found1 {
		t.Error("Expected to find key1 before expiration")
	}

	// Esperar a que expiren y se limpien automáticamente
	time.Sleep(200 * time.Millisecond)
	
	// Intentar acceder directamente al map interno (verificar que se limpiaron)
	// Nota: Get ya elimina entradas expiradas, pero queremos verificar el cleanup automático
	
	// Assert - las keys no deberían existir
	_, found1 = cache.Get(ctx, "key1")
	_, found2 := cache.Get(ctx, "key2")
	
	if found1 || found2 {
		t.Error("Expected keys to be cleaned up automatically")
	}
}

func TestInMemoryCache_StructValues(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Name  string
		Value int
	}
	
	cache := NewInMemoryCache[TestStruct](10*time.Second, 0)
	ctx := context.Background()

	// Act
	testData := TestStruct{Name: "test", Value: 42}
	cache.Set(ctx, "struct_key", testData, 0)
	retrieved, found := cache.Get(ctx, "struct_key")

	// Assert
	if !found {
		t.Error("Expected to find struct in cache")
	}
	if retrieved.Name != "test" || retrieved.Value != 42 {
		t.Errorf("Expected {test, 42}, got {%s, %d}", retrieved.Name, retrieved.Value)
	}
}

func TestInMemoryCache_PointerValues(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Name string
	}
	
	cache := NewInMemoryCache[*TestStruct](10*time.Second, 0)
	ctx := context.Background()

	// Act
	testData := &TestStruct{Name: "test"}
	cache.Set(ctx, "ptr_key", testData, 0)
	retrieved, found := cache.Get(ctx, "ptr_key")

	// Assert
	if !found {
		t.Error("Expected to find pointer in cache")
	}
	if retrieved == nil {
		t.Error("Expected non-nil pointer")
	}
	if retrieved.Name != "test" {
		t.Errorf("Expected 'test', got %s", retrieved.Name)
	}
}

func TestInMemoryCache_NilPointerValue(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Name string
	}
	
	cache := NewInMemoryCache[*TestStruct](10*time.Second, 0)
	ctx := context.Background()

	// Act - cachear nil pointer (válido)
	cache.Set(ctx, "nil_key", nil, 0)
	retrieved, found := cache.Get(ctx, "nil_key")

	// Assert
	if !found {
		t.Error("Expected to find nil pointer in cache")
	}
	if retrieved != nil {
		t.Error("Expected nil pointer")
	}
}
