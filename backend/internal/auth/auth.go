package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Service struct {
	DB        *pgxpool.Pool
	Secret    []byte
	dummyHash []byte
}

func PasswordHash(password string) ([]byte, error) {
	s := sha256.Sum256([]byte(password))
	return bcrypt.GenerateFromPassword([]byte(base64.RawStdEncoding.EncodeToString(s[:])), bcrypt.DefaultCost)
}
func CheckPassword(hash []byte, password string) bool {
	s := sha256.Sum256([]byte(password))
	return bcrypt.CompareHashAndPassword(hash, []byte(base64.RawStdEncoding.EncodeToString(s[:]))) == nil
}
func New(db *pgxpool.Pool, secret string) *Service {
	h, _ := PasswordHash(uuid.NewString())
	return &Service{db, []byte(secret), h}
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (s *Service) Login(c *gin.Context) {
	r, e := platform.Body(c, []string{"username", "password"}, []string{"username", "password"})
	if e == nil {
		e = platform.String(r, "username", 1, 100, false)
	}
	if e == nil {
		v, ok := r["password"].(string)
		if !ok || utf8.RuneCountInString(v) < 1 || utf8.RuneCountInString(v) > 256 {
			e = domain.Invalid("Senha deve ter entre 1 e 256 caracteres.")
		}
	}
	if e != nil {
		platform.Fail(c, e)
		return
	}
	var id, hash string
	e = s.DB.QueryRow(c.Request.Context(), "SELECT id::text,password_hash FROM administrators WHERE username=$1", platform.Text(r, "username")).Scan(&id, &hash)
	if e != nil {
		CheckPassword(s.dummyHash, platform.Text(r, "password"))
		platform.Fail(c, domain.Unauthorized())
		return
	}
	if !CheckPassword([]byte(hash), platform.Text(r, "password")) {
		platform.Fail(c, domain.Unauthorized())
		return
	}
	now := time.Now().UTC()
	expires := now.Add(time.Hour)
	claims := Claims{Role: "admin", RegisteredClaims: jwt.RegisteredClaims{Issuer: "oficina", Subject: id, Audience: jwt.ClaimStrings{"oficina-admin"}, ExpiresAt: jwt.NewNumericDate(expires), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ID: uuid.NewString()}}
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.Secret)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	c.JSON(http.StatusOK, gin.H{"accessToken": token, "tokenType": "Bearer", "expiresAt": expires})
}
func bearer(c *gin.Context) string {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
func (s *Service) Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c)
		claims := &Claims{}
		token, e := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) { return s.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("oficina"), jwt.WithAudience("oficina-admin"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if e != nil || !token.Valid || claims.Role != "admin" || platform.UUID(claims.Subject) != nil {
			platform.Fail(c, domain.Unauthorized())
			return
		}
		c.Next()
	}
}
func Customer() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c)
		bytes, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || len(bytes) != 32 || len(raw) != 43 {
			platform.Fail(c, domain.Unauthorized())
			return
		}
		hash := sha256.Sum256([]byte(raw))
		c.Set("customerTokenHash", hash[:])
		c.Next()
	}
}
func NewCustomerToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", nil, e
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

type window struct {
	count int
	until time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]window
	limit   int
	period  time.Duration
}

func RateLimit(limit int, period time.Duration) gin.HandlerFunc {
	l := &limiter{entries: map[string]window{}, limit: limit, period: period}
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()
		l.mu.Lock()
		if len(l.entries) >= 10000 {
			for k, v := range l.entries {
				if now.After(v.until) {
					delete(l.entries, k)
				}
			}
		}
		w, ok := l.entries[key]
		if !ok || now.After(w.until) {
			w = window{until: now.Add(l.period)}
		}
		w.count++
		allowed := w.count <= l.limit
		if !ok && len(l.entries) >= 10000 {
			allowed = false
		} else {
			l.entries[key] = w
		}
		l.mu.Unlock()
		if !allowed {
			c.Header("Retry-After", "60")
			platform.Fail(c, &domain.Error{Status: 429, Code: "RATE_LIMITED", Detail: "Muitas tentativas. Aguarde antes de tentar novamente."})
			return
		}
		c.Next()
	}
}
