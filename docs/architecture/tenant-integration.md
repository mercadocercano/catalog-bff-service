# Integración catalog-bff-service → tenant-service

## ✅ Estado: COMPLETADO

La integración entre `catalog-bff-service` y `tenant-service` ha sido implementada exitosamente siguiendo los principios de arquitectura hexagonal y DDD.

---

## 📋 Resumen de Implementación

### **Objetivo Cumplido**

El BFF ahora resuelve la **Stock Policy** de cada tenant consultando el `tenant-service` en tiempo real, con fallback seguro en caso de error.

### **Componentes Creados**

1. ✅ **HTTP Client** (`src/infrastructure/tenant/client/tenant_config_client.go`)
   - Timeout: 500ms
   - Maneja 404, 5xx, timeouts
   - No propaga errores al caller

2. ✅ **Domain Resolver** (`src/domain/tenant_stock_policy_resolver.go`)
   - Orquesta el client
   - Aplica fallback: `REQUIRE_STOCK`
   - Loggea decisiones
   - Nunca falla

3. ✅ **Handler Refactorizado** (`src/handler/sellable_variants_handler_v2.go`)
   - Inyección de dependencias
   - Resuelve policy una vez por request
   - Aplica policy a cada variante

4. ✅ **Main Actualizado** (`main.go`)
   - Inicializa client y resolver
   - Inyecta dependencias al handler
   - Loggea configuración

5. ✅ **Tests Completos** (`src/domain/tenant_stock_policy_resolver_test.go`)
   - 7 casos de prueba
   - Todos pasan ✅
   - Cobertura de fallbacks

---

## 🔧 Configuración

### **Variable de Entorno**

```bash
TENANT_SERVICE_URL=http://tenant-service:8120
```

### **Comportamiento**

| Escenario | Resultado |
|-----------|-----------|
| `TENANT_SERVICE_URL` configurada | Consulta tenant-service |
| `TENANT_SERVICE_URL` vacía | Fallback: `REQUIRE_STOCK` |
| tenant-service responde `IGNORE_STOCK` | Usa `IGNORE_STOCK` |
| tenant-service responde `REQUIRE_STOCK` | Usa `REQUIRE_STOCK` |
| tenant-service responde 404 | Fallback: `REQUIRE_STOCK` |
| tenant-service responde 5xx | Fallback: `REQUIRE_STOCK` |
| tenant-service timeout | Fallback: `REQUIRE_STOCK` |

---

## 🚀 Uso

### **Iniciar catalog-bff-service**

```bash
cd services/catalog-bff-service

# Con tenant-service
TENANT_SERVICE_URL=http://localhost:8125 \
PIM_SERVICE_URL=http://localhost:8090 \
STOCK_SERVICE_URL=http://localhost:8100 \
PORT=8087 \
go run .
```

### **Consultar Endpoint**

```bash
curl -X GET http://localhost:8087/api/v1/catalog/sellable-variants \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001"
```

**Respuesta:**
```json
{
  "items": [
    {
      "variant_id": "variant-123",
      "product_id": "product-456",
      "variant_name": "Producto Ejemplo",
      "sku": "SKU-001",
      "is_default": true,
      "available_quantity": 0,
      "reserved_quantity": 0,
      "is_sellable": true  // ← true porque tenant tiene IGNORE_STOCK
    }
  ],
  "total_count": 1
}
```

---

## 🧪 Tests

### **Ejecutar Tests**

```bash
cd services/catalog-bff-service
go test ./src/domain/... -v
```

**Resultado:**
```
=== RUN   TestTenantStockPolicyResolver_Resolve_IgnoreStock
--- PASS: TestTenantStockPolicyResolver_Resolve_IgnoreStock (0.00s)
=== RUN   TestTenantStockPolicyResolver_Resolve_RequireStock
--- PASS: TestTenantStockPolicyResolver_Resolve_RequireStock (0.00s)
=== RUN   TestTenantStockPolicyResolver_Resolve_ValidateStock
--- PASS: TestTenantStockPolicyResolver_Resolve_ValidateStock (0.00s)
=== RUN   TestTenantStockPolicyResolver_Resolve_ConfigNotFound
--- PASS: TestTenantStockPolicyResolver_Resolve_ConfigNotFound (0.00s)
=== RUN   TestTenantStockPolicyResolver_Resolve_ServiceError
--- PASS: TestTenantStockPolicyResolver_Resolve_ServiceError (0.00s)
=== RUN   TestTenantStockPolicyResolver_Resolve_UnknownValue
--- PASS: TestTenantStockPolicyResolver_Resolve_UnknownValue (0.00s)
=== RUN   TestTenantStockPolicyResolver_Resolve_NoClient
--- PASS: TestTenantStockPolicyResolver_Resolve_NoClient (0.00s)
PASS
ok      catalog-bff-service/src/domain  0.879s
```

---

## 📊 Flujo de Datos

```
1. Request → catalog-bff-service
   ↓
2. Extraer X-Tenant-ID
   ↓
3. TenantStockPolicyResolver.Resolve(tenantID)
   ↓
4. HTTPTenantConfigClient.GetConfig("catalog.stock_policy")
   ↓
5. HTTP GET → tenant-service:8120/api/v1/tenant/config/catalog.stock_policy
   ↓
6. tenant-service → PostgreSQL
   ↓
7. Response: {"key": "catalog.stock_policy", "value": "IGNORE_STOCK"}
   ↓
8. Resolver mapea "IGNORE_STOCK" → domain.IgnoreStock
   ↓
9. Handler aplica policy a cada variante
   ↓
10. Response con is_sellable calculado
```

---

## 🔒 Reglas Arquitectónicas Cumplidas

| Regla | Estado |
|-------|--------|
| ✅ BFF NO persiste datos | **CUMPLIDO** - Solo consulta HTTP |
| ✅ BFF NO conoce BD de tenant-service | **CUMPLIDO** - Solo HTTP client |
| ✅ Integración HTTP | **CUMPLIDO** - HTTPTenantConfigClient |
| ✅ tenant-service es source of truth | **CUMPLIDO** - BFF solo consulta |
| ✅ Fallback obligatorio | **CUMPLIDO** - Siempre devuelve policy válida |
| ✅ Sin dependencias innecesarias | **CUMPLIDO** - Solo net/http |
| ✅ Arquitectura hexagonal | **CUMPLIDO** - Domain/Application/Infrastructure |
| ✅ Código simple y testeable | **CUMPLIDO** - 7 tests, 100% pass |

---

## 🎯 Decisiones de Diseño

### **1. Timeout Agresivo (500ms)**

**Razón:** El BFF no puede bloquearse esperando tenant-service. Mejor fallback rápido que timeout largo.

### **2. Fallback a REQUIRE_STOCK**

**Razón:** Política conservadora. Mejor rechazar venta sin stock que vender sin inventario.

### **3. Resolver en Domain Layer**

**Razón:** La lógica de resolución de policy es parte del dominio del BFF, no de infraestructura.

### **4. Client en Infrastructure Layer**

**Razón:** HTTP es un detalle de implementación. El dominio solo conoce la interfaz.

### **5. No Cache**

**Razón:** Simplicidad en v1. Si se necesita performance, agregar cache después.

### **6. Logs Explícitos**

**Razón:** Debugging y observabilidad. Cada decisión se loggea.

---

## 📝 Archivos Modificados/Creados

### **Nuevos Archivos**

```
src/infrastructure/tenant/client/
└── tenant_config_client.go                    # HTTP client

src/domain/
└── tenant_stock_policy_resolver.go            # Domain resolver
└── tenant_stock_policy_resolver_test.go       # Tests

src/handler/
└── sellable_variants_handler_v2.go            # Handler refactorizado
```

### **Archivos Modificados**

```
main.go                                        # Inicialización + DI
docker-compose.yml                             # TENANT_SERVICE_URL
```

### **Archivos Deprecados (NO eliminados)**

```
src/domain/tenant_stock_policy.go              # Versión hardcoded (legacy)
src/handler/sellable_variants_handler.go       # Handler sin DI (legacy)
```

---

## 🚧 Próximos Pasos (Futuro)

### **v1.1 - Cache**
- Agregar cache en memoria (TTL: 60s)
- Reducir llamadas a tenant-service

### **v1.2 - Métricas**
- Prometheus metrics para llamadas a tenant-service
- Latencia, errores, fallbacks

### **v1.3 - Circuit Breaker**
- Implementar circuit breaker si tenant-service falla mucho
- Evitar cascading failures

### **v1.4 - Más Configs**
- Soportar más configuraciones por tenant
- `catalog.auto_publish`, `catalog.max_variants`, etc.

---

## 🐛 Troubleshooting

### **Error: "No tenant client configured"**

**Causa:** `TENANT_SERVICE_URL` no está configurada.

**Solución:**
```bash
export TENANT_SERVICE_URL=http://tenant-service:8120
```

### **Warning: "Error fetching policy for tenant"**

**Causa:** tenant-service no responde o timeout.

**Comportamiento:** Usa fallback `REQUIRE_STOCK` (esperado).

**Acción:** Verificar que tenant-service esté corriendo:
```bash
curl http://localhost:8125/health
```

### **Policy siempre es REQUIRE_STOCK**

**Posibles causas:**
1. Tenant no tiene configuración en `tenant_config`
2. tenant-service devuelve 404
3. tenant-service está caído

**Verificar:**
```bash
# 1. Verificar que el tenant tenga config
docker-compose exec postgres psql -U postgres -d tenant_db \
  -c "SELECT * FROM tenant_config WHERE tenant_id = '00000000-0000-0000-0000-000000000001';"

# 2. Probar tenant-service directamente
curl http://localhost:8125/api/v1/tenant/config/catalog.stock_policy \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001"
```

---

## ✅ Checklist de Integración

- [x] HTTP Client creado
- [x] Resolver implementado
- [x] Handler refactorizado con DI
- [x] Main actualizado
- [x] Tests escritos (7 casos)
- [x] Tests pasan (100%)
- [x] Compila sin errores
- [x] Configuración documentada
- [x] Fallback funciona
- [x] Logs informativos
- [x] Sin dependencias innecesarias
- [x] Arquitectura hexagonal respetada
- [x] BFF sigue siendo stateless

---

## 🎉 Conclusión

La integración está **completa y funcional**. El `catalog-bff-service` ahora:

✅ Consulta `tenant-service` para resolver Stock Policy  
✅ Aplica fallback seguro si falla  
✅ Mantiene arquitectura hexagonal  
✅ Es testeable y simple  
✅ No rompe flujos existentes  

**La Stock Policy ahora es 100% controlada por `tenant-service`.**

---

**Fecha de Implementación:** 2026-02-03  
**Versión:** 1.0.0  
**Autor:** SaaS MT Team - Mercado Cercano
