package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Tests de QA de ACC-E04 T5 (Fase TEST) sobre el composition root. main() no se
// puede ejecutar en un test (levanta el server), así que se verifica el cableado
// sobre el AST: qué variable recibe NewHTTPTenantConfigClient y de qué env sale.

func parseMain(t *testing.T) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	return f
}

// envSources devuelve variable → nombre de env para las asignaciones `x := env.Get("NAME", ...)`.
func envSources(f *ast.File) map[string]string {
	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		id, ok := as.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Get" {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if name, err := strconv.Unquote(lit.Value); err == nil {
				out[id.Name] = name
			}
		}
		return true
	})
	return out
}

// configClientCalls devuelve los argumentos (como identificadores) de cada llamada a NewHTTPTenantConfigClient.
func configClientCalls(f *ast.File) [][]string {
	var calls [][]string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewHTTPTenantConfigClient" {
			return true
		}
		var args []string
		for _, a := range call.Args {
			if id, ok := a.(*ast.Ident); ok {
				args = append(args, id.Name)
			} else {
				args = append(args, "<expr>")
			}
		}
		calls = append(calls, args)
		return true
	})
	return calls
}

// T5-QA-08 · el destino del cliente de config es ACCOUNT_SERVICE_URL y la
// credencial es ACCOUNT_SERVICE_S2S_KEY (propia del BFF, DD-1), no la
// S2S_API_KEY compartida ni TENANT_SERVICE_URL.
func TestT5_Wiring_ConfigClientPointsToAccountServiceWithOwnCredential(t *testing.T) {
	f := parseMain(t)
	sources := envSources(f)
	calls := configClientCalls(f)

	if len(calls) != 1 {
		t.Fatalf("esperado exactamente 1 NewHTTPTenantConfigClient en main.go (una sola fuente de config), got %d", len(calls))
	}
	args := calls[0]
	if len(args) != 2 {
		t.Fatalf("NewHTTPTenantConfigClient debe recibir (baseURL, apiKey), got %v", args)
	}
	if got := sources[args[0]]; got != "ACCOUNT_SERVICE_URL" {
		t.Errorf("baseURL del cliente de config sale de %q (variable %s); esperado ACCOUNT_SERVICE_URL", got, args[0])
	}
	if got := sources[args[1]]; got != "ACCOUNT_SERVICE_S2S_KEY" {
		t.Errorf("credencial del cliente de config sale de %q (variable %s); esperado ACCOUNT_SERVICE_S2S_KEY", got, args[1])
	}
}

// T5-QA-09 · NEGATIVA de destino viejo: TENANT_SERVICE_URL no puede alimentar
// el cliente de config ni el resolver. Si alguien vuelve a pasar
// tenantServiceURL, habría dos fuentes y el corte no está hecho.
func TestT5_Wiring_TenantServiceURLNeverFeedsConfigClient(t *testing.T) {
	f := parseMain(t)
	sources := envSources(f)

	for _, args := range configClientCalls(f) {
		for _, a := range args {
			if sources[a] == "TENANT_SERVICE_URL" || sources[a] == "S2S_API_KEY" {
				t.Errorf("NewHTTPTenantConfigClient recibe %s (env %s): destino/credencial viejos", a, sources[a])
			}
		}
	}
}

// T5-QA-10 · criterio (b) por intención: ningún literal de código vivo (no
// tests, no comentarios) bajo src/ referencia tenant-service/8120 en contexto
// de config, y el path "/tenant/config" sólo aparece en el cliente cuyo
// baseURL es ACCOUNT_SERVICE_URL (es la ruta GET /api/v1/tenant/config/:key de
// account-service, tenant_config_controller.go RegisterReadRoute).
//
// Nota: el grep LITERAL de "Hecho cuando (b)" devuelve 1 hit
// (tenant_config_client.go, el path) — este test codifica la intención
// ("ninguna referencia viva al destino viejo"), no el literal.
func TestT5_NoLiveReferenceToOldConfigDestination(t *testing.T) {
	const allowedPathFile = "src/infrastructure/tenant/client/tenant_config_client.go"
	var pathHits []string

	err := filepath.WalkDir("src", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0) // sin comentarios
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s := strings.ToLower(lit.Value)
			if (strings.Contains(s, "tenant-service") || strings.Contains(s, "8120")) && strings.Contains(s, "config") {
				t.Errorf("%s: literal vivo hacia el destino viejo de config: %s", path, lit.Value)
			}
			if strings.Contains(s, "/tenant/config") {
				pathHits = append(pathHits, filepath.ToSlash(path))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk src: %v", err)
	}

	for _, p := range pathHits {
		if p != allowedPathFile {
			t.Errorf("%s: path /tenant/config fuera del cliente de account-service", p)
		}
	}
	if len(pathHits) != 1 {
		t.Errorf("esperado el path de config en exactamente 1 literal (el cliente), got %d: %v", len(pathHits), pathHits)
	}
}
