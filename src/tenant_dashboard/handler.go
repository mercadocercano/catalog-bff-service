package tenant_dashboard

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"catalog-bff-service/src/domain/port"
)

type Handler struct {
	service *Service
	logger  port.CatalogBFFEventLogger
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithLogger crea un handler inyectando el logger canónico.
func NewHandlerWithLogger(service *Service, logger port.CatalogBFFEventLogger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) log(e port.CatalogBFFEvent) {
	if h.logger != nil {
		h.logger.Log(e)
	}
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

	data, err := h.service.GetDashboard(c.Request.Context(), tenantID, authHeader)
	if err != nil {
		h.log(port.CatalogBFFEvent{
			Event:    "catalog_bff.tenant_dashboard_failed",
			TenantID: tenantID,
			Reason:   err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Error al obtener datos del dashboard",
		})
		return
	}

	elapsed := time.Since(start)

	// Si el inventory tiene SKUs = 0 y hay productos en catálogo, es una degradación parcial
	if data.Inventory.Totals.TotalSKUs == 0 && data.Catalog.TotalProducts > 0 {
		h.log(port.CatalogBFFEvent{
			Event:           "catalog_bff.tenant_dashboard_partial",
			TenantID:        tenantID,
			UpstreamService: "stock",
			DurationMs:      elapsed.Milliseconds(),
		})
	} else {
		h.log(port.CatalogBFFEvent{
			Event:      "catalog_bff.tenant_dashboard_fetched",
			TenantID:   tenantID,
			DurationMs: elapsed.Milliseconds(),
			Count:      data.Catalog.TotalProducts,
		})
	}

	c.JSON(http.StatusOK, data)
}
