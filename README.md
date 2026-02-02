# Catalog Service

Servicio de orquestación para consultas agregadas de catálogo.

## HITO 1: Variante + Stock

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

- `PIM_SERVICE_URL`: URL del PIM service (default: `http://localhost:8090`)
- `STOCK_SERVICE_URL`: URL del Stock service (default: `http://localhost:8100`)
- `PORT`: Puerto del servicio (default: `8085`)

### Ejecución local

```bash
go run main.go
```

### Docker

```bash
docker build -t catalog-service .
docker run -p 8085:8085 catalog-service
```
