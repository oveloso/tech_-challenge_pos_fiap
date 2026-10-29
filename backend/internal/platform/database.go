package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"oficina/internal/domain"
	"time"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Record map[string]any

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	c, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, e
	}
	c.MaxConns = 12
	c.MinConns = 1
	c.MaxConnLifetime = time.Hour
	c.ConnConfig.ConnectTimeout = 5 * time.Second
	c.ConnConfig.RuntimeParams["timezone"] = "UTC"
	c.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	p, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return p, nil
}
func Decode(data []byte) (Record, error) {
	var r Record
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	e := d.Decode(&r)
	return r, e
}
func Get(ctx context.Context, db DB, table, id string, lock bool) (Record, error) {
	q := "SELECT data FROM " + table + " WHERE id=$1"
	if lock {
		q += " FOR UPDATE"
	}
	var raw []byte
	if e := db.QueryRow(ctx, q, id).Scan(&raw); e != nil {
		return nil, MapError(e)
	}
	return Decode(raw)
}
func Save(ctx context.Context, db DB, table string, r Record, create bool) error {
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if create {
		_, e = db.Exec(ctx, "INSERT INTO "+table+"(id,data) VALUES($1,$2)", r["id"], raw)
	} else {
		_, e = db.Exec(ctx, "UPDATE "+table+" SET data=$2 WHERE id=$1", r["id"], raw)
	}
	return MapError(e)
}
func Text(r Record, k string) string { s, _ := r[k].(string); return s }
func Number(r Record, k string) int64 {
	switch v := r[k].(type) {
	case json.Number:
		n, _ := v.Int64()
		return n
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}
func Active(r Record) bool { b, _ := r["active"].(bool); return b }
func MapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Missing()
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			code := "DUPLICATE_RESOURCE"
			switch p.ConstraintName {
			case "customers_document_key":
				code = "DUPLICATE_DOCUMENT"
			case "vehicles_plate_key":
				code = "DUPLICATE_PLATE"
			case "inventory_items_sku_key":
				code = "DUPLICATE_SKU"
			}
			return domain.Conflict(code, "Já existe um cadastro com este identificador.")
		case "23503":
			return domain.Invalid("Referência de cadastro inválida.")
		case "40001", "40P01":
			return domain.Conflict("VERSION_CONFLICT", "Operação concorrente. Atualize e tente novamente.")
		}
	}
	return err
}
