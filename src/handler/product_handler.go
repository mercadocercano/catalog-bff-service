package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	
	"catalog-bff-service/src/dto"
)

// ProductHandler maneja las operaciones de productos para backoffice
type ProductHandler struct {
	pimServiceURL string
	httpClient    *http.Client
}

// NewProductHandler crea un nuevo handler de productos
func NewProductHandler(pimServiceURL string) *ProductHandler {
	return &ProductHandler{
		pimServiceURL: pimServiceURL,
		httpClient:    &http.Client{},
	}
}

// ============================================================================
// Listado de Productos
// ============================================================================

// ListProducts lista productos con filtros y paginación
// GET /api/v1/backoffice/products
func (h *ProductHandler) ListProducts(c *gin.Context) {
	var req dto.ProductListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: "Parámetros de consulta inválidos",
			Details: map[string]string{"validation": err.Error()},
		})
		return
	}

	// Defaults
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Construir query params para PIM
	queryParams := h.buildPIMQueryParams(req)

	// Llamar a PIM Service
	pimURL := fmt.Sprintf("%s/api/v1/products?%s", h.pimServiceURL, queryParams)
	pimReq, err := http.NewRequest("GET", pimURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Error al construir request a PIM",
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error en PIM Service: %s", string(body)),
		})
		return
	}

	// Parsear response de PIM
	var pimResp dto.PIMProductListResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta de PIM",
		})
		return
	}

	// Mapear a DTOs de backoffice
	response := h.mapToProductListResponse(pimResp)

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// Detalle de Producto
// ============================================================================

// GetProduct obtiene el detalle de un producto con sus variantes
// GET /api/v1/backoffice/products/:id
func (h *ProductHandler) GetProduct(c *gin.Context) {
	productID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "missing_tenant",
			Message: "Header X-Tenant-ID es requerido",
		})
		return
	}

	// Llamar a PIM para obtener producto
	pimURL := fmt.Sprintf("%s/api/v1/products/%s", h.pimServiceURL, productID)
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

	var pimProduct dto.PIMProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimProduct); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	// Obtener variantes del producto
	variants, err := h.getProductVariants(productID, tenantID, c.GetHeader("Authorization"))
	if err != nil {
		// Log error pero continuar sin variantes
		log.Printf("⚠️ Error obteniendo variantes para producto %s: %v", productID, err)
		variants = []dto.VariantSummary{}
	}

	// Mapear a DTO de backoffice
	response := dto.ProductDetailResponse{
		ID:           pimProduct.ID,
		Name:         pimProduct.Name,
		Description:  pimProduct.Description,
		CategoryID:   pimProduct.CategoryID,
		BrandID:      pimProduct.BrandID,
		Status:       pimProduct.Status,
		Variants:     variants,
		CreatedAt:    pimProduct.CreatedAt,
		UpdatedAt:    pimProduct.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// ============================================================================
// Crear Producto
// ============================================================================

// CreateProduct crea un nuevo producto
// POST /api/v1/backoffice/products
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req dto.CreateProductRequest
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
	if err := h.validateCreateProduct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	// Mapear a request de PIM
	pimReq := h.mapToPIMCreateRequest(req)

	log.Printf("Creating product with %d variants", len(req.Variants))

	// Llamar a PIM Service
	pimURL := fmt.Sprintf("%s/api/v1/products", h.pimServiceURL)
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

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, dto.ErrorResponse{
			Error:   "pim_error",
			Message: fmt.Sprintf("Error al crear producto: %s", string(body)),
		})
		return
	}

	var pimResp dto.PIMProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	// Si se enviaron variantes, crearlas
	if len(req.Variants) > 0 {
		log.Printf("🔧 Creando %d variantes para producto %s", len(req.Variants), pimResp.ID)
		for i, variant := range req.Variants {
			// Construir request para PIM (endpoint /product-variants)
			// NO enviar is_default - el PIM decide
			variantReq := map[string]interface{}{
				"product_id": pimResp.ID,
				"name":       variant.Name,
				"sku":        variant.SKU,
			}
			
			// Agregar atributos si existen
			if len(variant.Attributes) > 0 {
				variantReq["attributes"] = variant.Attributes
			}
			
			variantURL := fmt.Sprintf("%s/api/v1/product-variants", h.pimServiceURL)
			variantBody, _ := json.Marshal(variantReq)
			
			log.Printf("🔧 Variante %d: %s", i+1, string(variantBody))
			
			httpReq, _ := http.NewRequest("POST", variantURL, bytes.NewBuffer(variantBody))
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("X-Tenant-ID", tenantID)
			if authHeader := c.GetHeader("Authorization"); authHeader != "" {
				httpReq.Header.Set("Authorization", authHeader)
			}
			
			variantResp, err := h.httpClient.Do(httpReq)
			if err != nil {
				log.Printf("❌ Error creando variante %d: %v", i+1, err)
				continue
			}
			
			if variantResp.StatusCode != http.StatusCreated {
				respBody, _ := io.ReadAll(variantResp.Body)
				log.Printf("❌ Error en PIM al crear variante %d: %s", i+1, string(respBody))
			} else {
				log.Printf("✅ Variante %d creada exitosamente", i+1)
			}
			
			variantResp.Body.Close()
		}
	} else {
		log.Printf("⚠️ No se enviaron variantes en el request")
	}

	response := dto.ProductCreatedResponse{
		ID:        pimResp.ID,
		Name:      pimResp.Name,
		Status:    pimResp.Status,
		CreatedAt: pimResp.CreatedAt,
	}

	c.JSON(http.StatusCreated, response)
}

// ============================================================================
// Actualizar Producto
// ============================================================================

// UpdateProduct actualiza un producto existente
// PUT /api/v1/backoffice/products/:id
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	productID := c.Param("id")

	var req dto.UpdateProductRequest
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

	// Llamar a PIM Service
	pimURL := fmt.Sprintf("%s/api/v1/products/%s", h.pimServiceURL, productID)
	body, _ := json.Marshal(req)

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
			Error:   "product_not_found",
			Message: "Producto no encontrado",
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

	var pimResp dto.PIMProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "parse_error",
			Message: "Error al parsear respuesta",
		})
		return
	}

	// Si se enviaron variantes, actualizarlas (A2)
	if len(req.Variants) > 0 {
		log.Printf("🔧 Actualizando %d variantes", len(req.Variants))
		for i, variant := range req.Variants {
			if variant.ID == "" {
				log.Printf("⚠️ Variante %d sin ID, saltando", i+1)
				continue
			}

			variantURL := fmt.Sprintf("%s/api/v1/product-variants/%s?product_id=%s", h.pimServiceURL, variant.ID, productID)
			
			// Construir payload con todos los campos necesarios
			variantReq := map[string]interface{}{
				"name":  variant.Name,
				"price": variant.Price,
				"stock": variant.Stock,
			}
			
			// SKU es opcional pero debe cumplir min si se envía
			if variant.SKU != "" {
				variantReq["sku"] = variant.SKU
			}
			
			// Atributos si existen
			if len(variant.Attributes) > 0 {
				variantReq["attributes"] = variant.Attributes
			}
			
			variantBody, _ := json.Marshal(variantReq)

			log.Printf("🔧 Actualizando variante %d (%s): price=%v", i+1, variant.ID, variant.Price)

			httpReq, _ := http.NewRequest("PUT", variantURL, bytes.NewBuffer(variantBody))
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("X-Tenant-ID", tenantID)
			if authHeader := c.GetHeader("Authorization"); authHeader != "" {
				httpReq.Header.Set("Authorization", authHeader)
			}

			variantResp, err := h.httpClient.Do(httpReq)
			if err != nil {
				log.Printf("❌ Error actualizando variante %d: %v", i+1, err)
				continue
			}

			if variantResp.StatusCode != http.StatusOK {
				respBody, _ := io.ReadAll(variantResp.Body)
				log.Printf("❌ PIM error al actualizar variante %d: %s", i+1, string(respBody))
			} else {
				log.Printf("✅ Variante %d actualizada exitosamente", i+1)
			}

			variantResp.Body.Close()
		}
	}

	c.JSON(http.StatusOK, pimResp)
}

// ============================================================================
// Helpers
// ============================================================================

func (h *ProductHandler) buildPIMQueryParams(req dto.ProductListRequest) string {
	params := []string{
		fmt.Sprintf("page=%d", req.Page),
		fmt.Sprintf("page_size=%d", req.PageSize),
	}

	if req.Search != "" {
		params = append(params, fmt.Sprintf("search=%s", req.Search))
	}
	if req.CategoryID != "" {
		params = append(params, fmt.Sprintf("category_id=%s", req.CategoryID))
	}
	if req.BrandID != "" {
		params = append(params, fmt.Sprintf("brand_id=%s", req.BrandID))
	}
	if req.Status != "" {
		params = append(params, fmt.Sprintf("status=%s", req.Status))
	}
	if req.SortBy != "" {
		params = append(params, fmt.Sprintf("sort_by=%s", req.SortBy))
	}
	if req.SortDir != "" {
		params = append(params, fmt.Sprintf("sort_dir=%s", req.SortDir))
	}

	return strings.Join(params, "&")
}

func (h *ProductHandler) mapToProductListResponse(pimResp dto.PIMProductListResponse) dto.ProductListResponse {
	items := make([]dto.ProductSummary, len(pimResp.Products))
	for i, item := range pimResp.Products {
		items[i] = dto.ProductSummary{
			ID:            item.ID,
			Name:          item.Name,
			Status:        item.Status,
			VariantsCount: 0, // TODO: obtener de PIM si está disponible
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		}
	}

	return dto.ProductListResponse{
		Items:      items,
		TotalCount: pimResp.Pagination.TotalItems,
		Page:       pimResp.Pagination.Page,
		PageSize:   pimResp.Pagination.PageSize,
		TotalPages: pimResp.Pagination.TotalPages,
	}
}

func (h *ProductHandler) getProductVariants(productID, tenantID, authHeader string) ([]dto.VariantSummary, error) {
	// Usar el endpoint anidado /products/:id/variants
	pimURL := fmt.Sprintf("%s/api/v1/products/%s/variants", h.pimServiceURL, productID)
	req, err := http.NewRequest("GET", pimURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Printf("❌ Error llamando a PIM variants: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("❌ PIM devolvió %d para variantes: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("error getting variants: %d", resp.StatusCode)
	}

	// El PIM devuelve {variants: [...], pagination: {...}}
	var pimResp struct {
		Variants   []dto.PIMVariantResponse `json:"variants"`
		Pagination map[string]interface{}   `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pimResp); err != nil {
		log.Printf("❌ Error parseando variantes: %v", err)
		return nil, err
	}
	
	variants := pimResp.Variants
	log.Printf("✅ Cargadas %d variantes para producto %s", len(variants), productID)

	// Mapear a VariantSummary
	result := make([]dto.VariantSummary, len(variants))
	for i, v := range variants {
		result[i] = dto.VariantSummary{
			ID:         v.ID,
			Name:       v.Name,
			SKU:        v.SKU,
			Price:      v.Price,
			IsDefault:  v.IsDefault,
			IsActive:   v.Status == "active",
			Attributes: v.Attributes,
			Stock: &dto.StockInfo{
				Available:    float64(v.Stock),
				Reserved:     0,
				Total:        float64(v.Stock),
				IsLowStock:   v.Stock < 10,
				IsOutOfStock: v.Stock == 0,
			},
			CreatedAt:  v.CreatedAt,
			UpdatedAt:  v.UpdatedAt,
		}
	}

	return result, nil
}

func (h *ProductHandler) validateCreateProduct(req dto.CreateProductRequest) error {
	// Validaciones adicionales de negocio
	if req.Name == "" {
		return fmt.Errorf("el nombre del producto es requerido")
	}

	// Validar que si hay variantes, al menos una sea default
	if len(req.Variants) > 0 {
		hasDefault := false
		for _, v := range req.Variants {
			if v.IsDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			return fmt.Errorf("debe haber al menos una variante marcada como default")
		}
	}

	return nil
}

func (h *ProductHandler) mapToPIMCreateRequest(req dto.CreateProductRequest) map[string]interface{} {
	pimReq := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"status":      req.Status,
	}

	if req.CategoryID != "" {
		pimReq["category_id"] = req.CategoryID
	}
	if req.BrandID != "" {
		pimReq["brand_id"] = req.BrandID
	}
	if len(req.Variants) > 0 {
		pimReq["variants"] = req.Variants
	}

	return pimReq
}
