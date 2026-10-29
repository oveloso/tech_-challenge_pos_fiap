package workorders

import (
	"github.com/gin-gonic/gin"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"time"
)

func interval(c *gin.Context) (time.Time, time.Time, error) {
	from, e1 := time.Parse(time.RFC3339Nano, c.Query("from"))
	to, e2 := time.Parse(time.RFC3339Nano, c.Query("to"))
	if e1 != nil || e2 != nil || !from.Before(to) {
		return from, to, domain.Invalid("Informe from e to em RFC 3339, com from anterior a to.")
	}
	return from.UTC(), to.UTC(), nil
}
func (s *Service) orderMetrics(c *gin.Context) {
	from, to, e := interval(c)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	ctx := c.Request.Context()
	var count int64
	var mean *float64
	e = s.DB.QueryRow(ctx, `SELECT count(*),avg(extract(epoch FROM ((data->>'completedAt')::timestamptz-(data->>'executionStartedAt')::timestamptz)))::float8 FROM service_orders WHERE (data->>'completedAt')::timestamptz >= $1 AND (data->>'completedAt')::timestamptz < $2`, from, to).Scan(&count, &mean)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	rows, e := s.DB.Query(ctx, "SELECT status,count(*) FROM service_orders GROUP BY status")
	if e != nil {
		platform.Fail(c, e)
		return
	}
	defer rows.Close()
	counts := map[string]int64{}
	for _, st := range domain.Statuses {
		counts[st] = 0
	}
	for rows.Next() {
		var st string
		var n int64
		if e = rows.Scan(&st, &n); e != nil {
			platform.Fail(c, e)
			return
		}
		counts[st] = n
	}
	if e = rows.Err(); e != nil {
		platform.Fail(c, e)
		return
	}
	c.JSON(200, gin.H{"from": from, "to": to, "completedCount": count, "meanExecutionSeconds": mean, "currentStatusCounts": counts})
}
func (s *Service) serviceMetrics(c *gin.Context) {
	from, to, e := interval(c)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	p, sz, e := platform.Page(c)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	id := c.Query("serviceId")
	if id != "" {
		if e = platform.UUID(id); e != nil {
			platform.Fail(c, e)
			return
		}
	}
	ctx := c.Request.Context()
	var total int64
	e = s.DB.QueryRow(ctx, "SELECT count(*) FROM services WHERE ($1='' OR id::text=$1)", id).Scan(&total)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	rows, e := s.DB.Query(ctx, `
 WITH execution AS (
 SELECT item->>'serviceId' AS service_id,
 extract(epoch FROM ((item->>'completedAt')::timestamptz-(item->>'startedAt')::timestamptz)) AS duration
 FROM service_orders CROSS JOIN LATERAL jsonb_array_elements(data->'services') item
 WHERE item->>'status'='COMPLETED' AND (item->>'completedAt')::timestamptz >= $1 AND (item->>'completedAt')::timestamptz < $2
 )
 SELECT s.id::text,s.data->>'name',count(e.service_id),avg(e.duration)::float8
 FROM services s LEFT JOIN execution e ON e.service_id=s.id::text
 WHERE ($3='' OR s.id::text=$3)
 GROUP BY s.id,s.data->>'name' ORDER BY s.data->>'name',s.id LIMIT $4 OFFSET $5`, from, to, id, sz, (p-1)*sz)
	if e != nil {
		platform.Fail(c, e)
		return
	}
	defer rows.Close()
	data := []any{}
	for rows.Next() {
		var sid, name string
		var count int64
		var mean *float64
		if e = rows.Scan(&sid, &name, &count, &mean); e != nil {
			platform.Fail(c, e)
			return
		}
		data = append(data, gin.H{"serviceId": sid, "serviceName": name, "completedCount": count, "meanExecutionSeconds": mean})
	}
	if e = rows.Err(); e != nil {
		platform.Fail(c, e)
		return
	}
	platform.PageResponse(c, data, total, p, sz)
}
