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

`PIM_SERVICE_URL`, `STOCK_SERVICE_URL`, `TENANT_SERVICE_URL`, `SCRAPER_SERVICE_URL`, `IAM_SERVICE_URL`, `PORT=8085`, `JWT_SECRET`, `TENANT_CONFIG_CACHE_TTL` (p. ej. 60s), `STOCK_CACHE_TTL` (p. ej. 5s), `CACHE_CLEANUP_INTERVAL`.

## Caché

In-memory con TTL configurable; no distribuida.

## Reglas compartidas

Desde el workspace: **`ai-tools/rules/`** (`architecture.md`, `api-gateway.md`, `multi-tenant.md`, `api-response-format.md`). Ruta relativa típica: `../../ai-tools/rules/`.
