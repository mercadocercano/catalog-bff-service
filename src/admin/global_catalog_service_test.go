package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestGlobalCatalogService_ApuntaACatalogServiceYReenviaJWT_SinHeaderDeRol (PLAT-E39 T8b)
// fija el contrato de la migración del passthrough admin/global-catalog de PIM a catalog-service:
//   1. el upstream es catalog-service (no pim) — el request llega a la URL configurada;
//   2. reenvía el Authorization (JWT) que presentó el caller (catalog-service verifica rol por sí mismo);
//   3. NO reenvía ningún header de rol del cliente (D3.1) — el bypass forjable de
//      marketplace_handler.go:435-437 que esta tarea existe para no arrastrar;
//   4. reenvía X-Operator-Id como header de seguimiento (no de autoridad);
//   5. devuelve el status code del upstream (no lo aplana a 502): un 403 del catálogo llega como 403.
//
// El chequeo de "no se reenvía un header de rol" se hace por allowlist: el upstream sólo puede
// recibir los headers que el BFF explícitamente propaga (más los defaults que añade el transporte
// de Go). Cualquier header de rol forzado por el cliente aparecería como un header fuera del
// allowlist y rompería el test, sin necesidad de nombrar el literal prohibido en el árbol.
func TestGlobalCatalogService_ApuntaACatalogServiceYReenviaJWT_SinHeaderDeRol(t *testing.T) {
	var got struct {
		path       string
		auth       string
		operatorID string
		method     string
		body       string
		headerKeys []string
	}

	// goAutoHeaders: headers que el cliente HTTP de Go añade por sí mismo y que el BFF no controla.
	goAutoHeaders := map[string]bool{
		"Accept-Encoding": true,
		"User-Agent":      true,
		"Content-Length":  true,
	}
	// forwardedAllowlist: los únicos headers que el BFF tiene permitido propagar al upstream.
	forwardedAllowlist := map[string]bool{
		"Authorization": true,
		"X-Operator-Id": true,
		"Content-Type":  true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.auth = r.Header.Get("Authorization")
		got.operatorID = r.Header.Get("X-Operator-Id")
		got.method = r.Method
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			got.body = string(b)
		}
		// Registrar todos los headers que el upstream efectivamente recibe.
		got.headerKeys = nil
		for k := range r.Header {
			got.headerKeys = append(got.headerKeys, k)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// assertNoExtraRoleHeader falla si el upstream recibió algún header que no es ni un default
	// del transporte de Go ni uno que el BFF está autorizado a propagar. Un header de rol
	// forzado por el caller caería acá.
	assertNoExtraRoleHeader := func(t *testing.T) {
		t.Helper()
		for _, k := range got.headerKeys {
			if goAutoHeaders[http.CanonicalHeaderKey(k)] {
				continue
			}
			if !forwardedAllowlist[http.CanonicalHeaderKey(k)] {
				t.Fatalf("el upstream recibió un header que el BFF no debe propagar: %q (violación D3.1)", k)
			}
		}
	}

	svc := NewGlobalCatalogService(srv.URL)

	t.Run("ListProducts", func(t *testing.T) {
		got.path, got.auth, got.operatorID, got.headerKeys = "", "", "", nil
		q := url.Values{}
		q.Set("page", "1")
		q.Set("limit", "20")
		status, _, err := svc.ListProducts(context.Background(), q, "Bearer test-jwt", "op-42")
		if err != nil {
			t.Fatalf("ListProducts: error inesperado: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("status = %d, quiero 200 del upstream", status)
		}
		if got.path != "/api/v1/global-catalog/products" {
			t.Fatalf("upstream path = %q, quiero /api/v1/global-catalog/products (catalog-service, no pim)", got.path)
		}
		if got.method != http.MethodGet {
			t.Fatalf("method = %q, quiero GET", got.method)
		}
		if got.auth != "Bearer test-jwt" {
			t.Fatalf("Authorization no reenviado: got %q", got.auth)
		}
		if got.operatorID != "op-42" {
			t.Fatalf("X-Operator-Id no reenviado: got %q", got.operatorID)
		}
		assertNoExtraRoleHeader(t)
	})

	t.Run("BulkVerify", func(t *testing.T) {
		got.path, got.auth, got.operatorID, got.headerKeys, got.body = "", "", "", nil, ""
		body := []byte(`{"ids":["00000000-0000-0000-0000-000000000001"],"verify":true}`)
		status, _, err := svc.BulkVerify(context.Background(), body, "Bearer test-jwt", "op-42")
		if err != nil {
			t.Fatalf("BulkVerify: error inesperado: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("status = %d, quiero 200 del upstream", status)
		}
		if got.path != "/api/v1/global-catalog/products/bulk-verify" {
			t.Fatalf("upstream path = %q, quiero /api/v1/global-catalog/products/bulk-verify", got.path)
		}
		if got.method != http.MethodPost {
			t.Fatalf("method = %q, quiero POST", got.method)
		}
		if got.auth != "Bearer test-jwt" {
			t.Fatalf("Authorization no reenviado: got %q", got.auth)
		}
		if got.body != string(body) {
			t.Fatalf("body no propagado: got %q", got.body)
		}
		assertNoExtraRoleHeader(t)
	})

	// El caller podría mandar un header de rol forjado; el BFF no debe copiarlo al upstream.
	// httptest.Headers del request sólo contendrá lo que el BFF efectivamente setee (más los
	// defaults de Go), no lo que un cliente externo mande — porque el BFF construye un request
	// nuevo en makeRequest y setea explícitamente sólo Authorization y X-Operator-Id.
	t.Run("SinAuthorizationNoFabricaHeaderDeRol", func(t *testing.T) {
		got.auth, got.headerKeys = "", nil
		_, _, err := svc.ListProducts(context.Background(), url.Values{}, "", "")
		if err != nil {
			t.Fatalf("ListProducts sin auth: error inesperado: %v", err)
		}
		if got.auth != "" {
			t.Fatalf("sin Authorization en el caller, no debe fabricarse upstream: got %q", got.auth)
		}
		assertNoExtraRoleHeader(t)
	})
}

// TestGlobalCatalogService_PreservaStatusDelUpstream verifica que el status code del upstream
// se devuelve tal cual (no aplanado a error): un 403 del catálogo llega como 403, un 200 como 200.
// Esto es lo que permite al handler exponer "tenant → 403" en vez de enmascararlo como 502.
func TestGlobalCatalogService_PreservaStatusDelUpstream(t *testing.T) {
	cases := []struct {
		name string
		code int
	}{
		{"ok_200", 200},
		{"forbidden_403", 403},
		{"unauthorized_401", 401},
		{"not_found_404", 404},
		{"internal_500", 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"x":1}`))
			}))
			defer srv.Close()
			svc := NewGlobalCatalogService(srv.URL)
			status, _, err := svc.ListProducts(context.Background(), url.Values{}, "Bearer t", "")
			if err != nil {
				t.Fatalf("err inesperado para upstream %d: %v", tc.code, err)
			}
			if status != tc.code {
				t.Fatalf("status = %d, quiero %d (debe preservarse el del upstream)", status, tc.code)
			}
		})
	}
}

// TestGlobalCatalogService_FallaTransporteDevuelveErrSinStatus verifica que una falla de
// transporte (no hubo respuesta del upstream) devuelve err != nil y status 0 — el handler
// responde 502 en ese caso, único donde 502 es correcto.
func TestGlobalCatalogService_FallaTransporteDevuelveErrSinStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // servidor cerrado → conexión rechazada

	svc := NewGlobalCatalogService(srv.URL)
	status, _, err := svc.ListProducts(context.Background(), url.Values{}, "Bearer t", "")
	if err == nil {
		t.Fatalf("esperaba error de transporte, got nil")
	}
	if status != 0 {
		t.Fatalf("status = %d, quiero 0 (no hubo respuesta del upstream)", status)
	}
}

