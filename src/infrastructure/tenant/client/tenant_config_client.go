package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TenantConfigClient define el contrato para obtener configuraciones de tenant
type TenantConfigClient interface {
	GetConfig(ctx context.Context, tenantID string, key string) (string, error)
}

// HTTPTenantConfigClient implementa el client HTTP S2S contra account-service
// (módulo tenancy, ACC-E04 T5). Contrato DD-1 (b) de ADR-005: X-API-Key con
// scope config:read + X-Tenant-ID; el tenant se fija en RLS tras el gate.
type HTTPTenantConfigClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// TenantConfigResponse representa la respuesta de account-service
type TenantConfigResponse struct {
	Key   string  `json:"key"`
	Value *string `json:"value"` // Nullable para indicar que no existe
}

// NewHTTPTenantConfigClient crea una nueva instancia del client con autenticación S2S.
// apiKey es la credencial propia del BFF ante account-service (S2S_KEY_CATALOG_BFF
// del lado servidor); vacía → el request sale sin X-API-Key y account-service
// responde 401, que el resolver trata como error → fallback REQUIRE_STOCK.
func NewHTTPTenantConfigClient(baseURL string, apiKey string) TenantConfigClient {
	return &HTTPTenantConfigClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 500 * time.Millisecond, // Timeout agresivo para no bloquear el BFF
		},
		apiKey: apiKey,
	}
}

// GetConfig obtiene una configuración de account-service
// Retorna error si:
// - No puede conectar al servicio
// - Timeout
// - Error 5xx
//
// Retorna ("", nil) si la configuración no existe (404)
func (c *HTTPTenantConfigClient) GetConfig(ctx context.Context, tenantID string, key string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/tenant/config/%s", c.baseURL, key)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Tenant-ID", tenantID)
	// Autenticación S2S via API Key
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call account-service: %w", err)
	}
	defer resp.Body.Close()

	// Si es 404, la configuración no existe (no es error)
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}

	// Si es error del servidor, propagar error
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("account-service error: status %d", resp.StatusCode)
	}

	// Si no es 200, error inesperado
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from account-service: %d", resp.StatusCode)
	}

	// Parsear respuesta
	var configResp TenantConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return "", fmt.Errorf("failed to parse account-service response: %w", err)
	}

	// Si value es null, la configuración no existe
	if configResp.Value == nil {
		return "", nil
	}

	return *configResp.Value, nil
}
