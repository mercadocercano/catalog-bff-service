#!/bin/bash

# Script de prueba para el endpoint de dashboard stats
# Uso: ./test-dashboard.sh [jwt_token]

set -e

CATALOG_BFF_URL="${CATALOG_BFF_URL:-http://localhost:8085}"
JWT_TOKEN="${1:-}"

echo "=================================================="
echo "🧪 Test: Dashboard Stats Endpoint"
echo "=================================================="
echo ""
echo "Endpoint: GET ${CATALOG_BFF_URL}/api/v1/admin/dashboard/stats"
echo ""

# Construir headers
HEADERS=(-H "Content-Type: application/json")

if [ -n "$JWT_TOKEN" ]; then
  echo "🔐 Usando JWT token para autenticación"
  HEADERS+=(-H "Authorization: Bearer $JWT_TOKEN")
else
  echo "⚠️  No se proporcionó JWT token (puede fallar si hay autenticación)"
fi

echo ""
echo "------------------------------------------------"
echo "📊 Obteniendo estadísticas del dashboard..."
echo "------------------------------------------------"
echo ""

# Hacer request
RESPONSE=$(curl -s -w "\n%{http_code}" \
  "${HEADERS[@]}" \
  "${CATALOG_BFF_URL}/api/v1/admin/dashboard/stats")

# Separar body y status code
BODY=$(echo "$RESPONSE" | head -n -1)
STATUS=$(echo "$RESPONSE" | tail -n 1)

echo "Status: $STATUS"
echo ""

# Verificar status code
if [ "$STATUS" -eq 200 ]; then
  echo "✅ Request exitoso"
  echo ""
  echo "📋 Response (formato JSON):"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  echo ""
  
  # Mostrar resumen
  echo "------------------------------------------------"
  echo "📊 Resumen de Estadísticas"
  echo "------------------------------------------------"
  
  # Curación
  PENDING=$(echo "$BODY" | jq -r '.curation.pending // 0' 2>/dev/null || echo "N/A")
  APPROVED=$(echo "$BODY" | jq -r '.curation.approved_today // 0' 2>/dev/null || echo "N/A")
  REJECTED=$(echo "$BODY" | jq -r '.curation.rejected_today // 0' 2>/dev/null || echo "N/A")
  SCRAPED=$(echo "$BODY" | jq -r '.curation.total_scraped // 0' 2>/dev/null || echo "N/A")
  
  echo "🔍 Curación:"
  echo "   - Pendientes: $PENDING"
  echo "   - Aprobados hoy: $APPROVED"
  echo "   - Rechazados hoy: $REJECTED"
  echo "   - Total scrapeados: $SCRAPED"
  echo ""
  
  # Catálogo
  PRODUCTS=$(echo "$BODY" | jq -r '.catalog.total_products // 0' 2>/dev/null || echo "N/A")
  VARIANTS=$(echo "$BODY" | jq -r '.catalog.total_variants // 0' 2>/dev/null || echo "N/A")
  ACTIVE=$(echo "$BODY" | jq -r '.catalog.active_products // 0' 2>/dev/null || echo "N/A")
  CATEGORIES=$(echo "$BODY" | jq -r '.catalog.categories_count // 0' 2>/dev/null || echo "N/A")
  
  echo "📦 Catálogo:"
  echo "   - Total productos: $PRODUCTS"
  echo "   - Total variantes: $VARIANTS"
  echo "   - Productos activos: $ACTIVE"
  echo "   - Categorías: $CATEGORIES"
  echo ""
  
  # Tenants
  TOTAL_TENANTS=$(echo "$BODY" | jq -r '.tenants.total // 0' 2>/dev/null || echo "N/A")
  ACTIVE_TENANTS=$(echo "$BODY" | jq -r '.tenants.active // 0' 2>/dev/null || echo "N/A")
  NEW_TENANTS=$(echo "$BODY" | jq -r '.tenants.new_this_month // 0' 2>/dev/null || echo "N/A")
  
  echo "🏢 Tenants:"
  echo "   - Total: $TOTAL_TENANTS"
  echo "   - Activos: $ACTIVE_TENANTS"
  echo "   - Nuevos este mes: $NEW_TENANTS"
  echo ""
  
  # Servicios
  SERVICES_COUNT=$(echo "$BODY" | jq -r '.services | length // 0' 2>/dev/null || echo "N/A")
  echo "🔧 Servicios verificados: $SERVICES_COUNT"
  
  if command -v jq &> /dev/null; then
    echo ""
    echo "$BODY" | jq -r '.services[] | "   - \(.name): \(.status) (\(.latency_ms)ms)"' 2>/dev/null || true
  fi
  
  echo ""
  echo "=================================================="
  echo "✅ Test completado exitosamente"
  echo "=================================================="
  
elif [ "$STATUS" -eq 401 ]; then
  echo "❌ Error: No autorizado (401)"
  echo ""
  echo "Posibles causas:"
  echo "  - JWT token inválido o expirado"
  echo "  - Falta header Authorization"
  echo ""
  echo "Response:"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  exit 1
  
elif [ "$STATUS" -eq 403 ]; then
  echo "❌ Error: Prohibido (403)"
  echo ""
  echo "Posibles causas:"
  echo "  - El usuario no tiene rol 'marketplace_admin' o 'admin'"
  echo ""
  echo "Response:"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  exit 1
  
elif [ "$STATUS" -eq 500 ]; then
  echo "❌ Error: Error interno del servidor (500)"
  echo ""
  echo "Response:"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  exit 1
  
else
  echo "❌ Error inesperado (Status: $STATUS)"
  echo ""
  echo "Response:"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  exit 1
fi
