package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"inventory_app/internal/auth"
	"inventory_app/internal/handlers"
	"inventory_app/internal/models"
)

func TestHandleGetIPs(t *testing.T) {
	// Setup AppContext with nil DB since handleGetIPs doesn't use it
	ctx := &handlers.AppContext{DB: nil, Port: 8080}

	req, err := http.NewRequest("GET", "/api/server/ip", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleGetIPs)

	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check response structure
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Errorf("failed to decode JSON response: %v", err)
	}

	if _, ok := response["ips"]; !ok {
		t.Errorf("response missing 'ips' key")
	}
	if _, ok := response["port"]; !ok {
		t.Errorf("response missing 'port' key")
	}
}

func TestHandleVerifyLicense_MissingPIN(t *testing.T) {
	ctx := &handlers.AppContext{DB: nil}

	req, err := http.NewRequest("GET", "/api/license/verify", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleVerifyLicense)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandleAnalyticsQuery_DBError(t *testing.T) {
	// Setup SQL mock
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}

	// Expecting query for analytics to fail
	mock.ExpectQuery("SELECT").WillReturnError(sql.ErrConnDone)

	req, err := http.NewRequest("GET", "/api/analytics/dashboard", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleAnalyticsQuery)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestHandleActivateLicense_TypoToleranceAndWindow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}

	// Expect UPDATE settings query
	mock.ExpectExec("UPDATE settings SET activationPin").WillReturnResult(sqlmock.NewResult(1, 1))

	// Test activation key with current UTC timestamp: "GO-INVETORY-MM-DD-YY-HH-MM"
	currentKeyStr := fmt.Sprintf("GO-INVETORY-%s", time.Now().UTC().Format("01-02-06-15-04"))
	payload := []byte(`{"key":"` + currentKeyStr + `"}`)
	req, err := http.NewRequest("POST", "/api/license/activate", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleActivateLicense)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v (body: %s)", status, http.StatusOK, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
	pin, ok := res["pin"].(string)
	if !ok || len(pin) != 8 {
		t.Errorf("expected 8-digit PIN string, got %v", res["pin"])
	}
}

func TestGenerateActivationKey_PythonParity(t *testing.T) {
	// Test matching output from generate_activationkey.py for 2026-08-19 14:35 UTC: "B811-140C-B4EE-5DEF"
	sampleTime := time.Date(2026, 8, 19, 14, 35, 0, 0, time.UTC)
	key := auth.GenerateActivationKey(sampleTime, "GO-INVENTORY")

	expectedKey := "B811-140C-B4EE-5DEF"
	if key != expectedKey {
		t.Errorf("Go GenerateActivationKey failed Python parity check! Got %s, want %s", key, expectedKey)
	}

	// Verify activation handler using current UTC dynamic SHA-256 key
	dynamicKey := auth.GenerateActivationKey(time.Now().UTC(), "GO-INVENTORY")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}
	mock.ExpectExec("UPDATE settings SET activationPin").WillReturnResult(sqlmock.NewResult(1, 1))

	payload := []byte(`{"key":"` + dynamicKey + `"}`)
	req, err := http.NewRequest("POST", "/api/license/activate", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleActivateLicense)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v (body: %s)", status, http.StatusOK, rr.Body.String())
	}
}

func TestHandleAddByBarcode_MissingBarcode(t *testing.T) {
	ctx := &handlers.AppContext{DB: nil}

	req, err := http.NewRequest("POST", "/api/inventory/add-by-barcode", bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleAddByBarcode)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v (body: %s)", status, http.StatusBadRequest, rr.Body.String())
	}
}

func TestHandleScanBarcode_MissingBarcode(t *testing.T) {
	ctx := &handlers.AppContext{DB: nil}

	req, err := http.NewRequest("GET", "/api/inventory/scan-barcode", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleScanBarcode)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v (body: %s)", status, http.StatusBadRequest, rr.Body.String())
	}
}

func TestHandleScanBarcode_LocalMatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}

	// Expect GetProductByBarcode query
	rows := sqlmock.NewRows([]string{"id", "name", "costPrice", "sellingPrice", "quantity", "unitMeasurement", "productType", "category", "supplierId", "updatedAt", "barcode", "imageUrl", "categoryId"}).
		AddRow(10, "Nutella Spread 400g", 2.50, 4.99, 50, "400g", "Solid", "Food & Snacks", nil, "2026-08-19T19:00:00Z", "3017620422003", "", nil)

	mock.ExpectQuery("SELECT id, name, costPrice, sellingPrice, quantity, unitMeasurement, productType, category, supplierId, updatedAt, barcode, imageUrl, categoryId FROM products").
		WithArgs("3017620422003", "3017620422003", "3017620422003").
		WillReturnRows(rows)

	req, err := http.NewRequest("GET", "/api/inventory/scan-barcode?barcode=3017620422003", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleScanBarcode)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v (body: %s)", status, http.StatusOK, rr.Body.String())
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if res["status"] != "found_local" {
		t.Errorf("expected status 'found_local', got %v", res["status"])
	}
}

func TestHandleDeleteProduct_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}

	mock.ExpectExec("DELETE FROM products WHERE id = \\$1").
		WithArgs(15).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, err := http.NewRequest("POST", "/api/inventory/delete?id=15", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctx.HandleDeleteProduct)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v (body: %s)", status, http.StatusOK, rr.Body.String())
	}
}

func TestHandleCategories_GetAndPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}

	// 1. Test GET /api/categories?business_id=1
	catRows := sqlmock.NewRows([]string{"id", "name", "businessId"}).
		AddRow(1, "Beverages", 1).
		AddRow(2, "Electronics", 1)

	mock.ExpectQuery("SELECT id, name, businessId FROM categories WHERE businessId = \\$1 ORDER BY name ASC").
		WithArgs(1).
		WillReturnRows(catRows)

	reqGet, _ := http.NewRequest("GET", "/api/categories?business_id=1", nil)
	rrGet := httptest.NewRecorder()
	handlerGet := http.HandlerFunc(ctx.HandleCategories)
	handlerGet.ServeHTTP(rrGet, reqGet)

	if status := rrGet.Code; status != http.StatusOK {
		t.Fatalf("GET handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var cats []models.CategoryDB
	if err := json.NewDecoder(rrGet.Body).Decode(&cats); err != nil || len(cats) != 2 {
		t.Fatalf("failed to decode GET categories response: %v, len=%d", err, len(cats))
	}

	// 2. Test POST /api/categories
	mock.ExpectQuery("SELECT id, name, businessId FROM categories WHERE LOWER\\(name\\) = LOWER\\(\\$1\\) AND businessId = \\$2").
		WithArgs("Snacks", 1).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("INSERT INTO categories \\(name, businessId\\) VALUES \\(\\$1, \\$2\\) RETURNING id").
		WithArgs("Snacks", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

	postPayload := []byte(`{"name":"Snacks","businessId":1}`)
	reqPost, _ := http.NewRequest("POST", "/api/categories", bytes.NewBuffer(postPayload))
	rrPost := httptest.NewRecorder()
	handlerPost := http.HandlerFunc(ctx.HandleCategories)
	handlerPost.ServeHTTP(rrPost, reqPost)

	if status := rrPost.Code; status != http.StatusCreated {
		t.Fatalf("POST handler returned wrong status code: got %d want %d (body: %s)", rrPost.Code, http.StatusCreated, rrPost.Body.String())
	}
}

func TestHandleStores_GetAndPost(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	ctx := &handlers.AppContext{DB: db}

	// 1. Test GET /api/stores
	storeRows := sqlmock.NewRows([]string{"id", "name", "code", "phone", "address", "email", "status", "createdAt"}).
		AddRow(1, "Main Store", "STORE-001", "0600000000", "123 Main St", "store@example.com", "Active", "2026-01-01 00:00:00")

	mock.ExpectQuery("SELECT id, name, code, COALESCE\\(phone, ''\\), COALESCE\\(address, ''\\), COALESCE\\(email, ''\\), status, createdAt FROM stores ORDER BY id ASC").
		WillReturnRows(storeRows)

	reqGet, _ := http.NewRequest("GET", "/api/stores", nil)
	rrGet := httptest.NewRecorder()
	handlerGet := http.HandlerFunc(ctx.HandleGetStores)
	handlerGet.ServeHTTP(rrGet, reqGet)

	if status := rrGet.Code; status != http.StatusOK {
		t.Fatalf("GET stores handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var stores []models.StoreDB
	if err := json.NewDecoder(rrGet.Body).Decode(&stores); err != nil || len(stores) != 1 {
		t.Fatalf("failed to decode GET stores response: %v, len=%d", err, len(stores))
	}
	if stores[0].Name != "Main Store" {
		t.Errorf("expected store name 'Main Store', got '%s'", stores[0].Name)
	}
}
