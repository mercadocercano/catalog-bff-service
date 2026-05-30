package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	
	"catalog-bff-service/src/domain"
	"catalog-bff-service/src/infrastructure/stock/client"
)

// PIMVariantsListResponse representa la respuesta de listado de PIM
type PIMVariantsListResponse struct {
	Variants   []PIMVariantItem `json:"variants"`
	Pagination PIMPagination    `json:"pagination"`
}

type PIMPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type PIMVariantItem struct {
	ID        string  `json:"id"`
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	SKU       *string `json:"sku"`
	Status    string  `json:"status"`
	IsDefault bool    `json:"is_default"`
}

// SellableVariantResponse es el DTO de respuesta para variantes vendibles
type SellableVariantResponse struct {
	VariantID         string  `json:"variant_id"`
	ProductID         string  `json:"product_id"`
	VariantName       string  `json:"variant_name"`
	SKU               string  `json:"sku"`
	IsDefault         bool    `json:"is_default"`
	AvailableQuantity float64 `json:"available_quantity"`
	ReservedQuantity  float64 `json:"reserved_quantity"`
	IsSellable        bool    `json:"is_sellable"`
}

// SellableVariantsListResponse es la respuesta del endpoint
type SellableVariantsListResponse struct {
	Items      []SellableVariantResponse `json:"items"`
	TotalCount int                       `json:"total_count"`
}

// SellableVariantsHandler maneja el endpoint de variantes vendibles
// con inyección de dependencias para el resolver de Stock Policy
type SellableVariantsHandler struct {
	policyResolver *domain.TenantStockPolicyResolver
	pimServiceURL  string
	stockClient    client.StockAvailabilityClient
}

// NewSellableVariantsHandler crea una nueva instancia del handler
func NewSellableVariantsHandler(
	policyResolver *domain.TenantStockPolicyResolver,
	pimServiceURL string,
	stockClient client.StockAvailabilityClient,
) *SellableVariantsHandler {
	return &SellableVariantsHandler{
		policyResolver: policyResolver,
		pimServiceURL:  pimServiceURL,
		stockClient:    stockClient,
	}
}

// Handle procesa la petición de variantes vendibles
func (h *SellableVariantsHandler) Handle(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	authHeader := c.GetHeader("Authorization")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	// PASO 0: Resolver Stock Policy para este tenant
	ctx := c.Request.Context()
	stockPolicy := h.resolveStockPolicy(ctx, tenantID)

	// PASO 1: Obtener todas las variantes del tenant desde PIM
	pimReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/product-variants?page_size=1000", h.pimServiceURL), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to create PIM request"})
		return
	}
	pimReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader != "" {
		pimReq.Header.Set("Authorization", authHeader)
	}

	pimResp, err := http.DefaultClient.Do(pimReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "PIM service unavailable", "details": err.Error()})
		return
	}
	defer pimResp.Body.Close()

	if pimResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pimResp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": "PIM service error", "status": pimResp.StatusCode, "details": string(body)})
		return
	}

	var pimData PIMVariantsListResponse
	if err := json.NewDecoder(pimResp.Body).Decode(&pimData); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to parse PIM response"})
		return
	}

	// PASO 2: Para cada variante, obtener stock y aplicar policy
	sellableVariants := make([]SellableVariantResponse, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, variant := range pimData.Variants {
		// Validar que tenga SKU
		if variant.SKU == nil || *variant.SKU == "" {
			continue
		}

		wg.Add(1)
		go func(v PIMVariantItem, policy domain.StockPolicy) {
			defer wg.Done()

			// Obtener stock para este SKU usando el cliente con cache
			availability, err := h.stockClient.GetAvailability(ctx, tenantID, *v.SKU)
			
			// Inicializar con stock en 0
			availableQty := 0.0
			reservedQty := 0.0

			// Si hay disponibilidad, usar los valores
			if err == nil && availability != nil {
				availableQty = availability.AvailableQuantity
				reservedQty = availability.ReservedQuantity
			}
			// Si hay error o nil, quedamos con 0s (comportamiento seguro)

			// Aplicar Stock Policy para determinar vendibilidad
			isSellable := domain.IsSellable(policy, availableQty)

			// CAMBIO IMPORTANTE: Devolver TODAS las variantes, no solo las vendibles
			// El frontend decide qué mostrar basándose en is_sellable
			sellableVariant := SellableVariantResponse{
				VariantID:         v.ID,
				ProductID:         v.ProductID,
				VariantName:       v.Name,
				SKU:               *v.SKU,
				IsDefault:         v.IsDefault,
				AvailableQuantity: availableQty,
				ReservedQuantity:  reservedQty,
				IsSellable:        isSellable,
			}

			mu.Lock()
			sellableVariants = append(sellableVariants, sellableVariant)
			mu.Unlock()
		}(variant, stockPolicy)
	}

	wg.Wait()

	// PASO 3: Responder con catálogo completo
	response := SellableVariantsListResponse{
		Items:      sellableVariants,
		TotalCount: len(sellableVariants),
	}

	c.JSON(http.StatusOK, response)
}

// resolveStockPolicy resuelve la policy del tenant usando el resolver
func (h *SellableVariantsHandler) resolveStockPolicy(ctx context.Context, tenantID string) domain.StockPolicy {
	if h.policyResolver == nil {
		// Fallback si no hay resolver configurado
		return domain.RequireStock
	}
	return h.policyResolver.Resolve(ctx, tenantID)
}
