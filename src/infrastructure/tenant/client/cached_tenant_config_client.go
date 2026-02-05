package client

import (
	"context"
	"fmt"
	"log"

	"catalog-bff-service/src/domain"
)

// CachedTenantConfigClient envuelve un TenantConfigClient con cache
// Implementa el patrón Decorator para agregar caching transparente
type CachedTenantConfigClient struct {
	underlying domain.TenantConfigClient
	cache      domain.Cache[string]
}

// NewCachedTenantConfigClient crea un client con cache
func NewCachedTenantConfigClient(
	underlying domain.TenantConfigClient,
	cache domain.Cache[string],
) domain.TenantConfigClient {
	return &CachedTenantConfigClient{
		underlying: underlying,
		cache:      cache,
	}
}

// GetConfig obtiene configuración con cache
// 
// Estrategia:
// 1. Intenta leer del cache
// 2. Si cache miss, consulta al servicio
// 3. Si el servicio responde OK, cachea el resultado
// 4. Si el servicio falla, NO cachea (fallback inmediato)
// 5. Nunca falla: siempre propaga el error del underlying
func (c *CachedTenantConfigClient) GetConfig(ctx context.Context, tenantID string, key string) (string, error) {
	// Generar cache key
	cacheKey := domain.CacheKey("tenant_config", tenantID, key)

	// 1. Intentar leer del cache
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		log.Printf("[CachedTenantConfigClient] Cache HIT for tenant=%s key=%s", tenantID, key)
		return cachedValue, nil
	}

	log.Printf("[CachedTenantConfigClient] Cache MISS for tenant=%s key=%s", tenantID, key)

	// 2. Cache miss: consultar al servicio
	value, err := c.underlying.GetConfig(ctx, tenantID, key)

	// 3. Si hubo error, NO cachear (propagar error)
	if err != nil {
		log.Printf("[CachedTenantConfigClient] Error from underlying service: %v (not caching)", err)
		return "", err
	}

	// 4. Cachear el resultado (incluso si es empty string = "no existe")
	// Esto evita consultas repetidas al servicio para configs inexistentes
	c.cache.Set(ctx, cacheKey, value, 0) // Usa TTL por defecto del cache
	log.Printf("[CachedTenantConfigClient] Cached value for tenant=%s key=%s", tenantID, key)

	return value, nil
}

// InvalidateConfig permite invalidar manualmente una entrada del cache
// Útil si se sabe que cambió la configuración (ej: webhook, admin update)
func (c *CachedTenantConfigClient) InvalidateConfig(ctx context.Context, tenantID string, key string) {
	cacheKey := domain.CacheKey("tenant_config", tenantID, key)
	c.cache.Delete(ctx, cacheKey)
	log.Printf("[CachedTenantConfigClient] Invalidated cache for tenant=%s key=%s", tenantID, key)
}

// InvalidateTenant invalida todas las configs de un tenant
func (c *CachedTenantConfigClient) InvalidateTenant(ctx context.Context, tenantID string) {
	// Nota: sync.Map no soporta prefix scan eficiente
	// Para implementar esto correctamente, se necesitaría un índice adicional
	// Por ahora, dejamos este método como placeholder
	log.Printf("[CachedTenantConfigClient] InvalidateTenant not fully implemented (tenant=%s)", tenantID)
	fmt.Println("⚠️  InvalidateTenant requires prefix scan - not implemented with sync.Map")
}
