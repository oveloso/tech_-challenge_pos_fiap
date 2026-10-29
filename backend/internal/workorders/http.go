package workorders

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"strconv"
	"strings"
)

func (s *Service) Register(admin, customer *gin.RouterGroup) {
	admin.POST("/service-orders", s.create)
	admin.GET("/service-orders", s.list)
	admin.GET("/service-orders/:serviceOrderId", func(c *gin.Context) {
		id := c.Param("serviceOrderId")
		if e := platform.UUID(id); e != nil {
			platform.Fail(c, e)
			return
		}
		o, e := Load(c.Request.Context(), s.DB, id, false)
		if e != nil {
			platform.Fail(c, e)
			return
		}
		c.JSON(200, o)
	})
	for _, action := range []string{"diagnosis/start", "diagnosis/reopen", "quotes", "complete", "deliver", "customer-access"} {
		a := action
		admin.POST("/service-orders/:serviceOrderId/"+a, func(c *gin.Context) { s.command(c, a) })
	}
	admin.PUT("/service-orders/:serviceOrderId/items", func(c *gin.Context) { s.command(c, "items") })
	admin.POST("/service-orders/:serviceOrderId/services/:serviceOrderServiceId/start", func(c *gin.Context) { s.command(c, "services/start") })
	admin.POST("/service-orders/:serviceOrderId/services/:serviceOrderServiceId/complete", func(c *gin.Context) { s.command(c, "services/complete") })
	customer.GET("/service-order", func(c *gin.Context) {
		o, e := LoadCustomer(c.Request.Context(), s.DB, c.MustGet("customerTokenHash").([]byte), false)
		if e != nil {
			platform.Fail(c, e)
			return
		}
		c.JSON(200, o.CustomerView())
	})
	customer.POST("/service-order/quote-decision", func(c *gin.Context) {
		var request quoteDecisionRequest
		e := decodeOrderRequest(c, &request)
		if e == nil {
			e = request.validate()
		}
		if e != nil {
			platform.Fail(c, e)
			return
		}
		o, e := s.Decide(c.Request.Context(), c.MustGet("customerTokenHash").([]byte), request.QuoteID, request.Decision)
		if e != nil {
			platform.Fail(c, e)
			return
		}
		c.JSON(200, o.CustomerView())
	})
	admin.GET("/metrics/service-orders", s.orderMetrics)
	admin.GET("/metrics/service-execution", s.serviceMetrics)
}
func (s *Service) create(c *gin.Context) {
	var request createOrderRequest
	e := decodeOrderRequest(c, &request)
	if e == nil {
		e = request.validate()
	}
	if e != nil {
		platform.Fail(c, e)
		return
	}
	o, e := s.Create(c.Request.Context(), request.CustomerID, request.VehicleID, request.ProblemDescription, request.Selection)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	c.JSON(201, o)
}
func (s *Service) command(c *gin.Context, action string) {
	id := c.Param("serviceOrderId")
	if e := platform.UUID(id); e != nil {
		platform.Fail(c, e)
		return
	}
	sid := strings.ToLower(c.Param("serviceOrderServiceId"))
	if sid != "" {
		if e := platform.UUID(sid); e != nil {
			platform.Fail(c, e)
			return
		}
	}
	var version int64
	var selection Selection
	var e error
	if action == "items" {
		var request replaceItemsRequest
		e = decodeOrderRequest(c, &request)
		if e == nil {
			e = request.validate()
		}
		version, selection = request.ExpectedVersion, request.Selection
	} else {
		var request versionRequest
		e = decodeOrderRequest(c, &request)
		if e == nil {
			e = request.validate()
		}
		version = request.ExpectedVersion
	}
	if e != nil {
		platform.Fail(c, e)
		return
	}
	out, e := s.Execute(c.Request.Context(), id, version, action, sid, selection)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	status := 200
	if action == "customer-access" {
		status = 201
	}
	c.JSON(status, out)
}
func (s *Service) list(c *gin.Context) {
	p, sz, e := platform.Page(c)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	where := []string{"TRUE"}
	args := []any{}
	add := func(expr string, v any) { args = append(args, v); where = append(where, fmt.Sprintf(expr, len(args))) }
	if id, ok := c.GetQuery("customerId"); ok {
		if e = platform.UUID(id); e != nil {
			platform.Fail(c, e)
			return
		}
		add("customer_id=$%d", id)
	}
	if status, ok := c.GetQuery("status"); ok {
		valid := false
		for _, st := range domain.Statuses {
			if status == st {
				valid = true
			}
		}
		if !valid {
			platform.Fail(c, domain.Invalid("Status inválido."))
			return
		}
		add("status=$%d", status)
	}
	if number, ok := c.GetQuery("number"); ok {
		n, err := strconv.ParseInt(number, 10, 64)
		if err != nil || n < 1 {
			platform.Fail(c, domain.Invalid("Número de OS inválido."))
			return
		}
		add("number=$%d", n)
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if e = s.DB.QueryRow(c.Request.Context(), "SELECT count(*) FROM service_orders WHERE "+clause, args...).Scan(&total); e != nil {
		platform.Fail(c, e)
		return
	}
	args = append(args, sz, (p-1)*sz)
	rows, e := s.DB.Query(c.Request.Context(), "SELECT data FROM service_orders WHERE "+clause+fmt.Sprintf(" ORDER BY number DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			platform.Fail(c, e)
			return
		}
		var o domain.Order
		if e = json.Unmarshal(raw, &o); e != nil {
			platform.Fail(c, e)
			return
		}
		out = append(out, map[string]any{"id": o.ID, "number": o.Number, "customerId": o.CustomerID, "customerName": o.CustomerName, "vehicleId": o.VehicleID, "vehiclePlate": o.VehiclePlate, "status": o.Status, "version": o.Version, "createdAt": o.CreatedAt, "executionStartedAt": o.ExecutionStartedAt, "completedAt": o.CompletedAt, "deliveredAt": o.DeliveredAt})
	}
	if e = rows.Err(); e != nil {
		platform.Fail(c, e)
		return
	}
	platform.PageResponse(c, out, total, p, sz)
}
