package workorders

import (
	"context"
	"encoding/json"
	"oficina/internal/domain"
	"oficina/internal/platform"
)

func Load(ctx context.Context, db platform.DB, id string, lock bool) (*domain.Order, error) {
	query := "SELECT data FROM service_orders WHERE id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	var raw []byte
	if e := db.QueryRow(ctx, query, id).Scan(&raw); e != nil {
		return nil, platform.MapError(e)
	}
	var o domain.Order
	if e := json.Unmarshal(raw, &o); e != nil {
		return nil, e
	}
	return &o, nil
}
func LoadCustomer(ctx context.Context, db platform.DB, hash []byte, lock bool) (*domain.Order, error) {
	query := "SELECT data FROM service_orders WHERE token_hash=$1 AND token_expires_at>now()"
	if lock {
		query += " FOR UPDATE"
	}
	var raw []byte
	if e := db.QueryRow(ctx, query, hash).Scan(&raw); e != nil {
		if de, ok := platform.MapError(e).(*domain.Error); ok && de.Status == 404 {
			return nil, domain.Unauthorized()
		}
		return nil, e
	}
	var o domain.Order
	if e := json.Unmarshal(raw, &o); e != nil {
		return nil, e
	}
	return &o, nil
}
func Save(ctx context.Context, db platform.DB, o *domain.Order, create bool) error {
	raw, e := json.Marshal(o)
	if e != nil {
		return e
	}
	if create {
		_, e = db.Exec(ctx, "INSERT INTO service_orders(id,data) VALUES($1,$2)", o.ID, raw)
	} else {
		_, e = db.Exec(ctx, "UPDATE service_orders SET data=$2 WHERE id=$1", o.ID, raw)
	}
	return platform.MapError(e)
}
