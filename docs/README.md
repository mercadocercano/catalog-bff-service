# Documentación — catalog-bff-service

BFF de composición de lecturas (PIM + Stock y auxiliares) del ecosistema Mercado Cercano. Índice navegable de la documentación del servicio.

> Para una visión general, comandos y variables de entorno, ver el [README raíz](../README.md).

## Architecture Decision Records

| ADR | Título | Estado | Fecha |
|-----|--------|--------|-------|
| [ADR-001](adr/ADR-001-bff-composicion-lecturas.md) | catalog-bff-service como servicio de composición de lecturas | Aceptado | 2026-06-10 |

## Arquitectura

- [Estrategia de cache in-memory](architecture/cache-strategy.md) — best-effort, decorator pattern, TTL configurable
- [Integración con tenant-service](architecture/tenant-integration.md) — resolución de stock policy con fallback

## Setup

- [Guía de despliegue](setup/deployment.md) — pre-requisitos, build, Docker, Kong

## Guías

- [Backoffice CRUD](guides/backoffice-crud.md) — endpoints de gestión de productos y variantes
- [Dashboard stats — endpoint](guides/dashboard-endpoint.md) — contrato del endpoint agregado de métricas
- [Dashboard stats — implementación](guides/dashboard-implementation.md) — orquestación, goroutines, archivos
- [Dashboard stats — resumen ejecutivo](guides/dashboard-summary.md) — visión de alto nivel
- [Dashboard stats — checklist](guides/dashboard-checklist.md) — checklist de implementación
- [Integración frontend (dashboard)](guides/frontend-integration.md) — consumo desde marketplace-admin
- [Testing](guides/testing.md) — estructura y ejecución de tests

## API

El contrato de API está versionado como OpenAPI en [`api-docs/openapi.yaml`](../api-docs/openapi.yaml).
