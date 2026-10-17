package admin

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"catalog-bff-service/src/domain/port"
)

// GlobalCatalogHandler maneja el passthrough de admin hacia PIM global-catalog.
type GlobalCatalogHandler struct {
	service *GlobalCatalogService
	logger  port.CatalogBFFEventLogger
}

// NewGlobalCatalogHandler crea un nuevo handler.
func NewGlobalCatalogHandler(service *GlobalCatalogService) *GlobalCatalogHandler {
	return &GlobalCatalogHandler{service: service}
}

// NewGlobalCatalogHandlerWithLogger crea un handler inyectando el logger canónico.
func NewGlobalCatalogHandlerWithLogger(service *GlobalCatalogService, logger port.CatalogBFFEventLogger) *GlobalCatalogHandler {
	return &GlobalCatalogHandler{service: service, logger: logger}
}

func (h *GlobalCatalogHandler) log(e port.CatalogBFFEvent) {
	if h.logger != nil {
		h.logger.Log(e)
	}
}

// ListProducts GET /api/v1/admin/global-catalog/products
func (h *GlobalCatalogHandler) ListProducts(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()

	authHeader := c.GetHeader("Authorization")
	operatorID := c.GetHeader("X-Operator-Id")

	statusCode, body, err := h.service.ListProducts(ctx, c.Request.URL.Query(), authHeader, operatorID)
	elapsed := time.Since(start)

	if err != nil {
		// Falla de transporte: no hubo respuesta del upstream → 502.
		h.log(port.CatalogBFFEvent{
			Event:  "catalog_bff.admin_global_catalog_list_failed",
			Reason: err.Error(),
		})
		c.Data(http.StatusBadGateway, "application/json", body)
		return
	}

	if statusCode >= 400 {
		// El upstream respondió con un error (401/403/404/…). Preservar el status code:
		// un tenant sin rol system_admin debe ver 403, no un 502 que enmascara la denegación
		// de autorización que catalog-service ya decidió (D3.1 — el BFF es passthrough).
		h.log(port.CatalogBFFEvent{
			Event:  "catalog_bff.admin_global_catalog_list_upstream_error",
			Reason: fmt.Sprintf("upstream status %d", statusCode),
		})
		c.Data(statusCode, "application/json", body)
		return
	}

	h.log(port.CatalogBFFEvent{
		Event:      "catalog_bff.admin_global_catalog_list_fetched",
		DurationMs: elapsed.Milliseconds(),
	})

	c.Data(http.StatusOK, "application/json", body)
}

// BulkVerify POST /api/v1/admin/global-catalog/products/bulk-verify
func (h *GlobalCatalogHandler) BulkVerify(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()

	authHeader := c.GetHeader("Authorization")
	operatorID := c.GetHeader("X-Operator-Id")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_body",
			"message": "Error al leer el body",
		})
		return
	}

	statusCode, respBody, err := h.service.BulkVerify(ctx, body, authHeader, operatorID)
	elapsed := time.Since(start)

	if err != nil {
		h.log(port.CatalogBFFEvent{
			Event:  "catalog_bff.admin_global_catalog_bulk_verify_failed",
			Reason: err.Error(),
		})
		c.Data(http.StatusBadGateway, "application/json", respBody)
		return
	}

	if statusCode >= 400 {
		h.log(port.CatalogBFFEvent{
			Event:  "catalog_bff.admin_global_catalog_bulk_verify_upstream_error",
			Reason: fmt.Sprintf("upstream status %d", statusCode),
		})
		c.Data(statusCode, "application/json", respBody)
		return
	}

	h.log(port.CatalogBFFEvent{
		Event:      "catalog_bff.admin_global_catalog_bulk_verify_completed",
		DurationMs: elapsed.Milliseconds(),
	})

	c.Data(http.StatusOK, "application/json", respBody)
}
