package admin

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"catalog-bff-service/src/domain/port"
)

// Handler maneja las peticiones del dashboard de administración
type Handler struct {
	service *DashboardService
	logger  port.CatalogBFFEventLogger
}

// NewHandler crea un nuevo handler de administración
func NewHandler(service *DashboardService) *Handler {
	return &Handler{
		service: service,
	}
}

// NewHandlerWithLogger crea un handler inyectando el logger canónico.
func NewHandlerWithLogger(service *DashboardService, logger port.CatalogBFFEventLogger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) log(e port.CatalogBFFEvent) {
	if h.logger != nil {
		h.logger.Log(e)
	}
}

// GetDashboardStats obtiene las estadísticas consolidadas del dashboard
// GET /api/v1/admin/dashboard/stats
func (h *Handler) GetDashboardStats(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()

	stats, err := h.service.GetDashboardStats(ctx)
	if err != nil {
		h.log(port.CatalogBFFEvent{
			Event:  "catalog_bff.dashboard_stats_failed",
			Reason: err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Error al obtener estadísticas del dashboard",
		})
		return
	}

	elapsed := time.Since(start)
	h.log(port.CatalogBFFEvent{
		Event:      "catalog_bff.dashboard_stats_fetched",
		DurationMs: elapsed.Milliseconds(),
		Count:      len(stats.Services),
	})

	c.JSON(http.StatusOK, stats)
}
