package domain

import "time"

const (
	Received  = "RECEIVED"
	Diagnosis = "IN_DIAGNOSIS"
	Awaiting  = "AWAITING_APPROVAL"
	Executing = "IN_EXECUTION"
	Completed = "COMPLETED"
	Delivered = "DELIVERED"
)

var Statuses = []string{Received, Diagnosis, Awaiting, Executing, Completed, Delivered}

type VehicleSnapshot struct {
	Plate string `json:"plate"`
	Brand string `json:"brand"`
	Model string `json:"model"`
	Year  int64  `json:"year"`
}
type OrderService struct {
	ID          string     `json:"id"`
	ServiceID   string     `json:"serviceId"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`
}
type OrderPart struct {
	InventoryItemID string `json:"inventoryItemId"`
	SKU             string `json:"sku"`
	Name            string `json:"name"`
	Quantity        int64  `json:"quantity"`
}
type QuoteService struct {
	OrderServiceID string `json:"serviceOrderServiceId"`
	ServiceID      string `json:"serviceId"`
	Name           string `json:"name"`
	Price          int64  `json:"priceCents"`
}
type QuotePart struct {
	InventoryItemID string `json:"inventoryItemId"`
	Name            string `json:"name"`
	Quantity        int64  `json:"quantity"`
	UnitPrice       int64  `json:"unitPriceCents"`
	Subtotal        int64  `json:"subtotalCents"`
}
type Quote struct {
	ID          string         `json:"id"`
	Version     int64          `json:"version"`
	Status      string         `json:"status"`
	Services    []QuoteService `json:"services"`
	Parts       []QuotePart    `json:"parts"`
	Total       int64          `json:"totalCents"`
	PublishedAt time.Time      `json:"publishedAt"`
	DecidedAt   *time.Time     `json:"decidedAt"`
	Decision    *string        `json:"decision"`
}
type Event struct {
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	OccurredAt time.Time `json:"occurredAt"`
}
type Order struct {
	ID                 string          `json:"id"`
	Number             int64           `json:"number"`
	CustomerID         string          `json:"customerId"`
	CustomerName       string          `json:"customerName"`
	VehicleID          string          `json:"vehicleId"`
	VehiclePlate       string          `json:"vehiclePlate"`
	Status             string          `json:"status"`
	Version            int64           `json:"version"`
	CreatedAt          time.Time       `json:"createdAt"`
	ExecutionStartedAt *time.Time      `json:"executionStartedAt"`
	CompletedAt        *time.Time      `json:"completedAt"`
	DeliveredAt        *time.Time      `json:"deliveredAt"`
	Problem            string          `json:"problemDescription"`
	Services           []OrderService  `json:"services"`
	Parts              []OrderPart     `json:"parts"`
	CurrentQuote       *Quote          `json:"currentQuote"`
	Quotes             []Quote         `json:"quotes"`
	History            []Event         `json:"history"`
	StockConsumedAt    *time.Time      `json:"stockConsumedAt"`
	Vehicle            VehicleSnapshot `json:"vehicle"`
}

func (o *Order) CheckVersion(v int64) error {
	if v < 1 {
		return Invalid("expectedVersion deve ser positivo.")
	}
	if v != o.Version {
		return Conflict("VERSION_CONFLICT", "A OS foi alterada. Atualize os dados e tente novamente.")
	}
	return nil
}
func (o *Order) Editable() error {
	if o.Status != Received && o.Status != Diagnosis {
		return Conflict("INVALID_STATE", "Itens só podem ser alterados durante recebimento ou diagnóstico.")
	}
	return nil
}
func (o *Order) event(kind string, now time.Time) {
	o.Version++
	o.History = append(o.History, Event{kind, o.Status, now})
}
func (o *Order) syncQuote() {
	if len(o.Quotes) > 0 {
		q := o.Quotes[len(o.Quotes)-1]
		o.CurrentQuote = &q
	}
}
func (o *Order) StartDiagnosis(now time.Time) error {
	if o.Status != Received {
		return Conflict("INVALID_STATE", "A OS precisa estar recebida.")
	}
	o.Status = Diagnosis
	o.event("DIAGNOSIS_STARTED", now)
	return nil
}
func (o *Order) Reopen(now time.Time) error {
	if o.Status != Awaiting || o.ExecutionStartedAt != nil {
		return Conflict("INVALID_STATE", "Diagnóstico não pode ser reaberto neste estado.")
	}
	if len(o.Quotes) == 0 {
		return Conflict("INVALID_STATE", "Não há orçamento vigente.")
	}
	o.Quotes[len(o.Quotes)-1].Status = "SUPERSEDED"
	o.syncQuote()
	o.Status = Diagnosis
	o.event("DIAGNOSIS_REOPENED", now)
	return nil
}
func (o *Order) Publish(id string, services []QuoteService, parts []QuotePart, now time.Time) error {
	if o.Status != Diagnosis {
		return Conflict("INVALID_STATE", "Inicie o diagnóstico antes de publicar o orçamento.")
	}
	if len(services) == 0 {
		return Invalid("Orçamento exige pelo menos um serviço.")
	}
	total := int64(0)
	var err error
	for _, s := range services {
		total, err = AddMoney(total, s.Price)
		if err != nil {
			return err
		}
	}
	frozenParts := append([]QuotePart{}, parts...)
	for i := range frozenParts {
		p := &frozenParts[i]
		p.Subtotal, err = MultiplyMoney(p.UnitPrice, p.Quantity)
		if err != nil {
			return err
		}
		total, err = AddMoney(total, p.Subtotal)
		if err != nil {
			return err
		}
	}
	q := Quote{ID: id, Version: int64(len(o.Quotes) + 1), Status: "PENDING", Services: append([]QuoteService{}, services...), Parts: frozenParts, Total: total, PublishedAt: now}
	o.Quotes = append(o.Quotes, q)
	o.syncQuote()
	o.Status = Awaiting
	o.event("QUOTE_PUBLISHED", now)
	return nil
}
func (o *Order) Decide(id, decision string, now time.Time) (bool, error) {
	if decision != "APPROVED" && decision != "REJECTED" {
		return false, Invalid("Decisão inválida.")
	}
	if len(o.Quotes) == 0 {
		return false, Missing()
	}
	q := &o.Quotes[len(o.Quotes)-1]
	if q.ID != id {
		known := false
		for _, historical := range o.Quotes {
			if historical.ID == id {
				known = true
				break
			}
		}
		if !known {
			return false, Missing()
		}
	}
	if q.ID != id || q.Status == "SUPERSEDED" {
		return false, Conflict("QUOTE_SUPERSEDED", "Este orçamento não é mais vigente.")
	}
	if q.Decision != nil {
		if *q.Decision == decision {
			return false, nil
		}
		return false, Conflict("DECISION_CONFLICT", "Já existe outra decisão para este orçamento.")
	}
	if q.Status != "PENDING" || o.Status != Awaiting {
		return false, Conflict("INVALID_STATE", "Orçamento não aceita decisão.")
	}
	q.Status = decision
	q.Decision = &decision
	q.DecidedAt = &now
	o.syncQuote()
	if decision == "REJECTED" {
		o.Status = Diagnosis
	}
	o.event("QUOTE_"+decision, now)
	return true, nil
}
func (o *Order) StartService(id string, now time.Time) (bool, error) {
	if o.Status != Awaiting && o.Status != Executing {
		return false, Conflict("INVALID_STATE", "OS não está pronta para execução.")
	}
	if o.CurrentQuote == nil || o.CurrentQuote.Status != "APPROVED" {
		return false, Conflict("INVALID_STATE", "Orçamento vigente precisa estar aprovado.")
	}
	for i := range o.Services {
		s := &o.Services[i]
		if s.ID != id {
			continue
		}
		if s.Status != "NOT_STARTED" {
			return false, Conflict("INVALID_STATE", "Serviço já iniciado.")
		}
		first := o.ExecutionStartedAt == nil
		s.Status = "IN_PROGRESS"
		s.StartedAt = &now
		if first {
			o.ExecutionStartedAt = &now
			o.StockConsumedAt = &now
		}
		o.Status = Executing
		o.event("SERVICE_STARTED", now)
		return first, nil
	}
	return false, Missing()
}
func (o *Order) CompleteService(id string, now time.Time) error {
	if o.Status != Executing {
		return Conflict("INVALID_STATE", "OS não está em execução.")
	}
	for i := range o.Services {
		s := &o.Services[i]
		if s.ID != id {
			continue
		}
		if s.Status != "IN_PROGRESS" {
			return Conflict("INVALID_STATE", "Serviço precisa estar em andamento.")
		}
		s.Status = "COMPLETED"
		s.CompletedAt = &now
		o.event("SERVICE_COMPLETED", now)
		return nil
	}
	return Missing()
}
func (o *Order) Complete(now time.Time) error {
	if o.Status != Executing || len(o.Services) == 0 {
		return Conflict("INVALID_STATE", "OS não está em execução.")
	}
	for _, s := range o.Services {
		if s.Status != "COMPLETED" {
			return Conflict("INVALID_STATE", "Conclua todos os serviços antes de finalizar.")
		}
	}
	o.Status = Completed
	o.CompletedAt = &now
	o.event("ORDER_COMPLETED", now)
	return nil
}
func (o *Order) Deliver(now time.Time) error {
	if o.Status != Completed {
		return Conflict("INVALID_STATE", "OS precisa estar finalizada antes da entrega.")
	}
	o.Status = Delivered
	o.DeliveredAt = &now
	o.event("ORDER_DELIVERED", now)
	return nil
}

func (o *Order) CustomerView() map[string]any {
	services := make([]any, 0, len(o.Services))
	for _, s := range o.Services {
		services = append(services, map[string]any{"name": s.Name, "status": s.Status, "startedAt": s.StartedAt, "completedAt": s.CompletedAt})
	}
	parts := make([]any, 0, len(o.Parts))
	for _, p := range o.Parts {
		parts = append(parts, map[string]any{"name": p.Name, "quantity": p.Quantity})
	}
	var quote any
	if q := o.CurrentQuote; q != nil {
		qs := make([]any, 0, len(q.Services))
		for _, s := range q.Services {
			qs = append(qs, map[string]any{"name": s.Name, "priceCents": s.Price})
		}
		qp := make([]any, 0, len(q.Parts))
		for _, p := range q.Parts {
			qp = append(qp, map[string]any{"name": p.Name, "quantity": p.Quantity, "unitPriceCents": p.UnitPrice, "subtotalCents": p.Subtotal})
		}
		quote = map[string]any{"id": q.ID, "version": q.Version, "status": q.Status, "services": qs, "parts": qp, "totalCents": q.Total, "publishedAt": q.PublishedAt, "decidedAt": q.DecidedAt, "decision": q.Decision}
	}
	return map[string]any{"number": o.Number, "vehicle": o.Vehicle, "status": o.Status, "services": services, "parts": parts, "currentQuote": quote, "history": o.History, "createdAt": o.CreatedAt, "executionStartedAt": o.ExecutionStartedAt, "completedAt": o.CompletedAt, "deliveredAt": o.DeliveredAt}
}
