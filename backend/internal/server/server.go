package server

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"net"
	"net/http"
	"oficina/internal/auth"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"oficina/internal/registry"
	"oficina/internal/workorders"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL, JWTSecret, Host, Port, OpenAPIPath string
	Origins                                         []string
	CustomerTokenTTL                                time.Duration
}

func ConfigFromEnv() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), JWTSecret: os.Getenv("JWT_SECRET"), Host: os.Getenv("HOST"), Port: os.Getenv("PORT"), OpenAPIPath: os.Getenv("OPENAPI_PATH"), Origins: strings.Split(os.Getenv("CORS_ORIGINS"), ","), CustomerTokenTTL: 30 * 24 * time.Hour}
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if net.ParseIP(c.Host) == nil {
		return c, domain.Invalid("HOST deve ser um endereço IPv4 ou IPv6 válido.")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.OpenAPIPath == "" {
		c.OpenAPIPath = "../docs/api/openapi.yaml"
	}
	p, e := strconv.Atoi(c.Port)
	if e != nil || p < 1 || p > 65535 {
		return c, domain.Invalid("PORT inválida.")
	}
	if c.DatabaseURL == "" {
		return c, domain.Invalid("DATABASE_URL é obrigatória.")
	}
	if len(c.JWTSecret) < 32 {
		return c, domain.Invalid("JWT_SECRET deve ter pelo menos 32 bytes.")
	}
	if raw := os.Getenv("CUSTOMER_TOKEN_TTL"); raw != "" {
		if strings.HasSuffix(raw, "d") {
			d, e := strconv.Atoi(strings.TrimSuffix(raw, "d"))
			if e != nil || d < 1 || d > 3650 {
				return c, domain.Invalid("CUSTOMER_TOKEN_TTL inválido.")
			}
			c.CustomerTokenTTL = time.Duration(d) * 24 * time.Hour
		} else {
			ttl, e := time.ParseDuration(raw)
			if e != nil || ttl < time.Minute || ttl > 3650*24*time.Hour {
				return c, domain.Invalid("CUSTOMER_TOKEN_TTL inválido.")
			}
			c.CustomerTokenTTL = ttl
		}
	}
	for _, origin := range c.Origins {
		if strings.TrimSpace(origin) == "*" {
			return c, domain.Invalid("CORS exige origens explícitas.")
		}
	}
	return c, nil
}
func New(db *pgxpool.Pool, cfg Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.MaxMultipartMemory = 1 << 20
	r.Use(func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				platform.Fail(c, &domain.Error{Status: 500, Code: "INTERNAL_ERROR", Detail: "Não foi possível processar a solicitação."})
			}
		}()
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	allowed := map[string]bool{}
	for _, origin := range cfg.Origins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Vary", "Origin")
			if allowed[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Max-Age", "600")
			}
		}
		if c.Request.Method == http.MethodOptions {
			if origin != "" && !allowed[origin] {
				platform.Fail(c, &domain.Error{Status: 403, Code: "ORIGIN_NOT_ALLOWED", Detail: "Origem não permitida."})
				return
			}
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	health := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if e := db.Ping(ctx); e != nil {
			platform.Fail(c, &domain.Error{Status: 503, Code: "UNAVAILABLE", Detail: "Banco de dados indisponível."})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	}
	r.GET("/health", health)
	r.GET("/docs", serveDocs)
	r.GET("/docs/", serveDocs)
	api := r.Group("/api/v1")
	api.GET("/health", health)
	if cfg.OpenAPIPath != "" {
		serve := func(c *gin.Context) { c.Header("Content-Type", "application/yaml"); c.File(cfg.OpenAPIPath) }
		r.GET("/openapi.yaml", serve)
		api.GET("/openapi.yaml", serve)
	}
	authentication := auth.New(db, cfg.JWTSecret)
	api.POST("/auth/login", auth.RateLimit(10, time.Minute), authentication.Login)
	admin := api.Group("/admin", authentication.Admin())
	customer := api.Group("/customer", auth.RateLimit(60, time.Minute), auth.Customer())
	(&registry.Service{DB: db}).Register(admin)
	(&workorders.Service{DB: db, TokenTTL: cfg.CustomerTokenTTL}).Register(admin, customer)
	r.NoRoute(func(c *gin.Context) { platform.Fail(c, domain.Missing()) })
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		platform.Fail(c, &domain.Error{Status: 405, Code: "METHOD_NOT_ALLOWED", Detail: "Método não permitido."})
	})
	return r
}
