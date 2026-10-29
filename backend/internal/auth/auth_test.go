package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPasswords(t *testing.T) {
	password := strings.Repeat("p", 200)
	hash, e := PasswordHash(password)
	if e != nil {
		t.Fatal(e)
	}
	if !CheckPassword(hash, password) || CheckPassword(hash, password+"x") {
		t.Fatal("password mismatch")
	}
}
func TestCustomerTokens(t *testing.T) {
	one, hash, e := NewCustomerToken()
	if e != nil {
		t.Fatal(e)
	}
	two, _, e := NewCustomerToken()
	if e != nil {
		t.Fatal(e)
	}
	want := sha256.Sum256([]byte(one))
	decoded, e := base64.RawURLEncoding.DecodeString(one)
	if e != nil || len(decoded) != 32 || len(one) != 43 || one == two || !bytes.Equal(hash, want[:]) {
		t.Fatal("invalid token generation")
	}
}
func TestCredentialIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := New(nil, strings.Repeat("s", 32))
	opaque, _, _ := NewCustomerToken()
	validClaims := Claims{Role: "admin", RegisteredClaims: jwt.RegisteredClaims{Issuer: "oficina", Subject: uuid.NewString(), Audience: jwt.ClaimStrings{"oficina-admin"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	valid, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims).SignedString(s.Secret)
	expiredClaims := validClaims
	expiredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	expired, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString(s.Secret)
	wrongAudience := validClaims
	wrongAudience.Audience = jwt.ClaimStrings{"customer"}
	wrong, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongAudience).SignedString(s.Secret)
	for _, tc := range []struct {
		name, token string
		middleware  gin.HandlerFunc
		want        int
	}{{"valid admin", valid, s.Admin(), 204}, {"opaque as admin", opaque, s.Admin(), 401}, {"expired", expired, s.Admin(), 401}, {"wrong audience", wrong, s.Admin(), 401}, {"missing", "", s.Admin(), 401}, {"jwt as customer", valid, Customer(), 401}, {"valid opaque", opaque, Customer(), 204}, {"malformed opaque", "abc", Customer(), 401}} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", tc.middleware, func(c *gin.Context) { c.Status(204) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			out := httptest.NewRecorder()
			r.ServeHTTP(out, req)
			if out.Code != tc.want {
				t.Fatalf("status %d: %s", out.Code, out.Body)
			}
		})
	}
}
func TestRateLimit(t *testing.T) {
	r := gin.New()
	r.GET("/", RateLimit(2, time.Minute), func(c *gin.Context) { c.Status(204) })
	for i := 0; i < 3; i++ {
		out := httptest.NewRecorder()
		r.ServeHTTP(out, httptest.NewRequest("GET", "/", nil))
		want := 204
		if i == 2 {
			want = 429
		}
		if out.Code != want {
			t.Fatal(out.Code)
		}
	}
}
