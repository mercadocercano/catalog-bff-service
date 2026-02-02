# Catalog Service — Decisión de Arquitectura

## Contexto

En el **HITO 1** del proyecto SaaS Multi-Tenant "Tienda Vecina", se necesitaba exponer un endpoint que combinara datos de **producto/variante** (PIM) con **stock disponible** (Stock Service) en una sola respuesta consumible por frontends.

## Problema

### Lo que se intentó resolver

Responder a la pregunta:

> **"¿Qué puedo vender ahora y en qué cantidad?"**

Esta pregunta requiere datos de **dos dominios diferentes**:
1. **PIM Service** → ¿Qué es este producto/variante? (nombre, SKU, atributos)
2. **Stock Service** → ¿Cuánto hay disponible? (cantidad, reservas)

### Por qué no usar solo PIM

**PIM es fuente de verdad del producto**, no debería:
- ❌ Calcular stock
- ❌ Llamar a otros servicios
- ❌ Orquestar consultas
- ❌ Construir vistas agregadas

**Responsabilidad de PIM:**
- ✅ Definir qué es un producto
- ✅ Gestionar variantes
- ✅ Aplicar reglas de dominio
- ✅ Ser escritura-heavy (CRUD de productos)

### Por qué no usar Kong Gateway

**Kong Gateway** es un API Gateway tradicional que:
- ✅ Enruta requests
- ✅ Autentica (JWT, API Key)
- ✅ Rate limiting
- ✅ Transforma headers

Pero **NO orquesta**:
- ❌ No hace múltiples llamadas upstream
- ❌ No mergea responses
- ❌ No maneja lógica condicional

**Opciones evaluadas:**
1. **Plugin custom de Kong** → Alto costo, acoplamiento, complejo de mantener
2. **Servicio de orquestación ligero** → ✅ Elegido

### Por qué no en el frontend

Hacer la orquestación en el frontend implica:
- ❌ 2 requests HTTP separados (latencia)
- ❌ Lógica de merge en cada cliente
- ❌ Manejo de errores parciales duplicado
- ❌ Mayor consumo de ancho de banda
- ❌ Exposición de múltiples endpoints internos

---

## Decisión

### Crear `catalog-service` como servicio de lectura agregada

**Rol:**
- Servicio de **composición de lecturas** (query/read-side)
- **NO es un dominio nuevo**
- **NO persiste datos**
- **NO tiene reglas de negocio propias**

**Patrón aplicado:**
- CQRS light (separación query/command)
- API Composition Pattern
- Backend for Frontend (BFF) simplificado

### Arquitectura

```
┌─────────────────┐
│   Frontend      │
│  (backoffice,   │
│  marketplace)   │
└────────┬────────┘
         │ 1 request
         ▼
┌─────────────────┐
│ catalog-service │ ← Orquestación
│   (port 8085)   │
└────────┬────────┘
         │
    ┌────┴─────┐
    │          │ 2 requests internos
    ▼          ▼
┌────────┐  ┌────────┐
│  PIM   │  │ Stock  │
│ :8090  │  │ :8100  │
└────────┘  └────────┘
```

---

## Comparación clara: PIM vs Catalog

| Aspecto | PIM Service | catalog-service |
|---------|-------------|-----------------|
| **Rol** | Fuente de verdad del producto | Vista agregada de lectura |
| **Tipo** | Dominio core | Servicio de composición |
| **Persiste datos** | ✅ Sí (PostgreSQL) | ❌ No |
| **Tiene reglas de negocio** | ✅ Sí (validaciones, agregados) | ❌ No |
| **Llama a otros servicios** | ❌ No | ✅ Sí (PIM + Stock) |
| **Cambia estado** | ✅ Sí (CRUD) | ❌ No (solo lee) |
| **Pensado para frontend** | ❌ No (API de dominio) | ✅ Sí (vistas optimizadas) |
| **Impacto de error** | 🔴 Alto (crítico) | 🟡 Medio (degradación) |
| **Escalabilidad** | Vertical (DB-bound) | Horizontal (stateless) |

---

## Consecuencias

### ✅ Ventajas

1. **Separación de responsabilidades clara**
   - PIM sigue siendo fuente de verdad pura
   - Stock maneja solo inventario
   - Catalog orquesta vistas

2. **Mejor experiencia de frontend**
   - 1 request en lugar de 2
   - Respuesta unificada
   - Menor latencia percibida

3. **Reversible**
   - Si se cambia de gateway (ej: GraphQL, gRPC)
   - O se implementa BFF dedicado
   - → `catalog-service` se puede **eliminar sin impacto**

4. **Patrón establecido**
   - Base para futuras orquestaciones
   - Ejemplo: orden + productos + stock + envío

5. **Stateless y escalable**
   - No tiene DB
   - Puede escalar horizontalmente sin límite
   - Cache fácil de implementar

### ⚠️ Desventajas (asumidas)

1. **Un componente más**
   - Deploy adicional
   - Monitoreo adicional
   - Punto de falla adicional (mitigable con retry/fallback)

2. **Latencia agregada**
   - 2 llamadas secuenciales internas
   - Mitigable con:
     - Llamadas paralelas (si no hay dependencia)
     - Cache de variantes
     - Circuit breaker

3. **Riesgo de "God Service"**
   - Si se empieza a agregar lógica de negocio → ❌ Anti-patrón
   - **Regla:** Catalog **solo lee y mergea**, nunca decide

---

## Alternativas consideradas

### ❌ Opción 1: PIM llama a Stock

**Rechazada porque:**
- Acopla PIM a Stock
- PIM deja de ser bounded context puro
- Dificulta testing y evolución independiente

### ❌ Opción 2: Plugin custom de Kong

**Rechazada porque:**
- Lua/Go custom en Kong es difícil de mantener
- Acoplamiento fuerte al gateway
- Dificulta testing local
- No hay separación clara de responsabilidades

### ❌ Opción 3: Frontend hace 2 requests

**Rechazada porque:**
- Latencia percibida alta
- Lógica duplicada en cada cliente
- Mayor consumo de ancho de banda
- Exposición de endpoints internos

### ✅ Opción elegida: Servicio ligero de orquestación

**Razones:**
- Bajo acoplamiento
- Fácil de testear
- Reversible
- Patrón claro y extensible

---

## Cuándo considerar cambiar esto

`catalog-service` se puede **reemplazar o eliminar** si:

1. **Se adopta GraphQL Federation**
   - Apollo Gateway orquestaría naturalmente

2. **Se implementa BFF por frontend**
   - Backoffice BFF tendría su propia orquestación
   - Marketplace BFF la suya

3. **Se migra a arquitectura event-driven**
   - Vista materializada de catálogo+stock en read-model

4. **Kong se reemplaza por API Gateway con orquestación**
   - AWS API Gateway + Lambda
   - Azure API Management
   - Traefik con plugins custom

---

## Reglas de evolución

### ✅ Permitido en catalog-service

- Llamar a múltiples servicios
- Mergear responses
- Aplicar transformaciones simples (mapeo de campos)
- Cache de lecturas
- Retry y circuit breaker
- Logging y métricas

### ❌ Prohibido en catalog-service

- **Persistir datos** (no debe tener DB propia)
- **Validar reglas de negocio** (eso es del dominio)
- **Cambiar estado** (no debe hacer writes a dominios)
- **Agregar lógica compleja** (si crece, refactorizar)

---

## Nomenclatura

### Por qué "catalog" y no "product"

- **catalog** → Concepto de "lo que se puede vender/mostrar"
- **product** → Ya usado por PIM (fuente de verdad)

Evita confusión con el dominio core.

### Alternativas evaluadas

- `catalog-query-service` (más preciso pero verbose)
- `product-read-service` (confunde con PIM)
- `storefront-service` (acopla a frontend específico)
- `catalog-service` ✅ (balance entre claridad y brevedad)

---

## Métricas de éxito

### Indicadores de que la decisión fue correcta

- ✅ PIM y Stock evolucionan independientemente
- ✅ Frontends consumen 1 endpoint en lugar de 2
- ✅ Latencia agregada < 200ms (localhost < 100ms)
- ✅ Código de orquestación < 300 LOC
- ✅ Sin lógica de negocio en catalog-service

### Señales de alarma (cuándo refactorizar)

- 🚨 Catalog empieza a tener validaciones
- 🚨 Catalog persiste datos
- 🚨 Catalog tiene >1000 LOC
- 🚨 Catalog llama a >5 servicios
- 🚨 Lógica de merge es muy compleja

---

## Referencias

### Patrones aplicados

- **API Composition Pattern** (Microservices Patterns - Richardson)
- **Backend for Frontend (BFF)** (Sam Newman)
- **CQRS Light** (separación query/command sin event sourcing)

### Documentos relacionados

- `/documentation/HITO_1_CATALOG_VARIANTS.md` - Validación del hito
- `/services/api-gateway/kong.yml` - Configuración de routing
- `/services/pim-service/README.md` - Responsabilidades de PIM
- `/services/stock-service/README.md` - Responsabilidades de Stock

---

## Conclusión

`catalog-service` **NO es un dominio nuevo**, es un **servicio de composición de lecturas** que permite cerrar el flujo end-to-end sin acoplar dominios core ni forzar lógica en el gateway.

Es una **decisión pragmática y reversible** que:
- ✅ Cierra el HITO 1
- ✅ Mantiene separación de responsabilidades
- ✅ No bloquea evoluciones futuras
- ✅ Establece un patrón claro para orquestaciones

**Estado:** Implementado y validado en HITO 1.

**Próxima revisión:** Al definir HITO 2 o si se adopta GraphQL/BFF.
