package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsUsesStaticContractWithoutPersistingAuthorization(t *testing.T) {
	contract := "openapi: 3.0.3\ninfo:\n  title: Oficina API\n"
	path := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(path, []byte(contract), 0600); err != nil {
		t.Fatal(err)
	}
	router := New(nil, Config{JWTSecret: strings.Repeat("s", 32), OpenAPIPath: path})
	for _, route := range []string{"/docs", "/docs/"} {
		t.Run(route, func(t *testing.T) {
			out := httptest.NewRecorder()
			router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, route, nil))
			if out.Code != http.StatusOK || !strings.HasPrefix(out.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("status %d, content type %s", out.Code, out.Header().Get("Content-Type"))
			}
			html := out.Body.String()
			for _, expected := range []string{`lang="pt-BR"`, "<title>Oficina API</title>", `url: "/openapi.yaml"`, "persistAuthorization: false", "validatorUrl: null", "deepLinking: true", "swagger-ui-dist@5.33.1/swagger-ui.css", "swagger-ui-dist@5.33.1/swagger-ui-bundle.js"} {
				if !strings.Contains(html, expected) {
					t.Errorf("missing %q", expected)
				}
			}
			for _, forbidden := range []string{"localStorage", "sessionStorage", "persistAuthorization: true"} {
				if strings.Contains(html, forbidden) {
					t.Errorf("unexpected credential persistence: %s", forbidden)
				}
			}
			hash := sha256.Sum256([]byte(docsInitializer))
			if !strings.Contains(out.Header().Get("Content-Security-Policy"), "'sha256-"+base64.StdEncoding.EncodeToString(hash[:])+"'") {
				t.Fatal("CSP must authorize the exact initializer")
			}
		})
	}
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if out.Code != http.StatusOK || out.Body.String() != contract {
		t.Fatal("OpenAPI route must serve the unchanged static file")
	}
}
