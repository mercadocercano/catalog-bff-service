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

// GlobalCatalogService expone passthrough hacia PIM global-catalog para operaciones de admin.
type GlobalCatalogService struct {
	pimServiceURL string
	httpClient    *http.Client
}

// NewGlobalCatalogService crea una nueva instancia del servicio.
func NewGlobalCatalogService(pimURL string) *GlobalCatalogService {
	return &GlobalCatalogService{
		pimServiceURL: pimURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ListProducts delega el listado filtrado de global_products a PIM.
// Propaga Authorization y X-Operator-Id; retorna el body crudo para no transformar.
func (s *GlobalCatalogService) ListProducts(ctx context.Context, query url.Values, authHeader, operatorID string) ([]byte, error) {
	upstreamURL := fmt.Sprintf("%s/api/v1/global-catalog/products?%s", s.pimServiceURL, query.Encode())
	return s.makeRequest(ctx, http.MethodGet, upstreamURL, nil, authHeader, operatorID)
}

// BulkVerify delega la operación masiva de verificación/desverificación a PIM.
func (s *GlobalCatalogService) BulkVerify(ctx context.Context, body []byte, authHeader, operatorID string) ([]byte, error) {
	upstreamURL := fmt.Sprintf("%s/api/v1/global-catalog/products/bulk-verify", s.pimServiceURL)
	return s.makeRequest(ctx, http.MethodPost, upstreamURL, body, authHeader, operatorID)
}

func (s *GlobalCatalogService) makeRequest(ctx context.Context, method, upstreamURL string, body []byte, authHeader, operatorID string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, upstreamURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
		return nil, fmt.Errorf("upstream request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

