package admin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GlobalCatalogService expone passthrough hacia catalog-service (PLAT-E39 T8b) para las
// operaciones de admin sobre el catálogo global. Antes (MC-E34) pasaba directo a PIM; al
// migrar a catalog-service el BFF deja de tocar pim_db y consume la API canónica de plataforma.
//
// D3.1: propaga el Authorization (JWT) que el caller presentó — catalog-service verifica la
// firma y deriva el rol DENTRO del servicio, así que el control no depende de headers
// forjables. NUNCA reenvía el header de rol crudo del cliente: ése es el agujero de
// marketplace_handler.go:435-437 (rol inyectado por el cliente, forjable mientras PLAT-E37 no
// strippee). No arrastrarlo es la condición de cierre de esta tarea.
type GlobalCatalogService struct {
	catalogServiceURL string
	httpClient         *http.Client
}

// NewGlobalCatalogService crea una nueva instancia del servicio.
func NewGlobalCatalogService(catalogURL string) *GlobalCatalogService {
	return &GlobalCatalogService{
		catalogServiceURL: catalogURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ListProducts delega el listado filtrado de global_products a catalog-service.
// Propaga Authorization y X-Operator-Id; retorna el status code y body crudo del upstream
// para que el handler preserve la semántica del catálogo (401/403/404 llegan como tales).
// err != nil sólo para fallas de transporte (sin respuesta del upstream).
func (s *GlobalCatalogService) ListProducts(ctx context.Context, query url.Values, authHeader, operatorID string) (int, []byte, error) {
	upstreamURL := fmt.Sprintf("%s/api/v1/global-catalog/products?%s", s.catalogServiceURL, query.Encode())
	return s.makeRequest(ctx, http.MethodGet, upstreamURL, nil, authHeader, operatorID)
}

// BulkVerify delega la operación masiva de verificación/desverificación a catalog-service.
func (s *GlobalCatalogService) BulkVerify(ctx context.Context, body []byte, authHeader, operatorID string) (int, []byte, error) {
	upstreamURL := fmt.Sprintf("%s/api/v1/global-catalog/products/bulk-verify", s.catalogServiceURL)
	return s.makeRequest(ctx, http.MethodPost, upstreamURL, body, authHeader, operatorID)
}

// makeRequest arma el request hacia catalog-service. Reenvía el Authorization (JWT) tal cual
// lo presentó el caller — catalog-service lo verifica por sí mismo — y el X-Operator-Id de
// seguimiento. No reenvía el header de rol del cliente (D3.1): ése es exactamente el bypass de
// aislamiento que motivó PLAT-E39; el rol lo deriva catalog-service del claim del JWT verificado,
// no de un header forjable.
//
// Devuelve (statusCode, body, err). err != nil SÓLO para fallas de transporte (no hubo
// respuesta del upstream): ahí el handler responde 502. Cuando el upstream respondió — aun con
// 4xx/5xx — err es nil y el handler preserva el status code original, para que un tenant sin
// rol system_admin vea 403 (no un 502 que enmascara la denegación de autorización).
func (s *GlobalCatalogService) makeRequest(ctx context.Context, method, upstreamURL string, body []byte, authHeader, operatorID string) (int, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, upstreamURL, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if operatorID != "" {
		req.Header.Set("X-Operator-Id", operatorID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read upstream body: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

