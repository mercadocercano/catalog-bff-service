package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	
	"catalog-bff-service/src/dto"
	"catalog-bff-service/src/infrastructure/stock/client"
)

// ProductVariantHandler maneja las operaciones de variantes para backoffice
type ProductVariantHandler struct {
	pimServiceURL string
	stockClient   client.StockAvailabilityClient
	httpClient    *http.Client
}

// NewProductVariantHandler crea un nuevo handler de variantes
func NewProductVariantHandler(pimServiceURL string, stockClient client.StockAvailabilityClient) *ProductVariantHandler {
	return &ProductVariantHandler{
		pimServiceURL: pimServiceURL,
		stockClient:   stockClient,
		httpClient:    &http.Client{},
	}
}

// ============================================================================
// Crear Variante
// ============================================================================

// CreateVariant crea una nueva variante para un producto
// POST /api/v1/backoffice/products/:product_id/variants
func (h *ProductVariantHandler) CreateVariant(c *gin.Context) {
	productID := c.Param("id")

	var req dto.CreateVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: "Datos inválidos",
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}

	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Validaciones de negocio
	if err := h.validateCreateVariant(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	// Construir payload para PIM (incluir product_id requerido)
	pimReq := map[string]interface{}{
		"product_id": productID,
		"name":       req.Name,
	}
	
	// SKU es opcional en PIM pero si viene del BFF, enviarlo
	if req.SKU != "" {
		pimReq["sku"] = req.SKU
	}
	
	// Price es requerido
	pimReq["price"] = req.Price
	
	// is_default es opcional
	if req.IsDefault {
		pimReq["is_default"] = req.IsDefault
	}
	
	// Attributes opcionales
	if len(req.Attributes) > 0 {
		pimReq["attributes"] = req.Attributes
	}

	// Llamar a PIM Service
	pimURL := fmt.Sprintf("%s/api/v1/products/%s/variants", h.pimServiceURL, productID)
	body, _ := json.Marshal(pimReq)

	httpReq, err := http.NewRequest("POST", pimURL, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Error al construir request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, dto.ErrorResponse{
			Error:   "pim_unavailable",
			Message: "PIM Service no disponible",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "product_not_found",
			Message: "Producto no encontrado",
		})
		return
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error al crear variante: %s", string(body)),
		})
		return
	}

	var pimResp dto.PIMVariantResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	response := dto.VariantCreatedResponse{
		ID:        pimResp.ID,
		ProductID: pimResp.ProductID,
		Name:      pimResp.Name,
		SKU:       pimResp.SKU,
		CreatedAt: pimResp.CreatedAt,
	}

	c.JSON(http.StatusCreated, response)
}

// ============================================================================
// Listar Variantes de un Producto
// ============================================================================

// ListProductVariants lista las variantes de un producto con stock
// GET /api/v1/backoffice/products/:product_id/variants
func (h *ProductVariantHandler) ListProductVariants(c *gin.Context) {
	productID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Llamar a PIM para obtener variantes
	pimURL := fmt.Sprintf("%s/api/v1/products/%s/variants", h.pimServiceURL, productID)
	pimReq, err := http.NewRequest("GET", pimURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Error al construir request",
		})
		return
	}

	pimReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		pimReq.Header.Set("Authorization", authHeader)
	}

	resp, err := h.httpClient.Do(pimReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, dto.ErrorResponse{
			Error:   "pim_unavailable",
			Message: "PIM Service no disponible",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "product_not_found",
			Message: "Producto no encontrado",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error en PIM: %s", string(body)),
		})
		return
	}

	// El PIM devuelve {variants: [...], pagination: {...}}
	var pimResp struct {
		Variants   []dto.PIMVariantResponse `json:"variants"`
		Pagination map[string]interface{}   `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	// Enriquecer con información de stock
	variants := h.enrichVariantsWithStock(pimResp.Variants, tenantID)

	c.JSON(http.StatusOK, variants)
}

// ============================================================================
// Obtener Variante Específica
// ============================================================================

// GetVariant obtiene el detalle de una variante con stock
// GET /api/v1/backoffice/products/:product_id/variants/:variant_id
func (h *ProductVariantHandler) GetVariant(c *gin.Context) {
	productID := c.Param("id")
	variantID := c.Param("variant_id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Llamar a PIM
	pimURL := fmt.Sprintf("%s/api/v1/products/%s/variants/%s", h.pimServiceURL, productID, variantID)
	pimReq, err := http.NewRequest("GET", pimURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Error al construir request",
		})
		return
	}

	pimReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		pimReq.Header.Set("Authorization", authHeader)
	}

	resp, err := h.httpClient.Do(pimReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, dto.ErrorResponse{
			Error:   "pim_unavailable",
			Message: "PIM Service no disponible",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "variant_not_found",
			Message: "Variante no encontrada",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error en PIM: %s", string(body)),
		})
		return
	}

	var pimVariant dto.PIMVariantResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimVariant); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	// Enriquecer con stock
	variant := h.mapVariantWithStock(pimVariant, tenantID)

	c.JSON(http.StatusOK, variant)
}

// ============================================================================
// Actualizar Variante
// ============================================================================

// UpdateVariant actualiza una variante existente
// PUT /api/v1/backoffice/products/:product_id/variants/:variant_id
func (h *ProductVariantHandler) UpdateVariant(c *gin.Context) {
	productID := c.Param("id")
	variantID := c.Param("variant_id")

	var req dto.UpdateVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: "Datos inválidos",
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}

	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Construir payload para PIM (solo campos no vacíos)
	pimReq := make(map[string]interface{})
	
	if req.Name != "" {
		pimReq["name"] = req.Name
	}
	if req.SKU != "" {
		pimReq["sku"] = req.SKU
	}
	if req.Price > 0 {
		pimReq["price"] = req.Price
	}
	if req.IsDefault {
		pimReq["is_default"] = req.IsDefault
	}
	if len(req.Attributes) > 0 {
		pimReq["attributes"] = req.Attributes
	}

	// Llamar a PIM Service
	pimURL := fmt.Sprintf("%s/api/v1/products/%s/variants/%s", h.pimServiceURL, productID, variantID)
	body, _ := json.Marshal(pimReq)

	httpReq, err := http.NewRequest("PUT", pimURL, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Error al construir request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, dto.ErrorResponse{
			Error:   "pim_unavailable",
			Message: "PIM Service no disponible",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "variant_not_found",
			Message: "Variante no encontrada",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error al actualizar: %s", string(body)),
		})
		return
	}

	var pimResp dto.PIMVariantResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	// Mapear a DTO de backoffice
	variant := h.mapVariantWithStock(pimResp, tenantID)

	c.JSON(http.StatusOK, variant)
}

// ============================================================================
// Activar/Desactivar Variante
// ============================================================================

// ToggleVariantStatus activa o desactiva una variante
// PATCH /api/v1/backoffice/products/:product_id/variants/:variant_id/status
func (h *ProductVariantHandler) ToggleVariantStatus(c *gin.Context) {
	productID := c.Param("id")
	variantID := c.Param("variant_id")

	var req dto.ToggleVariantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: "Datos inválidos",
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}

	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Primero obtener la variante actual para preservar sus datos
	getURL := fmt.Sprintf("%s/api/v1/products/%s/variants/%s", h.pimServiceURL, productID, variantID)
	getReq, _ := http.NewRequest("GET", getURL, nil)
	getReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		getReq.Header.Set("Authorization", authHeader)
	}

	getResp, err := h.httpClient.Do(getReq)
	if err != nil || getResp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, dto.ErrorResponse{
			Error:   "pim_unavailable",
			Message: "No se pudo obtener la variante",
		})
		return
	}

	var currentVariant dto.PIMVariantResponse
	json.NewDecoder(getResp.Body).Decode(&currentVariant)
	getResp.Body.Close()

	// Determinar el nuevo status
	newStatus := "inactive"
	if req.IsActive {
		newStatus = "active"
	}

	// Construir update con todos los campos necesarios
	updateReq := map[string]interface{}{
		"name":   currentVariant.Name,
		"status": newStatus,
	}
	
	// Incluir SKU si existe (evitar validación min)
	if currentVariant.SKU != "" {
		updateReq["sku"] = currentVariant.SKU
	}

	pimURL := fmt.Sprintf("%s/api/v1/products/%s/variants/%s", h.pimServiceURL, productID, variantID)
	body, _ := json.Marshal(updateReq)

	httpReq, err := http.NewRequest("PUT", pimURL, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Error al construir request",
		})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		httpReq.Header.Set("Authorization", authHeader)
	}

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, dto.ErrorResponse{
			Error:   "pim_unavailable",
			Message: "PIM Service no disponible",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "variant_not_found",
			Message: "Variante no encontrada",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error al cambiar status: %s", string(body)),
		})
		return
	}

	var pimResp dto.PIMVariantResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        pimResp.ID,
		"status":    pimResp.Status,
		"is_active": pimResp.Status == "active",
		"message":   fmt.Sprintf("Variante %s correctamente", newStatus),
	})
}

// ============================================================================
// Helpers
// ============================================================================

func (h *ProductVariantHandler) enrichVariantsWithStock(pimVariants []dto.PIMVariantResponse, tenantID string) []dto.VariantSummary {
	variants := make([]dto.VariantSummary, len(pimVariants))

	for i, pv := range pimVariants {
		variants[i] = h.mapVariantWithStock(pv, tenantID)
	}

	return variants
}

func (h *ProductVariantHandler) mapVariantWithStock(pv dto.PIMVariantResponse, tenantID string) dto.VariantSummary {
	variant := dto.VariantSummary{
		ID:         pv.ID,
		Name:       pv.Name,
		SKU:        pv.SKU,
		Price:      pv.Price,
		IsDefault:  pv.IsDefault,
		IsActive:   pv.Status == "active",
		Attributes: pv.Attributes,
		CreatedAt:  pv.CreatedAt,
		UpdatedAt:  pv.UpdatedAt,
	}

	// Obtener stock si hay SKU
	if pv.SKU != "" {
		if stockInfo, err := h.stockClient.GetAvailability(context.Background(), pv.SKU, tenantID); err == nil && stockInfo != nil {
			variant.Stock = &dto.StockInfo{
				Available:    stockInfo.AvailableQuantity,
				Reserved:     stockInfo.ReservedQuantity,
				Total:        stockInfo.TotalQuantity,
				IsLowStock:   stockInfo.IsLowStock,
				IsOutOfStock: stockInfo.IsOutOfStock,
			}
		}
	}

	return variant
}

func (h *ProductVariantHandler) validateCreateVariant(req dto.CreateVariantRequest) error {
	if req.Name == "" {
		return fmt.Errorf("el nombre de la variante es requerido")
	}
	if req.SKU == "" {
		return fmt.Errorf("el SKU es requerido")
	}
	if req.Price < 0 {
		return fmt.Errorf("el precio no puede ser negativo")
	}
	return nil
}
