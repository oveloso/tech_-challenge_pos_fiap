package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var instant = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func fresh() *Order {
	return &Order{ID: "order", Number: 1, CustomerID: "private-customer", CustomerName: "Private Name", VehicleID: "private-vehicle", Vehicle: VehicleSnapshot{Plate: "DEM1A23", Brand: "Demo", Model: "Demo", Year: 2024}, Status: Received, Version: 1, CreatedAt: instant, Services: []OrderService{{ID: "execution", ServiceID: "service", Name: "Repair", Status: "NOT_STARTED"}}, Parts: []OrderPart{{InventoryItemID: "part", Name: "Part", SKU: "PRIVATE-SKU", Quantity: 2}}, Quotes: []Quote{}, History: []Event{}}
}
func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func published(t *testing.T) *Order {
	t.Helper()
	o := fresh()
	must(t, o.StartDiagnosis(instant))
	must(t, o.Publish("quote", []QuoteService{{OrderServiceID: "execution", ServiceID: "service", Name: "Repair", Price: 100}}, []QuotePart{{InventoryItemID: "part", Name: "Part", Quantity: 2, UnitPrice: 50}}, instant))
	return o
}
func approved(t *testing.T) *Order {
	t.Helper()
	o := published(t)
	changed, e := o.Decide("quote", "APPROVED", instant)
	must(t, e)
	if !changed {
		t.Fatal("first approval must change state")
	}
	return o
}
func TestFullOrderLifecycle(t *testing.T) {
	o := approved(t)
	if o.Status != Awaiting {
		t.Fatal(o.Status)
	}
	first, e := o.StartService("execution", instant.Add(time.Minute))
	must(t, e)
	if !first || o.StockConsumedAt == nil || o.Status != Executing {
		t.Fatal("first service must consume stock")
	}
	if e = o.Complete(instant); e == nil {
		t.Fatal("unfinished service accepted")
	}
	must(t, o.CompleteService("execution", instant.Add(3*time.Minute)))
	must(t, o.Complete(instant.Add(4*time.Minute)))
	must(t, o.Deliver(instant.Add(5*time.Minute)))
	if o.Status != Delivered || o.CompletedAt == nil || o.DeliveredAt == nil || o.Version != 8 {
		t.Fatalf("unexpected result %+v", o)
	}
	if len(o.History) != 7 {
		t.Fatal(o.History)
	}
}
func TestQuoteSnapshotsAndOverflow(t *testing.T) {
	o := fresh()
	must(t, o.StartDiagnosis(instant))
	sv := []QuoteService{{Name: "Original", Price: 100}}
	parts := []QuotePart{{Name: "Original part", Quantity: 2, UnitPrice: 30}}
	must(t, o.Publish("q", sv, parts, instant))
	sv[0].Name = "Edited"
	sv[0].Price = 200
	parts[0].Name = "Edited"
	parts[0].UnitPrice = 90
	if o.CurrentQuote.Total != 160 || o.CurrentQuote.Services[0].Name != "Original" || o.CurrentQuote.Parts[0].Subtotal != 60 {
		t.Fatal("snapshot changed")
	}
	for _, tc := range []struct {
		services []QuoteService
		parts    []QuotePart
	}{{nil, nil}, {[]QuoteService{{Price: MaxMoney}, {Price: 1}}, nil}, {[]QuoteService{{Price: 1}}, []QuotePart{{Quantity: 2, UnitPrice: MaxMoney}}}, {[]QuoteService{{Price: MaxMoney}}, []QuotePart{{Quantity: 1, UnitPrice: 1}}}} {
		o = fresh()
		must(t, o.StartDiagnosis(instant))
		before, _ := json.Marshal(o)
		if o.Publish("q", tc.services, tc.parts, instant) == nil {
			t.Fatal("invalid quote accepted")
		}
		after, _ := json.Marshal(o)
		if string(before) != string(after) {
			t.Fatal("failed quote changed order")
		}
	}
}
func TestDecisionIdempotenceAndRevision(t *testing.T) {
	o := published(t)
	if _, e := o.Decide("unknown-quote", "APPROVED", instant); e == nil || e.(*Error).Status != 404 {
		t.Fatal("quote outside this order must be hidden")
	}
	changed, e := o.Decide("quote", "REJECTED", instant)
	must(t, e)
	if !changed || o.Status != Diagnosis {
		t.Fatal("rejection must reopen diagnosis")
	}
	v := o.Version
	changed, e = o.Decide("quote", "REJECTED", instant)
	must(t, e)
	if changed || o.Version != v {
		t.Fatal("duplicate rejection changed order")
	}
	if _, e = o.Decide("quote", "APPROVED", instant); e == nil {
		t.Fatal("opposite decision accepted")
	}
	must(t, o.Publish("q2", []QuoteService{{Price: 200}}, nil, instant))
	if _, e = o.Decide("quote", "REJECTED", instant); e == nil {
		t.Fatal("old decision accepted")
	}
	_, e = o.Decide("q2", "APPROVED", instant)
	must(t, e)
	must(t, o.Reopen(instant))
	if o.CurrentQuote.Status != "SUPERSEDED" || o.Status != Diagnosis {
		t.Fatal("reopen did not supersede")
	}
	if _, e = o.Decide("q2", "APPROVED", instant); e == nil {
		t.Fatal("superseded decision accepted")
	}
	must(t, o.Publish("q3", []QuoteService{{Price: 300}}, nil, instant))
	if o.CurrentQuote.Version != 3 || o.Quotes[0].Status != "REJECTED" || o.Quotes[1].Status != "SUPERSEDED" {
		t.Fatal("history lost")
	}
}
func TestServiceConcurrencyRules(t *testing.T) {
	o := approved(t)
	o.Services = append(o.Services, OrderService{ID: "second", ServiceID: "other", Status: "NOT_STARTED"})
	first, e := o.StartService("execution", instant)
	must(t, e)
	if !first {
		t.Fatal("first service")
	}
	first, e = o.StartService("second", instant)
	must(t, e)
	if first {
		t.Fatal("stock consumed twice")
	}
	if _, e = o.StartService("execution", instant); e == nil {
		t.Fatal("duplicate start accepted")
	}
	must(t, o.CompleteService("execution", instant))
	if e = o.CompleteService("execution", instant); e == nil {
		t.Fatal("duplicate completion accepted")
	}
	if e = o.Complete(instant); e == nil {
		t.Fatal("second service incomplete")
	}
	must(t, o.CompleteService("second", instant))
	must(t, o.Complete(instant))
}
func TestInvalidTransitions(t *testing.T) {
	o := fresh()
	if e := o.CheckVersion(0); e == nil {
		t.Fatal("version zero")
	}
	if e := o.CheckVersion(2); e == nil {
		t.Fatal("stale version")
	}
	must(t, o.CheckVersion(1))
	must(t, o.Editable())
	if e := o.Reopen(instant); e == nil {
		t.Fatal("reopen received")
	}
	if e := o.Publish("q", []QuoteService{{Price: 1}}, nil, instant); e == nil {
		t.Fatal("publish received")
	}
	if _, e := o.Decide("q", "OTHER", instant); e == nil {
		t.Fatal("invalid decision")
	}
	if _, e := o.Decide("q", "APPROVED", instant); e == nil {
		t.Fatal("no quote")
	}
	if _, e := o.StartService("execution", instant); e == nil {
		t.Fatal("start received")
	}
	if e := o.CompleteService("execution", instant); e == nil {
		t.Fatal("complete received service")
	}
	if e := o.Complete(instant); e == nil {
		t.Fatal("complete received")
	}
	if e := o.Deliver(instant); e == nil {
		t.Fatal("deliver received")
	}
	o = published(t)
	if e := o.StartDiagnosis(instant); e == nil {
		t.Fatal("diagnosis already started")
	}
	if e := o.Editable(); e == nil {
		t.Fatal("edit published")
	}
	if _, e := o.StartService("execution", instant); e == nil {
		t.Fatal("unapproved start")
	}
	must(t, o.Reopen(instant))
	o = approved(t)
	if _, e := o.StartService("missing", instant); e == nil {
		t.Fatal("missing service")
	}
	_, e := o.StartService("execution", instant)
	must(t, e)
	if e = o.Reopen(instant); e == nil {
		t.Fatal("reopen execution")
	}
	if e = o.CompleteService("missing", instant); e == nil {
		t.Fatal("missing completion")
	}
	o = fresh()
	o.Status = Awaiting
	if e = o.Reopen(instant); e == nil {
		t.Fatal("reopen without quote")
	}
	if _, e = o.StartService("execution", instant); e == nil {
		t.Fatal("start without quote")
	}
	o = published(t)
	o.Status = Executing
	if _, e = o.Decide("quote", "APPROVED", instant); e == nil {
		t.Fatal("decision wrong state")
	}
}
func TestCustomerProjectionExcludesAdministrativeData(t *testing.T) {
	o := published(t)
	data, e := json.Marshal(o.CustomerView())
	must(t, e)
	s := string(data)
	for _, private := range []string{"private-customer", "private-vehicle", "Private Name", "PRIVATE-SKU", "inventoryItemId", "serviceId", "serviceOrderServiceId", "customerId", "customerName", "problemDescription", "stockConsumedAt", "\"quotes\""} {
		if strings.Contains(s, private) {
			t.Fatalf("projection leaked %s: %s", private, s)
		}
	}
	if !strings.Contains(s, "Repair") || !strings.Contains(s, "priceCents") {
		t.Fatal("missing quote")
	}
	o = fresh()
	if o.CustomerView()["currentQuote"] != nil {
		t.Fatal("expected null quote")
	}
}
