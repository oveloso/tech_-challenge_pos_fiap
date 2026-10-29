package platform

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"oficina/internal/domain"
)

func contextFor(body, contentType, url string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	out := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(out)
	c.Request = httptest.NewRequest("POST", url, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	return c, out
}
func TestBodyRejectsAmbiguousInput(t *testing.T) {
	for _, tc := range []struct {
		body, contentType string
		valid             bool
	}{{`{"name":"x"}`, "application/json", true}, {`{"name":"x"}`, "application/json; charset=utf-8", true}, {`{"name":"x"}`, "text/plain", false}, {`{"name":"x","admin":true}`, "application/json", false}, {`{"name":null}`, "application/json", false}, {`{}`, "application/json", false}, {`null`, "application/json", false}, {`[]`, "application/json", false}, {`{"name":"x"} {}`, "application/json", false}, {`{"name":"` + strings.Repeat("x", 1<<20) + `"}`, "application/json", false}} {
		c, _ := contextFor(tc.body, tc.contentType, "/")
		_, e := Body(c, []string{"name"}, []string{"name"})
		if (e == nil) != tc.valid {
			t.Errorf("body result %v; expected valid=%v", e, tc.valid)
		}
	}
}
func TestScalarValidation(t *testing.T) {
	r := Record{"text": "  ação  ", "money": json.Number("9007199254740991"), "nullable": nil, "id": strings.ToUpper(uuid.NewString())}
	if String(r, "text", 1, 4, false) != nil || Text(r, "text") != "ação" {
		t.Fatal("trim/unicode length")
	}
	if String(r, "nullable", 0, 5, true) != nil || String(r, "absent", 1, 1, false) != nil {
		t.Fatal("optional string")
	}
	if Integer(r, "money", 0, domain.MaxMoney) != nil || Number(r, "money") != domain.MaxMoney {
		t.Fatal("money precision")
	}
	if ID(r, "id") != nil || Text(r, "id") != strings.ToLower(Text(r, "id")) {
		t.Fatal("uuid canonicalization")
	}
	if UUID("") == nil || UUID(uuid.Nil.String()) == nil || ID(Record{}, "id") != nil {
		t.Fatal("uuid validation")
	}
	for _, v := range []any{nil, 42, "1", json.Number("1.1"), json.Number("9223372036854775808"), json.Number("-1")} {
		if Integer(Record{"n": v}, "n", 0, 10) == nil {
			t.Fatalf("accepted invalid integer %v", v)
		}
	}
	if String(Record{"s": nil}, "s", 1, 2, false) == nil || String(Record{"s": "long"}, "s", 1, 2, false) == nil {
		t.Fatal("invalid string")
	}
	if Number(Record{"n": json.Number("99")}, "n") != 99 || Number(Record{"n": 3}, "n") != 3 || Number(Record{}, "n") != 0 || Active(Record{"active": true}) != true {
		t.Fatal("record projection")
	}
}
func TestPaginationAndErrors(t *testing.T) {
	for _, tc := range []struct {
		url   string
		valid bool
	}{{"/", true}, {"/?page=2&pageSize=100", true}, {"/?page=0", false}, {"/?page=no", false}, {"/?pageSize=101", false}, {"/?pageSize=0", false}} {
		c, _ := contextFor("", "", tc.url)
		_, _, e := Page(c)
		if (e == nil) != tc.valid {
			t.Errorf("page %s: %v", tc.url, e)
		}
	}
	c, out := contextFor("", "", "/")
	Fail(c, errors.New("secret SQL password"))
	if out.Code != 500 || strings.Contains(out.Body.String(), "secret") || out.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatal(out.Body.String())
	}
	c, out = contextFor("", "", "/")
	PageResponse(c, []any{}, 0, 1, 20)
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"data":[]`) {
		t.Fatal(out.Body.String())
	}
}
func TestDatabaseErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{{pgx.ErrNoRows, 404, "NOT_FOUND"}, {&pgconn.PgError{Code: "23505", ConstraintName: "customers_document_key"}, 409, "DUPLICATE_DOCUMENT"}, {&pgconn.PgError{Code: "23505", ConstraintName: "vehicles_plate_key"}, 409, "DUPLICATE_PLATE"}, {&pgconn.PgError{Code: "23505", ConstraintName: "inventory_items_sku_key"}, 409, "DUPLICATE_SKU"}, {&pgconn.PgError{Code: "23505"}, 409, "DUPLICATE_RESOURCE"}, {&pgconn.PgError{Code: "23503"}, 400, "VALIDATION_ERROR"}, {&pgconn.PgError{Code: "40001"}, 409, "VERSION_CONFLICT"}, {&pgconn.PgError{Code: "40P01"}, 409, "VERSION_CONFLICT"}} {
		var e *domain.Error
		if !errors.As(MapError(tc.err), &e) || e.Status != tc.status || e.Code != tc.code {
			t.Fatal(tc.err)
		}
	}
	if MapError(nil) != nil {
		t.Fatal("nil error")
	}
	r, e := Decode([]byte(`{"value":9007199254740991}`))
	if e != nil || Number(r, "value") != domain.MaxMoney {
		t.Fatal("precision lost")
	}
}
