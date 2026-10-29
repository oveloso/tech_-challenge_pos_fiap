package registry

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"time"
)

type Kind struct {
	Path, Table, IDParam string
	Fields, Required     []string
}

var (
	CustomerKind = Kind{
		Path: "customers", Table: "customers", IDParam: "customerId",
		Fields: []string{"name", "documentType", "document", "email", "phone"}, Required: []string{"name", "documentType", "document"},
	}
	VehicleKind = Kind{
		Path: "vehicles", Table: "vehicles", IDParam: "vehicleId",
		Fields: []string{"customerId", "plate", "brand", "model", "year"}, Required: []string{"customerId", "plate", "brand", "model", "year"},
	}
	CatalogServiceKind = Kind{
		Path: "services", Table: "services", IDParam: "serviceId",
		Fields: []string{"name", "description", "priceCents"}, Required: []string{"name", "priceCents"},
	}
	InventoryItemKind = Kind{
		Path: "inventory-items", Table: "inventory_items", IDParam: "inventoryItemId",
		Fields: []string{"sku", "name", "type", "priceCents", "initialQuantity"}, Required: []string{"sku", "name", "type", "priceCents"},
	}
	Kinds = []Kind{CustomerKind, VehicleKind, CatalogServiceKind, InventoryItemKind}
)

type Service struct{ DB *pgxpool.Pool }

type MovementResult struct {
	Movement platform.Record `json:"movement"`
	Balance  int64           `json:"balance"`
}

func (s *Service) RecordMovement(ctx context.Context, itemID, kind string, delta int64, reason string) (MovementResult, error) {
	if (kind != "REPLENISHMENT" && kind != "ADJUSTMENT") || (kind == "REPLENISHMENT" && delta <= 0) || delta == 0 {
		return MovementResult{}, domain.Invalid("Tipo ou quantidade de movimento inválidos.")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return MovementResult{}, err
	}
	defer tx.Rollback(ctx)
	item, err := platform.Get(ctx, tx, "inventory_items", itemID, true)
	if err != nil {
		return MovementResult{}, err
	}
	if !platform.Active(item) {
		return MovementResult{}, domain.Conflict("INACTIVE_RECORD", "Reative o item antes de registrar movimentos manuais.")
	}
	balance, err := domain.AdjustStock(platform.Number(item, "balance"), delta)
	if err != nil {
		return MovementResult{}, err
	}
	now := time.Now().UTC()
	item["balance"] = balance
	item["updatedAt"] = now
	if err = platform.Save(ctx, tx, "inventory_items", item, false); err != nil {
		return MovementResult{}, err
	}
	movement, err := InsertMovement(ctx, tx, itemID, "", kind, delta, reason, now)
	if err != nil {
		return MovementResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MovementResult{}, platform.MapError(err)
	}
	return MovementResult{Movement: movement, Balance: balance}, nil
}

func (s *Service) Write(ctx context.Context, k Kind, id string, input platform.Record, create bool) (platform.Record, error) {
	if e := validate(k.Table, input); e != nil {
		return nil, e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	r := platform.Record{}
	if create {
		r["id"] = uuid.NewString()
		r["createdAt"] = now
		r["active"] = true
		switch k.Table {
		case "customers":
			r["email"] = nil
			r["phone"] = nil
		case "services":
			r["description"] = nil
		case "inventory_items":
			r["balance"] = int64(0)
		}
	} else {
		r, e = platform.Get(ctx, tx, k.Table, id, true)
		if e != nil {
			return nil, e
		}
	}
	previousCustomer := platform.Text(r, "customerId")
	for key, v := range input {
		if key != "initialQuantity" {
			r[key] = v
		}
	}
	r["updatedAt"] = now
	if k.Table == "vehicles" && (create || previousCustomer != platform.Text(r, "customerId")) {
		customer, e := platform.Get(ctx, tx, "customers", platform.Text(r, "customerId"), true)
		if e != nil {
			return nil, e
		}
		if !platform.Active(customer) {
			return nil, domain.Conflict("INACTIVE_RECORD", "Cliente está inativo.")
		}
		if !create {
			var open bool
			if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM service_orders WHERE vehicle_id=$1 AND status <> 'DELIVERED')", id).Scan(&open); e != nil {
				return nil, e
			}
			if open {
				return nil, domain.Conflict("VEHICLE_HAS_OPEN_ORDERS", "Veículo possui OS ainda não entregue.")
			}
		}
	}
	if create && k.Table == "inventory_items" {
		r["balance"] = platform.Number(input, "initialQuantity")
	}
	if e = platform.Save(ctx, tx, k.Table, r, create); e != nil {
		return nil, e
	}
	if create && k.Table == "inventory_items" && platform.Number(r, "balance") > 0 {
		_, e = InsertMovement(ctx, tx, platform.Text(r, "id"), "", "OPENING", platform.Number(r, "balance"), "Saldo inicial", now)
		if e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, platform.MapError(e)
	}
	return r, nil
}
