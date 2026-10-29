package registry

import (
	"oficina/internal/platform"
	"testing"
)

func TestRegistryValidation(t *testing.T) {
	for _, tc := range []struct {
		kind, raw string
		valid     bool
	}{
		{"customers", `{"name":"Cliente","documentType":"CPF","document":"529.982.247-25","email":"cliente@example.com","phone":null}`, true},
		{"customers", `{"document":"52998224725"}`, false}, {"customers", `{"documentType":"CPF"}`, false}, {"customers", `{"documentType":"CPF","document":"11111111111"}`, false}, {"customers", `{"email":"Pessoa <p@example.com>"}`, false}, {"customers", `{"name":null}`, false}, {"customers", `{"name":" "}`, false}, {"customers", `{"active":"true"}`, false},
		{"vehicles", `{"customerId":"66B19196-8826-4A60-A22F-3478234ADAD8","plate":"ABC-1234","brand":"Marca","model":"Modelo","year":2020}`, true},
		{"vehicles", `{"plate":"ABC"}`, false}, {"vehicles", `{"year":1800}`, false}, {"vehicles", `{"customerId":"invalid"}`, false},
		{"services", `{"name":"Serviço","description":null,"priceCents":0,"active":true}`, true}, {"services", `{"priceCents":9007199254740992}`, false}, {"services", `{"priceCents":1.5}`, false},
		{"inventory_items", `{"sku":" demo ","name":"Peça","type":"PART","priceCents":1,"initialQuantity":5}`, true}, {"inventory_items", `{"type":"OTHER"}`, false}, {"inventory_items", `{"initialQuantity":-1}`, false},
	} {
		r, e := platform.Decode([]byte(tc.raw))
		if e != nil {
			t.Fatal(e)
		}
		e = validate(tc.kind, r)
		if (e == nil) != tc.valid {
			t.Errorf("%s %s: %v", tc.kind, tc.raw, e)
		}
	}
}
