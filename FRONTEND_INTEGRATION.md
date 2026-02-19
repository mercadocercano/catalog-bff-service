# Frontend Integration Guide - Dashboard Stats

Esta guía describe cómo integrar el endpoint `/api/v1/admin/dashboard/stats` en el frontend marketplace-admin.

## 🎯 Endpoint a Consumir

```
GET http://localhost:8001/catalog-bff/api/v1/admin/dashboard/stats
```

**IMPORTANTE**: Usar Kong Gateway (`:8001`), NO conectar directamente al BFF (`:8085`).

---

## 📦 Paso 1: Crear el Hook de React

Crear archivo: `src/hooks/useDashboardStats.ts`

```typescript
import { useState, useEffect } from 'react';

interface CurationStats {
  pending: number;
  approved_today: number;
  rejected_today: number;
  total_scraped: number;
}

interface CategoryCount {
  id: string;
  name: string;
  count: number;
}

interface CatalogStats {
  total_products: number;
  total_variants: number;
  active_products: number;
  categories_count: number;
  top_categories: CategoryCount[];
}

interface TenantInfo {
  id: string;
  name: string;
  plan: string;
  status: string;
  last_activity: string;
}

interface TenantStats {
  total: number;
  active: number;
  new_this_month: number;
  recent: TenantInfo[];
}

interface ServiceHealth {
  name: string;
  status: 'up' | 'down' | 'degraded';
  latency_ms: number;
  uptime_percent: number;
  last_check: string;
}

interface DashboardStats {
  curation: CurationStats;
  catalog: CatalogStats;
  tenants: TenantStats;
  services: ServiceHealth[];
}

interface UseDashboardStatsResult {
  stats: DashboardStats | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

export function useDashboardStats(autoRefresh = true, intervalMs = 30000): UseDashboardStatsResult {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await fetch('http://localhost:8001/catalog-bff/api/v1/admin/dashboard/stats', {
        headers: {
          'Content-Type': 'application/json',
          // TODO: Agregar JWT token cuando esté implementado
          // 'Authorization': `Bearer ${getToken()}`,
        },
      });

      if (!response.ok) {
        throw new Error(`Error ${response.status}: ${response.statusText}`);
      }

      const data = await response.json();
      setStats(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error desconocido');
      console.error('Error fetching dashboard stats:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();

    if (autoRefresh) {
      const interval = setInterval(fetchStats, intervalMs);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, intervalMs]);

  return {
    stats,
    loading,
    error,
    refresh: fetchStats,
  };
}
```

---

## 🎨 Paso 2: Crear la Página de Dashboard

Crear archivo: `src/app/admin/dashboard/page.tsx`

```typescript
'use client';

import { useDashboardStats } from '@/hooks/useDashboardStats';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';

export default function DashboardPage() {
  const { stats, loading, error, refresh } = useDashboardStats(true, 30000);

  if (loading && !stats) {
    return <DashboardSkeleton />;
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <h3 className="text-red-800 font-semibold">Error al cargar estadísticas</h3>
          <p className="text-red-600 mt-2">{error}</p>
          <button
            onClick={refresh}
            className="mt-4 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700"
          >
            Reintentar
          </button>
        </div>
      </div>
    );
  }

  if (!stats) {
    return null;
  }

  return (
    <div className="p-8 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Dashboard Administrativo</h1>
          <p className="text-gray-500 mt-1">Vista consolidada del sistema</p>
        </div>
        <button
          onClick={refresh}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
          disabled={loading}
        >
          {loading ? 'Actualizando...' : 'Actualizar'}
        </button>
      </div>

      {/* Curación Stats */}
      <div>
        <h2 className="text-xl font-semibold mb-4">📋 Curación de Productos</h2>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <StatCard
            title="Pendientes"
            value={stats.curation.pending}
            color="yellow"
          />
          <StatCard
            title="Aprobados Hoy"
            value={stats.curation.approved_today}
            color="green"
          />
          <StatCard
            title="Rechazados Hoy"
            value={stats.curation.rejected_today}
            color="red"
          />
          <StatCard
            title="Total Scrapeados"
            value={stats.curation.total_scraped}
            color="blue"
          />
        </div>
      </div>

      {/* Catálogo Stats */}
      <div>
        <h2 className="text-xl font-semibold mb-4">📦 Catálogo</h2>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <StatCard
            title="Total Productos"
            value={stats.catalog.total_products}
            color="blue"
          />
          <StatCard
            title="Variantes"
            value={stats.catalog.total_variants}
            color="purple"
          />
          <StatCard
            title="Productos Activos"
            value={stats.catalog.active_products}
            color="green"
          />
          <StatCard
            title="Categorías"
            value={stats.catalog.categories_count}
            color="orange"
          />
        </div>

        {/* Top Categorías */}
        {stats.catalog.top_categories.length > 0 && (
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>Top Categorías</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                {stats.catalog.top_categories.map((cat, idx) => (
                  <div
                    key={cat.id}
                    className="flex justify-between items-center p-2 hover:bg-gray-50 rounded"
                  >
                    <div className="flex items-center gap-2">
                      <span className="text-gray-500 font-mono">#{idx + 1}</span>
                      <span className="font-medium">{cat.name}</span>
                    </div>
                    <Badge variant="secondary">{cat.count} productos</Badge>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Tenants Stats */}
      <div>
        <h2 className="text-xl font-semibold mb-4">🏢 Tenants</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <StatCard
            title="Total Tenants"
            value={stats.tenants.total}
            color="blue"
          />
          <StatCard
            title="Activos"
            value={stats.tenants.active}
            color="green"
          />
          <StatCard
            title="Nuevos Este Mes"
            value={stats.tenants.new_this_month}
            color="purple"
          />
        </div>

        {/* Tenants Recientes */}
        {stats.tenants.recent.length > 0 && (
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>Tenants Recientes</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                {stats.tenants.recent.map((tenant) => (
                  <div
                    key={tenant.id}
                    className="flex justify-between items-center p-2 hover:bg-gray-50 rounded"
                  >
                    <div>
                      <p className="font-medium">{tenant.name}</p>
                      <p className="text-sm text-gray-500">Plan: {tenant.plan}</p>
                    </div>
                    <Badge
                      variant={tenant.status === 'active' ? 'default' : 'secondary'}
                    >
                      {tenant.status}
                    </Badge>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Services Health */}
      <div>
        <h2 className="text-xl font-semibold mb-4">🔧 Estado de Servicios</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {stats.services.map((service) => (
            <Card key={service.name}>
              <CardHeader>
                <CardTitle className="text-sm flex justify-between items-center">
                  <span>{service.name}</span>
                  <StatusBadge status={service.status} />
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-gray-500">Latencia:</span>
                    <span className="font-mono">{service.latency_ms}ms</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-gray-500">Uptime:</span>
                    <span className="font-mono">{service.uptime_percent}%</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </div>
  );
}

// Componentes auxiliares

function StatCard({ title, value, color }: { title: string; value: number; color: string }) {
  const colorClasses = {
    blue: 'bg-blue-50 border-blue-200 text-blue-700',
    green: 'bg-green-50 border-green-200 text-green-700',
    red: 'bg-red-50 border-red-200 text-red-700',
    yellow: 'bg-yellow-50 border-yellow-200 text-yellow-700',
    purple: 'bg-purple-50 border-purple-200 text-purple-700',
    orange: 'bg-orange-50 border-orange-200 text-orange-700',
  };

  return (
    <Card>
      <CardContent className="pt-6">
        <p className="text-sm text-gray-500 mb-2">{title}</p>
        <p className={`text-3xl font-bold ${colorClasses[color as keyof typeof colorClasses]}`}>
          {value.toLocaleString()}
        </p>
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: string }) {
  const variants = {
    up: 'default',
    down: 'destructive',
    degraded: 'secondary',
  };

  return (
    <Badge variant={variants[status as keyof typeof variants] as any}>
      {status}
    </Badge>
  );
}

function DashboardSkeleton() {
  return (
    <div className="p-8 space-y-6">
      <Skeleton className="h-12 w-64" />
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-32" />
        ))}
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-32" />
        ))}
      </div>
    </div>
  );
}
```

---

## 🔐 Paso 3: Agregar Autenticación (Cuando esté lista)

Modificar `useDashboardStats.ts` para incluir JWT:

```typescript
const fetchStats = async () => {
  // Obtener token de autenticación
  const token = localStorage.getItem('jwt_token'); // O tu método de obtención

  const response = await fetch('http://localhost:8001/catalog-bff/api/v1/admin/dashboard/stats', {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  });

  // ... resto del código
};
```

---

## 📡 Paso 4: Configurar Kong Gateway (Opcional)

Si Kong no está configurado para el endpoint, agregar en `kong.yml`:

```yaml
services:
  - name: catalog-bff-service
    url: http://catalog-bff-service:8085
    routes:
      - name: catalog-bff-admin-dashboard
        paths:
          - /catalog-bff/api/v1/admin/dashboard
        strip_path: false
    plugins:
      - name: jwt
        config:
          claims_to_verify:
            - exp
      - name: rate-limiting
        config:
          minute: 60
          policy: local
```

---

## 🧪 Testing del Frontend

### 1. Verificar endpoint funciona

```bash
curl http://localhost:8001/catalog-bff/api/v1/admin/dashboard/stats | jq '.'
```

### 2. Probar en desarrollo

```bash
cd marketplace-admin
npm run dev
```

Visitar: `http://localhost:3004/admin/dashboard`

### 3. Verificar auto-refresh

El hook está configurado para refrescar cada 30 segundos automáticamente.

---

## 🎨 Diseño Recomendado

### Layout

```
┌─────────────────────────────────────────────────┐
│  Dashboard Administrativo        [Actualizar]   │
├─────────────────────────────────────────────────┤
│                                                 │
│  📋 Curación de Productos                       │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐          │
│  │ 12   │ │  5   │ │  2   │ │ 1543 │          │
│  │Pend. │ │Aprob.│ │Rech. │ │Scrap.│          │
│  └──────┘ └──────┘ └──────┘ └──────┘          │
│                                                 │
│  📦 Catálogo                                    │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐          │
│  │ 2341 │ │ 8923 │ │ 2103 │ │  45  │          │
│  │Prod. │ │Vari. │ │Activ.│ │Categ.│          │
│  └──────┘ └──────┘ └──────┘ └──────┘          │
│                                                 │
│  Top Categorías                                 │
│  #1 Herramientas         543 productos          │
│  #2 Materiales          412 productos          │
│                                                 │
│  🏢 Tenants                                     │
│  ┌──────┐ ┌──────┐ ┌──────┐                   │
│  │  15  │ │  14  │ │   3  │                   │
│  │Total │ │Activ.│ │Nuevos│                   │
│  └──────┘ └──────┘ └──────┘                   │
│                                                 │
│  🔧 Estado de Servicios                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │pim-serv  │ │scraper   │ │iam-serv  │       │
│  │  UP ✓    │ │  UP ✓    │ │  UP ✓    │       │
│  │  45ms    │ │ 120ms    │ │  30ms    │       │
│  │ 99.8%    │ │ 99.5%    │ │ 99.9%    │       │
│  └──────────┘ └──────────┘ └──────────┘       │
└─────────────────────────────────────────────────┘
```

---

## 🚀 Optimizaciones

### 1. Cache en el Frontend

```typescript
// Guardar en sessionStorage para evitar llamadas innecesarias
const cacheKey = 'dashboard-stats';
const cacheTTL = 30000; // 30 segundos

const cachedData = sessionStorage.getItem(cacheKey);
if (cachedData) {
  const { data, timestamp } = JSON.parse(cachedData);
  if (Date.now() - timestamp < cacheTTL) {
    setStats(data);
    setLoading(false);
    return;
  }
}
```

### 2. Error Boundaries

```typescript
import { ErrorBoundary } from 'react-error-boundary';

<ErrorBoundary
  fallback={<div>Error al cargar dashboard</div>}
  onReset={() => window.location.reload()}
>
  <DashboardPage />
</ErrorBoundary>
```

### 3. Loading States Optimistas

Mostrar datos anteriores mientras se actualiza:

```typescript
const [previousStats, setPreviousStats] = useState<DashboardStats | null>(null);

// En fetchStats:
if (stats) {
  setPreviousStats(stats);
}

// En el render:
const displayStats = stats || previousStats;
```

---

## 📝 Notas Importantes

1. **Kong Gateway**: SIEMPRE usar Kong (`:8001`), nunca conectar directo al BFF
2. **Auto-refresh**: Configurado cada 30 segundos por defecto
3. **Error handling**: El hook maneja errores gracefully
4. **Loading states**: Skeleton loader mientras carga
5. **TypeScript**: Interfaces completas para type safety

---

## 🔗 Referencias

- Documentación del endpoint: [DASHBOARD_ENDPOINT.md](./DASHBOARD_ENDPOINT.md)
- Implementación backend: [IMPLEMENTACION_DASHBOARD.md](./IMPLEMENTACION_DASHBOARD.md)
- Resumen ejecutivo: [DASHBOARD_SUMMARY.md](./DASHBOARD_SUMMARY.md)
