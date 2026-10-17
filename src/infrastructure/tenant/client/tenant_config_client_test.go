package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Tests del cliente HTTP contra account-service (ACC-E04 T5). Fijan el contrato
// DD-1 (b) de ADR-005 —X-API-Key + X-Tenant-ID, sin Authorization— y los casos
// que el resolver usa para caer a REQUIRE_STOCK: 404/value null → ("", nil),
// 5xx/401/timeout → error.

const (
	testTenantID = "11111111-1111-1111-1111-111111111111"
	testAPIKey   = "catalog-bff-test-key"
	testKey      = "catalog.stock_policy"
)

func TestHTTPTenantConfigClient_OK_SendsDD1ContractAndReturnsValue(t *testing.T) {
	var gotMethod, gotPath, gotAPIKey, gotTenant, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAPIKey = r.Header.Get("X-API-Key")
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key":"catalog.stock_policy","value":"IGNORE_STOCK"}`))
	}))
	defer srv.Close()

	value, err := NewHTTPTenantConfigClient(srv.URL, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if value != "IGNORE_STOCK" {
		t.Errorf("Expected IGNORE_STOCK, got %q", value)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v1/tenant/config/catalog.stock_policy" {
		t.Errorf("Expected GET /api/v1/tenant/config/catalog.stock_policy, got %s %s", gotMethod, gotPath)
	}
	if gotAPIKey != testAPIKey {
		t.Errorf("Expected X-API-Key %q, got %q", testAPIKey, gotAPIKey)
	}
	if gotTenant != testTenantID {
		t.Errorf("Expected X-Tenant-ID %q, got %q", testTenantID, gotTenant)
	}
	if gotAuth != "" {
		t.Errorf("Expected no Authorization header (DD-1 b es S2S, no forward de JWT), got %q", gotAuth)
	}
}

func TestHTTPTenantConfigClient_EmptyAPIKey_OmitsHeader(t *testing.T) {
	var hasAPIKey bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAPIKey = r.Header["X-Api-Key"]
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := NewHTTPTenantConfigClient(srv.URL, "").GetConfig(context.Background(), testTenantID, testKey)

	if hasAPIKey {
		t.Error("Expected no X-API-Key header when apiKey is empty")
	}
	if err == nil {
		t.Error("Expected error on 401 (sin credencial), got nil")
	}
}

func TestHTTPTenantConfigClient_NotFound_ReturnsEmptyNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"key":"catalog.stock_policy","value":null}`))
	}))
	defer srv.Close()

	value, err := NewHTTPTenantConfigClient(srv.URL, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)

	if err != nil {
		t.Fatalf("Expected no error on 404, got %v", err)
	}
	if value != "" {
		t.Errorf("Expected empty value on 404, got %q", value)
	}
}

func TestHTTPTenantConfigClient_NullValue_ReturnsEmptyNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key":"catalog.stock_policy","value":null}`))
	}))
	defer srv.Close()

	value, err := NewHTTPTenantConfigClient(srv.URL, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)

	if err != nil || value != "" {
		t.Errorf("Expected (\"\", nil) on value null, got (%q, %v)", value, err)
	}
}

func TestHTTPTenantConfigClient_ErrorStatuses_ReturnError(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"500", http.StatusInternalServerError},
		{"503", http.StatusServiceUnavailable},
		{"401 sin credencial válida", http.StatusUnauthorized},
		{"403 sin scope config:read", http.StatusForbidden},
		{"400 tenant inválido", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			value, err := NewHTTPTenantConfigClient(srv.URL, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)

			if err == nil {
				t.Errorf("Expected error on %d, got nil (value %q)", tc.status, value)
			}
		})
	}
}

func TestHTTPTenantConfigClient_Timeout_ReturnsErrorWithin500ms(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	_, err := NewHTTPTenantConfigClient(srv.URL, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	if elapsed > time.Second {
		t.Errorf("Expected the 500ms client timeout to cut the call, took %s", elapsed)
	}
}

func TestHTTPTenantConfigClient_MalformedBody_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	_, err := NewHTTPTenantConfigClient(srv.URL, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)

	if err == nil {
		t.Error("Expected parse error, got nil")
	}
}

func TestHTTPTenantConfigClient_Unreachable_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	_, err := NewHTTPTenantConfigClient(url, testAPIKey).GetConfig(context.Background(), testTenantID, testKey)

	if err == nil {
		t.Error("Expected connection error, got nil")
	}
}
