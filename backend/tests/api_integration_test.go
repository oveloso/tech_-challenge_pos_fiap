package tests

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type object = map[string]any

type apiClient struct {
	base  string
	token string
	http  *http.Client
}

type apiResponse struct {
	status      int
	contentType string
	body        object
}

func TestAPIIntegration(t *testing.T) {
	base := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if base == "" {
		t.Skip("API_BASE_URL não configurada: execute contra uma API com PostgreSQL de teste e administrador semeado; configure TEST_ADMIN_USERNAME e TEST_ADMIN_PASSWORD")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		t.Fatal("API_BASE_URL deve ser uma URL HTTP(S) absoluta")
	}
	if u.Path == "" || u.Path == "/" {
		base += "/api/v1"
	}
	username, password := os.Getenv("TEST_ADMIN_USERNAME"), os.Getenv("TEST_ADMIN_PASSWORD")
	if username == "" || password == "" {
		t.Fatal("configure TEST_ADMIN_USERNAME e TEST_ADMIN_PASSWORD com as credenciais do administrador do banco de teste")
	}
	public := apiClient{base: base, http: &http.Client{Timeout: 15 * time.Second}}
	login := public.call(t, "POST", "/auth/login", object{"username": username, "password": password}, 200)
	admin := public
	admin.token = stringField(t, login, "accessToken")
	assertField(t, login, "tokenType", "Bearer")
	assertTimestamp(t, login, "expiresAt")

	t.Run("authentication_and_input_validation", func(t *testing.T) {
		public.call(t, "GET", "/admin/customers", nil, 401)
		public.call(t, "GET", "/customer/service-order", nil, 401)
		admin.call(t, "GET", "/customer/service-order", nil, 401)
		public.call(t, "POST", "/auth/login", object{"username": "missing-" + unique(t), "password": unique(t)}, 401)
		for _, doc := range []object{
			{"name": "Documento inválido", "documentType": "CPF", "document": "11111111111"},
			{"name": "Documento inválido", "documentType": "CNPJ", "document": "AAAAAAAAAAAA00"},
		} {
			admin.call(t, "POST", "/admin/customers", doc, 400)
		}
		admin.call(t, "POST", "/admin/services", object{"name": "Preço inválido", "priceCents": -1}, 400)
		admin.call(t, "POST", "/admin/services", object{"name": "Overflow", "priceCents": int64(9007199254740992)}, 400)
		admin.call(t, "GET", "/admin/customers?page=0", nil, 400)
		admin.call(t, "GET", "/admin/customers?pageSize=101", nil, 400)
	})

	t.Run("registrations_soft_delete_and_inventory", func(t *testing.T) {
		customer := createCustomer(t, admin, "CPF")
		company := createCustomer(t, admin, "CNPJ")
		vehicle := createVehicle(t, admin, stringField(t, customer, "id"))
		service := createService(t, admin, 1234)
		item := createInventory(t, admin, 567, 3)
		customerID, vehicleID := stringField(t, customer, "id"), stringField(t, vehicle, "id")
		serviceID, itemID := stringField(t, service, "id"), stringField(t, item, "id")
		customerPath, vehiclePath := "/admin/customers/"+customerID, "/admin/vehicles/"+vehicleID
		itemPath := "/admin/inventory-items/" + itemID
		admin.call(t, "POST", "/admin/customers", object{"name": "Duplicado", "documentType": "CPF", "document": customer["document"]}, 409)
		admin.call(t, "POST", "/admin/customers", object{"name": "Duplicado", "documentType": "CNPJ", "document": strings.ToLower(stringField(t, company, "document"))}, 409)
		admin.call(t, "POST", "/admin/vehicles", object{"customerId": customerID, "plate": "ABC123", "brand": "Teste", "model": "Teste", "year": 2024}, 400)
		admin.call(t, "PATCH", vehiclePath, object{"plate": "123ABCD"}, 400)
		assertField(t, admin.call(t, "PATCH", customerPath, object{"name": "Cliente atualizado", "email": nil, "phone": nil}, 200), "name", "Cliente atualizado")
		assertField(t, admin.call(t, "PATCH", vehiclePath, object{"model": "Modelo atualizado"}, 200), "model", "Modelo atualizado")
		legacyPlate := []byte(syntheticPlate(t))
		legacyPlate[4] = '0' + legacyPlate[4]%10
		vehicle = admin.call(t, "PATCH", vehiclePath, object{"plate": string(legacyPlate)}, 200)
		assertField(t, vehicle, "plate", string(legacyPlate))
		assertInt(t, admin.call(t, "PATCH", "/admin/services/"+serviceID, object{"priceCents": 1456}, 200), "priceCents", 1456)
		assertField(t, admin.call(t, "PATCH", itemPath, object{"name": "Peça atualizada"}, 200), "name", "Peça atualizada")
		filtered := admin.call(t, "GET", "/admin/customers?document="+url.QueryEscape(stringField(t, customer, "document")), nil, 200)
		assertInt(t, filtered, "total", 1)
		assertField(t, arrayObjects(t, filtered, "data")[0], "id", customerID)
		filtered = admin.call(t, "GET", "/admin/vehicles?plate="+stringField(t, vehicle, "plate"), nil, 200)
		assertInt(t, filtered, "total", 1)
		assertField(t, arrayObjects(t, filtered, "data")[0], "id", vehicleID)
		assertInt(t, admin.call(t, "GET", itemPath, nil, 200), "balance", 3)
		movements := admin.call(t, "GET", itemPath+"/movements", nil, 200)
		assertInt(t, movements, "total", 1)
		assertField(t, arrayObjects(t, movements, "data")[0], "type", "OPENING")
		admin.call(t, "PATCH", itemPath, object{"balance": 999}, 400)
		admin.call(t, "POST", itemPath+"/movements", object{"type": "ADJUSTMENT", "quantityDelta": -4, "reason": "Teste de saldo"}, 409)
		admin.call(t, "POST", itemPath+"/movements", object{"type": "REPLENISHMENT", "quantityDelta": 1, "reason": ""}, 400)
		assertInt(t, admin.call(t, "GET", itemPath, nil, 200), "balance", 3)
		assertInt(t, admin.call(t, "GET", itemPath+"/movements", nil, 200), "total", 1)
		assertInt(t, admin.call(t, "POST", itemPath+"/movements", object{"type": "ADJUSTMENT", "quantityDelta": -1, "reason": "Contagem sintética"}, 201), "balance", 2)
		body := orderBody(customerID, vehicleID, []string{serviceID}, []object{{"inventoryItemId": itemID, "quantity": 1}})
		for _, path := range []string{customerPath, vehiclePath, "/admin/services/" + serviceID, itemPath} {
			admin.call(t, "DELETE", path, nil, 204)
			admin.call(t, "DELETE", path, nil, 204)
			assertField(t, admin.call(t, "GET", path, nil, 200), "active", false)
			admin.call(t, "POST", "/admin/service-orders", body, 409)
			switch path {
			case customerPath:
				admin.call(t, "POST", "/admin/customers", object{"name": "Duplicado inativo", "documentType": "CPF", "document": customer["document"]}, 409)
			case vehiclePath:
				admin.call(t, "POST", "/admin/vehicles", object{"customerId": customerID, "plate": vehicle["plate"], "brand": "Teste", "model": "Teste", "year": 2024}, 409)
			case itemPath:
				admin.call(t, "POST", "/admin/inventory-items", object{"sku": item["sku"], "name": "Duplicado inativo", "type": "PART", "priceCents": 1}, 409)
				admin.call(t, "POST", itemPath+"/movements", object{"type": "REPLENISHMENT", "quantityDelta": 1, "reason": "Item inativo"}, 409)
			}
			assertField(t, admin.call(t, "PATCH", path, object{"active": true}, 200), "active", true)
		}
		body["customerId"] = company["id"]
		admin.call(t, "POST", "/admin/service-orders", body, 409)
	})

	t.Run("complete_service_order_workflow", func(t *testing.T) {
		customer := createCustomer(t, admin, "CPF")
		newOwner := createCustomer(t, admin, "CNPJ")
		vehicle := createVehicle(t, admin, stringField(t, customer, "id"))
		serviceA, serviceB := createService(t, admin, 10000), createService(t, admin, 7000)
		partA, partB := createInventory(t, admin, 2500, 5), createInventory(t, admin, 1500, 0)
		customerID, vehicleID := stringField(t, customer, "id"), stringField(t, vehicle, "id")
		serviceAID, serviceBID := stringField(t, serviceA, "id"), stringField(t, serviceB, "id")
		partAID, partBID := stringField(t, partA, "id"), stringField(t, partB, "id")
		parts := []object{{"inventoryItemId": partAID, "quantity": 2}, {"inventoryItemId": partBID, "quantity": 1}}
		order := admin.call(t, "POST", "/admin/service-orders", orderBody(customerID, vehicleID, []string{serviceAID}, parts), 201)
		orderPath := "/admin/service-orders/" + stringField(t, order, "id")
		var customerAPI apiClient
		var quote1, quote2, quote3 object
		var orderServiceA, orderServiceB string
		interval := "?from=" + url.QueryEscape(time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)) + "&to=" + url.QueryEscape(time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339))
		baseline := admin.call(t, "GET", "/admin/metrics/service-orders"+interval, nil, 200)
		run := func(name string, test func(*testing.T)) {
			t.Helper()
			if !t.Run(name, test) {
				t.FailNow()
			}
		}
		run("creation_snapshots_and_token_isolation", func(t *testing.T) {
			assertField(t, order, "status", "RECEIVED")
			assertInt(t, order, "version", 1)
			assertField(t, order, "customerName", customer["name"])
			assertField(t, order, "vehiclePlate", vehicle["plate"])
			assertField(t, order, "currentQuote", nil)
			assertSnapshot(t, order, vehicle)
			orderServiceA = stringField(t, findObject(t, arrayObjects(t, order, "services"), "serviceId", serviceAID), "id")
			admin.call(t, "PATCH", "/admin/vehicles/"+vehicleID, object{"customerId": newOwner["id"]}, 409)
			admin.call(t, "PATCH", "/admin/customers/"+customerID, object{"name": "Nome alterado após abertura"}, 200)
			admin.call(t, "PATCH", "/admin/vehicles/"+vehicleID, object{"plate": syntheticPlate(t), "brand": "Marca alterada", "model": "Modelo alterado", "year": 2025}, 200)
			order = admin.call(t, "GET", orderPath, nil, 200)
			assertField(t, order, "customerName", customer["name"])
			assertField(t, order, "vehiclePlate", vehicle["plate"])
			assertSnapshot(t, order, vehicle)
			access := admin.call(t, "POST", orderPath+"/customer-access", versionBody(t, order), 201)
			customerAPI = admin
			customerAPI.token = stringField(t, access, "token")
			if len(customerAPI.token) < 43 || customerAPI.token == admin.token {
				t.Fatal("token opaco do cliente inválido")
			}
			assertTimestamp(t, access, "expiresAt")
			assertInt(t, access, "workOrderVersion", numberField(t, order, "version")+1)
			order = admin.call(t, "GET", orderPath, nil, 200)
			customerAPI.call(t, "GET", "/admin/customers", nil, 401)
			customerAPI.call(t, "GET", orderPath, nil, 401)
			visible := customerAPI.call(t, "GET", "/customer/service-order", nil, 200)
			assertInt(t, visible, "number", numberField(t, order, "number"))
			assertSnapshot(t, visible, vehicle)
			assertCustomerProjection(t, visible)
		})
		run("diagnosis_items_and_stale_version", func(t *testing.T) {
			stale := versionBody(t, order)
			order = admin.call(t, "POST", orderPath+"/diagnosis/start", stale, 200)
			assertField(t, order, "status", "IN_DIAGNOSIS")
			admin.call(t, "POST", orderPath+"/diagnosis/start", stale, 409)
			assertUnchanged(t, admin, orderPath, order)
			order = admin.call(t, "PUT", orderPath+"/items", object{"expectedVersion": numberField(t, order, "version"), "serviceIds": []string{serviceAID, serviceBID}, "parts": parts}, 200)
			assertField(t, findObject(t, arrayObjects(t, order, "services"), "serviceId", serviceAID), "id", orderServiceA)
			orderServiceB = stringField(t, findObject(t, arrayObjects(t, order, "services"), "serviceId", serviceBID), "id")
			admin.call(t, "PUT", orderPath+"/items", object{"expectedVersion": numberField(t, order, "version") - 1, "serviceIds": []string{}, "parts": []object{}}, 409)
			assertUnchanged(t, admin, orderPath, order)
		})
		run("quote_total_and_immutable_prices", func(t *testing.T) {
			admin.call(t, "POST", orderPath+"/quotes", object{"expectedVersion": numberField(t, order, "version"), "totalCents": 1}, 400)
			assertUnchanged(t, admin, orderPath, order)
			order = admin.call(t, "POST", orderPath+"/quotes", versionBody(t, order), 200)
			assertField(t, order, "status", "AWAITING_APPROVAL")
			quote1 = objectField(t, order, "currentQuote")
			assertInt(t, quote1, "version", 1)
			assertInt(t, quote1, "totalCents", 23500)
			assertField(t, quote1, "status", "PENDING")
			assertInt(t, findObject(t, arrayObjects(t, quote1, "parts"), "inventoryItemId", partAID), "subtotalCents", 5000)
			admin.call(t, "PATCH", "/admin/services/"+serviceAID, object{"name": "Serviço renomeado", "priceCents": 15000}, 200)
			admin.call(t, "PATCH", "/admin/inventory-items/"+partAID, object{"name": "Peça renomeada", "priceCents": 4000}, 200)
			order = admin.call(t, "GET", orderPath, nil, 200)
			if !reflect.DeepEqual(quote1, objectField(t, order, "currentQuote")) {
				t.Fatal("alteração de catálogo reescreveu o orçamento publicado")
			}
			admin.call(t, "PUT", orderPath+"/items", object{"expectedVersion": numberField(t, order, "version"), "serviceIds": []string{serviceAID}, "parts": []object{}}, 409)
			admin.call(t, "POST", orderPath+"/services/"+orderServiceA+"/start", versionBody(t, order), 409)
			assertUnchanged(t, admin, orderPath, order)
			visible := customerAPI.call(t, "GET", "/customer/service-order", nil, 200)
			assertInt(t, objectField(t, visible, "currentQuote"), "totalCents", 23500)
			assertCustomerProjection(t, visible)
		})
		run("rejection_revision_approval_and_superseded_decisions", func(t *testing.T) {
			decision(t, customerAPI, quote1, "REJECTED", 200)
			order = admin.call(t, "GET", orderPath, nil, 200)
			assertField(t, order, "status", "IN_DIAGNOSIS")
			assertField(t, objectField(t, order, "currentQuote"), "status", "REJECTED")
			decision(t, customerAPI, quote1, "REJECTED", 200)
			assertUnchanged(t, admin, orderPath, order)
			order = admin.call(t, "POST", orderPath+"/quotes", versionBody(t, order), 200)
			quote2 = objectField(t, order, "currentQuote")
			assertInt(t, quote2, "version", 2)
			assertInt(t, quote2, "totalCents", 31500)
			assertField(t, findObject(t, arrayObjects(t, quote2, "services"), "serviceId", serviceAID), "name", "Serviço renomeado")
			decision(t, customerAPI, quote1, "REJECTED", 409)
			decision(t, customerAPI, quote2, "APPROVED", 200)
			order = admin.call(t, "GET", orderPath, nil, 200)
			assertField(t, order, "status", "AWAITING_APPROVAL")
			assertField(t, objectField(t, order, "currentQuote"), "status", "APPROVED")
			decision(t, customerAPI, quote2, "APPROVED", 200)
			decision(t, customerAPI, quote2, "REJECTED", 409)
			assertUnchanged(t, admin, orderPath, order)
			order = admin.call(t, "POST", orderPath+"/diagnosis/reopen", versionBody(t, order), 200)
			assertField(t, order, "status", "IN_DIAGNOSIS")
			assertField(t, objectField(t, order, "currentQuote"), "status", "SUPERSEDED")
			decision(t, customerAPI, quote2, "APPROVED", 409)
			order = admin.call(t, "POST", orderPath+"/quotes", versionBody(t, order), 200)
			quote3 = objectField(t, order, "currentQuote")
			assertInt(t, quote3, "version", 3)
			decision(t, customerAPI, quote2, "APPROVED", 409)
			decision(t, customerAPI, quote3, "APPROVED", 200)
			order = admin.call(t, "GET", orderPath, nil, 200)
			quotes := arrayObjects(t, order, "quotes")
			if len(quotes) != 3 {
				t.Fatal("histórico deve preservar as três versões do orçamento")
			}
			assertField(t, quotes[0], "status", "REJECTED")
			assertInt(t, quotes[0], "totalCents", 23500)
			assertField(t, quotes[1], "status", "SUPERSEDED")
			assertField(t, quotes[2], "status", "APPROVED")
		})
		run("insufficient_stock_rolls_back_entire_start", func(t *testing.T) {
			assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+partAID, nil, 200), "balance", 5)
			assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+partBID, nil, 200), "balance", 0)
			admin.call(t, "POST", orderPath+"/services/"+orderServiceA+"/start", versionBody(t, order), 409)
			assertUnchanged(t, admin, orderPath, order)
			assertField(t, order, "stockConsumedAt", nil)
			assertField(t, order, "executionStartedAt", nil)
			for _, service := range arrayObjects(t, order, "services") {
				assertField(t, service, "status", "NOT_STARTED")
				assertField(t, service, "startedAt", nil)
			}
			assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+partAID, nil, 200), "balance", 5)
			assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+partBID, nil, 200), "balance", 0)
			assertConsumption(t, admin, partAID, stringField(t, order, "id"), 0, 0)
			assertConsumption(t, admin, partBID, stringField(t, order, "id"), 0, 0)
			assertInt(t, admin.call(t, "POST", "/admin/inventory-items/"+partBID+"/movements", object{"type": "REPLENISHMENT", "quantityDelta": 1, "reason": "Reposição do teste"}, 201), "balance", 1)
		})
		run("concurrent_starts_consume_stock_once", func(t *testing.T) {
			startPath := orderPath + "/services/" + orderServiceA + "/start"
			stale := versionBody(t, order)
			concurrentRequests(t, admin, []string{startPath, startPath}, []object{stale, stale})
			order = admin.call(t, "GET", orderPath, nil, 200)
			assertField(t, order, "status", "IN_EXECUTION")
			assertInt(t, order, "version", numberField(t, stale, "expectedVersion")+1)
			assertTimestamp(t, order, "executionStartedAt")
			assertTimestamp(t, order, "stockConsumedAt")
			assertField(t, findObject(t, arrayObjects(t, order, "services"), "id", orderServiceA), "status", "IN_PROGRESS")
			admin.call(t, "POST", startPath, versionBody(t, order), 409)
			admin.call(t, "POST", orderPath+"/services/"+orderServiceB+"/start", stale, 409)
			assertUnchanged(t, admin, orderPath, order)
			assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+partAID, nil, 200), "balance", 3)
			assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+partBID, nil, 200), "balance", 0)
			assertConsumption(t, admin, partAID, stringField(t, order, "id"), 1, -2)
			assertConsumption(t, admin, partBID, stringField(t, order, "id"), 1, -1)
			decision(t, customerAPI, quote3, "APPROVED", 200)
			assertUnchanged(t, admin, orderPath, order)
		})
		run("finish_services_complete_order_and_deliver", func(t *testing.T) {
			admin.call(t, "POST", orderPath+"/complete", versionBody(t, order), 409)
			admin.call(t, "POST", orderPath+"/deliver", versionBody(t, order), 409)
			admin.call(t, "POST", orderPath+"/diagnosis/reopen", versionBody(t, order), 409)
			admin.call(t, "POST", orderPath+"/services/"+orderServiceB+"/complete", versionBody(t, order), 409)
			order = admin.call(t, "POST", orderPath+"/services/"+orderServiceB+"/start", versionBody(t, order), 200)
			assertConsumption(t, admin, partAID, stringField(t, order, "id"), 1, -2)
			for _, serviceID := range []string{orderServiceA, orderServiceB} {
				order = admin.call(t, "POST", orderPath+"/services/"+serviceID+"/complete", versionBody(t, order), 200)
				service := findObject(t, arrayObjects(t, order, "services"), "id", serviceID)
				assertField(t, service, "status", "COMPLETED")
				assertDuration(t, service, "startedAt", "completedAt")
			}
			assertField(t, order, "status", "IN_EXECUTION")
			order = admin.call(t, "POST", orderPath+"/complete", versionBody(t, order), 200)
			assertField(t, order, "status", "COMPLETED")
			assertField(t, order, "deliveredAt", nil)
			assertDuration(t, order, "executionStartedAt", "completedAt")
			stale := versionBody(t, order)
			order = admin.call(t, "POST", orderPath+"/deliver", stale, 200)
			assertField(t, order, "status", "DELIVERED")
			assertDuration(t, order, "completedAt", "deliveredAt")
			admin.call(t, "POST", orderPath+"/deliver", stale, 409)
			assertUnchanged(t, admin, orderPath, order)
		})
		run("execution_metrics_and_empty_intervals", func(t *testing.T) {
			metrics := admin.call(t, "GET", "/admin/metrics/service-orders"+interval, nil, 200)
			if numberField(t, metrics, "completedCount") < numberField(t, baseline, "completedCount")+1 {
				t.Fatal("métrica de OS não contabilizou a OS entregue pela data de finalização")
			}
			assertNonnegative(t, metrics, "meanExecutionSeconds")
			counts := objectField(t, metrics, "currentStatusCounts")
			for _, state := range []string{"RECEIVED", "IN_DIAGNOSIS", "AWAITING_APPROVAL", "IN_EXECUTION", "COMPLETED", "DELIVERED"} {
				assertNonnegative(t, counts, state)
			}
			for _, id := range []string{serviceAID, serviceBID} {
				metric := findServiceMetric(t, admin, interval, id)
				assertInt(t, metric, "completedCount", 1)
				assertNonnegative(t, metric, "meanExecutionSeconds")
			}
			empty := "?from=2199-01-01T00%3A00%3A00Z&to=2199-01-02T00%3A00%3A00Z"
			metrics = admin.call(t, "GET", "/admin/metrics/service-orders"+empty, nil, 200)
			assertInt(t, metrics, "completedCount", 0)
			assertField(t, metrics, "meanExecutionSeconds", nil)
			metric := findServiceMetric(t, admin, empty, serviceAID)
			assertInt(t, metric, "completedCount", 0)
			assertField(t, metric, "meanExecutionSeconds", nil)
		})
		run("token_rotation_transfer_and_preserved_history", func(t *testing.T) {
			admin.call(t, "POST", orderPath+"/customer-access", object{"expectedVersion": numberField(t, order, "version") - 1}, 409)
			customerAPI.call(t, "GET", "/customer/service-order", nil, 200)
			access := admin.call(t, "POST", orderPath+"/customer-access", versionBody(t, order), 201)
			customerAPI.call(t, "GET", "/customer/service-order", nil, 401)
			rotated := stringField(t, access, "token")
			if rotated == customerAPI.token {
				t.Fatal("rotação reutilizou o token anterior")
			}
			customerAPI.token = rotated
			admin.call(t, "PATCH", "/admin/vehicles/"+vehicleID, object{"customerId": newOwner["id"]}, 200)
			order = admin.call(t, "GET", orderPath, nil, 200)
			assertField(t, order, "customerId", customerID)
			assertField(t, order, "vehicleId", vehicleID)
			assertField(t, order, "customerName", customer["name"])
			assertField(t, order, "vehiclePlate", vehicle["plate"])
			assertSnapshot(t, order, vehicle)
			visible := customerAPI.call(t, "GET", "/customer/service-order", nil, 200)
			assertInt(t, visible, "number", numberField(t, order, "number"))
			assertSnapshot(t, visible, vehicle)
			assertCustomerProjection(t, visible)
			newOrder := admin.call(t, "POST", "/admin/service-orders", orderBody(stringField(t, newOwner, "id"), vehicleID, []string{serviceAID}, []object{}), 201)
			newPath := "/admin/service-orders/" + stringField(t, newOrder, "id")
			newOrder = admin.call(t, "POST", newPath+"/diagnosis/start", versionBody(t, newOrder), 200)
			newOrder = admin.call(t, "POST", newPath+"/quotes", versionBody(t, newOrder), 200)
			decision(t, customerAPI, objectField(t, newOrder, "currentQuote"), "APPROVED", 404)
			for _, path := range []string{"/admin/customers/" + customerID, "/admin/services/" + serviceAID, "/admin/inventory-items/" + partAID} {
				admin.call(t, "DELETE", path, nil, 204)
			}
			order = admin.call(t, "GET", orderPath, nil, 200)
			assertField(t, order, "status", "DELIVERED")
			assertInt(t, objectField(t, order, "currentQuote"), "totalCents", 31500)
			assertCustomerProjection(t, customerAPI.call(t, "GET", "/customer/service-order", nil, 200))
		})
	})

	t.Run("two_orders_compete_for_one_stock_unit", func(t *testing.T) {
		item := createInventory(t, admin, 500, 1)
		service := createService(t, admin, 1000)
		itemID := stringField(t, item, "id")
		orders, customers, paths, bodies := make([]object, 2), make([]apiClient, 2), make([]string, 2), make([]object, 2)
		for i := range orders {
			customer := createCustomer(t, admin, "CPF")
			vehicle := createVehicle(t, admin, stringField(t, customer, "id"))
			orders[i] = admin.call(t, "POST", "/admin/service-orders", orderBody(stringField(t, customer, "id"), stringField(t, vehicle, "id"), []string{stringField(t, service, "id")}, []object{{"inventoryItemId": itemID, "quantity": 1}}), 201)
			path := "/admin/service-orders/" + stringField(t, orders[i], "id")
			orders[i] = admin.call(t, "POST", path+"/diagnosis/start", versionBody(t, orders[i]), 200)
			orders[i] = admin.call(t, "POST", path+"/quotes", versionBody(t, orders[i]), 200)
			access := admin.call(t, "POST", path+"/customer-access", versionBody(t, orders[i]), 201)
			customers[i] = admin
			customers[i].token = stringField(t, access, "token")
			visible := customers[i].call(t, "GET", "/customer/service-order", nil, 200)
			assertInt(t, visible, "number", numberField(t, orders[i], "number"))
			assertCustomerProjection(t, visible)
			decision(t, customers[i], objectField(t, orders[i], "currentQuote"), "APPROVED", 200)
			orders[i] = admin.call(t, "GET", path, nil, 200)
			paths[i] = path + "/services/" + stringField(t, arrayObjects(t, orders[i], "services")[0], "id") + "/start"
			bodies[i] = versionBody(t, orders[i])
		}
		assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+itemID, nil, 200), "balance", 1)
		concurrentRequests(t, admin, paths, bodies)
		assertInt(t, admin.call(t, "GET", "/admin/inventory-items/"+itemID, nil, 200), "balance", 0)
		started, waiting := 0, 0
		for i, before := range orders {
			orderID := stringField(t, before, "id")
			after := admin.call(t, "GET", "/admin/service-orders/"+orderID, nil, 200)
			switch stringField(t, after, "status") {
			case "IN_EXECUTION":
				started++
				assertInt(t, after, "version", numberField(t, before, "version")+1)
				assertConsumption(t, admin, itemID, orderID, 1, -1)
			case "AWAITING_APPROVAL":
				waiting++
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("OS concorrente %d sofreu alterações parciais após falta de estoque", i)
				}
				assertField(t, objectField(t, after, "currentQuote"), "status", "APPROVED")
				assertConsumption(t, admin, itemID, orderID, 0, 0)
			default:
				t.Fatal("estado inesperado após disputa de estoque")
			}
		}
		if started != 1 || waiting != 1 {
			t.Fatalf("disputa deve iniciar uma OS e preservar outra; iniciadas=%d aguardando=%d", started, waiting)
		}
		decision(t, customers[0], objectField(t, orders[1], "currentQuote"), "APPROVED", 404)
	})
}

func (a apiClient) request(method, path string, body any) (apiResponse, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return apiResponse{}, fmt.Errorf("codificar requisição: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, a.base+path, payload)
	if err != nil {
		return apiResponse{}, fmt.Errorf("criar requisição: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	response, err := a.http.Do(req)
	if err != nil {
		return apiResponse{}, fmt.Errorf("executar requisição: %w", err)
	}
	defer response.Body.Close()
	result := apiResponse{status: response.StatusCode, contentType: response.Header.Get("Content-Type")}
	if response.StatusCode != http.StatusNoContent {
		decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
		decoder.UseNumber()
		if err := decoder.Decode(&result.body); err != nil {
			return result, fmt.Errorf("resposta HTTP %d não contém objeto JSON válido", response.StatusCode)
		}
	}
	return result, nil
}

func (a apiClient) call(t *testing.T, method, path string, body any, expected int) object {
	t.Helper()
	response, err := a.request(method, path, body)
	if err != nil {
		t.Fatalf("%s %s: %v", method, safePath(path), err)
	}
	assertResponse(t, method, path, response, expected)
	return response.body
}

func safePath(path string) string { return strings.SplitN(path, "?", 2)[0] }

func assertResponse(t *testing.T, method, path string, response apiResponse, expected int) {
	t.Helper()
	if response.status != expected {
		t.Fatalf("%s %s: HTTP %d; esperado %d (corpo omitido para preservar credenciais)", method, safePath(path), response.status, expected)
	}
	if expected >= 400 {
		if !strings.HasPrefix(response.contentType, "application/problem+json") {
			t.Fatalf("%s %s: erro deve usar application/problem+json", method, safePath(path))
		}
		assertInt(t, response.body, "status", int64(expected))
		for _, key := range []string{"type", "title", "detail", "code"} {
			stringField(t, response.body, key)
		}
	}
}

func concurrentRequests(t *testing.T, api apiClient, paths []string, bodies []object) {
	t.Helper()
	type outcome struct {
		index    int
		response apiResponse
		err      error
	}
	gate, ready, results := make(chan struct{}), make(chan struct{}, len(paths)), make(chan outcome, len(paths))
	for i := range paths {
		go func(i int) {
			ready <- struct{}{}
			<-gate
			response, err := api.request("POST", paths[i], bodies[i])
			results <- outcome{i, response, err}
		}(i)
	}
	for range paths {
		<-ready
	}
	close(gate)
	successes, conflicts := 0, 0
	for range paths {
		result := <-results
		if result.err != nil {
			t.Errorf("requisição concorrente %d: %v", result.index, result.err)
			continue
		}
		switch result.response.status {
		case 200:
			successes++
		case 409:
			conflicts++
			assertResponse(t, "POST", paths[result.index], result.response, 409)
		default:
			t.Errorf("requisição concorrente %d retornou HTTP %d", result.index, result.response.status)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("esperado um sucesso e um conflito; sucessos=%d conflitos=%d", successes, conflicts)
	}
}

func createCustomer(t *testing.T, api apiClient, documentType string) object {
	t.Helper()
	document := syntheticCPF(t)
	if documentType == "CNPJ" {
		document = syntheticCNPJ(t)
	}
	formatted := document[:3] + "." + document[3:6] + "." + document[6:9] + "-" + document[9:]
	if documentType == "CNPJ" {
		formatted = strings.ToLower(document[:2] + "." + document[2:5] + "." + document[5:8] + "/" + document[8:12] + "-" + document[12:])
	}
	result := api.call(t, "POST", "/admin/customers", object{"name": "Cliente sintético " + unique(t), "documentType": documentType, "document": " " + formatted + " ", "email": "teste@example.invalid", "phone": "11999990000"}, 201)
	assertField(t, result, "document", document)
	assertField(t, result, "active", true)
	stringField(t, result, "id")
	return result
}

func createVehicle(t *testing.T, api apiClient, customerID string) object {
	t.Helper()
	plate := syntheticPlate(t)
	result := api.call(t, "POST", "/admin/vehicles", object{"customerId": customerID, "plate": strings.ToLower(plate[:3] + "-" + plate[3:]), "brand": "Marca sintética", "model": "Modelo sintético", "year": 2024}, 201)
	assertField(t, result, "plate", plate)
	return result
}

func createService(t *testing.T, api apiClient, price int64) object {
	t.Helper()
	return api.call(t, "POST", "/admin/services", object{"name": "Serviço sintético " + unique(t), "description": "Integração HTTP", "priceCents": price}, 201)
}

func createInventory(t *testing.T, api apiClient, price, quantity int64) object {
	t.Helper()
	return api.call(t, "POST", "/admin/inventory-items", object{"sku": "TEST-" + unique(t), "name": "Peça sintética " + unique(t), "type": "PART", "priceCents": price, "initialQuantity": quantity}, 201)
}

func orderBody(customer, vehicle string, services []string, parts []object) object {
	return object{"customerId": customer, "vehicleId": vehicle, "problemDescription": "Verificação sintética de integração", "serviceIds": services, "parts": parts}
}

func versionBody(t *testing.T, order object) object {
	t.Helper()
	return object{"expectedVersion": numberField(t, order, "version")}
}

func decision(t *testing.T, api apiClient, quote object, value string, status int) object {
	t.Helper()
	return api.call(t, "POST", "/customer/service-order/quote-decision", object{"quoteId": stringField(t, quote, "id"), "decision": value}, status)
}

func assertUnchanged(t *testing.T, api apiClient, path string, before object) {
	t.Helper()
	if !reflect.DeepEqual(before, api.call(t, "GET", path, nil, 200)) {
		t.Fatal("operação rejeitada ou idempotente alterou o estado persistido da OS")
	}
}

func assertSnapshot(t *testing.T, order, vehicle object) {
	t.Helper()
	snapshot := objectField(t, order, "vehicle")
	for _, key := range []string{"plate", "brand", "model", "year"} {
		assertField(t, snapshot, key, vehicle[key])
	}
}

func assertCustomerProjection(t *testing.T, value any) {
	t.Helper()
	forbidden := map[string]bool{"document": true, "documentType": true, "email": true, "phone": true, "customerId": true, "customerName": true, "vehicleId": true, "inventoryItemId": true, "serviceId": true, "serviceOrderServiceId": true, "sku": true, "balance": true, "stockConsumedAt": true, "quotes": true, "password": true, "passwordHash": true, "token": true, "accessToken": true, "tokenHash": true, "username": true, "employeeId": true}
	var visit func(any)
	visit = func(node any) {
		switch current := node.(type) {
		case map[string]any:
			for key, child := range current {
				if forbidden[key] {
					t.Errorf("projeção do cliente contém campo administrativo proibido: %s", key)
				}
				visit(child)
			}
		case []any:
			for _, child := range current {
				visit(child)
			}
		}
	}
	visit(value)
}

func assertConsumption(t *testing.T, api apiClient, itemID, orderID string, count int, delta int64) {
	t.Helper()
	page := api.call(t, "GET", "/admin/inventory-items/"+itemID+"/movements?pageSize=100", nil, 200)
	actualCount, actualDelta := 0, int64(0)
	for _, movement := range arrayObjects(t, page, "data") {
		if movement["serviceOrderId"] == orderID {
			assertField(t, movement, "type", "SERVICE_ORDER_CONSUMPTION")
			actualCount++
			actualDelta += numberField(t, movement, "quantityDelta")
		}
	}
	if actualCount != count || actualDelta != delta {
		t.Fatalf("baixas de estoque: quantidade=%d delta=%d; esperado quantidade=%d delta=%d", actualCount, actualDelta, count, delta)
	}
}

func findServiceMetric(t *testing.T, api apiClient, interval, serviceID string) object {
	t.Helper()
	for page := 1; ; page++ {
		result := api.call(t, "GET", fmt.Sprintf("/admin/metrics/service-execution%s&pageSize=100&page=%d", interval, page), nil, 200)
		for _, metric := range arrayObjects(t, result, "data") {
			if metric["serviceId"] == serviceID {
				return metric
			}
		}
		if int64(page*100) >= numberField(t, result, "total") {
			t.Fatal("serviço criado pelo teste ausente das métricas")
		}
	}
}

func stringField(t *testing.T, value object, key string) string {
	t.Helper()
	result, ok := value[key].(string)
	if !ok || result == "" {
		t.Fatalf("campo %s deve ser string não vazia", key)
	}
	return result
}

func numberField(t *testing.T, value object, key string) int64 {
	t.Helper()
	switch v := value[key].(type) {
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return n
		}
	case int64:
		return v
	case int:
		return int64(v)
	}
	t.Fatalf("campo %s deve ser inteiro", key)
	return 0
}

func objectField(t *testing.T, value object, key string) object {
	t.Helper()
	result, ok := value[key].(map[string]any)
	if !ok {
		t.Fatalf("campo %s deve ser objeto", key)
	}
	return result
}

func arrayObjects(t *testing.T, value object, key string) []object {
	t.Helper()
	array, ok := value[key].([]any)
	if !ok {
		t.Fatalf("campo %s deve ser array", key)
	}
	result := make([]object, 0, len(array))
	for _, element := range array {
		item, ok := element.(map[string]any)
		if !ok {
			t.Fatalf("campo %s deve conter objetos", key)
		}
		result = append(result, item)
	}
	return result
}

func findObject(t *testing.T, values []object, key string, expected any) object {
	t.Helper()
	for _, value := range values {
		if value[key] == expected {
			return value
		}
	}
	t.Fatalf("nenhum item encontrado com o campo %s esperado", key)
	return nil
}

func assertField(t *testing.T, value object, key string, expected any) {
	t.Helper()
	actual, exists := value[key]
	if !exists || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("campo %s difere do valor esperado", key)
	}
}

func assertInt(t *testing.T, value object, key string, expected int64) {
	t.Helper()
	if actual := numberField(t, value, key); actual != expected {
		t.Fatalf("campo %s=%d; esperado %d", key, actual, expected)
	}
}

func assertNonnegative(t *testing.T, value object, key string) {
	t.Helper()
	number, ok := value[key].(json.Number)
	if !ok {
		t.Fatalf("campo %s deve ser número não nulo", key)
	}
	v, err := number.Float64()
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		t.Fatalf("campo %s deve ser número finito não negativo", key)
	}
}

func assertTimestamp(t *testing.T, value object, key string) time.Time {
	t.Helper()
	stamp := stringField(t, value, key)
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatalf("campo %s deve ser uma data RFC 3339", key)
	}
	_, offset := parsed.Zone()
	if offset != 0 {
		t.Fatalf("campo %s deve usar UTC", key)
	}
	return parsed
}

func assertDuration(t *testing.T, value object, start, end string) {
	t.Helper()
	if assertTimestamp(t, value, end).Before(assertTimestamp(t, value, start)) {
		t.Fatalf("campo %s precede %s", end, start)
	}
}

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal("falha ao gerar dados sintéticos aleatórios")
	}
	return value
}

func unique(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%x", randomBytes(t, 12))
}

func syntheticCPF(t *testing.T) string {
	t.Helper()
	value := randomBytes(t, 9)
	for i := range value {
		value[i] = '0' + value[i]%10
	}
	if bytes.Count(value, value[:1]) == len(value) {
		value[0] = '0' + (value[0]-'0'+1)%10
	}
	for len(value) < 11 {
		sum := 0
		for i, digit := range value {
			sum += int(digit-'0') * (len(value) + 1 - i)
		}
		digit := 11 - sum%11
		if digit >= 10 {
			digit = 0
		}
		value = append(value, byte('0'+digit))
	}
	return string(value)
}

func syntheticCNPJ(t *testing.T) string {
	t.Helper()
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	value := randomBytes(t, 12)
	for i := range value {
		value[i] = alphabet[int(value[i])%len(alphabet)]
	}
	value[0] = 'T'
	for len(value) < 14 {
		sum, weight := 0, 2
		for i := len(value) - 1; i >= 0; i-- {
			sum += int(value[i]-'0') * weight
			weight++
			if weight == 10 {
				weight = 2
			}
		}
		digit := 11 - sum%11
		if digit >= 10 {
			digit = 0
		}
		value = append(value, byte('0'+digit))
	}
	return string(value)
}

func syntheticPlate(t *testing.T) string {
	t.Helper()
	value := randomBytes(t, 7)
	for i := range value {
		if i < 3 || i == 4 {
			value[i] = 'A' + value[i]%26
		} else {
			value[i] = '0' + value[i]%10
		}
	}
	return string(value)
}
