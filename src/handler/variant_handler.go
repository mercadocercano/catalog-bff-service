package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// PIMVariantResponse representa la respuesta de PIM
type PIMVariantResponse struct {
	ID         string                 `json:"id"`
	ProductID  string                 `json:"product_id"`
	Name       string                 `json:"name"`
	SKU        *string                `json:"sku"`
	Status     string                 `json:"status"`
	IsDefault  bool                   `json:"is_default"`
	SortOrder  int                    `json:"sort_order"`
	Attributes []VariantAttribute     `json:"attributes"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

type VariantAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// StockAvailabilityResponse representa la respuesta de Stock
type StockAvailabilityResponse struct {
	ProductSKU        string    `json:"product_sku"`
	ProductName       string    `json:"product_name"`
	AvailableQuantity float64   `json:"available_quantity"`
	ReservedQuantity  float64   `json:"reserved_quantity"`
	TotalQuantity     float64   `json:"total_quantity"`
	UnitOfMeasure     string    `json:"unit_of_measure"`
	AvgUnitCost       float64   `json:"avg_unit_cost"`
	TotalValue        float64   `json:"total_value"`
	IsLowStock        bool      `json:"is_low_stock"`
	IsOutOfStock      bool      `json:"is_out_of_stock"`
	LastEntryAt       time.Time `json:"last_entry_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CatalogVariantResponse es la respuesta agregada
type CatalogVariantResponse struct {
	VariantID    string       `json:"variant_id"`
	ProductID    string       `json:"product_id"`
	ProductName  string       `json:"product_name"`
	VariantName  string       `json:"variant_name"`
	SKU          string       `json:"sku"`
	IsDefault    bool         `json:"is_default"`
	Stock        StockInfo    `json:"stock"`
}

type StockInfo struct {
	Available float64 `json:"available"`
	Reserved  float64 `json:"reserved"`
	Total     float64 `json:"total"`
}

// GetVariantWithStock orquesta PIM + Stock
func GetVariantWithStock(c *gin.Context) {
	variantID := c.Param("id")
	tenantID := c.GetHeader("X-Tenant-ID")
	authHeader := c.GetHeader("Authorization")

	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Tenant-ID header is required"})
		return
	}

	// URLs de servicios (desde env o default localhost)
	pimURL := getEnvOrDefault("PIM_SERVICE_URL", "http://localhost:8090")
	stockURL := getEnvOrDefault("STOCK_SERVICE_URL", "http://localhost:8100")

	// 1. Llamar a PIM para obtener variante
	pimReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/product-variants/%s", pimURL, variantID), nil)
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

	var variant PIMVariantResponse
	if err := json.NewDecoder(pimResp.Body).Decode(&variant); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to parse PIM response"})
		return
	}

	// Validar que tenga SKU
	if variant.SKU == nil || *variant.SKU == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Variant has no SKU"})
		return
	}

	// 2. Llamar a Stock para obtener disponibilidad
	stockReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/availability?sku=%s", stockURL, *variant.SKU), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to create Stock request"})
		return
	}
	stockReq.Header.Set("X-Tenant-ID", tenantID)
	if authHeader != "" {
		stockReq.Header.Set("Authorization", authHeader)
	}

	stockResp, err := http.DefaultClient.Do(stockReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Stock service unavailable", "details": err.Error()})
		return
	}
	defer stockResp.Body.Close()

	// Stock puede devolver 404 si no existe (stock = 0)
	var stockInfo StockInfo
	if stockResp.StatusCode == http.StatusOK {
		var stockData StockAvailabilityResponse
		if err := json.NewDecoder(stockResp.Body).Decode(&stockData); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to parse Stock response"})
			return
		}
		stockInfo = StockInfo{
			Available: stockData.AvailableQuantity,
			Reserved:  stockData.ReservedQuantity,
			Total:     stockData.TotalQuantity,
		}
	} else if stockResp.StatusCode == http.StatusNotFound {
		// No hay stock registrado, devolver 0s
		stockInfo = StockInfo{
			Available: 0,
			Reserved:  0,
			Total:     0,
		}
	} else {
		body, _ := io.ReadAll(stockResp.Body)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Stock service error", "status": stockResp.StatusCode, "details": string(body)})
		return
	}

	// 3. Merge y respuesta
	response := CatalogVariantResponse{
		VariantID:   variant.ID,
		ProductID:   variant.ProductID,
		ProductName: "", // PIM no devuelve product_name en variant, dejar vacío o hacer otra llamada
		VariantName: variant.Name,
		SKU:         *variant.SKU,
		IsDefault:   variant.IsDefault,
		Stock:       stockInfo,
	}

	c.JSON(http.StatusOK, response)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := getEnv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnv(key string) string {
	// Simple env lookup (en producción usar os.Getenv)
	return ""
}
