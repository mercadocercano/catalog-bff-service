# Deployment Guide - Catalog BFF Service

## 🚀 Guía de Despliegue

Este documento describe cómo desplegar el Catalog BFF Service con los nuevos endpoints de Backoffice CRUD.

---

## 📋 Pre-requisitos

### Servicios Requeridos

- ✅ **PIM Service** corriendo en puerto 8090
- ✅ **Stock Service** corriendo en puerto 8100 (opcional, para enriquecimiento)
- ✅ **Kong Gateway** configurado en puerto 8001
- ✅ **PostgreSQL** para PIM Service

### Herramientas

- Go 1.21+
- Docker & Docker Compose
- Make

---

## 🔧 Configuración

### Variables de Entorno

Crear archivo `.env` en la raíz del servicio:

```bash
# Servicios backend
PIM_SERVICE_URL=http://localhost:8090
STOCK_SERVICE_URL=http://localhost:8100
TENANT_SERVICE_URL=http://localhost:8080

# Puerto del servicio
PORT=8085

# Cache TTLs
TENANT_CONFIG_CACHE_TTL=60s
STOCK_CACHE_TTL=5s
CACHE_CLEANUP_INTERVAL=60s

# Logging
GIN_MODE=release  # o "debug" para desarrollo
LOG_LEVEL=info
```

### Docker Compose

El servicio ya está incluido en el `docker-compose.yml` principal del proyecto:

```yaml
catalog-bff-service:
  build:
    context: ./services/catalog-bff-service
    dockerfile: Dockerfile
  ports:
    - "8085:8085"
  environment:
    - PIM_SERVICE_URL=http://pim-service:8090
    - STOCK_SERVICE_URL=http://stock-service:8100
    - PORT=8085
  depends_on:
    - pim-service
    - stock-service
  networks:
    - saas-network
```

---

## 🏗️ Build

### Local (desarrollo)

```bash
cd services/catalog-bff-service

# Instalar dependencias
go mod download

# Build
go build -o catalog-bff-service

# Ejecutar
./catalog-bff-service
```

### Docker

```bash
# Build imagen
docker build -t catalog-bff-service:latest .

# Ejecutar contenedor
docker run -p 8085:8085 \
  -e PIM_SERVICE_URL=http://pim-service:8090 \
  -e STOCK_SERVICE_URL=http://stock-service:8100 \
  catalog-bff-service:latest
```

### Con Make (recomendado)

Desde la raíz del proyecto:

```bash
# Iniciar todos los servicios
make dev-start

# Solo BFF
make bff-start

# Rebuild BFF
make bff-rebuild
```

---

## 🧪 Verificación

### 1. Health Check

```bash
curl http://localhost:8085/health
```

**Respuesta esperada:**
```json
{
  "status": "healthy",
  "service": "catalog-bff-service"
}
```

### 2. Test de Endpoints

Ejecutar el script de prueba:

```bash
cd services/catalog-bff-service
./test-endpoints.sh
```

### 3. Logs

```bash
# Docker
docker logs catalog-bff-service -f

# Local
# Los logs se muestran en stdout
```

---

## 🔍 Troubleshooting

### Error: "PIM Service no disponible"

**Causa:** El BFF no puede conectarse a PIM Service

**Solución:**
```bash
# Verificar que PIM está corriendo
curl http://localhost:8090/health

# Verificar variable de entorno
echo $PIM_SERVICE_URL

# En Docker, usar nombre del servicio
PIM_SERVICE_URL=http://pim-service:8090
```

### Error: "missing_tenant"

**Causa:** Request sin header `X-Tenant-ID`

**Solución:**
```bash
# Agregar header en todos los requests
curl -H "X-Tenant-ID: your-tenant-id" ...
```

### Error: "connection refused"

**Causa:** Puerto 8085 ya está en uso

**Solución:**
```bash
# Verificar qué proceso usa el puerto
lsof -ti:8085

# Matar proceso
lsof -ti:8085 | xargs kill -9

# O cambiar puerto
PORT=8086 ./catalog-bff-service
```

### Logs de Debug

Activar modo debug:

```bash
GIN_MODE=debug ./catalog-bff-service
```

---

## 🌐 Kong Gateway Integration

### Configurar Rutas en Kong

```bash
# Crear servicio en Kong
curl -X POST http://localhost:8001/services \
  --data name=catalog-bff \
  --data url=http://catalog-bff-service:8085

# Crear ruta para backoffice
curl -X POST http://localhost:8001/services/catalog-bff/routes \
  --data paths[]=/backoffice \
  --data strip_path=false

# Agregar plugin de autenticación JWT
curl -X POST http://localhost:8001/services/catalog-bff/plugins \
  --data name=jwt
```

### Acceso vía Kong

Una vez configurado:

```bash
# Antes (directo al BFF)
curl http://localhost:8085/api/v1/backoffice/products

# Después (vía Kong)
curl http://localhost:8001/api/v1/backoffice/products \
  -H "Authorization: Bearer $TOKEN"
```

---

## 📊 Monitoreo

### Métricas

El servicio expone métricas básicas:

- Requests por endpoint
- Latencia promedio
- Errores por tipo
- Cache hit/miss ratio

### Health Checks

```bash
# Health check simple
curl http://localhost:8085/health

# Health check detallado (futuro)
curl http://localhost:8085/health/detailed
```

### Logs Estructurados

Los logs incluyen:
- Timestamp
- Nivel (INFO, WARN, ERROR)
- Tenant ID
- Request ID
- Duración de request
- Status code

Ejemplo:
```
2026-02-02T10:30:45Z INFO [tenant:123e4567] GET /api/v1/backoffice/products 200 45ms
```

---

## 🔒 Seguridad

### Headers Obligatorios

Todos los endpoints requieren:

```bash
X-Tenant-ID: {tenant_uuid}
Authorization: Bearer {jwt_token}
```

### CORS

En producción, configurar CORS en Kong Gateway:

```bash
curl -X POST http://localhost:8001/services/catalog-bff/plugins \
  --data name=cors \
  --data config.origins=https://backoffice.tudominio.com \
  --data config.methods=GET,POST,PUT,PATCH,DELETE \
  --data config.headers=X-Tenant-ID,Authorization,Content-Type
```

### Rate Limiting

Configurar en Kong:

```bash
curl -X POST http://localhost:8001/services/catalog-bff/plugins \
  --data name=rate-limiting \
  --data config.minute=100 \
  --data config.policy=local
```

---

## 🚀 Deployment a Producción

### Checklist Pre-Deploy

- [ ] Tests pasando (`go test ./test/... -v`)
- [ ] Build exitoso (`go build`)
- [ ] Variables de entorno configuradas
- [ ] PIM Service accesible
- [ ] Kong Gateway configurado
- [ ] Health check funcionando
- [ ] Logs estructurados activos
- [ ] Métricas habilitadas

### Deploy con Docker Compose

```bash
# Desde raíz del proyecto
make prod-build
make prod-up

# Verificar
docker ps | grep catalog-bff
docker logs catalog-bff-service
```

### Deploy con Kubernetes (futuro)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog-bff-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: catalog-bff
  template:
    metadata:
      labels:
        app: catalog-bff
    spec:
      containers:
      - name: catalog-bff
        image: catalog-bff-service:latest
        ports:
        - containerPort: 8085
        env:
        - name: PIM_SERVICE_URL
          value: "http://pim-service:8090"
        - name: STOCK_SERVICE_URL
          value: "http://stock-service:8100"
        livenessProbe:
          httpGet:
            path: /health
            port: 8085
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8085
          initialDelaySeconds: 5
          periodSeconds: 10
```

---

## 📈 Escalabilidad

### Horizontal Scaling

El servicio es **stateless** y puede escalarse horizontalmente:

```bash
# Docker Compose
docker-compose up --scale catalog-bff-service=3

# Kubernetes
kubectl scale deployment catalog-bff-service --replicas=5
```

### Cache

El servicio usa cache in-memory para:
- Configuración de tenant (TTL: 60s)
- Stock availability (TTL: 5s)

En producción multi-instancia, considerar Redis:

```go
// Futuro: Redis cache
redisCache := cache.NewRedisCache(redisClient)
```

### Load Balancing

Kong Gateway hace load balancing automático:

```bash
# Agregar múltiples targets
curl -X POST http://localhost:8001/upstreams/catalog-bff/targets \
  --data target=catalog-bff-1:8085
curl -X POST http://localhost:8001/upstreams/catalog-bff/targets \
  --data target=catalog-bff-2:8085
```

---

## 🔄 Rollback

### Docker Compose

```bash
# Ver versiones disponibles
docker images | grep catalog-bff

# Rollback a versión anterior
docker-compose down
docker tag catalog-bff-service:v1.0.0 catalog-bff-service:latest
docker-compose up -d
```

### Kubernetes

```bash
# Ver historial de deployments
kubectl rollout history deployment/catalog-bff-service

# Rollback a versión anterior
kubectl rollout undo deployment/catalog-bff-service

# Rollback a versión específica
kubectl rollout undo deployment/catalog-bff-service --to-revision=2
```

---

## 📚 Referencias

- [README.md](./README.md) - Información general
- [BACKOFFICE_CRUD.md](./BACKOFFICE_CRUD.md) - Documentación de API
- [TEST_README.md](./TEST_README.md) - Guía de testing
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Decisiones de arquitectura

---

## 📞 Soporte

En caso de problemas:

1. Verificar logs: `docker logs catalog-bff-service -f`
2. Verificar health: `curl http://localhost:8085/health`
3. Ejecutar tests: `go test ./test/... -v`
4. Revisar variables de entorno
5. Verificar conectividad con PIM Service

---

**Última actualización:** 2026-02-02  
**Versión:** 1.0.0  
**Estado:** ✅ Production Ready
