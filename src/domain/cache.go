package domain

import (
	"context"
	"time"
)

// Cache define el contrato para un cache genérico con TTL
// Esta interface vive en domain para evitar dependencias de infraestructura
type Cache[T any] interface {
	// Get obtiene un valor del cache
	// Retorna (value, true) si existe y no ha expirado
	// Retorna (zero-value, false) si no existe o expiró
	Get(ctx context.Context, key string) (T, bool)

	// Set almacena un valor en el cache con TTL
	// Si ttl es 0, usa el TTL por defecto del cache
	Set(ctx context.Context, key string, value T, ttl time.Duration)

	// Delete elimina una entrada del cache
	Delete(ctx context.Context, key string)

	// Clear limpia todo el cache (útil para testing)
	Clear(ctx context.Context)
}

// CacheKey genera una key compuesta para cache multi-tenant
// Formato: "prefix:tenant_id:suffix"
func CacheKey(prefix, tenantID, suffix string) string {
	return prefix + ":" + tenantID + ":" + suffix
}
