package workorders

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOrderRequestsRejectInvalidInput(t *testing.T) {
	const id = "66b19196-8826-4a60-a22f-3478234adad8"
	validCreate := `{"customerId":"` + id + `","vehicleId":"` + id + `","problemDescription":" Freios ","serviceIds":[],"parts":[]}`
	for _, tc := range []struct {
		name, kind, body, contentType string
		valid                         bool
	}{
		{"create", "create", validCreate, "application/json", true},
		{"missing_customer", "create", strings.Replace(validCreate, `"customerId":"`+id+`",`, "", 1), "application/json", false},
		{"null_customer", "create", strings.Replace(validCreate, `"customerId":"`+id+`"`, `"customerId":null`, 1), "application/json", false},
		{"invalid_vehicle", "create", strings.Replace(validCreate, `"vehicleId":"`+id+`"`, `"vehicleId":"invalid"`, 1), "application/json", false},
		{"null_problem", "create", strings.Replace(validCreate, `"problemDescription":" Freios "`, `"problemDescription":null`, 1), "application/json", false},
		{"blank_problem", "create", strings.Replace(validCreate, `" Freios "`, `" "`, 1), "application/json", false},
		{"missing_services", "create", strings.Replace(validCreate, `"serviceIds":[],`, "", 1), "application/json", false},
		{"null_parts", "create", strings.Replace(validCreate, `"parts":[]`, `"parts":null`, 1), "application/json", false},
		{"extra_total", "create", strings.Replace(validCreate, `"parts":[]`, `"parts":[],"totalCents":1`, 1), "application/json", false},
		{"version", "version", `{"expectedVersion":1}`, "application/json", true},
		{"null_version", "version", `{"expectedVersion":null}`, "application/json", false},
		{"missing_version", "version", `{}`, "application/json", false},
		{"fractional_version", "version", `{"expectedVersion":1.5}`, "application/json", false},
		{"overflow_version", "version", `{"expectedVersion":9223372036854775808}`, "application/json", false},
		{"unexpected_selection", "version", `{"expectedVersion":1,"parts":[]}`, "application/json", false},
		{"replace_items", "items", `{"expectedVersion":2,"serviceIds":[],"parts":[]}`, "application/json", true},
		{"replace_without_version", "items", `{"serviceIds":[],"parts":[]}`, "application/json", false},
		{"replace_without_parts", "items", `{"expectedVersion":2,"serviceIds":[]}`, "application/json", false},
		{"decision", "decision", `{"quoteId":"` + id + `","decision":"APPROVED"}`, "application/json", true},
		{"decision_without_quote", "decision", `{"decision":"APPROVED"}`, "application/json", false},
		{"decision_null", "decision", `{"quoteId":"` + id + `","decision":null}`, "application/json", false},
		{"decision_unknown", "decision", `{"quoteId":"` + id + `","decision":"OTHER"}`, "application/json", false},
		{"null_body", "create", `null`, "application/json", false},
		{"array_body", "create", `[]`, "application/json", false},
		{"malformed_json", "create", `{`, "application/json", false},
		{"multiple_values", "create", validCreate + ` {}`, "application/json", false},
		{"wrong_content_type", "create", validCreate, "text/plain", false},
		{"too_large", "create", strings.Replace(validCreate, ` Freios `, strings.Repeat("a", 1<<20), 1), "application/json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", tc.contentType)
			var request interface{ validate() error }
			switch tc.kind {
			case "create":
				request = &createOrderRequest{}
			case "version":
				request = &versionRequest{}
			case "items":
				request = &replaceItemsRequest{}
			case "decision":
				request = &quoteDecisionRequest{}
			}
			err := decodeOrderRequest(c, request)
			if err == nil {
				err = request.validate()
			}
			if (err == nil) != tc.valid {
				t.Fatalf("expected valid=%v, got %v", tc.valid, err)
			}
			if tc.valid && tc.kind == "create" && request.(*createOrderRequest).ProblemDescription != "Freios" {
				t.Fatal("problem description was not trimmed")
			}
		})
	}
}
