package workorders

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"oficina/internal/auth"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"oficina/internal/registry"
	"sort"
	"strings"
	"time"
)

type Service struct {
	DB       *pgxpool.Pool
	TokenTTL time.Duration
}
type PartSelection struct {
	InventoryItemID string `json:"inventoryItemId"`
	Quantity        int64  `json:"quantity"`
}
type Selection struct {
	ServiceIDs []string        `json:"serviceIds"`
	Parts      []PartSelection `json:"parts"`
}

func (s *Selection) Validate() error {
	if s.ServiceIDs == nil || s.Parts == nil {
		return domain.Invalid("serviceIds e parts devem ser listas.")
	}
	if len(s.ServiceIDs) > 200 || len(s.Parts) > 200 {
		return domain.Invalid("Limite de 200 itens por lista.")
	}
	seen := map[string]bool{}
	for i, id := range s.ServiceIDs {
		if e := platform.UUID(id); e != nil {
			return e
		}
		id = strings.ToLower(id)
		s.ServiceIDs[i] = id
		if seen[id] {
			return domain.Invalid("Serviço duplicado.")
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for i, p := range s.Parts {
		if e := platform.UUID(p.InventoryItemID); e != nil {
			return e
		}
		p.InventoryItemID = strings.ToLower(p.InventoryItemID)
		s.Parts[i] = p
		if p.Quantity < 1 || seen[p.InventoryItemID] {
			return domain.Invalid("Peças devem ter quantidade positiva e IDs distintos.")
		}
		seen[p.InventoryItemID] = true
	}
	return nil
}

func applySelection(ctx context.Context, tx pgx.Tx, o *domain.Order, s Selection) error {
	if e := o.Editable(); e != nil {
		return e
	}
	services, e := registry.LoadActive(ctx, tx, "services", s.ServiceIDs)
	if e != nil {
		return e
	}
	ids := []string{}
	for _, p := range s.Parts {
		ids = append(ids, p.InventoryItemID)
	}
	parts, e := registry.LoadActive(ctx, tx, "inventory_items", ids)
	if e != nil {
		return e
	}
	old := map[string]domain.OrderService{}
	for _, sv := range o.Services {
		old[sv.ServiceID] = sv
	}
	next := []domain.OrderService{}
	for _, id := range s.ServiceIDs {
		sv, ok := old[id]
		if !ok {
			sv = domain.OrderService{ID: uuid.NewString(), ServiceID: id, Status: "NOT_STARTED"}
		}
		sv.Name = platform.Text(services[id], "name")
		next = append(next, sv)
	}
	o.Services = next
	o.Parts = []domain.OrderPart{}
	for _, p := range s.Parts {
		r := parts[p.InventoryItemID]
		o.Parts = append(o.Parts, domain.OrderPart{InventoryItemID: p.InventoryItemID, SKU: platform.Text(r, "sku"), Name: platform.Text(r, "name"), Quantity: p.Quantity})
	}
	return nil
}
func (s *Service) Create(ctx context.Context, customerID, vehicleID, problem string, selection Selection) (*domain.Order, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	v, e := platform.Get(ctx, tx, "vehicles", vehicleID, true)
	if e != nil {
		return nil, e
	}
	customer, e := platform.Get(ctx, tx, "customers", customerID, true)
	if e != nil {
		return nil, e
	}
	if !platform.Active(v) || !platform.Active(customer) {
		return nil, domain.Conflict("INACTIVE_RECORD", "Cliente e veículo devem estar ativos.")
	}
	if platform.Text(v, "customerId") != customerID {
		return nil, domain.Conflict("VEHICLE_CUSTOMER_MISMATCH", "Veículo não pertence ao cliente informado.")
	}
	now := time.Now().UTC()
	o := &domain.Order{ID: uuid.NewString(), CustomerID: customerID, CustomerName: platform.Text(customer, "name"), VehicleID: vehicleID, VehiclePlate: platform.Text(v, "plate"), Status: domain.Received, Version: 1, CreatedAt: now, Problem: problem, Vehicle: domain.VehicleSnapshot{Plate: platform.Text(v, "plate"), Brand: platform.Text(v, "brand"), Model: platform.Text(v, "model"), Year: platform.Number(v, "year")}, Quotes: []domain.Quote{}, History: []domain.Event{{Type: "ORDER_RECEIVED", Status: domain.Received, OccurredAt: now}}}
	if e = applySelection(ctx, tx, o, selection); e != nil {
		return nil, e
	}
	if e = tx.QueryRow(ctx, "SELECT nextval('service_order_number')").Scan(&o.Number); e != nil {
		return nil, e
	}
	if e = Save(ctx, tx, o, true); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, platform.MapError(e)
	}
	return o, nil
}
func publish(ctx context.Context, tx pgx.Tx, o *domain.Order, now time.Time) error {
	ids := []string{}
	for _, sv := range o.Services {
		ids = append(ids, sv.ServiceID)
	}
	services, e := registry.LoadActive(ctx, tx, "services", ids)
	if e != nil {
		return e
	}
	ids = []string{}
	for _, p := range o.Parts {
		ids = append(ids, p.InventoryItemID)
	}
	parts, e := registry.LoadActive(ctx, tx, "inventory_items", ids)
	if e != nil {
		return e
	}
	qs := []domain.QuoteService{}
	for i := range o.Services {
		sv := &o.Services[i]
		r := services[sv.ServiceID]
		sv.Name = platform.Text(r, "name")
		qs = append(qs, domain.QuoteService{OrderServiceID: sv.ID, ServiceID: sv.ServiceID, Name: sv.Name, Price: platform.Number(r, "priceCents")})
	}
	qp := []domain.QuotePart{}
	for i := range o.Parts {
		p := &o.Parts[i]
		r := parts[p.InventoryItemID]
		p.Name = platform.Text(r, "name")
		p.SKU = platform.Text(r, "sku")
		qp = append(qp, domain.QuotePart{InventoryItemID: p.InventoryItemID, Name: p.Name, Quantity: p.Quantity, UnitPrice: platform.Number(r, "priceCents")})
	}
	return o.Publish(uuid.NewString(), qs, qp, now)
}
func consume(ctx context.Context, tx pgx.Tx, o *domain.Order, now time.Time) error {
	parts := append([]domain.OrderPart{}, o.Parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].InventoryItemID < parts[j].InventoryItemID })
	for _, p := range parts {
		r, e := platform.Get(ctx, tx, "inventory_items", p.InventoryItemID, true)
		if e != nil {
			return e
		}
		balance, e := domain.AdjustStock(platform.Number(r, "balance"), -p.Quantity)
		if e != nil {
			return e
		}
		r["balance"] = balance
		r["updatedAt"] = now
		if e = platform.Save(ctx, tx, "inventory_items", r, false); e != nil {
			return e
		}
		if _, e = registry.InsertMovement(ctx, tx, p.InventoryItemID, o.ID, "SERVICE_ORDER_CONSUMPTION", -p.Quantity, "Consumo de peças da OS", now); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) Execute(ctx context.Context, id string, version int64, action, serviceID string, selection Selection) (any, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	o, e := Load(ctx, tx, id, true)
	if e != nil {
		return nil, e
	}
	if e = o.CheckVersion(version); e != nil {
		return nil, e
	}
	now := time.Now().UTC()
	var result any
	switch action {
	case "items":
		e = applySelection(ctx, tx, o, selection)
		if e == nil {
			o.Version++
		}
	case "diagnosis/start":
		e = o.StartDiagnosis(now)
	case "diagnosis/reopen":
		e = o.Reopen(now)
	case "quotes":
		e = publish(ctx, tx, o, now)
	case "complete":
		e = o.Complete(now)
	case "deliver":
		e = o.Deliver(now)
	case "services/start":
		var first bool
		first, e = o.StartService(serviceID, now)
		if e == nil && first {
			e = consume(ctx, tx, o, now)
		}
	case "services/complete":
		e = o.CompleteService(serviceID, now)
	case "customer-access":
		var token string
		var hash []byte
		token, hash, e = auth.NewCustomerToken()
		if e == nil {
			expires := now.Add(s.TokenTTL)
			_, e = tx.Exec(ctx, "UPDATE service_orders SET token_hash=$2,token_expires_at=$3 WHERE id=$1", o.ID, hash, expires)
			o.Version++
			result = map[string]any{"token": token, "expiresAt": expires, "workOrderVersion": o.Version}
		}
	default:
		e = domain.Missing()
	}
	if e != nil {
		return nil, e
	}
	if e = Save(ctx, tx, o, false); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, platform.MapError(e)
	}
	if result == nil {
		result = o
	}
	return result, nil
}
func (s *Service) Decide(ctx context.Context, hash []byte, quoteID, decision string) (*domain.Order, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	o, e := LoadCustomer(ctx, tx, hash, true)
	if e != nil {
		return nil, e
	}
	changed, e := o.Decide(quoteID, decision, time.Now().UTC())
	if e != nil {
		return nil, e
	}
	if changed {
		if e = Save(ctx, tx, o, false); e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, platform.MapError(e)
	}
	return o, nil
}
