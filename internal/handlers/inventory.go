package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// fetchFromOpenFoodFacts queries Open Food Facts API v2 (https://world.openfoodfacts.org/api/v2/product/{barcode})
func fetchFromOpenFoodFacts(barcode string) (*models.Product, *models.OFFProductItem, error) {
	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		return nil, nil, fmt.Errorf("barcode parameter is required")
	}

	offURL := fmt.Sprintf("https://world.openfoodfacts.org/api/v2/product/%s.json", barcode)
	req, err := http.NewRequest(http.MethodGet, offURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "InvotInventoryPOS/1.0 (contact@invotpos.com)")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Open Food Facts API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, fmt.Errorf("product not found in Open Food Facts database")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("Open Food Facts returned HTTP %d", resp.StatusCode)
	}

	var offData models.OpenFoodFactsResponse
	if err := json.NewDecoder(resp.Body).Decode(&offData); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if offData.Status != 1 || offData.Product == nil {
		return nil, nil, fmt.Errorf("product not found in Open Food Facts database")
	}

	offItem := offData.Product

	pName := offItem.ProductName
	if pName == "" {
		pName = offItem.ProductNameEn
	}
	if pName == "" && offItem.Brands != "" {
		pName = fmt.Sprintf("%s Product", offItem.Brands)
	}
	if pName == "" {
		pName = fmt.Sprintf("Item %s", barcode)
	}

	unit := offItem.Quantity
	if unit == "" {
		unit = "pcs"
	}

	p := &models.Product{
		Barcode:         barcode,
		Name:            pName,
		CostPrice:       2.50,
		SellingPrice:    4.99,
		Quantity:        50,
		UnitMeasurement: unit,
		ProductType:     "Solid",
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	return p, offItem, nil
}

// HandleMobileSync acts as a 2-way catalog synchronizer at /api/inventory/sync
func (ctx *AppContext) HandleMobileSync(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method == http.MethodPost {
		log.Printf("Received SYNC POST request at /api/inventory/sync")

		var products []models.Product
		bodyBytes, _ := io.ReadAll(r.Body)
		log.Printf("Raw Sync Payload: %s", string(bodyBytes))

		if err := json.Unmarshal(bodyBytes, &products); err == nil {
			log.Printf("Successfully parsed %d products from JSON array", len(products))
		} else {
			log.Printf("Sync JSON decoding failed: %v", err)
			middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("Invalid payload format: %v", err),
			})
			return
		}

		storeIDStr := r.URL.Query().Get("store_id")
		if storeIDStr == "" {
			storeIDStr = r.URL.Query().Get("storeId")
		}
		if storeIDStr == "" {
			storeIDStr = r.Header.Get("X-Store-ID")
		}
		storeID, _ := strconv.Atoi(storeIDStr)

		if storeID > 0 {
			for i := range products {
				if products[i].StoreID <= 0 {
					products[i].StoreID = storeID
				}
			}
		}

		if len(products) > 0 {
			if err := db.UpsertProducts(ctx.DB, products); err != nil {
				log.Printf("Error upserting products during sync: %v", err)
				middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to sync inventory products with database"})
				return
			}
			log.Printf("Successfully synchronized %d product catalog items from client", len(products))
		}

		middleware.WriteJSON(w, http.StatusOK, products)
		return
	} else if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	storeIDStr := r.URL.Query().Get("store_id")
	if storeIDStr == "" {
		storeIDStr = r.URL.Query().Get("storeId")
	}
	if storeIDStr == "" {
		storeIDStr = r.Header.Get("X-Store-ID")
	}
	storeID, _ := strconv.Atoi(storeIDStr)

	allProducts, err := db.GetProductsByStore(ctx.DB, storeID)
	if err != nil {
		log.Printf("Error fetching inventory catalog: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch products state from database"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, allProducts)
}

// HandleNewTransaction processes POS receipts and automatically decrements stock at /api/transactions/new
func (ctx *AppContext) HandleNewTransaction(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	log.Printf("New Transaction Payload: %s", string(bodyBytes))

	var req models.NewTransactionRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Printf("Error decoding transaction request: %v", err)
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid transaction payload format: %v", err)})
		return
	}

	if req.ReceiptID == "" || len(req.Items) == 0 {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Receipt ID and transaction items are required"})
		return
	}

	storeIDStr := r.URL.Query().Get("store_id")
	if storeIDStr == "" {
		storeIDStr = r.URL.Query().Get("storeId")
	}
	storeID, _ := strconv.Atoi(storeIDStr)
	if storeID <= 0 {
		storeID = req.StoreID
	}
	if storeID <= 0 {
		storeID = 1
	}

	receipt := models.Receipt{
		ReceiptID:     req.ReceiptID,
		Timestamp:     req.Timestamp,
		Subtotal:      req.Subtotal,
		TaxTotal:      req.TaxTotal,
		GrandTotal:    req.GrandTotal,
		PaymentMethod: req.PaymentMethod,
		StoreID:       storeID,
	}

	if err := db.ProcessTransaction(ctx.DB, receipt, req.Items); err != nil {
		log.Printf("Error processing retail transaction: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("Failed to record transaction: %v", err)})
		return
	}

	log.Printf("Transaction %s successfully processed for store %d.", req.ReceiptID, storeID)
	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"message":   "Transaction registered and stock levels adjusted",
		"receiptId": req.ReceiptID,
	})
}

// HandleDeleteProduct deletes a product catalog item by ID at /api/inventory/delete
func (ctx *AppContext) HandleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	idStr := r.URL.Query().Get("id")

	if idStr == "" {
		type DeleteReq struct {
			ID int `json:"id"`
		}
		var req DeleteReq
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.ID > 0 {
				idStr = fmt.Sprintf("%d", req.ID)
			}
		}
	}

	if idStr == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "ID parameter is required"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Valid ID parameter is required"})
		return
	}

	if err := db.DeleteProductByID(ctx.DB, id); err != nil {
		log.Printf("Error deleting product ID %d: %v", id, err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete product from database"})
		return
	}

	log.Printf("Successfully deleted product catalog item ID %d", id)
	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Product catalog item successfully deleted",
	})
}

// HandleScanBarcode queries Open Food Facts API (https://world.openfoodfacts.org/api/v2/product/{barcode})
// or local database to auto-populate inventory product details by barcode.
func (ctx *AppContext) HandleScanBarcode(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	barcode := r.URL.Query().Get("barcode")
	if barcode == "" {
		barcode = r.URL.Query().Get("code")
	}
	if barcode == "" {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/inventory/scan-barcode/") {
			barcode = strings.TrimPrefix(path, "/api/inventory/scan-barcode/")
		}
	}
	barcode = strings.TrimSpace(barcode)

	if barcode == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Barcode parameter is required"})
		return
	}

	// 1. Check local SQLite DB first
	localProd, err := db.GetProductByBarcode(ctx.DB, barcode)
	if err == nil && localProd != nil {
		middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "found_local",
			"barcode": barcode,
			"product": localProd,
		})
		return
	}

	// 2. Query Open Food Facts API v2 (https://world.openfoodfacts.org/api/v2/product/{barcode})
	prod, offItem, err := fetchFromOpenFoodFacts(barcode)
	if err != nil {
		log.Printf("Open Food Facts scan failed for barcode %s: %v", barcode, err)
		middleware.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "not_found",
			"barcode": barcode,
			"message": "Product not found in Open Food Facts database",
		})
		return
	}

	// If auto_add requested, save to database directly
	autoAdd := r.URL.Query().Get("auto_add") == "true" || r.URL.Query().Get("add") == "true" || r.Method == http.MethodPost
	if autoAdd {
		if err := db.UpsertProducts(ctx.DB, []models.Product{*prod}); err != nil {
			log.Printf("Error auto-adding product from barcode %s: %v", barcode, err)
		}
	}

	category := offItem.Categories
	if category == "" {
		category = "General Goods"
	} else if strings.Contains(category, ",") {
		category = strings.TrimSpace(strings.Split(category, ",")[0])
	}

	unit := offItem.Quantity
	if unit == "" {
		unit = "pcs"
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "found_external",
		"barcode":         barcode,
		"sku":             fmt.Sprintf("SKU-%s", barcode),
		"name":            prod.Name,
		"brand":           offItem.Brands,
		"category":        category,
		"unitMeasurement": unit,
		"costPrice":       prod.CostPrice,
		"sellingPrice":    prod.SellingPrice,
		"quantity":        prod.Quantity,
		"imageUrl":        offItem.ImageFrontURL,
		"source":          fmt.Sprintf("https://world.openfoodfacts.org/api/v2/product/%s", barcode),
		"product":         prod,
	})
}

// HandleAddByBarcode queries Open Food Facts API v2 (https://world.openfoodfacts.org/api/v2/product/{barcode})
// and saves/upserts the scanned product directly into inventory DB.
func (ctx *AppContext) HandleAddByBarcode(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var reqPayload models.AddByBarcodeRequest
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		if len(bodyBytes) > 0 {
			_ = json.Unmarshal(bodyBytes, &reqPayload)
		}
	}

	barcode := reqPayload.Barcode
	if barcode == "" {
		barcode = reqPayload.Code
	}
	if barcode == "" {
		barcode = r.URL.Query().Get("barcode")
	}
	if barcode == "" {
		barcode = r.URL.Query().Get("code")
	}

	// Support path parameters like /api/inventory/add/barcode/{barcode}
	if barcode == "" {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/inventory/add/barcode/") {
			barcode = strings.TrimPrefix(path, "/api/inventory/add/barcode/")
		} else if strings.HasPrefix(path, "/api/inventory/add-by-barcode/") {
			barcode = strings.TrimPrefix(path, "/api/inventory/add-by-barcode/")
		} else if strings.HasPrefix(path, "/api/inventory/scan-and-add/") {
			barcode = strings.TrimPrefix(path, "/api/inventory/scan-and-add/")
		}
	}

	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Barcode parameter is required"})
		return
	}

	// Fetch product details from Open Food Facts API v2
	prod, offItem, err := fetchFromOpenFoodFacts(barcode)
	if err != nil {
		log.Printf("Open Food Facts API v2 lookup failed for barcode %s: %v", barcode, err)
		middleware.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"status":  "not_found",
			"barcode": barcode,
			"error":   err.Error(),
			"message": "Product not found in Open Food Facts database",
		})
		return
	}

	// Apply custom parameter overrides from user payload/query if present
	if reqPayload.Name != "" {
		prod.Name = reqPayload.Name
	}
	if reqPayload.CostPrice > 0 {
		prod.CostPrice = reqPayload.CostPrice
	}
	if reqPayload.SellingPrice > 0 {
		prod.SellingPrice = reqPayload.SellingPrice
	}
	if reqPayload.Quantity > 0 {
		prod.Quantity = reqPayload.Quantity
	}
	if reqPayload.UnitMeasurement != "" {
		prod.UnitMeasurement = reqPayload.UnitMeasurement
	}
	if reqPayload.ProductType != "" {
		prod.ProductType = reqPayload.ProductType
	}
	if reqPayload.Category != "" {
		prod.Category = reqPayload.Category
	}
	if reqPayload.SupplierID > 0 {
		prod.SupplierID = reqPayload.SupplierID
	}

	// Check query string overrides if POST body was empty
	if cp := r.URL.Query().Get("costPrice"); cp != "" {
		if val, e := strconv.ParseFloat(cp, 64); e == nil && val > 0 {
			prod.CostPrice = val
		}
	}
	if sp := r.URL.Query().Get("sellingPrice"); sp != "" {
		if val, e := strconv.ParseFloat(sp, 64); e == nil && val > 0 {
			prod.SellingPrice = val
		}
	}
	if q := r.URL.Query().Get("quantity"); q != "" {
		if val, e := strconv.Atoi(q); e == nil && val > 0 {
			prod.Quantity = val
		}
	}

	prod.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Save to SQLite database
	if err := db.UpsertProducts(ctx.DB, []models.Product{*prod}); err != nil {
		log.Printf("Error saving scanned product %s (%s) to DB: %v", prod.Name, barcode, err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save product in SQLite DB"})
		return
	}

	category := offItem.Categories
	if category == "" {
		category = "General Goods"
	} else if strings.Contains(category, ",") {
		category = strings.TrimSpace(strings.Split(category, ",")[0])
	}

	// Record activity log
	_ = db.LogActivity(ctx.DB, models.ActivityLogDB{
		ID:        fmt.Sprintf("act_%d", time.Now().UnixNano()%100000),
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		UserID:    "usr_system",
		UserName:  "Barcode Scanner",
		Type:      "INVENTORY",
		Action:    "Add Barcode Product",
		Details:   fmt.Sprintf("Scanned barcode %s via Open Food Facts API v2 and added '%s' to inventory DB", barcode, prod.Name),
		Severity:  "success",
	})

	log.Printf("Successfully added/updated inventory product via barcode %s: %s (ID: %d)", barcode, prod.Name, prod.ID)

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"message":  fmt.Sprintf("Product '%s' (barcode: %s) successfully added to inventory DB", prod.Name, barcode),
		"barcode":  barcode,
		"sku":      fmt.Sprintf("SKU-%s", barcode),
		"brand":    offItem.Brands,
		"category": category,
		"imageUrl": offItem.ImageFrontURL,
		"source":   fmt.Sprintf("https://world.openfoodfacts.org/api/v2/product/%s", barcode),
		"product":  prod,
	})
}

// HandleAddProduct inserts or updates an inventory item at /api/inventory/add
func (ctx *AppContext) HandleAddProduct(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var p models.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid product payload"})
		return
	}

	// Auto-fetch from Open Food Facts if barcode is provided and name is empty
	if p.Barcode != "" && p.Name == "" {
		offProd, _, err := fetchFromOpenFoodFacts(p.Barcode)
		if err == nil && offProd != nil {
			p.Name = offProd.Name
			if p.UnitMeasurement == "" {
				p.UnitMeasurement = offProd.UnitMeasurement
			}
		}
	}

	if p.Name == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Product name is required"})
		return
	}

	if p.UpdatedAt == "" {
		p.UpdatedAt = time.Now().Format(time.RFC3339)
	}

	if err := db.UpsertProducts(ctx.DB, []models.Product{p}); err != nil {
		log.Printf("Error adding product %s: %v", p.Name, err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save product in SQLite DB"})
		return
	}

	log.Printf("Successfully added/updated inventory product: %s", p.Name)
	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Product '%s' successfully added to inventory DB", p.Name),
		"product": p,
	})
}

// HandleImageUpload processes multipart form image uploads and saves them locally at /api/inventory/upload
func (ctx *AppContext) HandleImageUpload(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Limit upload size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "File too large or invalid multipart form"})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to get image file from form"})
		return
	}
	defer file.Close()

	// Ensure destination uploads directory exists
	if err := os.MkdirAll("uploads", 0755); err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create uploads directory"})
		return
	}

	// Generate a unique filename using timestamp
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg" // default fallback
	}
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join("uploads", filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save file on server"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to write file"})
		return
	}

	// Return relative URL path so the client can format it with the current baseUrl
	relativeURL := fmt.Sprintf("/uploads/%s", filename)
	middleware.WriteJSON(w, http.StatusOK, map[string]string{
		"status":   "success",
		"imageUrl": relativeURL,
	})
}

// HandleCategories handles GET and POST requests for /api/categories
func (ctx *AppContext) HandleCategories(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method == http.MethodGet {
		businessIDStr := r.URL.Query().Get("business_id")
		if businessIDStr == "" {
			businessIDStr = r.URL.Query().Get("businessId")
		}
		businessID := 1
		if p, err := strconv.Atoi(businessIDStr); err == nil && p > 0 {
			businessID = p
		}

		cats, err := db.GetAllCategories(ctx.DB, businessID)
		if err != nil {
			log.Printf("Error fetching categories for business %d: %v", businessID, err)
			middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch categories"})
			return
		}

		middleware.WriteJSON(w, http.StatusOK, cats)
		return
	} else if r.Method == http.MethodPost {
		var req models.CategoryDB
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			return
		}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid JSON payload: %v", err)})
			return
		}

		if req.Name == "" {
			middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Category name is required"})
			return
		}

		cat, err := db.CreateCategory(ctx.DB, req)
		if err != nil {
			log.Printf("Error creating category: %v", err)
			middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create category"})
			return
		}

		middleware.WriteJSON(w, http.StatusCreated, cat)
		return
	}

	middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
}
