package tenant_dashboard

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetDashboard GET /api/v1/tenant/dashboard
func (h *Handler) GetDashboard(c *gin.Context) {
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_tenant",
			"message": "X-Tenant-ID header es requerido",
		})
		return
	}

	authHeader := c.GetHeader("Authorization")
	start := time.Now()

	log.Printf("📊 Tenant dashboard: obteniendo stats para tenant %s", tenantID)

	data, err := h.service.GetDashboard(c.Request.Context(), tenantID, authHeader)
	if err != nil {
		log.Printf("❌ Error obteniendo tenant dashboard: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Error al obtener datos del dashboard",
		})
		return
	}

	elapsed := time.Since(start)
	log.Printf("✅ Tenant dashboard obtenido en %v (products=%d, variants=%d, brands=%d, categories=%d, skus=%d)",
		elapsed,
		data.Catalog.TotalProducts,
		data.Catalog.TotalVariants,
		data.Catalog.BrandsCount,
		data.Catalog.CategoriesCount,
		data.Inventory.Totals.TotalSKUs,
	)

	c.JSON(http.StatusOK, data)
}
