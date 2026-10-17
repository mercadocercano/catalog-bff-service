package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"catalog-bff-service/src/domain"
	"catalog-bff-service/src/infrastructure/cache"
	"catalog-bff-service/src/infrastructure/tenant/client"
)

// Tests de QA de ACC-E04 T5 (Fase TEST). Recorren la cadena real que arma
// main.go —HTTPTenantConfigClient → CachedTenantConfigClient →
// TenantStockPolicyResolver— contra un account-service falso que aplica el
// gate de ADR-005 DD-1 (b): X-API-Key con scope config:read, tenant de
// X-Tenant-ID leído sólo tras el gate, 404 + value null si no hay fila.
//
// Contrato verificado: cambia el destino, se conservan 404→("",nil),
// 5xx→error, timeout 500ms, cache y fallback REQUIRE_STOCK; el resolver mapea
// IGNORE_STOCK/REQUIRE_STOCK igual que antes.

const (
	qaValidKey  = "qa-catalog-bff-s2s-key"
	qaTenantA   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" // catalog.stock_policy=IGNORE_STOCK
	qaTenantB   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" // sin fila de config
	qaTenantR   = "cccccccc-cccc-4ccc-8ccc-cccccccccccc" // catalog.stock_policy=REQUIRE_STOCK
	qaConfigKey = "catalog.stock_policy"
)

// fakeAccountService imita el read de config de account-service con el gate DD-1.
type fakeAccountService struct {
	mu      sync.Mutex
	configs map[string]map[string]string // tenant → key → value
	hits    map[string]int                // tenant → requests que pasaron el gate
	denied  int                           // requests rechazados por el gate
	status  int                           // si != 0, fuerza este status tras el gate
	delay   time.Duration
}

func newFakeAccountService() *fakeAccountService {
	return &fakeAccountService{
		configs: map[string]map[string]string{
			qaTenantA: {qaConfigKey: "IGNORE_STOCK"},
			qaTenantR: {qaConfigKey: "REQUIRE_STOCK"},
		},
		hits: map[string]int{},
	}
}

func (f *fakeAccountService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-r.Context().Done():
			return
		}
	}

	const prefix = "/api/v1/tenant/config/"
	if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, prefix) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Gate DD-1: sin key → 401; key que no resuelve a config:read → 403.
	switch r.Header.Get("X-API-Key") {
	case "":
		f.denied++
		w.WriteHeader(http.StatusUnauthorized)
		return
	case qaValidKey:
	default:
		f.denied++
		w.WriteHeader(http.StatusForbidden)
		return
	}

	tenant := r.Header.Get("X-Tenant-ID")
	f.hits[tenant]++
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, prefix)
	w.Header().Set("Content-Type", "application/json")
	value, ok := f.configs[tenant][key]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"key": key, "value": nil})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"key": key, "value": value})
}

func (f *fakeAccountService) hitsFor(tenant string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[tenant]
}

func (f *fakeAccountService) deniedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.denied
}

// newResolverChain arma la misma cadena que main.go con el destino y la key dados.
func newResolverChain(t *testing.T, baseURL, apiKey string) *domain.TenantStockPolicyResolver {
	t.Helper()
	t.Setenv("FORCE_STOCK_POLICY", "") // el override por env no debe enmascarar el destino
	httpClient := client.NewHTTPTenantConfigClient(baseURL, apiKey)
	cached := client.NewCachedTenantConfigClient(httpClient, cache.NewInMemoryCache[string](time.Minute, 0))
	return domain.NewTenantStockPolicyResolver(cached)
}

// T5-QA-01 · (c) mapeo: con credencial válida, IGNORE_STOCK y REQUIRE_STOCK del
// tenant llegan al resolver con el mismo mapeo que antes.
func TestT5_AccountService_ValidCredential_MapsPolicyPerTenant(t *testing.T) {
	// Arrange
	fake := newFakeAccountService()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	resolver := newResolverChain(t, srv.URL, qaValidKey)

	// Act
	gotA := resolver.Resolve(context.Background(), qaTenantA)
	gotR := resolver.Resolve(context.Background(), qaTenantR)

	// Assert
	if gotA != domain.IgnoreStock {
		t.Errorf("tenant A con IGNORE_STOCK en account-service: esperado IgnoreStock, got %q", gotA)
	}
	if gotR != domain.RequireStock {
		t.Errorf("tenant R con REQUIRE_STOCK en account-service: esperado RequireStock, got %q", gotR)
	}
	if fake.deniedCount() != 0 {
		t.Errorf("con la credencial válida ningún request debía ser rechazado por el gate, got %d", fake.deniedCount())
	}
}

// T5-QA-02 · NEGATIVA de credencial: sin key o con key ajena, account-service
// rechaza y el BFF NO obtiene la config real del tenant —cae fail-closed a
// REQUIRE_STOCK aunque el tenant tenga IGNORE_STOCK—. Si el cliente dejara de
// distinguir 401/403 de "no existe", o el fake no exigiera la key, esto se vería
// igual en el valor; por eso además se exige que el rechazo NO se cachee.
func TestT5_AccountService_MissingOrForeignCredential_FailsClosedAndIsNotCached(t *testing.T) {
	cases := []struct {
		name   string
		apiKey string
	}{
		{"sin X-API-Key → 401", ""},
		{"key sin scope config:read → 403", "otra-key-de-otro-servicio"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			fake := newFakeAccountService()
			srv := httptest.NewServer(fake)
			defer srv.Close()
			resolver := newResolverChain(t, srv.URL, tc.apiKey)

			// Act
			first := resolver.Resolve(context.Background(), qaTenantA)
			second := resolver.Resolve(context.Background(), qaTenantA)

			// Assert
			if first != domain.RequireStock || second != domain.RequireStock {
				t.Fatalf("sin credencial válida la policy debe caer a RequireStock, got %q/%q", first, second)
			}
			if fake.hitsFor(qaTenantA) != 0 {
				t.Errorf("el gate debía cortar antes de leer config del tenant, got %d lecturas", fake.hitsFor(qaTenantA))
			}
			if fake.deniedCount() != 2 {
				t.Errorf("un rechazo de auth no debe cachearse (cada Resolve tiene que reintentar): esperado 2 requests rechazados, got %d", fake.deniedCount())
			}
		})
	}
}

// T5-QA-03 · 404 conservado + sin fuga entre tenants por cache: A se resuelve
// primero (cachea IGNORE_STOCK); B, sin fila, debe ir al servicio con SU tenant
// y resolver REQUIRE_STOCK. Un cache key que ignorara el tenant le daría a B la
// policy de A.
func TestT5_AccountService_TenantWithoutConfig_FallsBackWithoutCrossTenantCacheLeak(t *testing.T) {
	// Arrange
	fake := newFakeAccountService()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	resolver := newResolverChain(t, srv.URL, qaValidKey)

	// Act
	gotA := resolver.Resolve(context.Background(), qaTenantA)
	gotB := resolver.Resolve(context.Background(), qaTenantB)

	// Assert
	if gotA != domain.IgnoreStock {
		t.Fatalf("precondición: tenant A esperado IgnoreStock, got %q", gotA)
	}
	if gotB != domain.RequireStock {
		t.Errorf("tenant B sin config (404) debe caer a RequireStock, got %q (¿fuga del cache de A?)", gotB)
	}
	if fake.hitsFor(qaTenantB) != 1 {
		t.Errorf("tenant B debía consultarse con su propio X-Tenant-ID exactamente 1 vez, got %d", fake.hitsFor(qaTenantB))
	}
}

// T5-QA-04 · cache conservado: la segunda resolución del mismo tenant no vuelve
// a pegarle a account-service (incluido el 404, que cachea "no existe").
func TestT5_AccountService_CacheWrapperPreserved(t *testing.T) {
	// Arrange
	fake := newFakeAccountService()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	resolver := newResolverChain(t, srv.URL, qaValidKey)

	// Act
	for i := 0; i < 3; i++ {
		resolver.Resolve(context.Background(), qaTenantA)
		resolver.Resolve(context.Background(), qaTenantB)
	}

	// Assert
	if fake.hitsFor(qaTenantA) != 1 {
		t.Errorf("tenant A: esperado 1 request (luego cache hit), got %d", fake.hitsFor(qaTenantA))
	}
	if fake.hitsFor(qaTenantB) != 1 {
		t.Errorf("tenant B (404): esperado 1 request (el 'no existe' se cachea), got %d", fake.hitsFor(qaTenantB))
	}
}

// T5-QA-05 · 5xx conservado: error → fallback REQUIRE_STOCK y NO se cachea; al
// recuperarse account-service, el tenant vuelve a su policy real.
func TestT5_AccountService_ServerError_FallsBackAndRecovers(t *testing.T) {
	// Arrange
	fake := newFakeAccountService()
	fake.status = http.StatusInternalServerError
	srv := httptest.NewServer(fake)
	defer srv.Close()
	resolver := newResolverChain(t, srv.URL, qaValidKey)

	// Act
	duringOutage := resolver.Resolve(context.Background(), qaTenantA)
	fake.mu.Lock()
	fake.status = 0
	fake.mu.Unlock()
	afterRecovery := resolver.Resolve(context.Background(), qaTenantA)

	// Assert
	if duringOutage != domain.RequireStock {
		t.Errorf("5xx debe caer a RequireStock, got %q", duringOutage)
	}
	if afterRecovery != domain.IgnoreStock {
		t.Errorf("tras recuperarse account-service el tenant A debe resolver IgnoreStock (el 5xx no se cachea), got %q", afterRecovery)
	}
	if fake.hitsFor(qaTenantA) != 2 {
		t.Errorf("esperado 2 requests (el error no se cachea), got %d", fake.hitsFor(qaTenantA))
	}
}

// T5-QA-06 · timeout conservado: account-service colgado → la resolución termina
// por el timeout de 500ms del cliente y cae a REQUIRE_STOCK, sin bloquear el BFF.
func TestT5_AccountService_Hung_TimesOutAndFallsBack(t *testing.T) {
	// Arrange
	fake := newFakeAccountService()
	fake.delay = 3 * time.Second
	srv := httptest.NewServer(fake)
	defer srv.Close()
	resolver := newResolverChain(t, srv.URL, qaValidKey)

	// Act
	start := time.Now()
	got := resolver.Resolve(context.Background(), qaTenantA)
	elapsed := time.Since(start)

	// Assert
	if got != domain.RequireStock {
		t.Errorf("timeout debe caer a RequireStock, got %q", got)
	}
	if elapsed < 400*time.Millisecond || elapsed > 1200*time.Millisecond {
		t.Errorf("la resolución debía cortarse por el timeout de ~500ms del cliente, tardó %s", elapsed)
	}
}

// T5-QA-07 · fallback sin destino: si ACCOUNT_SERVICE_URL no está configurado,
// main.go arma el resolver sin client → REQUIRE_STOCK, aun para un tenant que
// en account-service tendría IGNORE_STOCK.
func TestT5_NoDestinationConfigured_FallsBackToRequireStock(t *testing.T) {
	t.Setenv("FORCE_STOCK_POLICY", "")
	resolver := domain.NewTenantStockPolicyResolver(nil)

	if got := resolver.Resolve(context.Background(), qaTenantA); got != domain.RequireStock {
		t.Errorf("sin destino configurado esperado RequireStock, got %q", got)
	}
}
