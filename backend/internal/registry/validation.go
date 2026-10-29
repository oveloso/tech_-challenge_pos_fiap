package registry

import (
	"math"
	"net/mail"
	"oficina/internal/domain"
	"oficina/internal/platform"
	"strings"
)

func validate(kind string, r platform.Record) error {
	for k, v := range r {
		if v == nil && k != "description" && k != "email" && k != "phone" {
			return domain.Invalid(k + " não pode ser nulo.")
		}
	}
	for _, f := range []struct {
		k        string
		min, max int
		n        bool
	}{{"name", 1, 160, false}, {"document", 1, 32, false}, {"documentType", 1, 4, false}, {"email", 1, 254, true}, {"phone", 0, 32, true}, {"plate", 1, 16, false}, {"brand", 1, 80, false}, {"model", 1, 120, false}, {"description", 0, 2000, true}, {"sku", 1, 64, false}, {"type", 1, 6, false}} {
		if e := platform.String(r, f.k, f.min, f.max, f.n); e != nil {
			return e
		}
	}
	if v, ok := r["active"]; ok {
		if _, ok = v.(bool); !ok {
			return domain.Invalid("active deve ser booleano.")
		}
	}
	if e := platform.Integer(r, "priceCents", 0, domain.MaxMoney); e != nil {
		return e
	}
	if e := platform.Integer(r, "initialQuantity", 0, math.MaxInt64); e != nil {
		return e
	}
	if e := platform.Integer(r, "year", 1886, 9999); e != nil {
		return e
	}
	if e := platform.ID(r, "customerId"); e != nil {
		return e
	}
	switch kind {
	case "customers":
		_, dt := r["documentType"]
		_, doc := r["document"]
		if dt != doc {
			return domain.Invalid("Envie documentType e document juntos.")
		}
		if dt {
			d, e := domain.Document(platform.Text(r, "documentType"), platform.Text(r, "document"))
			if e != nil {
				return e
			}
			r["document"] = d
		}
		if email, ok := r["email"].(string); ok {
			a, e := mail.ParseAddress(email)
			if e != nil || a.Address != email {
				return domain.Invalid("Email inválido.")
			}
		}
	case "vehicles":
		if _, ok := r["plate"]; ok {
			p, e := domain.Plate(platform.Text(r, "plate"))
			if e != nil {
				return e
			}
			r["plate"] = p
		}
	case "inventory_items":
		if t, ok := r["type"]; ok && t != "PART" && t != "SUPPLY" {
			return domain.Invalid("Tipo deve ser PART ou SUPPLY.")
		}
		if sku, ok := r["sku"].(string); ok {
			r["sku"] = strings.ToUpper(sku)
		}
	}
	return nil
}
