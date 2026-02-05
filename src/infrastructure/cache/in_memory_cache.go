package cache

import (
	"context"
	"sync"
	"time"
)

// cacheEntry representa una entrada en el cache con timestamp de expiración
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// InMemoryCache implementa un cache thread-safe en memoria con TTL
// Usa sync.Map para mejor performance en lecturas concurrentes
type InMemoryCache[T any] struct {
	data       sync.Map
	defaultTTL time.Duration
	// Cleanup goroutine para evitar memory leaks
	stopCleanup chan struct{}
	cleanupOnce sync.Once
}

// NewInMemoryCache crea una nueva instancia del cache
// defaultTTL: tiempo de vida por defecto para entradas (si se pasa 0 en Set)
// cleanupInterval: frecuencia de limpieza de entradas expiradas (0 = sin cleanup automático)
func NewInMemoryCache[T any](defaultTTL time.Duration, cleanupInterval time.Duration) *InMemoryCache[T] {
	cache := &InMemoryCache[T]{
		defaultTTL:  defaultTTL,
		stopCleanup: make(chan struct{}),
	}

	// Iniciar goroutine de limpieza si se especificó intervalo
	if cleanupInterval > 0 {
		go cache.startCleanup(cleanupInterval)
	}

	return cache
}

// Get obtiene un valor del cache
func (c *InMemoryCache[T]) Get(ctx context.Context, key string) (T, bool) {
	var zero T

	value, ok := c.data.Load(key)
	if !ok {
		return zero, false
	}

	entry := value.(cacheEntry[T])

	// Verificar si expiró
	if time.Now().After(entry.expiresAt) {
		// Eliminar entrada expirada
		c.data.Delete(key)
		return zero, false
	}

	return entry.value, true
}

// Set almacena un valor en el cache
func (c *InMemoryCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) {
	if ttl == 0 {
		ttl = c.defaultTTL
	}

	entry := cacheEntry[T]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	c.data.Store(key, entry)
}

// Delete elimina una entrada del cache
func (c *InMemoryCache[T]) Delete(ctx context.Context, key string) {
	c.data.Delete(key)
}

// Clear limpia todo el cache
func (c *InMemoryCache[T]) Clear(ctx context.Context) {
	c.data.Range(func(key, value interface{}) bool {
		c.data.Delete(key)
		return true
	})
}

// Stop detiene el cleanup goroutine (llamar al cerrar la aplicación)
func (c *InMemoryCache[T]) Stop() {
	c.cleanupOnce.Do(func() {
		close(c.stopCleanup)
	})
}

// startCleanup ejecuta limpieza periódica de entradas expiradas
func (c *InMemoryCache[T]) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

// cleanup elimina todas las entradas expiradas
func (c *InMemoryCache[T]) cleanup() {
	now := time.Now()
	c.data.Range(func(key, value interface{}) bool {
		entry := value.(cacheEntry[T])
		if now.After(entry.expiresAt) {
			c.data.Delete(key)
		}
		return true
	})
}
