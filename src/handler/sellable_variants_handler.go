package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	
	"catalog-bff-service/src/domain"
)

// PIMVariantsListResponse representa la respuesta de listado de PIM
type PIMVariantsListResponse struct {
	Variants   []PIMVariantItem `json:"variants"`  // PIM usa "variants" no "items"
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
	VariantID          string    `json:"variant_id"`
	ProductID          string    `json:"product_id"`
	VariantName        string    `json:"variant_name"`
	SKU                string    `json:"sku"`
	IsDefault          bool      `json:"is_default"`
	AvailableQuantity  float64   `json:"available_quantity"`
	ReservedQuantity   float64   `json:"reserved_quantity"`
	IsSellable         bool      `json:"is_sellable"`
}

// SellableVariantsListResponse es la respuesta del endpoint
type SellableVariantsListResponse struct {
	Items      []SellableVariantResponse `json:"items"`
	TotalCount int                       `json:"total_count"`
}

// GetSellableVariants orquesta PIM + Stock para obtener catálogo vendible
// Aplica Stock Policy por tenant para determinar vendibilidad
func GetSellableVariants(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	authHeader := c.GetHeader("Authorization")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	// Resolver Stock Policy para este tenant
	stockPolicy := domain.GetTenantStockPolicy(tenantID)

	// URLs de servicios
	pimURL := getEnvOrDefault("PIM_SERVICE_URL", "http://localhost:8090")
	stockURL := getEnvOrDefault("STOCK_SERVICE_URL", "http://localhost:8100")

	// PASO 1: Obtener todas las variantes del tenant desde PIM
	pimReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/product-variants?page_size=1000", pimURL), nil)
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

	// PASO 2: Para cada variante, obtener stock en paralelo
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

			// Obtener stock para este SKU usando stockApi
			stockReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/availability?sku=%s", stockURL, *v.SKU), nil)
			if err != nil {
				return
			}
			stockReq.Header.Set("X-Tenant-ID", tenantID)
			if authHeader != "" {
				stockReq.Header.Set("Authorization", authHeader)
			}

			stockResp, err := http.DefaultClient.Do(stockReq)
			if err != nil {
				return
			}
			defer stockResp.Body.Close()

			// Inicializar con stock en 0 (para casos donde no existe registro)
			availableQty := 0.0
			reservedQty := 0.0

			// Procesar respuesta de Stock
			if stockResp.StatusCode == http.StatusOK {
				var stockData StockAvailabilityResponse
				if err := json.NewDecoder(stockResp.Body).Decode(&stockData); err != nil {
					return
				}
				availableQty = stockData.AvailableQuantity
				reservedQty = stockData.ReservedQuantity
			}
			// Si stock devuelve 404, significa que no hay registro (qty = 0)

			// Aplicar Stock Policy para determinar vendibilidad
			isSellable := domain.IsSellable(policy, availableQty)

			// Solo incluir si es vendible según la policy
			if isSellable {
				sellableVariant := SellableVariantResponse{
					VariantID:         v.ID,
					ProductID:         v.ProductID,
					VariantName:       v.Name,
					SKU:               *v.SKU,
					IsDefault:         v.IsDefault,
					AvailableQuantity: availableQty,
					ReservedQuantity:  reservedQty,
					IsSellable:        true,
				}

				mu.Lock()
				sellableVariants = append(sellableVariants, sellableVariant)
				mu.Unlock()
			}
		}(variant, stockPolicy)
	}

	wg.Wait()

	// PASO 3: Responder con catálogo vendible
	response := SellableVariantsListResponse{
		Items:      sellableVariants,
		TotalCount: len(sellableVariants),
	}

	c.JSON(http.StatusOK, response)
}
