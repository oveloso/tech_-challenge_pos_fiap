package registry

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"math"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"strings"
	"unicode/utf8"
)

func (s *Service) Register(g *gin.RouterGroup) {
	for _, kind := range Kinds {
		k := kind
		base := "/" + k.Path
		g.GET(base, func(c *gin.Context) { s.list(c, k) })
		g.POST(base, func(c *gin.Context) {
			r, e := platform.Body(c, k.Fields, k.Required)
			if e != nil {
				platform.Fail(c, e)
				return
			}
			out, e := s.Write(c.Request.Context(), k, "", r, true)
			if e != nil {
				platform.Fail(c, e)
				return
			}
			c.JSON(201, out)
		})
		path := base + "/:" + k.IDParam
		g.GET(path, func(c *gin.Context) {
			id := c.Param(k.IDParam)
			if e := platform.UUID(id); e != nil {
				platform.Fail(c, e)
				return
			}
			r, e := platform.Get(c.Request.Context(), s.DB, k.Table, id, false)
			if e != nil {
				platform.Fail(c, e)
				return
			}
			c.JSON(200, r)
		})
		g.PATCH(path, func(c *gin.Context) {
			id := c.Param(k.IDParam)
			if e := platform.UUID(id); e != nil {
				platform.Fail(c, e)
				return
			}
			fields := []string{"active"}
			for _, f := range k.Fields {
				if f != "initialQuantity" {
					fields = append(fields, f)
				}
			}
			r, e := platform.Body(c, fields, nil)
			if e == nil && len(r) == 0 {
				e = domain.Invalid("Informe ao menos um campo.")
			}
			if e != nil {
				platform.Fail(c, e)
				return
			}
			out, e := s.Write(c.Request.Context(), k, id, r, false)
			if e != nil {
				platform.Fail(c, e)
				return
			}
			c.JSON(200, out)
		})
		g.DELETE(path, func(c *gin.Context) {
			id := c.Param(k.IDParam)
			if e := platform.UUID(id); e != nil {
				platform.Fail(c, e)
				return
			}
			_, e := s.Write(c.Request.Context(), k, id, platform.Record{"active": false}, false)
			if e != nil {
				platform.Fail(c, e)
				return
			}
			c.Status(204)
		})
	}
	g.GET("/inventory-items/:inventoryItemId/movements", s.listMovements)
	g.POST("/inventory-items/:inventoryItemId/movements", s.movement)
}

func (s *Service) list(c *gin.Context, k Kind) {
	for field, limit := range map[string]int{"document": 32, "plate": 16, "sku": 64, "name": 160} {
		if value, present := c.GetQuery(field); present && utf8.RuneCountInString(value) > limit {
			platform.Fail(c, domain.Invalid("Filtro excede o tamanho permitido: "+field))
			return
		}
	}
	p, sz, e := platform.Page(c)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	where := []string{"TRUE"}
	args := []any{}
	add := func(expr string, v any) { args = append(args, v); where = append(where, fmt.Sprintf(expr, len(args))) }
	if a, ok := c.GetQuery("active"); ok {
		if a != "true" && a != "false" {
			platform.Fail(c, domain.Invalid("active deve ser true ou false."))
			return
		}
		add("data->>'active'=$%d", a)
	}
	if k.Table == "customers" {
		if doc, ok := c.GetQuery("document"); ok {
			add("document=$%d", domain.Normalize(doc))
		}
	}
	if k.Table == "vehicles" {
		if plate, ok := c.GetQuery("plate"); ok {
			add("plate=$%d", domain.Normalize(plate))
		}
		if id, ok := c.GetQuery("customerId"); ok {
			if e = platform.UUID(id); e != nil {
				platform.Fail(c, e)
				return
			}
			add("customer_id=$%d", id)
		}
	}
	if k.Table == "inventory_items" {
		if sku, ok := c.GetQuery("sku"); ok {
			add("sku=$%d", strings.ToUpper(strings.TrimSpace(sku)))
		}
	}
	if name, ok := c.GetQuery("name"); ok {
		add("position(lower($%d) in lower(data->>'name'))>0", strings.TrimSpace(name))
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if e = s.DB.QueryRow(c.Request.Context(), "SELECT count(*) FROM "+k.Table+" WHERE "+clause, args...).Scan(&total); e != nil {
		platform.Fail(c, e)
		return
	}
	args = append(args, sz, (p-1)*sz)
	query := "SELECT data FROM " + k.Table + " WHERE " + clause + fmt.Sprintf(" ORDER BY data->>'createdAt' DESC,id LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, e := s.DB.Query(c.Request.Context(), query, args...)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	defer rows.Close()
	data := []platform.Record{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			platform.Fail(c, e)
			return
		}
		r, e := platform.Decode(raw)
		if e != nil {
			platform.Fail(c, e)
			return
		}
		data = append(data, r)
	}
	if e = rows.Err(); e != nil {
		platform.Fail(c, e)
		return
	}
	platform.PageResponse(c, data, total, p, sz)
}

func (s *Service) movement(c *gin.Context) {
	id := c.Param("inventoryItemId")
	if e := platform.UUID(id); e != nil {
		platform.Fail(c, e)
		return
	}
	r, e := platform.Body(c, []string{"type", "quantityDelta", "reason"}, []string{"type", "quantityDelta", "reason"})
	if e == nil {
		e = platform.Integer(r, "quantityDelta", math.MinInt64, math.MaxInt64)
	}
	if e == nil {
		e = platform.String(r, "reason", 1, 500, false)
	}
	if e != nil {
		platform.Fail(c, e)
		return
	}
	result, e := s.RecordMovement(c.Request.Context(), id, platform.Text(r, "type"), platform.Number(r, "quantityDelta"), platform.Text(r, "reason"))
	if e != nil {
		platform.Fail(c, e)
		return
	}
	c.JSON(201, result)
}

func (s *Service) listMovements(c *gin.Context) {
	id := c.Param("inventoryItemId")
	if e := platform.UUID(id); e != nil {
		platform.Fail(c, e)
		return
	}
	p, sz, e := platform.Page(c)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	ctx := c.Request.Context()
	if _, e = platform.Get(ctx, s.DB, "inventory_items", id, false); e != nil {
		platform.Fail(c, e)
		return
	}
	var total int64
	if e = s.DB.QueryRow(ctx, "SELECT count(*) FROM inventory_movements WHERE inventory_item_id=$1", id).Scan(&total); e != nil {
		platform.Fail(c, e)
		return
	}
	rows, e := s.DB.Query(ctx, "SELECT data FROM inventory_movements WHERE inventory_item_id=$1 ORDER BY data->>'occurredAt' DESC,id DESC LIMIT $2 OFFSET $3", id, sz, (p-1)*sz)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	defer rows.Close()
	result := []platform.Record{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			platform.Fail(c, e)
			return
		}
		r, e := platform.Decode(raw)
		if e != nil {
			platform.Fail(c, e)
			return
		}
		result = append(result, r)
	}
	if e = rows.Err(); e != nil {
		platform.Fail(c, e)
		return
	}
	platform.PageResponse(c, result, total, p, sz)
}
