package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"oficina/internal/auth"
	"oficina/internal/platform"
	"oficina/internal/server"
	"oficina/migrations"
)

func TestPostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL não configurada: informe uma conexão PostgreSQL de teste para executar a integração com cobertura de handlers e persistência")
	}

	startup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := platform.Open(startup, databaseURL)
	if err != nil {
		t.Fatal("não foi possível conectar ao PostgreSQL de teste; detalhes de conexão omitidos")
	}
	t.Cleanup(db.Close)
	if err := migrations.Apply(startup, db); err != nil {
		t.Fatal("não foi possível aplicar as migrações ao PostgreSQL de teste")
	}

	username, password := "integration-"+unique(t), unique(t)+unique(t)
	hash, err := auth.PasswordHash(password)
	if err != nil {
		t.Fatal("não foi possível gerar o hash da senha sintética")
	}
	administratorID := uuid.NewString()
	if _, err := db.Exec(startup, "INSERT INTO administrators(id,username,password_hash) VALUES($1,$2,$3)", administratorID, username, string(hash)); err != nil {
		t.Fatal("não foi possível criar o administrador exclusivo do teste")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.Exec(ctx, "DELETE FROM administrators WHERE id=$1", administratorID); err != nil {
			t.Error("não foi possível remover o administrador sintético após o teste")
		}
	})

	const allowedOrigin = "http://localhost:5173"
	cfg := server.Config{
		DatabaseURL:      databaseURL,
		JWTSecret:        unique(t) + unique(t),
		Port:             "8080",
		Origins:          []string{allowedOrigin},
		CustomerTokenTTL: time.Hour,
	}
	if len(cfg.JWTSecret) < 32 || cfg.CustomerTokenTTL <= 0 {
		t.Fatal("configuração de autenticação do teste inválida")
	}
	httpServer := httptest.NewServer(server.New(db, cfg))
	t.Cleanup(httpServer.Close)
	client := httpServer.Client()
	client.Timeout = 15 * time.Second

	t.Run("health_and_cors", func(t *testing.T) {
		for _, test := range []struct {
			method string
			origin string
			status int
		}{
			{http.MethodGet, allowedOrigin, http.StatusOK},
			{http.MethodOptions, allowedOrigin, http.StatusNoContent},
			{http.MethodOptions, "http://untrusted.example.invalid", http.StatusForbidden},
		} {
			request, err := http.NewRequest(test.method, httpServer.URL+"/api/v1/health", nil)
			if err != nil {
				t.Fatal("não foi possível preparar a verificação HTTP")
			}
			request.Header.Set("Origin", test.origin)
			response, err := client.Do(request)
			if err != nil {
				t.Fatal("servidor HTTP de teste não respondeu")
			}
			response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("saúde/CORS retornou HTTP %d; esperado %d", response.StatusCode, test.status)
			}
			if test.origin == allowedOrigin && response.Header.Get("Access-Control-Allow-Origin") != allowedOrigin {
				t.Error("origem CORS explícita não foi autorizada")
			}
			if test.origin != allowedOrigin && response.Header.Get("Access-Control-Allow-Origin") != "" {
				t.Error("origem CORS desconhecida foi autorizada")
			}
			if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Error("cabeçalhos de segurança ausentes")
			}
		}
	})

	t.Setenv("API_BASE_URL", httpServer.URL+"/api/v1")
	t.Setenv("TEST_ADMIN_USERNAME", username)
	t.Setenv("TEST_ADMIN_PASSWORD", password)
	t.Run("api_contract", TestAPIIntegration)
	t.Run("hostile_inputs_and_query_contract", func(t *testing.T) {
		testHostileInputs(t, httpServer.URL+"/api/v1", client, username, password)
	})
}

func testHostileInputs(t *testing.T, base string, client *http.Client, username, password string) {
	api := apiClient{base: base, http: client}
	api.token = stringField(t, api.call(t, "POST", "/auth/login", object{"username": username, "password": password}, 200), "accessToken")
	customer := createCustomer(t, api, "CNPJ")
	vehicle := createVehicle(t, api, stringField(t, customer, "id"))
	service := createService(t, api, 100)
	item := createInventory(t, api, 50, 2)
	customerID, vehicleID := stringField(t, customer, "id"), stringField(t, vehicle, "id")
	serviceID, itemID := stringField(t, service, "id"), stringField(t, item, "id")
	customerPath, vehiclePath := "/admin/customers/"+customerID, "/admin/vehicles/"+vehicleID
	servicePath, itemPath := "/admin/services/"+serviceID, "/admin/inventory-items/"+itemID
	order := api.call(t, "POST", "/admin/service-orders", orderBody(customerID, vehicleID, []string{serviceID}, []object{}), 201)
	orderPath := "/admin/service-orders/" + stringField(t, order, "id")
	missing := uuid.NewString()

	t.Run("malformed_bodies_and_required_fields", func(t *testing.T) {
		for _, test := range []struct {
			name, method, path, body, contentType string
		}{
			{"malformed_json", "POST", "/admin/services", `{"name":`, "application/json"},
			{"null_body", "POST", "/admin/services", `null`, "application/json"},
			{"array_body", "POST", "/admin/services", `[]`, "application/json"},
			{"multiple_json_values", "POST", "/admin/services", `{"name":"Teste","priceCents":1} {}`, "application/json"},
			{"wrong_media_type", "POST", "/admin/services", `{"name":"Teste","priceCents":1}`, "text/plain"},
			{"missing_media_type", "POST", "/admin/services", `{}`, ""},
			{"extra_field", "POST", "/admin/services", `{"name":"Teste","priceCents":1,"active":true}`, "application/json"},
			{"missing_price", "POST", "/admin/services", `{"name":"Teste"}`, "application/json"},
			{"body_over_one_mib", "POST", "/admin/services", `{"name":"` + strings.Repeat("a", 1<<20) + `","priceCents":1}`, "application/json"},
			{"empty_patch", "PATCH", servicePath, `{}`, "application/json"},
			{"null_required_name", "PATCH", servicePath, `{"name":null}`, "application/json"},
			{"null_required_price", "PATCH", servicePath, `{"priceCents":null}`, "application/json"},
			{"object_in_text", "PATCH", servicePath, `{"name":{}}`, "application/json"},
			{"long_name", "PATCH", servicePath, `{"name":"` + strings.Repeat("a", 161) + `"}`, "application/json"},
			{"string_boolean", "PATCH", servicePath, `{"active":"true"}`, "application/json"},
			{"fractional_price", "PATCH", servicePath, `{"priceCents":1.5}`, "application/json"},
			{"text_price", "PATCH", servicePath, `{"priceCents":"100"}`, "application/json"},
			{"int64_price_overflow", "PATCH", servicePath, `{"priceCents":9223372036854775808}`, "application/json"},
			{"money_overflow", "PATCH", itemPath, `{"priceCents":9007199254740992}`, "application/json"},
			{"initial_stock_via_patch", "PATCH", itemPath, `{"initialQuantity":100}`, "application/json"},
			{"invalid_inventory_type", "PATCH", itemPath, `{"type":"TOOL"}`, "application/json"},
			{"document_without_type", "PATCH", customerPath, `{"document":"11111111111"}`, "application/json"},
			{"type_without_document", "PATCH", customerPath, `{"documentType":"CPF"}`, "application/json"},
			{"invalid_email", "PATCH", customerPath, `{"email":"invalid-email"}`, "application/json"},
			{"null_vehicle_owner", "PATCH", vehiclePath, `{"customerId":null}`, "application/json"},
			{"invalid_year", "PATCH", vehiclePath, `{"year":1885}`, "application/json"},
			{"missing_expected_version", "POST", orderPath + "/diagnosis/start", `{}`, "application/json"},
			{"fractional_version", "POST", orderPath + "/diagnosis/start", `{"expectedVersion":1.5}`, "application/json"},
			{"zero_version", "POST", orderPath + "/diagnosis/start", `{"expectedVersion":0}`, "application/json"},
		} {
			t.Run(test.name, func(t *testing.T) { rawHTTPCall(t, api, test.method, test.path, test.body, test.contentType, 400) })
		}
		for path, before := range map[string]object{customerPath: customer, vehiclePath: vehicle, servicePath: service, itemPath: item, orderPath: order} {
			assertUnchanged(t, api, path, before)
		}
	})

	t.Run("invalid_routes_identifiers_and_filters", func(t *testing.T) {
		interval := "?from=2026-01-01T00%3A00%3A00Z&to=2027-01-01T00%3A00%3A00Z"
		for _, test := range []struct {
			name, method, path string
			body               any
			status             int
		}{
			{"customer_uuid", "GET", "/admin/customers/not-a-uuid", nil, 400},
			{"patch_uuid", "PATCH", "/admin/vehicles/not-a-uuid", object{"model": "Teste"}, 400},
			{"delete_uuid", "DELETE", "/admin/services/not-a-uuid", nil, 400},
			{"nil_uuid", "GET", "/admin/customers/00000000-0000-0000-0000-000000000000", nil, 400},
			{"missing_resource", "GET", "/admin/customers/" + missing, nil, 404},
			{"missing_patch_resource", "PATCH", "/admin/services/" + missing, object{"name": "Ausente"}, 404},
			{"order_uuid", "GET", "/admin/service-orders/not-a-uuid", nil, 400},
			{"command_uuid", "POST", "/admin/service-orders/not-a-uuid/diagnosis/start", object{"expectedVersion": 1}, 400},
			{"service_item_uuid", "POST", orderPath + "/services/not-a-uuid/start", object{"expectedVersion": 1}, 400},
			{"missing_order", "GET", "/admin/service-orders/" + missing, nil, 404},
			{"missing_order_command", "POST", "/admin/service-orders/" + missing + "/diagnosis/start", object{"expectedVersion": 1}, 404},
			{"movements_uuid", "GET", "/admin/inventory-items/not-a-uuid/movements", nil, 400},
			{"movement_uuid", "POST", "/admin/inventory-items/not-a-uuid/movements", object{}, 400},
			{"missing_movement_item", "GET", "/admin/inventory-items/" + missing + "/movements", nil, 404},
			{"movement_page", "GET", itemPath + "/movements?page=0", nil, 400},
			{"active_boolean", "GET", "/admin/customers?active=maybe", nil, 400},
			{"oversized_document_filter", "GET", "/admin/customers?document=" + strings.Repeat("1", 33), nil, 400},
			{"oversized_name_filter", "GET", "/admin/services?name=" + strings.Repeat("a", 161), nil, 400},
			{"vehicle_customer_filter", "GET", "/admin/vehicles?customerId=invalid", nil, 400},
			{"page_not_integer", "GET", "/admin/customers?page=NaN", nil, 400},
			{"page_size_not_integer", "GET", "/admin/customers?pageSize=1.5", nil, 400},
			{"order_page", "GET", "/admin/service-orders?page=-1", nil, 400},
			{"order_customer_filter", "GET", "/admin/service-orders?customerId=invalid", nil, 400},
			{"order_status_filter", "GET", "/admin/service-orders?status=UNKNOWN", nil, 400},
			{"order_number_filter", "GET", "/admin/service-orders?number=0", nil, 400},
			{"order_number_overflow", "GET", "/admin/service-orders?number=9223372036854775808", nil, 400},
			{"missing_metric_interval", "GET", "/admin/metrics/service-orders", nil, 400},
			{"bad_metric_date", "GET", "/admin/metrics/service-execution?from=invalid&to=2027-01-01T00%3A00%3A00Z", nil, 400},
			{"reversed_metric_interval", "GET", "/admin/metrics/service-orders?from=2027-01-01T00%3A00%3A00Z&to=2026-01-01T00%3A00%3A00Z", nil, 400},
			{"metric_pagination", "GET", "/admin/metrics/service-execution" + interval + "&pageSize=0", nil, 400},
			{"unknown_route", "GET", "/admin/nonexistent-resource", nil, 404},
			{"unsupported_method", "PUT", servicePath, object{"name": "Teste"}, 405},
		} {
			t.Run(test.name, func(t *testing.T) { api.call(t, test.method, test.path, test.body, test.status) })
		}
	})

	t.Run("invalid_selections_and_stock_overflow", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			patch object
		}{
			{"invalid_customer", object{"customerId": "invalid"}},
			{"invalid_vehicle", object{"vehicleId": "invalid"}},
			{"empty_problem", object{"problemDescription": " "}},
			{"null_services", object{"serviceIds": nil}},
			{"duplicate_service", object{"serviceIds": []string{serviceID, serviceID}}},
			{"unknown_service", object{"serviceIds": []string{missing}}},
			{"null_parts", object{"parts": nil}},
			{"zero_quantity", object{"parts": []object{{"inventoryItemId": itemID, "quantity": 0}}}},
			{"fractional_quantity", object{"parts": []object{{"inventoryItemId": itemID, "quantity": 1.5}}}},
			{"quantity_int64_overflow", object{"parts": []object{{"inventoryItemId": itemID, "quantity": json.Number("9223372036854775808")}}}},
			{"duplicate_part", object{"parts": []object{{"inventoryItemId": itemID, "quantity": 1}, {"inventoryItemId": itemID, "quantity": 1}}}},
			{"extra_part_price", object{"parts": []object{{"inventoryItemId": itemID, "quantity": 1, "priceCents": 1}}}},
			{"unknown_part", object{"parts": []object{{"inventoryItemId": missing, "quantity": 1}}}},
		} {
			t.Run(test.name, func(t *testing.T) {
				body := orderBody(customerID, vehicleID, []string{serviceID}, []object{})
				for key, value := range test.patch {
					body[key] = value
				}
				status := 400
				if test.name == "unknown_service" || test.name == "unknown_part" {
					status = 404
				}
				api.call(t, "POST", "/admin/service-orders", body, status)
			})
		}
		for _, test := range []struct {
			name, kind string
			delta      any
			status     int
		}{
			{"zero", "ADJUSTMENT", 0, 400},
			{"negative_replenishment", "REPLENISHMENT", -1, 400},
			{"unknown_type", "OPENING", 1, 400},
			{"fractional", "REPLENISHMENT", 1.5, 400},
			{"int64_overflow", "REPLENISHMENT", json.Number("9223372036854775808"), 400},
			{"balance_overflow", "REPLENISHMENT", int64(math.MaxInt64), 400},
			{"minimum_negative", "ADJUSTMENT", int64(math.MinInt64), 409},
		} {
			t.Run(test.name, func(t *testing.T) {
				api.call(t, "POST", itemPath+"/movements", object{"type": test.kind, "quantityDelta": test.delta, "reason": "Validação sintética de limites"}, test.status)
			})
		}
		assertUnchanged(t, api, itemPath, item)
		assertInt(t, api.call(t, "GET", itemPath+"/movements", nil, 200), "total", 1)
		assertInt(t, api.call(t, "GET", "/admin/service-orders?customerId="+customerID, nil, 200), "total", 1)
	})

	t.Run("normalized_filters_pagination_and_inactive_records", func(t *testing.T) {
		document := stringField(t, customer, "document")
		formatted := strings.ToLower(document[:2] + "." + document[2:5] + "." + document[5:8] + "/" + document[8:12] + "-" + document[12:])
		assertInt(t, api.call(t, "GET", "/admin/customers?document="+url.QueryEscape(formatted)+"&active=true", nil, 200), "total", 1)
		assertInt(t, api.call(t, "GET", "/admin/vehicles?customerId="+customerID, nil, 200), "total", 1)
		assertInt(t, api.call(t, "GET", "/admin/inventory-items?sku="+url.QueryEscape(" "+strings.ToLower(stringField(t, item, "sku"))+" "), nil, 200), "total", 1)
		assertInt(t, api.call(t, "GET", "/admin/services?name="+url.QueryEscape(stringField(t, service, "name")), nil, 200), "total", 1)
		assertInt(t, api.call(t, "GET", "/admin/services?name="+url.QueryEscape("' OR 1=1 --"), nil, 200), "total", 0)
		second := api.call(t, "POST", "/admin/service-orders", orderBody(customerID, vehicleID, []string{serviceID}, []object{}), 201)
		secondPath := "/admin/service-orders/" + stringField(t, second, "id")
		second = api.call(t, "POST", secondPath+"/diagnosis/start", versionBody(t, second), 200)
		if numberField(t, second, "number") <= numberField(t, order, "number") {
			t.Fatal("número da OS deve crescer entre novas aberturas")
		}
		for page, expected := range []object{second, order} {
			result := api.call(t, "GET", fmt.Sprintf("/admin/service-orders?customerId=%s&pageSize=1&page=%d", customerID, page+1), nil, 200)
			assertInt(t, result, "total", 2)
			assertInt(t, result, "page", int64(page+1))
			assertInt(t, result, "pageSize", 1)
			rows := arrayObjects(t, result, "data")
			if len(rows) != 1 {
				t.Fatal("paginação de OS deve retornar exatamente um registro nesta página")
			}
			assertField(t, rows[0], "id", expected["id"])
		}
		for _, filter := range []string{"status=IN_DIAGNOSIS", fmt.Sprintf("number=%d", numberField(t, order, "number"))} {
			assertInt(t, api.call(t, "GET", "/admin/service-orders?customerId="+customerID+"&"+filter, nil, 200), "total", 1)
		}
		assertInt(t, api.call(t, "GET", "/admin/service-orders?number=9223372036854775807", nil, 200), "total", 0)
		api.call(t, "DELETE", customerPath, nil, 204)
		assertInt(t, api.call(t, "GET", "/admin/customers?document="+url.QueryEscape(formatted)+"&active=false", nil, 200), "total", 1)
		assertInt(t, api.call(t, "GET", "/admin/customers?document="+url.QueryEscape(formatted)+"&active=true", nil, 200), "total", 0)
		api.call(t, "POST", "/admin/vehicles", object{"customerId": customerID, "plate": syntheticPlate(t), "brand": "Teste", "model": "Teste", "year": 2024}, 409)
		assertUnchanged(t, api, orderPath, order)
		api.call(t, "PATCH", customerPath, object{"active": true}, 200)
		api.call(t, "DELETE", servicePath, nil, 204)
		api.call(t, "POST", secondPath+"/quotes", versionBody(t, second), 409)
		assertUnchanged(t, api, secondPath, second)
		api.call(t, "PATCH", servicePath, object{"active": true}, 200)
	})

	t.Run("quote_arithmetic_overflow_is_atomic", func(t *testing.T) {
		maximum := int64(9007199254740991)
		expensiveService := createService(t, api, maximum)
		expensivePart := createInventory(t, api, maximum, 0)
		for _, test := range []struct {
			name     string
			services []string
			parts    []object
		}{
			{"sum", []string{stringField(t, expensiveService, "id"), serviceID}, []object{}},
			{"multiplication", []string{serviceID}, []object{{"inventoryItemId": expensivePart["id"], "quantity": 2}}},
		} {
			t.Run(test.name, func(t *testing.T) {
				pending := api.call(t, "POST", "/admin/service-orders", orderBody(customerID, vehicleID, test.services, test.parts), 201)
				path := "/admin/service-orders/" + stringField(t, pending, "id")
				pending = api.call(t, "POST", path+"/diagnosis/start", versionBody(t, pending), 200)
				api.call(t, "POST", path+"/quotes", versionBody(t, pending), 400)
				assertUnchanged(t, api, path, pending)
				assertField(t, pending, "currentQuote", nil)
			})
		}
		assertUnchanged(t, api, "/admin/inventory-items/"+stringField(t, expensivePart, "id"), expensivePart)
	})
}

func rawHTTPCall(t *testing.T, api apiClient, method, path, body, contentType string, status int) {
	t.Helper()
	request, err := http.NewRequest(method, api.base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal("não foi possível preparar a requisição de validação")
	}
	request.Header.Set("Authorization", "Bearer "+api.token)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := api.http.Do(request)
	if err != nil {
		t.Fatal("servidor não respondeu à requisição de validação")
	}
	defer response.Body.Close()
	result := apiResponse{status: response.StatusCode, contentType: response.Header.Get("Content-Type")}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.UseNumber()
	if err := decoder.Decode(&result.body); err != nil {
		t.Fatal("resposta de erro não contém JSON válido")
	}
	assertResponse(t, method, path, result, status)
}
