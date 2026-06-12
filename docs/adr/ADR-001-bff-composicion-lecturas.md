# ADR-001: catalog-bff-service como servicio de composición de lecturas

**Estado**: Aceptado
**Fecha**: 2026-06-10
**Contexto**: En el HITO 1 de "Mercado Cercano" se necesitaba exponer un endpoint que combinara datos de producto/variante (PIM) con stock disponible (Stock Service) en una sola respuesta consumible por frontends, sin acoplar dominios core ni forzar lógica de orquestación en el API Gateway.

## Decisión

Creamos `catalog-bff-service` como servicio de **composición de lecturas** (query/read-side). **No es un dominio nuevo**, **no persiste datos** y **no tiene reglas de negocio propias**.

Patrones aplicados:
- CQRS light (separación query/command, sin event sourcing).
- API Composition Pattern (Microservices Patterns, Richardson).
- Backend for Frontend (BFF) simplificado (Sam Newman).

Flujo: el frontend hace **1 request** al BFF (puerto 8085); el BFF realiza las llamadas internas a PIM (`:8090`) y Stock (`:8100`), mergea las respuestas y devuelve una vista unificada.

```
Frontend ── 1 request ──▶ catalog-bff-service ──┬─▶ PIM   (:8090)
                                                 └─▶ Stock (:8100)
```

### Reglas de evolución

**Permitido**: llamar a múltiples servicios, mergear responses, transformaciones simples de mapeo, cache de lecturas, retry/circuit breaker, logging y métricas.

**Prohibido**: persistir datos, validar reglas de negocio, cambiar estado (writes a dominios), agregar lógica compleja. Si crece, refactorizar.

### Nomenclatura

Se eligió "catalog" (lo que se puede vender/mostrar) en vez de "product" (ya usado por PIM, fuente de verdad). Alternativas descartadas: `catalog-query-service` (verbose), `product-read-service` (confunde con PIM), `storefront-service` (acopla a frontend específico).

## Alternativas consideradas

| Opción | Por qué no |
|--------|-----------|
| PIM llama a Stock | Acopla PIM a Stock; PIM deja de ser bounded context puro; dificulta testing y evolución independiente. |
| Plugin custom de Kong (Lua/Go) | Difícil de mantener; acoplamiento fuerte al gateway; complica el testing local; sin separación clara de responsabilidades. |
| Frontend hace 2 requests | Latencia percibida alta; lógica de merge duplicada en cada cliente; mayor consumo de ancho de banda; expone endpoints internos. |
| **Servicio ligero de orquestación (elegido)** | Bajo acoplamiento, fácil de testear, reversible, patrón claro y extensible. |

## Consecuencias

**Positivas**:
- Separación de responsabilidades clara (PIM = fuente de verdad, Stock = inventario, BFF = vistas).
- Mejor experiencia de frontend: 1 request en vez de 2, respuesta unificada, menor latencia percibida.
- Stateless y escalable horizontalmente (sin DB).
- Reversible: se puede eliminar sin impacto si se adopta GraphQL Federation, BFF por frontend, arquitectura event-driven con read-models, o un gateway con orquestación.
- Establece un patrón base para futuras orquestaciones (ej. orden + productos + stock + envío).

**Negativas / trade-offs**:
- Un componente más: deploy, monitoreo y punto de falla adicionales (mitigable con retry/fallback).
- Latencia agregada por llamadas internas (mitigable con llamadas paralelas, cache, circuit breaker).
- Riesgo de "God Service" si se introduce lógica de negocio — anti-patrón. Regla: el BFF solo lee y mergea, nunca decide.

**Neutral**:
- Indicadores de éxito: PIM y Stock evolucionan independientemente; frontends consumen 1 endpoint; latencia agregada < 200ms (localhost < 100ms); orquestación < 300 LOC; sin lógica de negocio.
- Señales de alarma para refactorizar: el BFF empieza a tener validaciones, persiste datos, supera ~1000 LOC, llama a >5 servicios, o la lógica de merge se vuelve muy compleja.

## Referencias

- `../../../../../documentation/HITO_1_CATALOG_VARIANTS.md` — Validación del hito
- `../../../api-gateway/kong.yml` — Configuración de routing
- `../../../pim-service/README.md` — Responsabilidades de PIM
- `../../../stock-service/README.md` — Responsabilidades de Stock

**Próxima revisión**: al definir HITO 2 o si se adopta GraphQL/BFF dedicado.
