package registry

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"sort"
	"time"
)

func InsertMovement(ctx context.Context, db platform.DB, itemID, orderID, kind string, delta int64, reason string, now time.Time) (platform.Record, error) {
	var order any
	if orderID != "" {
		order = orderID
	}
	r := platform.Record{"id": uuid.NewString(), "inventoryItemId": itemID, "type": kind, "quantityDelta": delta, "reason": reason, "occurredAt": now, "serviceOrderId": order}
	raw, e := json.Marshal(r)
	if e != nil {
		return nil, e
	}
	_, e = db.Exec(ctx, "INSERT INTO inventory_movements(id,inventory_item_id,service_order_id,data) VALUES($1,$2,$3,$4)", r["id"], itemID, order, raw)
	return r, platform.MapError(e)
}

func LoadActive(ctx context.Context, db platform.DB, table string, ids []string) (map[string]platform.Record, error) {
	ordered := append([]string{}, ids...)
	sort.Strings(ordered)
	out := map[string]platform.Record{}
	for _, id := range ordered {
		if _, ok := out[id]; ok {
			return nil, domain.Invalid("IDs duplicados na seleção.")
		}
		r, e := platform.Get(ctx, db, table, id, true)
		if e != nil {
			return nil, e
		}
		if !platform.Active(r) {
			return nil, domain.Conflict("INACTIVE_RECORD", "Seleção contém cadastro inativo.")
		}
		out[id] = r
	}
	return out, nil
}
