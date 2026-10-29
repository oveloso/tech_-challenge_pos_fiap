package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"oficina/internal/domain"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Fail(c *gin.Context, err error) {
	err = MapError(err)
	var e *domain.Error
	if !errors.As(err, &e) {
		slog.Error("request failed", "method", c.Request.Method, "route", c.FullPath(), "errorType", fmt.Sprintf("%T", err))
		e = &domain.Error{Status: 500, Code: "INTERNAL_ERROR", Detail: "Não foi possível processar a solicitação."}
	}
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(e.Status, gin.H{"type": "about:blank", "title": http.StatusText(e.Status), "status": e.Status, "detail": e.Detail, "code": e.Code})
}
func Body(c *gin.Context, allowed, required []string) (Record, error) {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, domain.Invalid("Envie o corpo como application/json.")
	}
	d := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	d.UseNumber()
	var r Record
	if err = d.Decode(&r); err != nil || r == nil {
		return nil, domain.Invalid("JSON inválido ou corpo acima de 1 MiB.")
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return nil, domain.Invalid("Envie apenas um objeto JSON.")
	}
	for k := range r {
		found := false
		for _, a := range allowed {
			if k == a {
				found = true
				break
			}
		}
		if !found {
			return nil, domain.Invalid("Campo não permitido: " + k)
		}
	}
	for _, k := range required {
		v, ok := r[k]
		if !ok || v == nil {
			return nil, domain.Invalid("Campo obrigatório: " + k)
		}
	}
	return r, nil
}
func String(r Record, key string, min, max int, nullable bool) error {
	v, ok := r[key]
	if !ok {
		return nil
	}
	if v == nil && nullable {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return domain.Invalid(key + " deve ser texto.")
	}
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n < min || n > max {
		return domain.Invalid("Tamanho inválido para " + key + ".")
	}
	r[key] = s
	return nil
}
func Integer(r Record, key string, min, max int64) error {
	v, ok := r[key]
	if !ok {
		return nil
	}
	n, ok := v.(json.Number)
	if !ok {
		return domain.Invalid(key + " deve ser inteiro.")
	}
	i, e := n.Int64()
	if e != nil || i < min || i > max {
		return domain.Invalid("Valor inválido para " + key + ".")
	}
	r[key] = i
	return nil
}
func UUID(s string) error {
	u, e := uuid.Parse(s)
	if e != nil || u == uuid.Nil || len(s) != 36 {
		return domain.Invalid("UUID inválido.")
	}
	return nil
}
func ID(r Record, key string) error {
	if _, ok := r[key]; !ok {
		return nil
	}
	if e := UUID(Text(r, key)); e != nil {
		return e
	}
	r[key] = strings.ToLower(Text(r, key))
	return nil
}
func Page(c *gin.Context) (int, int, error) {
	p, sz := 1, 20
	var err error
	if s, ok := c.GetQuery("page"); ok {
		p, err = strconv.Atoi(s)
		if err != nil || p < 1 || p > 100000000 {
			return 0, 0, domain.Invalid("page inválida.")
		}
	}
	if s, ok := c.GetQuery("pageSize"); ok {
		sz, err = strconv.Atoi(s)
		if err != nil || sz < 1 || sz > 100 {
			return 0, 0, domain.Invalid("pageSize inválido.")
		}
	}
	return p, sz, nil
}
func PageResponse(c *gin.Context, data any, total int64, p, sz int) {
	c.JSON(200, gin.H{"data": data, "total": total, "page": p, "pageSize": sz})
}
