# Dashboard Stats Endpoint

## Descripción

Endpoint agregado que consolida métricas de múltiples microservicios para el dashboard del marketplace-admin.

## Endpoint

```
GET /api/v1/admin/dashboard/stats
```

## Autenticación

- **Header requerido**: `Authorization: Bearer <jwt_token>`
- **Roles permitidos**: `marketplace_admin`, `admin`

## Servicios Orquestados

El endpoint hace llamadas paralelas a los siguientes servicios:

1. **PIM Service** (`:8090`) - Estadísticas de productos y curación
2. **Scraper Service** (`:8086`) - Productos scrapeados
3. **Tenant Service** (`:8070`) - Información de tenants
4. **IAM Service** (`:8080`) - Health check

## Response Format

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
    "top_categories": [
      {
        "id": "uuid",
        "name": "Herramientas",
        "count": 543
      },
      {
        "id": "uuid",
        "name": "Materiales",
        "count": 412
      }
    ]
  },
  "tenants": {
    "total": 15,
    "active": 14,
    "new_this_month": 3,
    "recent": [
      {
        "id": "uuid",
        "name": "Ferretería El Tornillo",
        "plan": "pro",
        "status": "active",
        "last_activity": "2024-02-08T10:30:00Z"
      }
    ]
  },
  "services": [
    {
      "name": "pim-service",
      "status": "up",
      "latency_ms": 45,
      "uptime_percent": 99.8,
      "last_check": "2024-02-08T12:00:00Z"
    },
    {
      "name": "scraper-service",
      "status": "up",
      "latency_ms": 120,
      "uptime_percent": 99.5,
      "last_check": "2024-02-08T12:00:00Z"
    },
    {
      "name": "iam-service",
      "status": "up",
      "latency_ms": 30,
      "uptime_percent": 99.9,
      "last_check": "2024-02-08T12:00:00Z"
    },
    {
      "name": "tenant-service",
      "status": "up",
      "latency_ms": 25,
      "uptime_percent": 99.7,
      "last_check": "2024-02-08T12:00:00Z"
    }
  ]
}
```

## Campos de Respuesta

### Curation Stats

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `pending` | int | Productos pendientes de curación |
| `approved_today` | int | Productos aprobados hoy |
| `rejected_today` | int | Productos rechazados hoy |
| `total_scraped` | int | Total de productos scrapeados |

### Catalog Stats

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `total_products` | int | Total de productos en el catálogo |
| `total_variants` | int | Total de variantes de productos |
| `active_products` | int | Productos activos |
| `categories_count` | int | Total de categorías |
| `top_categories` | array | Top 5 categorías con más productos |

### Tenant Stats

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `total` | int | Total de tenants |
| `active` | int | Tenants activos |
| `new_this_month` | int | Nuevos tenants este mes |
| `recent` | array | Últimos 5 tenants registrados |

### Service Health

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `name` | string | Nombre del servicio |
| `status` | string | Estado: "up", "down", "degraded" |
| `latency_ms` | int | Latencia en milisegundos |
| `uptime_percent` | float | Porcentaje de uptime |
| `last_check` | string | Timestamp de última verificación |

## Performance

- **Timeout por servicio**: 2 segundos
- **Timeout total del endpoint**: ~5 segundos (llamadas paralelas)
- **Latencia esperada**: < 2 segundos en localhost

## Error Handling

El endpoint es **resiliente**: si un servicio falla, retorna valores vacíos para esa sección en lugar de fallar completamente.

### Ejemplo de degradación graceful

Si Tenant Service está caído:

```json
{
  "curation": { /* datos OK */ },
  "catalog": { /* datos OK */ },
  "tenants": {
    "total": 0,
    "active": 0,
    "new_this_month": 0,
    "recent": []
  },
  "services": [ /* incluirá tenant-service con status "down" */ ]
}
```

## Variables de Entorno

```bash
PIM_SERVICE_URL=http://localhost:8090
SCRAPER_SERVICE_URL=http://localhost:8086
IAM_SERVICE_URL=http://localhost:8080
TENANT_SERVICE_URL=http://localhost:8070  # Opcional
```

## Uso

### Con curl

```bash
curl -X GET http://localhost:8085/api/v1/admin/dashboard/stats \
  -H "Authorization: Bearer <jwt_token>" \
  | jq '.'
```

### Con el script de prueba

```bash
# Sin autenticación (puede fallar si hay middleware)
./test-dashboard.sh

# Con JWT token
./test-dashboard.sh "eyJhbGciOiJIUzI1NiIs..."
```

## Implementación

### Estructura de código

```
src/admin/
├── models.go     # DTOs y estructuras de respuesta
├── service.go    # Lógica de orquestación (llamadas paralelas)
└── handler.go    # Handler HTTP del endpoint
```

### Flujo de ejecución

```
1. Handler recibe request
   ↓
2. Service lanza 4 goroutines paralelas:
   - getCurationStats() → PIM Service
   - getCatalogStats() → PIM Service
   - getTenantStats() → Tenant Service
   - getServicesHealth() → Health checks
   ↓
3. WaitGroup sincroniza todas las goroutines
   ↓
4. Service consolida resultados
   ↓
5. Handler retorna JSON consolidado
```

### Características clave

- ✅ **Llamadas paralelas** con goroutines
- ✅ **Timeout de 2s por servicio**
- ✅ **Error handling resiliente**
- ✅ **Sin base de datos** (solo orquestación HTTP)
- ✅ **Logs detallados** para debugging
- ✅ **Mutex para sincronización** segura de datos

## Notas de Implementación

### Endpoints de PIM consumidos

```go
// Stats de curación
GET /api/v1/products?status=pending&page=1&page_size=1
GET /api/v1/products?status=approved&date_from={today}&page=1&page_size=1
GET /api/v1/products?status=rejected&date_from={today}&page=1&page_size=1

// Stats de catálogo
GET /api/v1/products?page=1&page_size=1
GET /api/v1/products?is_active=true&page=1&page_size=1
GET /api/v1/product-variants?page=1&page_size=1
GET /api/v1/categories?page=1&page_size=1
GET /api/v1/categories?page=1&page_size=5&sort_by=products_count&sort_dir=desc
```

### Endpoints de Tenant Service consumidos

```go
GET /api/v1/tenants?page=1&page_size=100
```

### Health Checks

```go
GET {service_url}/health
```

## Testing

### Verificar compilación

```bash
cd /Users/hornosg/MyProjects/saas-mt/services/catalog-bff-service
go build .
```

### Ejecutar pruebas

```bash
# Iniciar el servicio
PORT=8085 go run main.go

# En otra terminal, ejecutar test
./test-dashboard.sh
```

### Verificar logs

El servicio emite logs detallados:

```
📊 Obteniendo estadísticas del dashboard...
✅ Dashboard stats obtenidos en 1.234s
   - Curación: 12 pending, 5 approved today, 2 rejected today, 1543 scraped total
   - Catálogo: 2341 productos, 8923 variantes, 2103 activos, 45 categorías
   - Tenants: 15 total, 14 activos, 3 nuevos este mes
   - Servicios: 4 verificados
```

## Próximos Pasos

### Mejoras potenciales

1. **Cache**: Implementar cache in-memory con TTL de 30s
2. **Métricas reales de uptime**: Integrar con Prometheus
3. **Filtros de curación**: Endpoint específico en PIM para filtrar por `source=scraper`
4. **Circuit breaker**: Agregar circuit breaker para servicios lentos/caídos
5. **Autenticación real**: Integrar middleware de autenticación JWT

### Integración con frontend

Ver documentación en `marketplace-admin`:
- Componente: `src/app/admin/dashboard/page.tsx`
- Hook: `src/hooks/useDashboardStats.ts`

## Referencias

- [Arquitectura del catalog-bff-service (ADR-001)](../adr/ADR-001-bff-composicion-lecturas.md)
- [Patrón API Composition](https://microservices.io/patterns/data/api-composition.html)
- [Backend for Frontend (BFF)](https://learn.microsoft.com/en-us/azure/architecture/patterns/backends-for-frontends)
