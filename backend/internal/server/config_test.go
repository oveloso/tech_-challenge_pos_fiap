package server

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("PORT", "")
	t.Setenv("HOST", "")
	t.Setenv("CORS_ORIGINS", "http://localhost:5173")
	t.Setenv("CUSTOMER_TOKEN_TTL", "")
	t.Setenv("OPENAPI_PATH", "")
	c, e := ConfigFromEnv()
	if e != nil || c.Host != "127.0.0.1" || c.Port != "8080" || c.CustomerTokenTTL != 30*24*time.Hour || c.OpenAPIPath != "../docs/api/openapi.yaml" {
		t.Fatal(c, e)
	}
	for _, host := range []string{"0.0.0.0", "127.0.0.1", "::1", "::"} {
		t.Run(host, func(t *testing.T) {
			t.Setenv("HOST", host)
			cfg, err := ConfigFromEnv()
			if err != nil || cfg.Host != host {
				t.Fatalf("valid host %q rejected: %v", host, err)
			}
		})
	}
	for _, ttl := range []string{"7d", "24h"} {
		t.Setenv("CUSTOMER_TOKEN_TTL", ttl)
		if _, e = ConfigFromEnv(); e != nil {
			t.Fatal(e)
		}
	}
	for _, ttl := range []string{"0d", "-2d", "3660d", "garbaged", "1s", "bad"} {
		t.Setenv("CUSTOMER_TOKEN_TTL", ttl)
		if _, e = ConfigFromEnv(); e == nil {
			t.Fatalf("accepted %s", ttl)
		}
	}
	t.Setenv("CUSTOMER_TOKEN_TTL", "")
	for _, tc := range []struct{ k, v string }{{"HOST", "localhost"}, {"HOST", "hostnamegarbage"}, {"HOST", "999.0.0.1"}, {"PORT", "x"}, {"PORT", "0"}, {"DATABASE_URL", ""}, {"JWT_SECRET", "short"}, {"CORS_ORIGINS", "*"}} {
		t.Run(tc.k+tc.v, func(t *testing.T) {
			t.Setenv(tc.k, tc.v)
			if _, e := ConfigFromEnv(); e == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
