package workorders

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSelectionValidation(t *testing.T) {
	id := "66b19196-8826-4a60-a22f-3478234adad8"
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"serviceIds":[],"parts":[]}`, true}, {`{"serviceIds":["` + id + `"],"parts":[{"inventoryItemId":"` + id + `","quantity":1}]}`, true}, {`{"serviceIds":["` + id + `","` + strings.ToUpper(id) + `"],"parts":[]}`, false}, {`{"serviceIds":null,"parts":[]}`, false}, {`{"serviceIds":[],"parts":null}`, false}, {`{"serviceIds":["bad"],"parts":[]}`, false}, {`{"serviceIds":[],"parts":[{"inventoryItemId":"bad","quantity":1}]}`, false}, {`{"serviceIds":[],"parts":[{"inventoryItemId":"` + id + `","quantity":0}]}`, false}, {`{"serviceIds":[],"parts":[{"inventoryItemId":"` + id + `","quantity":1,"priceCents":0}]}`, false}, {`{"serviceIds":[],"parts":[{"inventoryItemId":"` + id + `","quantity":1.5}]}`, false}, {`{"serviceIds":[],"parts":[{"inventoryItemId":"` + id + `","quantity":1},{"inventoryItemId":"` + strings.ToUpper(id) + `","quantity":2}]}`, false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(tc.raw))
		c.Request.Header.Set("Content-Type", "application/json")
		var selection Selection
		e := decodeOrderRequest(c, &selection)
		if e == nil {
			e = selection.Validate()
		}
		if (e == nil) != tc.valid {
			t.Errorf("%s: %v", tc.raw, e)
		}
	}
}

func TestSelectionLimitsAndNormalization(t *testing.T) {
	id := "66b19196-8826-4a60-a22f-3478234adad8"
	selection := Selection{ServiceIDs: []string{strings.ToUpper(id)}, Parts: []PartSelection{{InventoryItemID: strings.ToUpper(id), Quantity: 2}}}
	if err := selection.Validate(); err != nil || selection.ServiceIDs[0] != id || selection.Parts[0].InventoryItemID != id {
		t.Fatalf("selection must normalize UUIDs: %v", err)
	}
	selection.ServiceIDs = make([]string, 201)
	if err := selection.Validate(); err == nil {
		t.Fatal("accepted more than 200 services")
	}
	selection.ServiceIDs = []string{}
	selection.Parts = make([]PartSelection, 201)
	if err := selection.Validate(); err == nil {
		t.Fatal("accepted more than 200 parts")
	}
}
