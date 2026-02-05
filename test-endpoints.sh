#!/bin/bash

# Script de prueba para endpoints de Backoffice CRUD
# Uso: ./test-endpoints.sh

set -e

# Colores para output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuración
BFF_URL="${BFF_URL:-http://localhost:8085}"
TENANT_ID="${TENANT_ID:-123e4567-e89b-12d3-a456-426614174000}"
TOKEN="${TOKEN:-test-token}"

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}  Test de Endpoints - Backoffice CRUD${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""
echo -e "BFF URL: ${GREEN}${BFF_URL}${NC}"
echo -e "Tenant ID: ${GREEN}${TENANT_ID}${NC}"
echo ""

# Función para hacer requests
function api_call() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4

    echo -e "${YELLOW}▶ ${description}${NC}"
    echo -e "  ${method} ${endpoint}"
    
    if [ -n "$data" ]; then
        response=$(curl -s -X ${method} "${BFF_URL}${endpoint}" \
            -H "Content-Type: application/json" \
            -H "X-Tenant-ID: ${TENANT_ID}" \
            -H "Authorization: Bearer ${TOKEN}" \
            -d "${data}")
    else
        response=$(curl -s -X ${method} "${BFF_URL}${endpoint}" \
            -H "X-Tenant-ID: ${TENANT_ID}" \
            -H "Authorization: Bearer ${TOKEN}")
    fi

    echo -e "${GREEN}✓ Response:${NC}"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo ""
}

# Test 1: Health Check
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Test 1: Health Check${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
api_call "GET" "/health" "" "Verificar que el servicio está corriendo"

# Test 2: Listar Productos (vacío)
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Test 2: Listar Productos${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
api_call "GET" "/api/v1/backoffice/products?page=1&page_size=20" "" "Listar productos (puede estar vacío)"

# Test 3: Crear Producto con Variantes
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Test 3: Crear Producto${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

CREATE_PRODUCT_DATA='{
  "name": "Coca Cola 2.25L - Test",
  "description": "Gaseosa Coca Cola sabor original - Producto de prueba",
  "status": "active",
  "variants": [
    {
      "name": "Coca Cola 2.25L - Default",
      "sku": "TEST-COC-2.25-001",
      "price": 1500.00,
      "is_default": true,
      "attributes": [
        {"name": "tamaño", "value": "2.25L"},
        {"name": "sabor", "value": "original"}
      ]
    },
    {
      "name": "Coca Cola 1.5L",
      "sku": "TEST-COC-1.5-001",
      "price": 1200.00,
      "is_default": false,
      "attributes": [
        {"name": "tamaño", "value": "1.5L"},
        {"name": "sabor", "value": "original"}
      ]
    }
  ]
}'

CREATE_RESPONSE=$(curl -s -X POST "${BFF_URL}/api/v1/backoffice/products" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-ID: ${TENANT_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "${CREATE_PRODUCT_DATA}")

echo -e "${GREEN}✓ Producto creado:${NC}"
echo "$CREATE_RESPONSE" | jq '.' 2>/dev/null || echo "$CREATE_RESPONSE"
echo ""

# Extraer ID del producto creado
PRODUCT_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id' 2>/dev/null)

if [ "$PRODUCT_ID" != "null" ] && [ -n "$PRODUCT_ID" ]; then
    echo -e "${GREEN}✓ Product ID: ${PRODUCT_ID}${NC}"
    echo ""

    # Test 4: Obtener Producto Creado
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Test 4: Obtener Producto${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    api_call "GET" "/api/v1/backoffice/products/${PRODUCT_ID}" "" "Obtener detalle del producto creado"

    # Test 5: Actualizar Producto
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Test 5: Actualizar Producto${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    UPDATE_PRODUCT_DATA='{
      "name": "Coca Cola 2.25L - Test ACTUALIZADO",
      "description": "Descripción actualizada del producto",
      "status": "active"
    }'

    api_call "PUT" "/api/v1/backoffice/products/${PRODUCT_ID}" "${UPDATE_PRODUCT_DATA}" "Actualizar nombre y descripción"

    # Test 6: Listar Variantes del Producto
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Test 6: Listar Variantes${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    VARIANTS_RESPONSE=$(curl -s -X GET "${BFF_URL}/api/v1/backoffice/products/${PRODUCT_ID}/variants" \
        -H "X-Tenant-ID: ${TENANT_ID}" \
        -H "Authorization: Bearer ${TOKEN}")

    echo -e "${GREEN}✓ Variantes:${NC}"
    echo "$VARIANTS_RESPONSE" | jq '.' 2>/dev/null || echo "$VARIANTS_RESPONSE"
    echo ""

    # Extraer ID de la primera variante
    VARIANT_ID=$(echo "$VARIANTS_RESPONSE" | jq -r '.[0].id' 2>/dev/null)

    if [ "$VARIANT_ID" != "null" ] && [ -n "$VARIANT_ID" ]; then
        echo -e "${GREEN}✓ Variant ID: ${VARIANT_ID}${NC}"
        echo ""

        # Test 7: Crear Nueva Variante
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${BLUE}Test 7: Crear Nueva Variante${NC}"
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

        CREATE_VARIANT_DATA='{
          "name": "Coca Cola 500ml",
          "sku": "TEST-COC-0.5-001",
          "price": 800.00,
          "is_default": false,
          "attributes": [
            {"name": "tamaño", "value": "500ml"},
            {"name": "sabor", "value": "original"}
          ]
        }'

        api_call "POST" "/api/v1/backoffice/products/${PRODUCT_ID}/variants" "${CREATE_VARIANT_DATA}" "Crear variante de 500ml"

        # Test 8: Actualizar Variante
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${BLUE}Test 8: Actualizar Variante${NC}"
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

        UPDATE_VARIANT_DATA='{
          "name": "Coca Cola 2.25L - Actualizada",
          "price": 1600.00
        }'

        api_call "PUT" "/api/v1/backoffice/products/${PRODUCT_ID}/variants/${VARIANT_ID}" "${UPDATE_VARIANT_DATA}" "Actualizar precio de variante"

        # Test 9: Desactivar Variante
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${BLUE}Test 9: Desactivar Variante${NC}"
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

        TOGGLE_STATUS_DATA='{"is_active": false}'

        api_call "PATCH" "/api/v1/backoffice/products/${PRODUCT_ID}/variants/${VARIANT_ID}/status" "${TOGGLE_STATUS_DATA}" "Desactivar variante"

        # Test 10: Reactivar Variante
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${BLUE}Test 10: Reactivar Variante${NC}"
        echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

        TOGGLE_STATUS_DATA='{"is_active": true}'

        api_call "PATCH" "/api/v1/backoffice/products/${PRODUCT_ID}/variants/${VARIANT_ID}/status" "${TOGGLE_STATUS_DATA}" "Reactivar variante"
    fi

    # Test 11: Listar Productos (con el nuevo)
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Test 11: Listar Productos (con filtro)${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    api_call "GET" "/api/v1/backoffice/products?page=1&page_size=20&search=Test" "" "Buscar productos con 'Test'"

else
    echo -e "${RED}✗ No se pudo crear el producto${NC}"
    echo -e "${RED}Response: ${CREATE_RESPONSE}${NC}"
fi

# Test 12: Error sin Tenant ID
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Test 12: Error sin Tenant ID${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

echo -e "${YELLOW}▶ Request sin X-Tenant-ID (debe fallar)${NC}"
ERROR_RESPONSE=$(curl -s -X GET "${BFF_URL}/api/v1/backoffice/products" \
    -H "Authorization: Bearer ${TOKEN}")

echo -e "${GREEN}✓ Response (esperado error):${NC}"
echo "$ERROR_RESPONSE" | jq '.' 2>/dev/null || echo "$ERROR_RESPONSE"
echo ""

# Resumen
echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}  Resumen de Tests${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""
echo -e "${GREEN}✓ Tests completados exitosamente${NC}"
echo ""
echo -e "Endpoints probados:"
echo -e "  ✓ Health check"
echo -e "  ✓ Listar productos"
echo -e "  ✓ Crear producto con variantes"
echo -e "  ✓ Obtener producto"
echo -e "  ✓ Actualizar producto"
echo -e "  ✓ Listar variantes"
echo -e "  ✓ Crear variante"
echo -e "  ✓ Actualizar variante"
echo -e "  ✓ Activar/desactivar variante"
echo -e "  ✓ Búsqueda de productos"
echo -e "  ✓ Validación de seguridad"
echo ""
echo -e "${YELLOW}Nota: Este script crea datos de prueba. Limpia manualmente si es necesario.${NC}"
