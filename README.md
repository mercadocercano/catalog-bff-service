# Catalog BFF Service

**Backend for Frontend (BFF)** de orquestación de lecturas agregadas para el proyecto SaaS Multi-Tenant "Mercado Cercano".

📦 **Repositorio oficial:** https://github.com/mercadocercano/catalog-bff-service

## 🎯 Propósito

Este servicio **NO es un dominio nuevo**. Es un **servicio de composición** (API Composition Pattern) que:

- ✅ Orquesta consultas a múltiples servicios
- ✅ Mergea respuestas en vistas unificadas
- ❌ No persiste datos
- ❌ No tiene reglas de negocio propias

**Rol:** Backend for Frontend (BFF) simplificado - separación query/command (CQRS light).

> 📖 **Decisión de arquitectura completa:** Ver [ARCHITECTURE.md](./ARCHITECTURE.md)

---

## 🏗️ Arquitectura

```
Frontend → catalog-service → [PIM + Stock] → Response unificada
```

**Patrón aplicado:**
- API Composition Pattern
- CQRS (separación lectura/escritura)
- Stateless orchestration

---

## 📊 Endpoints Disponibles

### 1. Admin Dashboard Stats

Endpoint agregado que consolida métricas de múltiples servicios para el dashboard de administración.

```
GET /api/v1/admin/dashboard/stats
```

**Headers:**
- `Authorization`: Bearer token JWT (obligatorio)

**Respuesta (200 OK):**
```json
{
  "curation": {
    "pending": 12,
    "approved_today": 5,
    "rejected_today": 2,
    "total_scraped": 1543
  },
  "catalog": {
    "total_products": 2341,
    "total_variants": 8923,
    "active_products": 2103,
    "categories_count": 45,
    "top_categories": [...]
  },
  "tenants": {
    "total": 15,
    "active": 14,
    "new_this_month": 3,
    "recent": [...]
  },
  "services": [...]
}
```

**Servicios orquestados:**
- PIM Service (curación y catálogo)
- Scraper Service (productos scrapeados)
- Tenant Service (tenants activos)
- Health checks de todos los servicios

**Documentación completa:** Ver [DASHBOARD_ENDPOINT.md](./DASHBOARD_ENDPOINT.md)

---

### 2. HITO 1: Variante + Stock

Endpoint que agrega datos de PIM y Stock para consultas unificadas.

### Endpoint

```
GET /api/v1/catalog/variants/{variant_id}
```

**Headers:**
- `X-Tenant-ID`: UUID del tenant (obligatorio)
- `Authorization`: Bearer token JWT (opcional según configuración Kong)

**Respuesta (200 OK):**
```json
{
  "variant_id": "69056319-124f-469b-a60f-2e494d1718fd",
  "product_id": "636f97af-44d1-41d5-be5e-fd3af2a18bf0",
  "product_name": "",
  "variant_name": "Coca Cola Test Hito1 - Default",
  "sku": "COC-HITO1",
  "is_default": true,
  "stock": {
    "available": 10,
    "reserved": 0,
    "total": 10
  }
}
```

### Orquestación

1. Llama a `pim-service`: `GET /api/v1/product-variants/{variant_id}`
2. Extrae el SKU de la variante
3. Llama a `stock-service`: `GET /api/v1/availability?sku={sku}`
4. Merge de respuestas

### Variables de entorno

**Servicios:**
- `PIM_SERVICE_URL`: URL del PIM service (default: `http://localhost:8090`)
- `STOCK_SERVICE_URL`: URL del Stock service (default: `http://localhost:8100`)
- `TENANT_SERVICE_URL`: URL del Tenant service (opcional)
- `SCRAPER_SERVICE_URL`: URL del Scraper service (default: `http://localhost:8086`)
- `IAM_SERVICE_URL`: URL del IAM service (default: `http://localhost:8080`)
- `PORT`: Puerto del servicio (default: `8085`)

**Cache (opcional):**
- `TENANT_CONFIG_CACHE_TTL`: TTL para cache de configuración de tenant (default: `60s`)
- `STOCK_CACHE_TTL`: TTL para cache de stock (default: `5s`)
- `CACHE_CLEANUP_INTERVAL`: Intervalo de limpieza automática (default: `60s`)

### Ejecución local

```bash
go run main.go
```

### Docker

```bash
docker build -t catalog-service .
docker run -p 8085:8085 catalog-service
```

---

## 🚀 Cache In-Memory

Este servicio implementa cache in-memory para mejorar la performance:

**Qué se cachea:**
- ✅ Tenant configuration (stock policy) - TTL: 60s
- ✅ Stock availability por SKU - TTL: 5s

**Características:**
- Cache best-effort (nunca rompe requests)
- Fallback automático si falla
- Thread-safe (sync.Map)
- Cleanup automático de entradas expiradas

**Documentación completa:** Ver [CACHE_STRATEGY.md](./CACHE_STRATEGY.md)

**Configuración:**
```bash
# Ejemplo: cache más agresivo
TENANT_CONFIG_CACHE_TTL=120s
STOCK_CACHE_TTL=10s
```
