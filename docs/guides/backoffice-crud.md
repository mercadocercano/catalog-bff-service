# Backoffice CRUD - Productos y Variantes

## 🎯 Propósito

Este documento describe los endpoints del **Catalog BFF** específicos para el **Backoffice Admin**, que permiten la gestión completa (CRUD) de productos y variantes.

## 🏗️ Arquitectura

```
┌─────────────────────┐
│  Backoffice Admin   │
│    (Frontend)       │
└──────────┬──────────┘
           │ HTTP
           ▼
┌─────────────────────┐
│   Catalog BFF       │ ← Validaciones + Orquestación
│   (port 8085)       │
└──────────┬──────────┘
           │ HTTP
           ▼
┌─────────────────────┐
│   PIM Service       │ ← Source of Truth
│   (port 8090)       │
└─────────────────────┘
```

### Principios

1. **Backoffice NO habla directo con PIM**: Todas las operaciones pasan por el BFF
2. **BFF no persiste nada**: Solo orquesta y valida
3. **PIM es source of truth**: Toda la lógica de negocio está en PIM
4. **DTOs claros**: No hay leak de detalles internos de PIM

---

## 📋 Endpoints Disponibles

### Base URL

```
http://localhost:8085/api/v1/backoffice
```

**Headers requeridos:**
```
X-Tenant-ID: {tenant_uuid}
Authorization: Bearer {jwt_token}
```

---

## 🛍️ Productos

### 1. Listar Productos

```http
GET /api/v1/backoffice/products
```

**Query Parameters:**
```
?page=1
&page_size=20
&search=coca
&category_id={uuid}
&brand_id={uuid}
&status=active|inactive|draft|archived
&sort_by=name|created_at|updated_at
&sort_dir=asc|desc
```

**Response 200:**
```json
{
  "items": [
    {
      "id": "uuid",
      "name": "Coca Cola 2.25L",
      "category_name": "Bebidas",
      "brand_name": "Coca Cola",
      "status": "active",
      "variants_count": 3,
      "has_active_stock": true,
      "created_at": "2026-02-01T10:00:00Z",
      "updated_at": "2026-02-02T15:30:00Z"
    }
  ],
  "total_count": 150,
  "page": 1,
  "page_size": 20,
  "total_pages": 8
}
```

---

### 2. Obtener Producto

```http
GET /api/v1/backoffice/products/{product_id}
```

**Response 200:**
```json
{
  "id": "uuid",
  "name": "Coca Cola 2.25L",
  "description": "Gaseosa Coca Cola sabor original",
  "category_id": "uuid",
  "category_name": "Bebidas",
  "brand_id": "uuid",
  "brand_name": "Coca Cola",
  "status": "active",
  "variants": [
    {
      "id": "uuid",
      "name": "Coca Cola 2.25L - Default",
      "sku": "COC-2.25-001",
      "price": 1500.00,
      "is_default": true,
      "is_active": true,
      "attributes": [
        {"name": "tamaño", "value": "2.25L"}
      ],
      "stock": {
        "available": 50,
        "reserved": 5,
        "total": 55,
        "is_low_stock": false,
        "is_out_of_stock": false
      },
      "created_at": "2026-02-01T10:00:00Z",
      "updated_at": "2026-02-02T15:30:00Z"
    }
  ],
  "created_at": "2026-02-01T10:00:00Z",
  "updated_at": "2026-02-02T15:30:00Z"
}
```

**Response 404:**
```json
{
  "error": "product_not_found",
  "message": "Producto no encontrado"
}
```

---

### 3. Crear Producto

```http
POST /api/v1/backoffice/products
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "Coca Cola 2.25L",
  "description": "Gaseosa Coca Cola sabor original",
  "category_id": "uuid",
  "brand_id": "uuid",
  "status": "active",
  "variants": [
    {
      "name": "Coca Cola 2.25L - Default",
      "sku": "COC-2.25-001",
      "price": 1500.00,
      "is_default": true,
      "attributes": [
        {"name": "tamaño", "value": "2.25L"}
      ]
    }
  ]
}
```

**Validaciones:**
- `name`: requerido, mínimo 3 caracteres, máximo 255
- `description`: opcional, máximo 2000 caracteres
- `category_id`: opcional, debe ser UUID válido
- `brand_id`: opcional, debe ser UUID válido
- `status`: opcional, valores permitidos: `active`, `inactive`, `draft`
- `variants`: opcional, si se incluyen, al menos una debe ser `is_default: true`

**Response 201:**
```json
{
  "id": "uuid",
  "name": "Coca Cola 2.25L",
  "status": "active",
  "created_at": "2026-02-02T16:00:00Z"
}
```

**Response 400:**
```json
{
  "error": "validation_error",
  "message": "debe haber al menos una variante marcada como default",
  "details": {
    "validation": "..."
  }
}
```

---

### 4. Actualizar Producto

```http
PUT /api/v1/backoffice/products/{product_id}
Content-Type: application/json
```

**Request Body (todos los campos son opcionales):**
```json
{
  "name": "Coca Cola 2.25L Retornable",
  "description": "Nueva descripción",
  "category_id": "uuid",
  "brand_id": "uuid",
  "status": "active"
}
```

**Response 200:**
```json
{
  "id": "uuid",
  "name": "Coca Cola 2.25L Retornable",
  "description": "Nueva descripción",
  "category_id": "uuid",
  "brand_id": "uuid",
  "status": "active",
  "created_at": "2026-02-01T10:00:00Z",
  "updated_at": "2026-02-02T16:30:00Z"
}
```

---

## 🎨 Variantes

### 5. Listar Variantes de un Producto

```http
GET /api/v1/backoffice/products/{product_id}/variants
```

**Response 200:**
```json
[
  {
    "id": "uuid",
    "name": "Coca Cola 2.25L - Default",
    "sku": "COC-2.25-001",
    "price": 1500.00,
    "is_default": true,
    "is_active": true,
    "attributes": [
      {"name": "tamaño", "value": "2.25L"}
    ],
    "stock": {
      "available": 50,
      "reserved": 5,
      "total": 55,
      "is_low_stock": false,
      "is_out_of_stock": false
    },
    "created_at": "2026-02-01T10:00:00Z",
    "updated_at": "2026-02-02T15:30:00Z"
  }
]
```

---

### 6. Obtener Variante Específica

```http
GET /api/v1/backoffice/products/{product_id}/variants/{variant_id}
```

**Response 200:** (mismo formato que item del listado)

**Response 404:**
```json
{
  "error": "variant_not_found",
  "message": "Variante no encontrada"
}
```

---

### 7. Crear Variante

```http
POST /api/v1/backoffice/products/{product_id}/variants
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "Coca Cola 1.5L",
  "sku": "COC-1.5-001",
  "price": 1200.00,
  "is_default": false,
  "attributes": [
    {"name": "tamaño", "value": "1.5L"}
  ]
}
```

**Validaciones:**
- `name`: requerido, mínimo 1 carácter, máximo 255
- `sku`: requerido, mínimo 1 carácter, máximo 100
- `price`: requerido, debe ser >= 0
- `is_default`: opcional, default `false`
- `attributes`: opcional

**Response 201:**
```json
{
  "id": "uuid",
  "product_id": "uuid",
  "name": "Coca Cola 1.5L",
  "sku": "COC-1.5-001",
  "created_at": "2026-02-02T16:45:00Z"
}
```

**Response 400:**
```json
{
  "error": "validation_error",
  "message": "el SKU es requerido"
}
```

**Response 404:**
```json
{
  "error": "product_not_found",
  "message": "Producto no encontrado"
}
```

---

### 8. Actualizar Variante

```http
PUT /api/v1/backoffice/products/{product_id}/variants/{variant_id}
Content-Type: application/json
```

**Request Body (todos los campos son opcionales):**
```json
{
  "name": "Coca Cola 1.5L Retornable",
  "sku": "COC-1.5-RET-001",
  "price": 1100.00,
  "is_default": false,
  "attributes": [
    {"name": "tamaño", "value": "1.5L"},
    {"name": "tipo", "value": "retornable"}
  ]
}
```

**Response 200:** (variante completa con stock)

---

### 9. Activar/Desactivar Variante

```http
PATCH /api/v1/backoffice/products/{product_id}/variants/{variant_id}/status
Content-Type: application/json
```

**Request Body:**
```json
{
  "is_active": false
}
```

**Response 200:**
```json
{
  "id": "uuid",
  "status": "inactive",
  "is_active": false,
  "message": "Variante inactive correctamente"
}
```

---

## ❌ Errores Comunes

### 400 Bad Request

```json
{
  "error": "invalid_request",
  "message": "Datos inválidos",
  "details": {
    "validation": "Key: 'CreateProductRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag"
  }
}
```

### 400 Missing Tenant

```json
{
  "error": "missing_tenant",
  "message": "Header X-Tenant-ID es requerido"
}
```

### 502 Bad Gateway

```json
{
  "error": "pim_unavailable",
  "message": "PIM Service no disponible"
}
```

---

## 🧪 Testing

### Ejemplo con curl

```bash
# Variables
TENANT_ID="123e4567-e89b-12d3-a456-426614174000"
TOKEN="eyJhbGc..."
BFF_URL="http://localhost:8085"

# 1. Listar productos
curl -X GET "$BFF_URL/api/v1/backoffice/products?page=1&page_size=10" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $TOKEN"

# 2. Crear producto con variante
curl -X POST "$BFF_URL/api/v1/backoffice/products" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Coca Cola 2.25L",
    "description": "Gaseosa Coca Cola sabor original",
    "status": "active",
    "variants": [
      {
        "name": "Coca Cola 2.25L - Default",
        "sku": "COC-2.25-001",
        "price": 1500.00,
        "is_default": true
      }
    ]
  }'

# 3. Obtener producto
PRODUCT_ID="uuid-del-producto"
curl -X GET "$BFF_URL/api/v1/backoffice/products/$PRODUCT_ID" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $TOKEN"

# 4. Crear variante adicional
curl -X POST "$BFF_URL/api/v1/backoffice/products/$PRODUCT_ID/variants" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Coca Cola 1.5L",
    "sku": "COC-1.5-001",
    "price": 1200.00,
    "is_default": false
  }'

# 5. Desactivar variante
VARIANT_ID="uuid-de-variante"
curl -X PATCH "$BFF_URL/api/v1/backoffice/products/$PRODUCT_ID/variants/$VARIANT_ID/status" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"is_active": false}'
```

---

## 🔒 Seguridad

### Multi-Tenancy

- **Todos** los endpoints requieren `X-Tenant-ID` header
- El BFF propaga el header a PIM Service
- PIM valida que el recurso pertenezca al tenant
- No es posible acceder a recursos de otro tenant

### Autenticación

- **Todos** los endpoints requieren `Authorization: Bearer {token}`
- El token es validado por Kong Gateway antes de llegar al BFF
- El BFF propaga el token a PIM Service

---

## 📊 Flujo de Datos

### Crear Producto con Variantes

```
1. Backoffice → POST /backoffice/products
   ↓
2. BFF valida request (nombre requerido, al menos una variante default)
   ↓
3. BFF → POST /pim/api/v1/products (con X-Tenant-ID)
   ↓
4. PIM valida negocio, crea producto + variantes, persiste en DB
   ↓
5. PIM → Response con producto creado
   ↓
6. BFF mapea a DTO de backoffice
   ↓
7. BFF → Response 201 a Backoffice
```

### Listar Productos con Stock

```
1. Backoffice → GET /backoffice/products
   ↓
2. BFF construye query params para PIM
   ↓
3. BFF → GET /pim/api/v1/products?page=1&...
   ↓
4. PIM consulta DB, aplica filtros y paginación
   ↓
5. PIM → Response con listado
   ↓
6. BFF mapea a DTOs de backoffice (sin leak de PIM)
   ↓
7. BFF → Response 200 a Backoffice
```

---

## 🚫 Lo que NO hace el BFF

- ❌ No persiste datos
- ❌ No tiene lógica de negocio de productos
- ❌ No valida reglas complejas (eso es responsabilidad de PIM)
- ❌ No maneja stock directamente
- ❌ No maneja pricing
- ❌ No maneja imágenes

## ✅ Lo que SÍ hace el BFF

- ✅ Valida formato de requests (DTOs)
- ✅ Orquesta llamadas a PIM
- ✅ Mapea respuestas a DTOs de backoffice
- ✅ Propaga headers (tenant, auth)
- ✅ Enriquece variantes con stock (cuando se solicita detalle)
- ✅ Maneja errores y los traduce a mensajes claros

---

## 📈 Próximos Pasos

### Fase 1 (Actual) ✅
- CRUD básico de productos
- CRUD básico de variantes
- Activar/desactivar variantes

### Fase 2 (Futuro)
- [ ] Búsqueda avanzada de productos
- [ ] Filtros por atributos dinámicos
- [ ] Bulk operations (activar/desactivar múltiples)
- [ ] Duplicar producto con variantes
- [ ] Importación masiva desde CSV

### Fase 3 (Futuro)
- [ ] Gestión de imágenes
- [ ] Historial de cambios
- [ ] Preview de cambios antes de guardar
- [ ] Validaciones asíncronas (SKU duplicado, etc.)

---

**Última actualización:** 2026-02-02  
**Versión:** 1.0.0  
**Estado:** ✅ Implementado y listo para uso
