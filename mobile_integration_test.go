package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"inventory_app/internal/db"
	"inventory_app/internal/handlers"
	"inventory_app/internal/models"
)

func TestMobileAppAPIIntegration_AllEndpoints(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	ctx := &handlers.AppContext{DB: sqlDB, Port: 8080}

	t.Run("1. GET /api/server/ip", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/server/ip", nil)
		rec := httptest.NewRecorder()
		ctx.HandleGetIPs(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("2. GET /api/stores", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "code", "phone", "address", "email", "status", "createdAt"}).
			AddRow(1, "Main Store", "STORE-001", "0600000000", "123 Main St", "store@example.com", "Active", "2026-01-01 00:00:00")
		mock.ExpectQuery("SELECT id, name, code, COALESCE\\(phone, ''\\), COALESCE\\(address, ''\\), COALESCE\\(email, ''\\), status, createdAt FROM stores ORDER BY id ASC").
			WillReturnRows(rows)

		req := httptest.NewRequest("GET", "/api/stores", nil)
		rec := httptest.NewRecorder()
		ctx.HandleGetStores(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("3. POST /api/stores/update", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO stores").
			WithArgs("Branch Store", "STORE-002", "0611111111", "456 Market St", "branch@example.com", "Active", sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))

		payload := []byte(`{"name":"Branch Store","code":"STORE-002","phone":"0611111111","address":"456 Market St","email":"branch@example.com"}`)
		req := httptest.NewRequest("POST", "/api/stores/update", bytes.NewBuffer(payload))
		rec := httptest.NewRecorder()
		ctx.HandleUpdateStore(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("4. GET /api/categories", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "businessId"}).
			AddRow(1, "General", 1).
			AddRow(2, "Electronics", 1)
		mock.ExpectQuery("SELECT id, name, businessId FROM categories WHERE businessId = \\$1 ORDER BY name ASC").
			WithArgs(1).
			WillReturnRows(rows)

		req := httptest.NewRequest("GET", "/api/categories?business_id=1", nil)
		rec := httptest.NewRecorder()
		ctx.HandleCategories(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("5. POST /api/categories", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, name, businessId FROM categories WHERE LOWER\\(name\\) = LOWER\\(\\$1\\) AND businessId = \\$2").
			WithArgs("Beverages", 1).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery("INSERT INTO categories \\(name, businessId\\) VALUES \\(\\$1, \\$2\\) RETURNING id").
			WithArgs("Beverages", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

		payload := []byte(`{"name":"Beverages","businessId":1}`)
		req := httptest.NewRequest("POST", "/api/categories", bytes.NewBuffer(payload))
		rec := httptest.NewRecorder()
		ctx.HandleCategories(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("6. GET /api/suppliers", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "phone", "email", "address", "storeId"}).
			AddRow(1, "Global Wholesale", "0622222222", "contact@global.com", "789 Supply Rd", 1)
		mock.ExpectQuery("SELECT id, name, phone, email, address, COALESCE\\(storeId, 1\\) FROM suppliers").
			WillReturnRows(rows)

		req := httptest.NewRequest("GET", "/api/suppliers", nil)
		rec := httptest.NewRecorder()
		ctx.HandleGetSuppliers(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("7. GET /api/settings", func(t *testing.T) {
		// Settings query
		rows := sqlmock.NewRows([]string{"companyName", "phone", "address", "logo", "ice", "taxPercentage", "displayById", "activationPin", "licenseActivated", "receiptFooterMessage", "storeId"}).
			AddRow("GoInvent Shop", "0500000000", "Downtown Plaza", "", "ICE12345", 20.0, 0, "123456", 1, "Thank you!", 1)
		mock.ExpectQuery("SELECT companyName, phone, address, logo, ice, taxPercentage, displayById, activationPin, licenseActivated, receiptFooterMessage, COALESCE\\(storeId, 1\\) FROM settings").
			WithArgs(1).
			WillReturnRows(rows)

		// Stores lookup query
		storeRows := sqlmock.NewRows([]string{"id", "name", "phone", "address", "email"}).
			AddRow(1, "GoInvent Shop", "0500000000", "Downtown Plaza", "")
		mock.ExpectQuery("SELECT id, name, COALESCE\\(phone, ''\\), COALESCE\\(address, ''\\), COALESCE\\(email, ''\\) FROM stores WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(storeRows)

		req := httptest.NewRequest("GET", "/api/settings", nil)
		rec := httptest.NewRecorder()
		ctx.HandleGetSettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("8. GET /api/inventory/sync", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "costPrice", "sellingPrice", "quantity", "unitMeasurement", "productType", "category", "supplierId", "updatedAt", "barcode", "imageUrl", "categoryId", "storeId"}).
			AddRow(1, "Coffee Beans", 5.0, 12.0, 100, "kg", "Solid", "Beverages", 1, "2026-01-01 00:00:00", "12345678", "http://example.com/coffee.jpg", 3, 1)
		mock.ExpectQuery("SELECT id, name, costPrice, sellingPrice, quantity, unitMeasurement, productType, category, supplierId, updatedAt, barcode, imageUrl, categoryId, COALESCE\\(storeId, 1\\) FROM products").
			WillReturnRows(rows)

		req := httptest.NewRequest("GET", "/api/inventory/sync", nil)
		rec := httptest.NewRecorder()
		ctx.HandleMobileSync(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var prods []models.Product
		if err := json.NewDecoder(rec.Body).Decode(&prods); err != nil || len(prods) != 1 {
			t.Fatalf("failed to decode inventory sync: %v", err)
		}
		if prods[0].Name != "Coffee Beans" {
			t.Errorf("expected product Coffee Beans, got %s", prods[0].Name)
		}
	})

	t.Run("9. Auth with Phone & Password Not Created", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "name", "email", "phone", "passwordHash", "role", "status", "avatar", "lastActive", "createdAt", "department", "actionsCount"}).
			AddRow("u-new", "New Staff", "staff@store.com", "0699999999", "", "Cashier", "Active", "", "2026-01-01", "2026-01-01", "POS", 0)

		mock.ExpectQuery("SELECT id, name, email, COALESCE\\(phone, ''\\), passwordHash, role, status, avatar, lastActive, createdAt, department, actionsCount FROM users WHERE").
			WithArgs("0699999999", "0699999999", "0699999999").
			WillReturnRows(rows)

		payload := []byte(`{"phone":"0699999999","password":""}`)
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBuffer(payload))
		rec := httptest.NewRecorder()
		ctx.HandleLogin(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		var body map[string]interface{}
		_ = json.NewDecoder(rec.Body).Decode(&body)
		if body["status"] != "PASSWORD_NOT_CREATED" {
			t.Fatalf("expected status PASSWORD_NOT_CREATED, got %v", body["status"])
		}
	})

	t.Run("10. Set Password POST /api/auth/set-password", func(t *testing.T) {
		mock.ExpectExec("UPDATE users SET passwordHash = \\$1 WHERE").
			WithArgs(db.HashPassword("newSecret123"), "0699999999", "0699999999").
			WillReturnResult(sqlmock.NewResult(1, 1))

		payload := []byte(`{"phone":"0699999999","newPassword":"newSecret123"}`)
		req := httptest.NewRequest("POST", "/api/auth/set-password", bytes.NewBuffer(payload))
		rec := httptest.NewRecorder()
		ctx.HandleSetPassword(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
}
