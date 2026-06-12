# Tests - Catalog BFF Service

## 🧪 Estructura de Tests

```
test/
└── integration_test.go    # Tests de integración BFF → PIM
```

## 🚀 Ejecutar Tests

### Todos los tests

```bash
cd services/catalog-bff-service
go test ./test/... -v
```

### Tests específicos

```bash
# Test de listado de productos
go test ./test/... -v -run TestProductHandlerListProducts

# Test de crear producto
go test ./test/... -v -run TestProductHandlerCreateProduct

# Test de variantes
go test ./test/... -v -run TestVariantHandler
```

### Con cobertura

```bash
go test ./test/... -v -cover
go test ./test/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 📋 Tests Implementados

### Tests de Productos

| Test | Descripción | Estado |
|------|-------------|--------|
| `TestProductHandlerListProducts` | Lista productos con paginación | ✅ |
| `TestProductHandlerGetProduct` | Obtiene producto por ID | ✅ |
| `TestProductHandlerCreateProduct` | Crea producto con variantes | ✅ |
| `TestProductHandlerUpdateProduct` | Actualiza producto existente | ✅ |

### Tests de Variantes

| Test | Descripción | Estado |
|------|-------------|--------|
| `TestVariantHandlerListVariants` | Lista variantes con stock | ✅ |
| `TestVariantHandlerCreateVariant` | Crea nueva variante | ✅ |

### Tests de Seguridad

| Test | Descripción | Estado |
|------|-------------|--------|
| `TestMissingTenantID` | Rechaza requests sin tenant | ✅ |

## 🎯 Estrategia de Testing

### 1. Tests de Integración (Actuales)

- **Objetivo**: Verificar que el BFF orquesta correctamente con PIM
- **Enfoque**: Mock de PIM Service
- **Cobertura**: Endpoints completos, validaciones, mapeo de DTOs

### 2. Tests de Contrato (Futuros)

- **Objetivo**: Garantizar compatibilidad con PIM Service real
- **Enfoque**: Contract Testing con Pact
- **Cobertura**: Schemas de request/response

### 3. Tests E2E (Futuros)

- **Objetivo**: Verificar flujo completo Backoffice → BFF → PIM
- **Enfoque**: Servicios reales en Docker
- **Cobertura**: Flujos de negocio completos

## 🔧 Mocks

### MockPIMServer

Simula el comportamiento de PIM Service para testing:

```go
mockPIM := MockPIMServer()
defer mockPIM.Close()

productHandler := handler.NewProductHandler(mockPIM.URL)
```

**Endpoints mockeados:**
- `GET /api/v1/products` - Listar productos
- `GET /api/v1/products/:id` - Obtener producto
- `POST /api/v1/products` - Crear producto
- `PUT /api/v1/products/:id` - Actualizar producto
- `GET /api/v1/products/:product_id/variants` - Listar variantes
- `POST /api/v1/products/:product_id/variants` - Crear variante

### MockStockClient

Simula el cliente de Stock Service:

```go
mockStockClient := &MockStockClient{}
variantHandler := handler.NewProductVariantHandler(pimURL, mockStockClient)
```

**Comportamiento:**
- Siempre retorna stock disponible (100 unidades)
- No falla (para simplificar tests)

## 📊 Casos de Prueba

### Crear Producto

**Caso exitoso:**
```json
{
  "name": "Nuevo Producto",
  "description": "Descripción",
  "status": "active",
  "variants": [
    {
      "name": "Variante Default",
      "sku": "NEW-SKU-001",
      "price": 1000.00,
      "is_default": true
    }
  ]
}
```

**Caso de error (sin nombre):**
```json
{
  "name": "",
  "status": "active"
}
```
→ Espera: `400 Bad Request`

**Caso de error (sin tenant):**
```
Headers: (sin X-Tenant-ID)
```
→ Espera: `400 Bad Request` con error `missing_tenant`

### Listar Productos

**Request:**
```
GET /api/v1/backoffice/products?page=1&page_size=20
Headers:
  X-Tenant-ID: test-tenant
  Authorization: Bearer test-token
```

**Response esperada:**
```json
{
  "items": [
    {
      "id": "prod-001",
      "name": "Producto Test 1",
      "status": "active",
      "variants_count": 0,
      "has_active_stock": false
    }
  ],
  "total_count": 2,
  "page": 1,
  "page_size": 20,
  "total_pages": 1
}
```

### Crear Variante

**Request:**
```json
{
  "name": "Nueva Variante",
  "sku": "NEW-VAR-001",
  "price": 1200.00,
  "is_default": false
}
```

**Response esperada:**
```json
{
  "id": "var-new",
  "product_id": "prod-001",
  "name": "Nueva Variante",
  "sku": "NEW-VAR-001",
  "created_at": "2026-02-02T..."
}
```

## 🐛 Debugging Tests

### Ver logs detallados

```bash
go test ./test/... -v -args -test.v
```

### Ejecutar un test específico con debug

```bash
go test ./test/... -v -run TestProductHandlerCreateProduct -args -test.v
```

### Ver requests HTTP

Los tests usan `httptest` que captura requests/responses. Para debugging:

```go
fmt.Printf("Request: %+v\n", req)
fmt.Printf("Response: %s\n", w.Body.String())
```

## 📝 Agregar Nuevos Tests

### Template para test de endpoint

```go
func TestNuevoEndpoint(t *testing.T) {
	// Setup
	mockPIM := MockPIMServer()
	defer mockPIM.Close()

	handler := handler.NewProductHandler(mockPIM.URL)

	router := gin.New()
	router.GET("/api/v1/backoffice/nuevo-endpoint", handler.NuevoEndpoint)

	// Test
	req := httptest.NewRequest("GET", "/api/v1/backoffice/nuevo-endpoint", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.NuevoResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "expected-value", response.Field)
}
```

## 🎯 Próximos Tests a Implementar

### Alta Prioridad
- [ ] Test de actualizar variante
- [ ] Test de activar/desactivar variante
- [ ] Test de validaciones de negocio (SKU duplicado, etc.)
- [ ] Test de manejo de errores de PIM Service

### Media Prioridad
- [ ] Test de paginación con múltiples páginas
- [ ] Test de filtros (por categoría, marca, status)
- [ ] Test de búsqueda por texto
- [ ] Test de ordenamiento

### Baja Prioridad
- [ ] Test de performance (muchos productos)
- [ ] Test de concurrencia
- [ ] Test de timeouts
- [ ] Test de circuit breaker (si se implementa)

## 📚 Referencias

- [Testing en Go](https://golang.org/pkg/testing/)
- [Testify - Assertions](https://github.com/stretchr/testify)
- [httptest - Testing HTTP](https://golang.org/pkg/net/http/httptest/)
- [Gin Testing](https://github.com/gin-gonic/gin#testing)

---

**Última actualización:** 2026-02-02  
**Cobertura actual:** ~70% de endpoints críticos  
**Estado:** ✅ Tests básicos implementados
