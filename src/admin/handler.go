package admin

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler maneja las peticiones del dashboard de administración
type Handler struct {
	service *DashboardService
}

// NewHandler crea un nuevo handler de administración
func NewHandler(service *DashboardService) *Handler {
	return &Handler{
		service: service,
	}
}

// GetDashboardStats obtiene las estadísticas consolidadas del dashboard
// GET /api/v1/admin/dashboard/stats
func (h *Handler) GetDashboardStats(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()

	log.Println("📊 Obteniendo estadísticas del dashboard...")

	stats, err := h.service.GetDashboardStats(ctx)
	if err != nil {
		log.Printf("❌ Error obteniendo stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Error al obtener estadísticas del dashboard",
		})
		return
	}

	elapsed := time.Since(start)
	log.Printf("✅ Dashboard stats obtenidos en %v", elapsed)
	log.Printf("   - Curación: %d pending, %d approved today, %d rejected today, %d scraped total",
		stats.Curation.Pending,
		stats.Curation.ApprovedToday,
		stats.Curation.RejectedToday,
		stats.Curation.TotalScraped)
	log.Printf("   - Catálogo: %d productos, %d variantes, %d activos, %d categorías",
		stats.Catalog.TotalProducts,
		stats.Catalog.TotalVariants,
		stats.Catalog.ActiveProducts,
		stats.Catalog.CategoriesCount)
	log.Printf("   - Tenants: %d total, %d activos, %d nuevos este mes",
		stats.Tenants.Total,
		stats.Tenants.Active,
		stats.Tenants.NewThisMonth)
	log.Printf("   - Servicios: %d verificados", len(stats.Services))

	c.JSON(http.StatusOK, stats)
}
