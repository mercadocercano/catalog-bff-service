# Estrategia de Cache In-Memory - Catalog BFF Service

## 📋 Resumen

Este documento describe la estrategia de cache implementada en `catalog-bff-service` para mejorar la performance y reducir la latencia de consultas a servicios externos.

**Tipo de cache:** In-memory (sin dependencias externas)  
**Patrón:** Best-effort caching (nunca rompe requests)  
**Arquitectura:** Decorator pattern con inyección de dependencias

---

## 🎯 Objetivos

1. **Reducir latencia** de consultas repetidas a tenant-service y stock-service
2. **Disminuir carga** en servicios upstream
3. **Mantener simplicidad** (sin Redis ni dependencias externas)
4. **Garantizar seguridad** (cache nunca debe romper funcionalidad)

---

## ✅ Qué SE Cachea

### 1. Tenant Configuration (Stock Policy)

**Endpoint cacheado:** `tenant-service → GET /api/v1/tenant/config/{key}`

**Cache Key:** `tenant_config:{tenant_id}:{config_key}`

**TTL por defecto:** 60 segundos (configurable con `TENANT_CONFIG_CACHE_TTL`)

**Razón:**
- La configuración de tenant cambia raramente
- Se consulta en cada request de variantes vendibles
- Alto impacto en performance (reduce latencia ~100-200ms)

**Comportamiento:**
- ✅ Cachea valores exitosos (incluyendo empty string = "no existe")
- ❌ NO cachea errores (permite reintentos inmediatos)
- ✅ Fallback seguro: si falla cache, consulta al servicio

**Ejemplo:**
```go
// Primera llamada: consulta tenant-service (200ms)
policy := resolver.Resolve(ctx, "tenant-123") // → REQUIRE_STOCK

// Segunda llamada: cache hit (< 1ms)
policy := resolver.Resolve(ctx, "tenant-123") // → REQUIRE_STOCK (cached)
```

---

### 2. Stock Availability

**Endpoint cacheado:** `stock-service → GET /api/v1/availability?sku={sku}`

**Cache Key:** `stock:{tenant_id}:{sku}`

**TTL por defecto:** 5 segundos (configurable con `STOCK_CACHE_TTL`)

**Razón:**
- Stock cambia frecuentemente pero no en tiempo real
- Se consulta para cada variante en catálogo (N consultas en paralelo)
- TTL corto (5s) balancea freshness vs performance

**Comportamiento:**
- ✅ Cachea respuestas exitosas con stock (incluyendo cantidad = 0)
- ❌ NO cachea nil (404 - no existe stock)
- ❌ NO cachea errores
- ✅ Fallback seguro: si falla cache o servicio, retorna 0

**Ejemplo:**
```go
// Primera llamada: consulta stock-service (150ms)
stock := client.GetAvailability(ctx, "tenant-123", "SKU-001") // → 100 units

// Segunda llamada dentro de 5s: cache hit (< 1ms)
stock := client.GetAvailability(ctx, "tenant-123", "SKU-001") // → 100 units (cached)

// Después de 5s: cache miss, consulta de nuevo
stock := client.GetAvailability(ctx, "tenant-123", "SKU-001") // → 95 units (fresh)
```

---

## ❌ Qué NO SE Cachea

### 1. Writes (Creación/Actualización)

**Razón:** Cache solo para lecturas (CQRS pattern)

**Servicios afectados:**
- ❌ POST/PUT/DELETE a cualquier servicio
- ❌ Operaciones que cambian estado

---

### 2. Errores de Servicios

**Razón:** Permitir reintentos inmediatos sin esperar TTL

**Casos:**
- ❌ 500 Internal Server Error
- ❌ Timeouts
- ❌ Network errors

**Comportamiento:** Si el servicio falla, el error se propaga y NO se cachea.

---

### 3. Respuestas Nil (Stock No Existe)

**Razón:** Stock puede crearse en cualquier momento

**Caso específico:**
- ❌ Stock-service retorna 404 (no existe registro de stock)

**Comportamiento:** Se retorna nil sin cachear, permitiendo que la próxima consulta detecte si se creó stock.

---

### 4. Datos de PIM (Variantes/Productos)

**Razón:** PIM ya es fuente de verdad y puede cambiar frecuentemente

**Servicios NO cacheados:**
- ❌ GET /api/v1/product-variants
- ❌ GET /api/v1/product-variants/{id}

**Consideración futura:** Si PIM se vuelve bottleneck, se puede agregar cache con TTL muy corto (1-2s).

---

## ⚙️ Configuración

### Variables de Entorno

```bash
# TTL de cache de configuración de tenant (default: 60s)
TENANT_CONFIG_CACHE_TTL=60s

# TTL de cache de stock (default: 5s)
STOCK_CACHE_TTL=5s

# Intervalo de limpieza automática de cache (default: 60s)
CACHE_CLEANUP_INTERVAL=60s
```

### Ejemplos de Configuración

**Alta frecuencia de cambios de stock:**
```bash
STOCK_CACHE_TTL=2s  # Cache más agresivo
```

**Tenant config muy estable:**
```bash
TENANT_CONFIG_CACHE_TTL=300s  # 5 minutos
```

**Deshabilitar cleanup automático (testing):**
```bash
CACHE_CLEANUP_INTERVAL=0
```

---

## 🏗️ Arquitectura

### Componentes

```
┌─────────────────────────────────────────────────────────┐
│                     Handler Layer                        │
│  (SellableVariantsHandler, VariantHandler)              │
└─────────────────┬───────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│                  Domain Layer                            │
│  • TenantStockPolicyResolver                            │
│  • Cache[T] interface (generic)                         │
└─────────────────┬───────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│              Infrastructure Layer                        │
│                                                          │
│  ┌────────────────────────────────────────────────┐    │
│  │  cache/                                         │    │
│  │  • InMemoryCache[T] (sync.Map + TTL)          │    │
│  └────────────────────────────────────────────────┘    │
│                                                          │
│  ┌────────────────────────────────────────────────┐    │
│  │  tenant/client/                                 │    │
│  │  • HTTPTenantConfigClient (base)               │    │
│  │  • CachedTenantConfigClient (decorator)        │    │
│  └────────────────────────────────────────────────┘    │
│                                                          │
│  ┌────────────────────────────────────────────────┐    │
│  │  stock/client/                                  │    │
│  │  • HTTPStockAvailabilityClient (base)          │    │
│  │  • CachedStockAvailabilityClient (decorator)   │    │
│  └────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### Patrón Decorator

```go
// Sin cache
baseClient := NewHTTPTenantConfigClient(url)

// Con cache (transparente)
cachedClient := NewCachedTenantConfigClient(baseClient, cache)

// Uso idéntico
value, err := cachedClient.GetConfig(ctx, tenantID, key)
```

**Ventajas:**
- ✅ Inyección de dependencias limpia
- ✅ Fácil de testear (mock del underlying client)
- ✅ Cache opcional (se puede deshabilitar sin cambiar código)

---

## 🧪 Testing

### Cobertura de Tests

**InMemoryCache:**
- ✅ Get/Set básico
- ✅ Cache miss
- ✅ TTL expiration
- ✅ Custom TTL
- ✅ Delete/Clear
- ✅ Acceso concurrente
- ✅ Auto-cleanup
- ✅ Tipos genéricos (structs, pointers, nil)

**CachedTenantConfigClient:**
- ✅ Cache hit/miss
- ✅ Errores no cacheados
- ✅ Empty string cacheado
- ✅ TTL expiration
- ✅ Multi-tenant isolation
- ✅ Invalidación manual

**CachedStockAvailabilityClient:**
- ✅ Cache hit/miss
- ✅ Errores no cacheados
- ✅ Nil (404) no cacheado
- ✅ Zero quantity cacheado
- ✅ TTL expiration
- ✅ Multi-tenant + multi-SKU isolation
- ✅ Invalidación manual

### Ejecutar Tests

```bash
# Todos los tests
go test ./src/...

# Tests de cache específicamente
go test ./src/infrastructure/cache/...
go test ./src/infrastructure/tenant/client/...
go test ./src/infrastructure/stock/client/...

# Con verbose
go test -v ./src/infrastructure/cache/...
```

---

## ⚠️ Riesgos Aceptados

### 1. Stale Data (Datos Desactualizados)

**Riesgo:**
- Tenant cambia stock policy → cache retorna valor viejo por hasta 60s
- Stock se actualiza → cache retorna cantidad vieja por hasta 5s

**Mitigación:**
- TTLs cortos (60s tenant config, 5s stock)
- Invalidación manual disponible (webhooks futuros)
- Fallback seguro: RequireStock es el default más conservador

**Aceptado porque:**
- ✅ Stock policy cambia raramente
- ✅ 5s de staleness en stock es aceptable para UX
- ✅ Peor caso: se muestra producto vendible cuando no lo es → error en checkout (validación final)

---

### 2. Memory Leaks (Sin Límite de Tamaño)

**Riesgo:**
- Cache crece indefinidamente si hay muchos tenants/SKUs

**Mitigación:**
- Cleanup automático cada 60s elimina entradas expiradas
- TTLs cortos limitan crecimiento
- Servicio stateless → restart limpia memoria

**Aceptado porque:**
- ✅ Catalog-bff es stateless (se puede reiniciar sin impacto)
- ✅ Uso típico: pocos tenants activos simultáneos
- ✅ Cada entrada es pequeña (~100 bytes)

**Estimación:**
- 1000 tenants × 1000 SKUs × 100 bytes = ~100 MB (aceptable)

---

### 3. Cache Stampede (Thundering Herd)

**Riesgo:**
- Cache expira → N requests simultáneos consultan el servicio

**Mitigación:**
- Parcial: TTLs escalonados por tenant/SKU (diferentes timestamps)
- NO implementado: cache locking (complejidad vs beneficio)

**Aceptado porque:**
- ✅ Servicios upstream (tenant/stock) soportan carga
- ✅ Timeouts agresivos (500ms) evitan bloqueos
- ✅ Impacto limitado (pocos requests simultáneos por tenant)

---

### 4. No Hay Invalidación Proactiva

**Riesgo:**
- Cambios en tenant-service o stock-service no invalidan cache automáticamente

**Mitigación:**
- TTLs cortos
- Métodos de invalidación manual disponibles:
  - `CachedTenantConfigClient.InvalidateConfig()`
  - `CachedStockAvailabilityClient.InvalidateStock()`

**Aceptado porque:**
- ✅ Implementar webhooks/eventos agrega complejidad
- ✅ TTLs cortos son suficientes para MVP
- ✅ Invalidación manual disponible para casos críticos

**Futuro:** Integrar con eventos de tenant-service/stock-service para invalidación automática.

---

## 📊 Métricas de Éxito

### KPIs Esperados

**Latencia:**
- ✅ Reducción de 50-70% en latencia de `/sellable-variants`
- ✅ Cache hit rate > 80% para tenant config
- ✅ Cache hit rate > 60% para stock (depende de tráfico)

**Carga en Servicios:**
- ✅ Reducción de 50-80% en requests a tenant-service
- ✅ Reducción de 40-60% en requests a stock-service

**Estabilidad:**
- ✅ Sin errores introducidos por cache
- ✅ Fallback funciona correctamente (tests de integración)

---

## 🔮 Evolución Futura

### Posibles Mejoras

**Corto plazo:**
- [ ] Métricas de cache (hit rate, miss rate, latency)
- [ ] Logs estructurados con tenant_id para debugging
- [ ] Health check que valide cache está funcionando

**Mediano plazo:**
- [ ] Invalidación proactiva vía eventos (tenant-service/stock-service)
- [ ] Cache distribuido (Redis) si se escala horizontalmente
- [ ] Cache warming en startup (pre-cargar tenants activos)

**Largo plazo:**
- [ ] Cache adaptativo (TTL dinámico según hit rate)
- [ ] Cache stampede protection (singleflight pattern)
- [ ] LRU eviction policy (límite de tamaño)

---

## 🔗 Referencias

### Código Relacionado

- `src/domain/cache.go` - Interface genérica de cache
- `src/infrastructure/cache/in_memory_cache.go` - Implementación in-memory
- `src/infrastructure/tenant/client/cached_tenant_config_client.go` - Wrapper con cache
- `src/infrastructure/stock/client/stock_client.go` - Cliente de stock con cache
- `main.go` - Inyección de dependencias y configuración

### Documentos Relacionados

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Decisión de arquitectura del servicio
- [README.md](./README.md) - Documentación general del servicio

### Patrones Aplicados

- **Decorator Pattern** - Para agregar cache transparente
- **Dependency Injection** - Para testabilidad
- **Best-Effort Caching** - Cache nunca rompe funcionalidad
- **CQRS Light** - Cache solo en lecturas, no en writes

---

## 📝 Changelog

**2026-02-03** - Implementación inicial
- ✅ Cache in-memory con TTL
- ✅ Tenant config caching (60s TTL)
- ✅ Stock availability caching (5s TTL)
- ✅ Tests unitarios completos
- ✅ Documentación de estrategia

---

## ❓ FAQ

**P: ¿Por qué no usar Redis?**  
R: Catalog-bff es stateless y no necesita persistencia. In-memory es más simple y suficiente para MVP.

**P: ¿Qué pasa si el cache falla?**  
R: El cache nunca falla: si hay error, consulta al servicio directamente (fallback seguro).

**P: ¿Cómo invalido el cache manualmente?**  
R: Usa los métodos `InvalidateConfig()` o `InvalidateStock()` de los clientes cacheados.

**P: ¿El cache es thread-safe?**  
R: Sí, usa `sync.Map` internamente para acceso concurrente seguro.

**P: ¿Puedo deshabilitar el cache?**  
R: Sí, no inyectes el wrapper cacheado en `main.go` (usa el cliente base directamente).

**P: ¿Qué pasa si tengo muchos tenants?**  
R: El cleanup automático elimina entradas expiradas. Si crece mucho, considera Redis.

---

**Estado:** ✅ Implementado y testeado  
**Última actualización:** 2026-02-03  
**Autor:** Catalog BFF Team
