# CLAUDE.md — catalog-bff-service

BFF de **composición** (PIM + Stock y auxiliares). **CQRS ligero**, **stateless**, sin persistencia de dominio; caché solo **en memoria** por instancia.

**Puerto**: 8085 | **Stack**: Go + Gin

Habla siempre en español.

## Comandos

```bash
go run main.go
go test ./...
```

## Endpoints

- `GET /api/v1/catalog/variants/:id` (en rutas Gin el parámetro es `:id`, UUID de variante)
- `GET /api/v1/admin/dashboard/stats`

## Variables de entorno

`PIM_SERVICE_URL`, `STOCK_SERVICE_URL`, `TENANT_SERVICE_URL`, `IAM_SERVICE_URL`, `PORT=8085`, `JWT_SECRET`, `TENANT_CONFIG_CACHE_TTL` (p. ej. 60s), `STOCK_CACHE_TTL` (p. ej. 5s), `CACHE_CLEANUP_INTERVAL`.

## Caché

In-memory con TTL configurable; no distribuida.

## Reglas compartidas

Desde el workspace: **`ai-tools/rules/`** (`architecture.md`, `api-gateway.md`, `multi-tenant.md`, `api-response-format.md`). Ruta relativa típica: `../../ai-tools/rules/`.

## Memoria persistente (Engram)

Tenés acceso a memoria persistente entre sesiones vía las herramientas MCP de Engram (`mem_save`, `mem_search`, `mem_context`, etc.). Proyecto: **`mercado-cercano`** (memoria unificada con el resto del ecosistema).

**Cuándo guardar** — sin esperar que te lo pidan:
- Al resolver un bug no trivial: síntoma, causa raíz, fix aplicado.
- Al tomar una decisión de diseño: qué se decidió y por qué.
- Al descubrir un patrón o convención del proyecto que no está documentada.
- Al completar una feature o refactor significativo: qué cambió y dónde.

**Cuándo buscar** — antes de empezar cualquier tarea:
- `mem_context` al inicio de sesión o tras una compaction para recuperar el estado anterior.
- `mem_search` cuando el usuario menciona algo que puede tener historial ("el bug de caché", "la composición PIM+Stock").

**Al cerrar sesión**: llamar `mem_session_summary` para dejar un resumen recuperable.
