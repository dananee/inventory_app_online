package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"inventory_app/internal/models"
)

// InitDatabase initializes the PostgreSQL database and sets up schemas
func InitDatabase() (*sql.DB, error) {
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		dbUrl = "postgres://postgres:postgres@localhost:5432/inventory_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	schema := `
	CREATE TABLE IF NOT EXISTS stores (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		code TEXT NOT NULL UNIQUE,
		phone TEXT NOT NULL DEFAULT '',
		address TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'Active',
		createdAt TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS suppliers (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		phone TEXT,
		email TEXT,
		address TEXT,
		storeId INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS categories (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		businessId INTEGER NOT NULL DEFAULT 1,
		UNIQUE(name, businessId)
	);

	CREATE TABLE IF NOT EXISTS products (
		id SERIAL PRIMARY KEY,
		barcode TEXT,
		name TEXT NOT NULL,
		costPrice DOUBLE PRECISION NOT NULL DEFAULT 0.0,
		sellingPrice DOUBLE PRECISION NOT NULL DEFAULT 0.0,
		quantity INTEGER NOT NULL,
		unitMeasurement TEXT NOT NULL DEFAULT 'pcs',
		productType TEXT NOT NULL DEFAULT 'Solid',
		category TEXT NOT NULL DEFAULT 'General',
		categoryId INTEGER REFERENCES categories(id),
		supplierId INTEGER REFERENCES suppliers(id),
		storeId INTEGER DEFAULT 1,
		updatedAt TEXT NOT NULL,
		imageUrl TEXT
	);

	CREATE TABLE IF NOT EXISTS receipts (
		receiptId TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		subtotal DOUBLE PRECISION NOT NULL,
		taxTotal DOUBLE PRECISION NOT NULL,
		grandTotal DOUBLE PRECISION NOT NULL,
		paymentMethod TEXT NOT NULL,
		storeId INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS receipt_items (
		id SERIAL PRIMARY KEY,
		receiptId TEXT NOT NULL REFERENCES receipts(receiptId) ON DELETE CASCADE,
		product_id INTEGER NOT NULL REFERENCES products(id),
		quantity INTEGER NOT NULL,
		price DOUBLE PRECISION NOT NULL,
		costPrice DOUBLE PRECISION NOT NULL DEFAULT 0.0
	);

	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		companyName TEXT NOT NULL DEFAULT 'GoInvent Shop',
		phone TEXT NOT NULL DEFAULT '',
		address TEXT NOT NULL DEFAULT '',
		logo TEXT NOT NULL DEFAULT '',
		ice TEXT NOT NULL DEFAULT '',
		taxPercentage DOUBLE PRECISION NOT NULL DEFAULT 0.0,
		displayById INTEGER NOT NULL DEFAULT 0,
		activationPin TEXT NOT NULL DEFAULT '',
		licenseActivated INTEGER NOT NULL DEFAULT 0,
		receiptFooterMessage TEXT NOT NULL DEFAULT 'Thank you for shopping with us!'
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		passwordHash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'Inventory Manager',
		status TEXT NOT NULL DEFAULT 'Active',
		avatar TEXT,
		lastActive TEXT,
		createdAt TEXT NOT NULL,
		department TEXT,
		actionsCount INTEGER NOT NULL DEFAULT 0,
		storeId INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS activity_logs (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		userId TEXT NOT NULL,
		userName TEXT NOT NULL,
		userAvatar TEXT,
		type TEXT NOT NULL,
		action TEXT NOT NULL,
		details TEXT,
		ipAddress TEXT,
		severity TEXT NOT NULL DEFAULT 'info',
		location TEXT,
		storeId INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		userId TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expiresAt TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS payments (
		id SERIAL PRIMARY KEY,
		userId TEXT NOT NULL,
		userName TEXT,
		userEmail TEXT,
		userPhone TEXT,
		planDuration TEXT NOT NULL,
		amount DOUBLE PRECISION NOT NULL DEFAULT 0.0,
		paymentMethod TEXT DEFAULT 'Cash',
		status TEXT DEFAULT 'Paid',
		startDate TEXT NOT NULL,
		endDate TEXT NOT NULL,
		notes TEXT,
		createdAt TEXT NOT NULL
	);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Dynamic column migrations for existing databases
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS costPrice DOUBLE PRECISION NOT NULL DEFAULT 0.0;")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS sellingPrice DOUBLE PRECISION NOT NULL DEFAULT 0.0;")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS supplierId INTEGER REFERENCES suppliers(id);")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS unitMeasurement TEXT NOT NULL DEFAULT 'pcs';")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS productType TEXT NOT NULL DEFAULT 'Solid';")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'General';")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS barcode TEXT;")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS imageUrl TEXT;")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS categoryId INTEGER REFERENCES categories(id);")
	_, _ = db.Exec("ALTER TABLE products ADD COLUMN IF NOT EXISTS storeId INTEGER DEFAULT 1;")

	_, _ = db.Exec("ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS storeId INTEGER DEFAULT 1;")
	_, _ = db.Exec("ALTER TABLE receipts ADD COLUMN IF NOT EXISTS storeId INTEGER DEFAULT 1;")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS storeId INTEGER DEFAULT 1;")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS storeIds TEXT DEFAULT '';")
	_, _ = db.Exec("ALTER TABLE activity_logs ADD COLUMN IF NOT EXISTS storeId INTEGER DEFAULT 1;")

	_, _ = db.Exec("ALTER TABLE receipt_items ADD COLUMN IF NOT EXISTS costPrice DOUBLE PRECISION NOT NULL DEFAULT 0.0;")

	_, _ = db.Exec("ALTER TABLE settings ADD COLUMN IF NOT EXISTS activationPin TEXT NOT NULL DEFAULT '';")
	_, _ = db.Exec("ALTER TABLE settings ADD COLUMN IF NOT EXISTS licenseActivated INTEGER NOT NULL DEFAULT 0;")
	_, _ = db.Exec("ALTER TABLE settings ADD COLUMN IF NOT EXISTS receiptFooterMessage TEXT NOT NULL DEFAULT 'Thank you for shopping with us!';")
	_, _ = db.Exec("ALTER TABLE settings ADD COLUMN IF NOT EXISTS storeId INTEGER DEFAULT 1;")

	// Seed default main store #1 if stores table is empty
	var storeCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM stores").Scan(&storeCount)
	if storeCount == 0 {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		_, _ = db.Exec("INSERT INTO stores (id, name, code, phone, address, email, status, createdAt) VALUES (1, 'Main Store', 'STORE-001', '', '', '', 'Active', $1) ON CONFLICT DO NOTHING", nowStr)
	}

	// Seed default categories if empty for businessId = 1
	var catCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&catCount)
	if catCount == 0 {
		log.Println("Seeding initial default categories into PostgreSQL...")
		defaultCats := []string{"General", "Electronics", "Beverages", "Food & Snacks", "Hardware", "Supplies"}
		for _, catName := range defaultCats {
			_, _ = db.Exec("INSERT INTO categories (name, businessId) VALUES ($1, 1) ON CONFLICT DO NOTHING", catName)
		}
	}

	// Initialize default settings if not exists
	_, _ = db.Exec(`
		INSERT INTO settings (id, companyName, phone, address, logo, ice, taxPercentage, displayById, activationPin, licenseActivated, receiptFooterMessage)
		VALUES (1, 'GoInvent Shop', '', '', '', '', 0.0, 0, '', 0, 'Thank you for shopping with us!')
		ON CONFLICT (id) DO NOTHING;
	`)

	// Remove legacy mock users
	_, _ = db.Exec("DELETE FROM users WHERE email IN ('sarah.chen@invot.io', 'emily.editor@invot.io', 'arthur.auditor@invot.io', 'sam.support@invot.io', 'david.o@invot.io', 'liam.g@external-vendor.com', 'sophia.patel@invot.io', 'viktor.m@invot.io')")

	// Seed default Super Admin user if not existing
	var adminCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE LOWER(email) IN ('admin', 'admin@invot.io') OR role = 'Super Admin'").Scan(&adminCount)
	if adminCount == 0 {
		log.Println("Seeding initial Super Admin account (admin / admin123) into PostgreSQL...")
		adminUser := models.UserDB{
			ID:           "usr_admin",
			Name:         "Super Admin",
			Email:        "admin",
			Role:         "Super Admin",
			Status:       "Active",
			Avatar:       "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='100' height='100' viewBox='0 0 24 24' fill='%23c084fc'%3E%3Cpath d='M12 2a5 5 0 1 0 5 5 5 5 0 0 0-5-5zm0 14a7 7 0 0 0-7 7h14a7 7 0 0 0-7-7z'/%3E%3C/svg%3E",
			LastActive:   time.Now().Format("2006-01-02 15:04:05"),
			CreatedAt:    time.Now().Format("2006-01-02"),
			Department:   "Executive Systems",
			ActionsCount: 1,
		}
		_ = UpsertUser(db, adminUser, "admin123")
	} else {
		_, _ = db.Exec("UPDATE users SET email = 'admin', name = 'Super Admin', avatar = 'data:image/svg+xml,%3Csvg xmlns=''http://www.w3.org/2000/svg'' width=''100'' height=''100'' viewBox=''0 0 24 24'' fill=''%23c084fc''%3E%3Cpath d=''M12 2a5 5 0 1 0 5 5 5 5 0 0 0-5-5zm0 14a7 7 0 0 0-7 7h14a7 7 0 0 0-7-7z''/%3E%3C/svg%3E' WHERE role = 'Super Admin'")
	}

	// Seed default products if empty
	var prodCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&prodCount)
	if prodCount == 0 {
		log.Println("Seeding initial inventory products into PostgreSQL...")
		initialProds := []models.Product{
			{Name: "Invot Wireless Scanner Pro", CostPrice: 45.00, SellingPrice: 89.99, Quantity: 485, UnitMeasurement: "pcs", ProductType: "Solid", UpdatedAt: time.Now().Format(time.RFC3339)},
			{Name: "Thermal Label Printer HD", CostPrice: 120.00, SellingPrice: 199.99, Quantity: 18, UnitMeasurement: "pcs", ProductType: "Solid", UpdatedAt: time.Now().Format(time.RFC3339)},
			{Name: "Heavy Duty Barcode Tags (Pack of 1000)", CostPrice: 8.50, SellingPrice: 19.99, Quantity: 1250, UnitMeasurement: "box", ProductType: "Solid", UpdatedAt: time.Now().Format(time.RFC3339)},
			{Name: "Invot Smart Warehouse Hub Gateway", CostPrice: 180.00, SellingPrice: 299.99, Quantity: 0, UnitMeasurement: "pcs", ProductType: "Solid", UpdatedAt: time.Now().Format(time.RFC3339)},
			{Name: "Ethiopian Yirgacheffe Beans", CostPrice: 18.00, SellingPrice: 34.99, Quantity: 142, UnitMeasurement: "kg", ProductType: "Perishable", UpdatedAt: time.Now().Format(time.RFC3339)},
			{Name: "Organic Whole Milk 2L", CostPrice: 2.20, SellingPrice: 4.50, Quantity: 480, UnitMeasurement: "L", ProductType: "Liquid", UpdatedAt: time.Now().Format(time.RFC3339)},
			{Name: "Compressed Industrial Nitrogen", CostPrice: 85.00, SellingPrice: 149.99, Quantity: 12, UnitMeasurement: "m3", ProductType: "Gas", UpdatedAt: time.Now().Format(time.RFC3339)},
		}
		_ = UpsertProducts(db, initialProds)
	}

	// Seed default activity logs if empty
	var actCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&actCount)
	if actCount == 0 {
		log.Println("Seeding initial activity logs into PostgreSQL...")
		seedAct1 := models.ActivityLogDB{
			ID:         "act_9011",
			Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
			UserID:     "usr_8921",
			UserName:   "Alex Vance",
			UserAvatar: "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150&auto=format&fit=crop&q=80",
			Type:       "BLOG",
			Action:     "Published Landing Page Blog",
			Details:    "Published 'How Real-Time Inventory Control Supercharges E-commerce Growth' with 12 SEO keywords",
			IPAddress:  "192.168.1.104",
			Severity:   "success",
			Location:   "London, UK",
		}
		seedAct2 := models.ActivityLogDB{
			ID:         "act_9010",
			Timestamp:  time.Now().Add(-20 * time.Minute).Format("2006-01-02 15:04:05"),
			UserID:     "usr_4402",
			UserName:   "Sarah Chen",
			UserAvatar: "https://images.unsplash.com/photo-1517841905240-472988babdf9?w=150&auto=format&fit=crop&q=80",
			Type:       "INVENTORY",
			Action:     "Bulk Stock Update",
			Details:    "Re-indexed SKU-99420 (Invot Wireless Scanner Pro) quantity increased by +450 units",
			IPAddress:  "10.0.4.12",
			Severity:   "info",
			Location:   "Warehouse Hub A",
		}
		_ = LogActivity(db, seedAct1)
		_ = LogActivity(db, seedAct2)
	}

	log.Println("PostgreSQL Database initialized successfully.")
	return db, nil
}

// GetAllProducts retrieves all products from the database
func GetAllProducts(db *sql.DB) ([]models.Product, error) {
	return GetProductsByStore(db, 0)
}

// GetProductsByStore retrieves products filtered by storeId (or all if storeId <= 0)
func GetProductsByStore(db *sql.DB, storeID int) ([]models.Product, error) {
	var rows *sql.Rows
	var err error

	if storeID > 0 {
		rows, err = db.Query("SELECT id, name, costPrice, sellingPrice, quantity, unitMeasurement, productType, category, supplierId, updatedAt, barcode, imageUrl, categoryId, COALESCE(storeId, 1) FROM products WHERE storeId = $1 ORDER BY updatedAt DESC", storeID)
	} else {
		rows, err = db.Query("SELECT id, name, costPrice, sellingPrice, quantity, unitMeasurement, productType, category, supplierId, updatedAt, barcode, imageUrl, categoryId, COALESCE(storeId, 1) FROM products ORDER BY updatedAt DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		var supplierID, categoryID sql.NullInt64
		var unit, pType, cat, bCode, imgUrl sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.CostPrice, &p.SellingPrice, &p.Quantity, &unit, &pType, &cat, &supplierID, &p.UpdatedAt, &bCode, &imgUrl, &categoryID, &p.StoreID); err != nil {
			return nil, err
		}
		if supplierID.Valid {
			p.SupplierID = int(supplierID.Int64)
		}
		if categoryID.Valid {
			id := int(categoryID.Int64)
			p.CategoryID = &id
		}
		p.UnitMeasurement = unit.String
		if p.UnitMeasurement == "" {
			p.UnitMeasurement = "pcs"
		}
		p.ProductType = pType.String
		if p.ProductType == "" {
			p.ProductType = "Solid"
		}
		p.Category = cat.String
		if p.Category == "" {
			p.Category = "General"
		}
		p.Barcode = bCode.String
		p.ImageUrl = imgUrl.String
		products = append(products, p)
	}

	if products == nil {
		products = []models.Product{}
	}

	return products, nil
}

// GetProductByBarcode retrieves a product from database matching the given barcode string, ID, or exact name.
func GetProductByBarcode(db *sql.DB, barcode string) (*models.Product, error) {
	if barcode == "" {
		return nil, sql.ErrNoRows
	}
	var p models.Product
	var supplierID, categoryID sql.NullInt64
	var unit, pType, cat, bCode, imgUrl sql.NullString
	err := db.QueryRow("SELECT id, name, costPrice, sellingPrice, quantity, unitMeasurement, productType, category, supplierId, updatedAt, barcode, imageUrl, categoryId FROM products WHERE barcode = $1 OR CAST(id AS TEXT) = $2 OR LOWER(name) = LOWER($3)", barcode, barcode, barcode).Scan(
		&p.ID, &p.Name, &p.CostPrice, &p.SellingPrice, &p.Quantity, &unit, &pType, &cat, &supplierID, &p.UpdatedAt, &bCode, &imgUrl, &categoryID,
	)
	if err != nil {
		return nil, err
	}
	if supplierID.Valid {
		p.SupplierID = int(supplierID.Int64)
	}
	if categoryID.Valid {
		id := int(categoryID.Int64)
		p.CategoryID = &id
	}
	p.UnitMeasurement = unit.String
	if p.UnitMeasurement == "" {
		p.UnitMeasurement = "pcs"
	}
	p.ProductType = pType.String
	if p.ProductType == "" {
		p.ProductType = "Solid"
	}
	p.Category = cat.String
	if p.Category == "" {
		p.Category = "General"
	}
	p.Barcode = bCode.String
	p.ImageUrl = imgUrl.String
	return &p, nil
}

// UpsertProducts updates or inserts products.
func UpsertProducts(db *sql.DB, products []models.Product) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := range products {
		p := &products[i]
		if p.UpdatedAt == "" {
			p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if p.UnitMeasurement == "" {
			p.UnitMeasurement = "pcs"
		}
		if p.ProductType == "" {
			p.ProductType = "Solid"
		}
		if p.Category == "" {
			p.Category = "General"
		}

		// Ensure categoryId and category name are linked
		if p.CategoryID != nil && *p.CategoryID > 0 {
			var catName string
			errCat := tx.QueryRow("SELECT name FROM categories WHERE id = $1", *p.CategoryID).Scan(&catName)
			if errCat == nil && catName != "" {
				p.Category = catName
			}
		} else if p.Category != "" {
			var catID int
			errCat := tx.QueryRow("SELECT id FROM categories WHERE LOWER(name) = LOWER($1) AND businessId = 1", p.Category).Scan(&catID)
			if errCat == nil {
				p.CategoryID = &catID
			} else {
				errIns := tx.QueryRow("INSERT INTO categories (name, businessId) VALUES ($1, 1) ON CONFLICT (name, businessId) DO UPDATE SET name = EXCLUDED.name RETURNING id", p.Category).Scan(&catID)
				if errIns == nil && catID > 0 {
					p.CategoryID = &catID
				}
			}
		}

		var supplierID interface{}
		if p.SupplierID > 0 {
			supplierID = p.SupplierID
		} else {
			supplierID = nil
		}

		var categoryID interface{}
		if p.CategoryID != nil && *p.CategoryID > 0 {
			categoryID = *p.CategoryID
		} else {
			categoryID = nil
		}

		var existingID int
		var existingUpdatedAt string

		// 1. Try to find by ID first
		if p.ID > 0 {
			err = tx.QueryRow("SELECT id, updatedAt FROM products WHERE id = $1", p.ID).Scan(&existingID, &existingUpdatedAt)
		} else {
			err = sql.ErrNoRows
		}

		// 2. Fallback to matching by barcode if not found by ID
		if err == sql.ErrNoRows && p.Barcode != "" {
			err = tx.QueryRow("SELECT id, updatedAt FROM products WHERE barcode = $1", p.Barcode).Scan(&existingID, &existingUpdatedAt)
		}

		// 3. Fallback to matching by name if not found by ID or barcode
		if err == sql.ErrNoRows && p.Name != "" {
			err = tx.QueryRow("SELECT id, updatedAt FROM products WHERE LOWER(name) = LOWER($1)", p.Name).Scan(&existingID, &existingUpdatedAt)
		}

		storeID := p.StoreID
		if storeID <= 0 {
			storeID = 1
		}

		if err == nil {
			// Found existing product - Update only if incoming data is not older
			if p.UpdatedAt >= existingUpdatedAt {
				_, err = tx.Exec(`
					UPDATE products SET barcode=$1, name=$2, costPrice=$3, sellingPrice=$4, quantity=$5, unitMeasurement=$6, productType=$7, category=$8, supplierId=$9, updatedAt=$10, imageUrl=$11, categoryId=$12, storeId=$13
					WHERE id=$14
				`, p.Barcode, p.Name, p.CostPrice, p.SellingPrice, p.Quantity, p.UnitMeasurement, p.ProductType, p.Category, supplierID, p.UpdatedAt, p.ImageUrl, categoryID, storeID, existingID)
				if err != nil {
					return fmt.Errorf("failed to update product %d: %w", existingID, err)
				}
			}
			p.ID = existingID
		} else if err == sql.ErrNoRows {
			// Not found - Insert new
			err := tx.QueryRow(`
				INSERT INTO products (barcode, name, costPrice, sellingPrice, quantity, unitMeasurement, productType, category, supplierId, updatedAt, imageUrl, categoryId, storeId) 
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				RETURNING id
			`, p.Barcode, p.Name, p.CostPrice, p.SellingPrice, p.Quantity, p.UnitMeasurement, p.ProductType, p.Category, supplierID, p.UpdatedAt, p.ImageUrl, categoryID, storeID).Scan(&p.ID)
			if err != nil {
				return fmt.Errorf("failed to insert product: %w", err)
			}
		} else {
			return err
		}
	}

	return tx.Commit()
}

// GetAllCategories retrieves all categories for a given business ID.
func GetAllCategories(db *sql.DB, businessID int) ([]models.CategoryDB, error) {
	if businessID <= 0 {
		businessID = 1
	}
	rows, err := db.Query("SELECT id, name, businessId FROM categories WHERE businessId = $1 ORDER BY name ASC", businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.CategoryDB
	for rows.Next() {
		var c models.CategoryDB
		if err := rows.Scan(&c.ID, &c.Name, &c.BusinessID); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	if categories == nil {
		categories = []models.CategoryDB{}
	}
	return categories, nil
}

// CreateCategory creates a new category for a business ID or returns an existing one.
func CreateCategory(db *sql.DB, c models.CategoryDB) (*models.CategoryDB, error) {
	if c.BusinessID <= 0 {
		c.BusinessID = 1
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return nil, fmt.Errorf("category name cannot be empty")
	}

	var existing models.CategoryDB
	err := db.QueryRow("SELECT id, name, businessId FROM categories WHERE LOWER(name) = LOWER($1) AND businessId = $2", c.Name, c.BusinessID).Scan(&existing.ID, &existing.Name, &existing.BusinessID)
	if err == nil {
		return &existing, nil
	}

	err = db.QueryRow("INSERT INTO categories (name, businessId) VALUES ($1, $2) RETURNING id", c.Name, c.BusinessID).Scan(&c.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}
	return &c, nil
}

// ProcessTransaction inserts a receipt and decrements the corresponding inventory items in a safe transaction
func ProcessTransaction(db *sql.DB, r models.Receipt, items []models.ReceiptItem) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	storeID := r.StoreID
	if storeID <= 0 {
		storeID = 1
	}

	// Insert receipt (ON CONFLICT DO NOTHING for idempotency)
	res, err := tx.Exec(`
		INSERT INTO receipts (receiptId, timestamp, subtotal, taxTotal, grandTotal, paymentMethod, storeId)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (receiptId) DO NOTHING
	`, r.ReceiptID, r.Timestamp, r.Subtotal, r.TaxTotal, r.GrandTotal, r.PaymentMethod, storeID)
	if err != nil {
		return fmt.Errorf("failed to insert receipt: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("Transaction %s already exists, skipping re-processing.", r.ReceiptID)
		return nil
	}

	// Insert receipt items and decrement stock levels
	itemStmt, err := tx.Prepare(`
		INSERT INTO receipt_items (receiptId, product_id, quantity, price, costPrice)
		VALUES ($1, $2, $3, $4, $5)
	`)
	if err != nil {
		return err
	}
	defer itemStmt.Close()

	decrementStmt, err := tx.Prepare(`
		UPDATE products 
		SET quantity = GREATEST(0, quantity - $1),
			updatedAt = $2
		WHERE id = $3
	`)
	if err != nil {
		return err
	}
	defer decrementStmt.Close()

	nowStr := time.Now().UTC().Format(time.RFC3339)

	for _, item := range items {
		if item.CostPrice == 0 {
			err := tx.QueryRow("SELECT costPrice FROM products WHERE id = $1", item.ProductID).Scan(&item.CostPrice)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("failed to fetch cost price for ProductID %d: %w", item.ProductID, err)
			}
		}

		log.Printf("Processing item: ProductID=%d, Qty=%d", item.ProductID, item.Quantity)

		if _, err := itemStmt.Exec(r.ReceiptID, item.ProductID, item.Quantity, item.Price, item.CostPrice); err != nil {
			return fmt.Errorf("failed to insert transaction item (ProductID %d): %w", item.ProductID, err)
		}

		res, err := decrementStmt.Exec(item.Quantity, nowStr, item.ProductID)
		if err != nil {
			return fmt.Errorf("failed to update product stock (ProductID %d): %w", item.ProductID, err)
		}

		affected, _ := res.RowsAffected()
		if affected == 0 {
			log.Printf("Warning: Attempted to decrement non-existent product ID %d", item.ProductID)
		}
	}

	return tx.Commit()
}

// GetAllReceipts returns all historical receipts including their items
func GetAllReceipts(db *sql.DB) ([]models.Receipt, error) {
	return GetReceiptsByStore(db, 0)
}

// GetReceiptsByStore returns receipts filtered by storeId (or all receipts if storeId <= 0)
func GetReceiptsByStore(db *sql.DB, storeID int) ([]models.Receipt, error) {
	var rows *sql.Rows
	var err error

	if storeID > 0 {
		rows, err = db.Query("SELECT receiptId, timestamp, subtotal, taxTotal, grandTotal, paymentMethod, COALESCE(storeId, 1) FROM receipts WHERE storeId = $1 ORDER BY timestamp DESC", storeID)
	} else {
		rows, err = db.Query("SELECT receiptId, timestamp, subtotal, taxTotal, grandTotal, paymentMethod, COALESCE(storeId, 1) FROM receipts ORDER BY timestamp DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var receipts []models.Receipt
	for rows.Next() {
		var r models.Receipt
		if err := rows.Scan(&r.ReceiptID, &r.Timestamp, &r.Subtotal, &r.TaxTotal, &r.GrandTotal, &r.PaymentMethod, &r.StoreID); err != nil {
			return nil, err
		}
		receipts = append(receipts, r)
	}
	rows.Close()

	for i := range receipts {
		items, err := GetReceiptItems(db, receipts[i].ReceiptID)
		if err != nil {
			return nil, err
		}
		receipts[i].Items = items
	}

	if receipts == nil {
		receipts = []models.Receipt{}
	}

	return receipts, nil
}

// GetReceiptItems returns all items for a specific receipt, joining with products to get names
func GetReceiptItems(db *sql.DB, receiptID string) ([]models.ReceiptItem, error) {
	query := `
		SELECT ri.product_id, COALESCE(p.name, 'Unknown Product'), ri.quantity, ri.price, ri.costPrice 
		FROM receipt_items ri
		LEFT JOIN products p ON ri.product_id = p.id
		WHERE ri.receiptId = $1
	`
	rows, err := db.Query(query, receiptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.ReceiptItem
	for rows.Next() {
		var item models.ReceiptItem
		if err := rows.Scan(&item.ProductID, &item.Name, &item.Quantity, &item.Price, &item.CostPrice); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// GetDashboardAnalytics calculates active metrics from the PostgreSQL database ledger
func GetDashboardAnalytics(db *sql.DB) (models.DashboardAnalytics, error) {
	return GetDashboardAnalyticsForStore(db, 0)
}

// GetDashboardAnalyticsForStore calculates active metrics filtered by storeId (or all if storeId <= 0)
func GetDashboardAnalyticsForStore(db *sql.DB, storeID int) (models.DashboardAnalytics, error) {
	var analytics models.DashboardAnalytics
	analytics.DatabaseConnected = true

	var err error
	if storeID > 0 {
		err = db.QueryRow("SELECT COUNT(*) FROM products WHERE storeId = $1", storeID).Scan(&analytics.TotalProducts)
		if err != nil {
			return analytics, err
		}

		err = db.QueryRow("SELECT COUNT(*) FROM products WHERE quantity < 5 AND storeId = $1", storeID).Scan(&analytics.LowStockCount)
		if err != nil {
			return analytics, err
		}

		err = db.QueryRow("SELECT COUNT(*) FROM receipts WHERE storeId = $1", storeID).Scan(&analytics.TotalTransactions)
		if err != nil {
			return analytics, err
		}

		var totalRev sql.NullFloat64
		err = db.QueryRow("SELECT SUM(grandTotal) FROM receipts WHERE storeId = $1", storeID).Scan(&totalRev)
		if err != nil {
			return analytics, err
		}
		if totalRev.Valid {
			analytics.TotalRevenue = totalRev.Float64
		}

		var totalProfit sql.NullFloat64
		err = db.QueryRow("SELECT SUM((ri.price - ri.costPrice) * ri.quantity) FROM receipts r JOIN receipt_items ri ON r.receiptId = ri.receiptId WHERE r.storeId = $1", storeID).Scan(&totalProfit)
		if err != nil {
			return analytics, err
		}
		if totalProfit.Valid {
			analytics.TotalProfit = totalProfit.Float64
		}
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&analytics.TotalProducts)
		if err != nil {
			return analytics, err
		}

		err = db.QueryRow("SELECT COUNT(*) FROM products WHERE quantity < 5").Scan(&analytics.LowStockCount)
		if err != nil {
			return analytics, err
		}

		err = db.QueryRow("SELECT COUNT(*) FROM receipts").Scan(&analytics.TotalTransactions)
		if err != nil {
			return analytics, err
		}

		var totalRev sql.NullFloat64
		err = db.QueryRow("SELECT SUM(grandTotal) FROM receipts").Scan(&totalRev)
		if err != nil {
			return analytics, err
		}
		if totalRev.Valid {
			analytics.TotalRevenue = totalRev.Float64
		}

		var totalProfit sql.NullFloat64
		err = db.QueryRow("SELECT SUM((price - costPrice) * quantity) FROM receipt_items").Scan(&totalProfit)
		if err != nil {
			return analytics, err
		}
		if totalProfit.Valid {
			analytics.TotalProfit = totalProfit.Float64
		}
	}

	recentReceipts, errRec := GetReceiptsByStore(db, storeID)
	if errRec == nil {
		if len(recentReceipts) > 10 {
			analytics.RecentTransactions = recentReceipts[:10]
		} else {
			analytics.RecentTransactions = recentReceipts
		}
	}
	if analytics.RecentTransactions == nil {
		analytics.RecentTransactions = []models.Receipt{}
	}

	var revRows *sql.Rows
	if storeID > 0 {
		revRows, err = db.Query(`
			SELECT SUBSTR(timestamp, 1, 10) as day, SUM(grandTotal) 
			FROM receipts 
			WHERE storeId = $1
			GROUP BY day 
			ORDER BY day DESC 
			LIMIT 30
		`, storeID)
	} else {
		revRows, err = db.Query(`
			SELECT SUBSTR(timestamp, 1, 10) as day, SUM(grandTotal) 
			FROM receipts 
			GROUP BY day 
			ORDER BY day DESC 
			LIMIT 30
		`)
	}
	if err == nil {
		defer revRows.Close()
		var points []models.RevenuePoint
		for revRows.Next() {
			var p models.RevenuePoint
			if err := revRows.Scan(&p.Date, &p.Revenue); err == nil {
				points = append(points, p)
			}
		}
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
		analytics.RevenueByDate = points
	}
	if analytics.RevenueByDate == nil {
		analytics.RevenueByDate = []models.RevenuePoint{}
	}

	var profitRows *sql.Rows
	if storeID > 0 {
		profitRows, err = db.Query(`
			SELECT SUBSTR(r.timestamp, 1, 10) as day, SUM((ri.price - ri.costPrice) * ri.quantity)
			FROM receipts r
			JOIN receipt_items ri ON r.receiptId = ri.receiptId
			WHERE r.storeId = $1
			GROUP BY day
			ORDER BY day DESC
			LIMIT 30
		`, storeID)
	} else {
		profitRows, err = db.Query(`
			SELECT SUBSTR(r.timestamp, 1, 10) as day, SUM((ri.price - ri.costPrice) * ri.quantity)
			FROM receipts r
			JOIN receipt_items ri ON r.receiptId = ri.receiptId
			GROUP BY day
			ORDER BY day DESC
			LIMIT 30
		`)
	}
	if err == nil {
		defer profitRows.Close()
		var points []models.ProfitPoint
		for profitRows.Next() {
			var p models.ProfitPoint
			if err := profitRows.Scan(&p.Date, &p.Profit); err == nil {
				points = append(points, p)
			}
		}
		for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
			points[i], points[j] = points[j], points[i]
		}
		analytics.ProfitByDate = points
	}
	if analytics.ProfitByDate == nil {
		analytics.ProfitByDate = []models.ProfitPoint{}
	}

	var velocityRows *sql.Rows
	if storeID > 0 {
		velocityRows, err = db.Query(`
			SELECT ri.product_id, p.name, SUM(ri.quantity) as sold
			FROM receipt_items ri
			JOIN receipts r ON ri.receiptId = r.receiptId
			JOIN products p ON ri.product_id = p.id
			WHERE r.storeId = $1
			GROUP BY ri.product_id, p.name
			ORDER BY sold DESC
			LIMIT 5
		`, storeID)
	} else {
		velocityRows, err = db.Query(`
			SELECT ri.product_id, p.name, SUM(ri.quantity) as sold
			FROM receipt_items ri
			JOIN products p ON ri.product_id = p.id
			GROUP BY ri.product_id, p.name
			ORDER BY sold DESC
			LIMIT 5
		`)
	}
	if err == nil {
		defer velocityRows.Close()
		for velocityRows.Next() {
			var pv models.ProductVelocity
			if err := velocityRows.Scan(&pv.ProductID, &pv.Name, &pv.UnitsSold); err == nil {
				analytics.TopProducts = append(analytics.TopProducts, pv)
			}
		}
	}
	if analytics.TopProducts == nil {
		analytics.TopProducts = []models.ProductVelocity{}
	}

	return analytics, nil
}

// GetAllSuppliers retrieves all suppliers from the database
func GetAllSuppliers(db *sql.DB) ([]models.Supplier, error) {
	return GetSuppliersByStore(db, 0)
}

// GetSuppliersByStore retrieves suppliers filtered by storeId (or all if storeId <= 0)
func GetSuppliersByStore(db *sql.DB, storeID int) ([]models.Supplier, error) {
	var rows *sql.Rows
	var err error

	if storeID > 0 {
		rows, err = db.Query("SELECT id, name, phone, email, address, COALESCE(storeId, 1) FROM suppliers WHERE storeId = $1 ORDER BY name ASC", storeID)
	} else {
		rows, err = db.Query("SELECT id, name, phone, email, address, COALESCE(storeId, 1) FROM suppliers ORDER BY name ASC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suppliers []models.Supplier
	for rows.Next() {
		var s models.Supplier
		var phone, email, address sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &phone, &email, &address, &s.StoreID); err != nil {
			return nil, err
		}
		s.Phone = phone.String
		s.Email = email.String
		s.Address = address.String
		suppliers = append(suppliers, s)
	}
	if suppliers == nil {
		suppliers = []models.Supplier{}
	}
	return suppliers, nil
}

// UpsertSupplier updates or inserts a supplier.
func UpsertSupplier(db *sql.DB, s models.Supplier) (int, error) {
	var existingID int
	var err error

	storeID := s.StoreID
	if storeID <= 0 {
		storeID = 1
	}

	if s.ID > 0 {
		err = db.QueryRow("SELECT id FROM suppliers WHERE id = $1", s.ID).Scan(&existingID)
	} else {
		err = sql.ErrNoRows
	}

	if err == sql.ErrNoRows {
		err = db.QueryRow("SELECT id FROM suppliers WHERE LOWER(name) = LOWER($1)", s.Name).Scan(&existingID)
	}

	if err == nil {
		_, err = db.Exec(`
			UPDATE suppliers SET name=$1, phone=$2, email=$3, address=$4, storeId=$5
			WHERE id=$6
		`, s.Name, s.Phone, s.Email, s.Address, storeID, existingID)
		return existingID, err
	} else if err == sql.ErrNoRows {
		err := db.QueryRow(`
			INSERT INTO suppliers (name, phone, email, address, storeId)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, s.Name, s.Phone, s.Email, s.Address, storeID).Scan(&existingID)
		if err != nil {
			return 0, err
		}
		return existingID, nil
	} else {
		return 0, err
	}
}

// DeleteProductByID deletes a product by its internal ID.
func DeleteProductByID(db *sql.DB, id int) error {
	_, err := db.Exec("DELETE FROM products WHERE id = $1", id)
	return err
}

// DeleteSupplierByID deletes a supplier by its internal ID.
func DeleteSupplierByID(db *sql.DB, id int) error {
	_, err := db.Exec("DELETE FROM suppliers WHERE id = $1", id)
	return err
}

// DeleteReceiptByID deletes a receipt and its associated items.
func DeleteReceiptByID(db *sql.DB, receiptID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM receipt_items WHERE receiptId = $1", receiptID); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM receipts WHERE receiptId = $1", receiptID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetSettings retrieves default main store configuration
func GetSettings(db *sql.DB) (models.Settings, error) {
	return GetSettingsForStore(db, 1)
}

// GetSettingsForStore retrieves or initializes settings for a specific store ID synced with stores table business data
func GetSettingsForStore(db *sql.DB, storeId int) (models.Settings, error) {
	var s models.Settings
	var displayByIDInt int
	var licenseActivatedInt int
	if storeId <= 0 {
		storeId = 1
	}

	var foundStoreID int
	err := db.QueryRow("SELECT companyName, phone, address, logo, ice, taxPercentage, displayById, activationPin, licenseActivated, receiptFooterMessage, COALESCE(storeId, 1) FROM settings WHERE storeId = $1 LIMIT 1", storeId).
		Scan(&s.CompanyName, &s.Phone, &s.Address, &s.Logo, &s.ICE, &s.TaxPercentage, &displayByIDInt, &s.ActivationPin, &licenseActivatedInt, &s.ReceiptFooterMessage, &foundStoreID)

	if err == sql.ErrNoRows {
		// Fetch store info and auto-create settings row for this storeId
		var st models.StoreDB
		errStore := db.QueryRow("SELECT id, name, COALESCE(phone, ''), COALESCE(address, ''), COALESCE(email, '') FROM stores WHERE id = $1", storeId).
			Scan(&st.ID, &st.Name, &st.Phone, &st.Address, &st.Email)
		
		if errStore == nil {
			s.CompanyName = st.Name
			s.Phone = st.Phone
			s.Address = st.Address
		}
		if strings.TrimSpace(s.CompanyName) == "" {
			s.CompanyName = fmt.Sprintf("Business Store #%d", storeId)
		}
		s.ReceiptFooterMessage = "Thank you for shopping with us!"
		s.StoreID = storeId

		_, _ = db.Exec(`
			INSERT INTO settings (companyName, phone, address, logo, ice, taxPercentage, displayById, activationPin, licenseActivated, receiptFooterMessage, storeId)
			VALUES ($1, $2, $3, '', '', 0.0, 0, '', 0, $4, $5)
		`, s.CompanyName, s.Phone, s.Address, s.ReceiptFooterMessage, storeId)
	} else if err != nil {
		return s, err
	} else {
		s.StoreID = foundStoreID
		s.DisplayByID = displayByIDInt == 1
		s.LicenseActivated = licenseActivatedInt == 1
	}

	// Sync business infos (Name, Phone, Address, Email) from matching store in stores table
	var st models.StoreDB
	errStore := db.QueryRow("SELECT id, name, COALESCE(phone, ''), COALESCE(address, ''), COALESCE(email, '') FROM stores WHERE id = $1", storeId).
		Scan(&st.ID, &st.Name, &st.Phone, &st.Address, &st.Email)
	if errStore == nil {
		if strings.TrimSpace(st.Name) != "" {
			s.CompanyName = st.Name
		}
		if strings.TrimSpace(st.Phone) != "" {
			s.Phone = st.Phone
		}
		if strings.TrimSpace(st.Address) != "" {
			s.Address = st.Address
		}
	}

	return s, nil
}

// UpdateSettings updates application configuration and syncs matching store in stores table
func UpdateSettings(db *sql.DB, s models.Settings) error {
	displayByIDInt := 0
	if s.DisplayByID {
		displayByIDInt = 1
	}
	if s.StoreID <= 0 {
		s.StoreID = 1
	}

	_, err := db.Exec(`
		UPDATE settings 
		SET companyName = $1, phone = $2, address = $3, logo = $4, ice = $5, taxPercentage = $6, displayById = $7, receiptFooterMessage = $8, storeId = $9
		WHERE id = 1 OR storeId = $9
	`, s.CompanyName, s.Phone, s.Address, s.Logo, s.ICE, s.TaxPercentage, displayByIDInt, s.ReceiptFooterMessage, s.StoreID)

	if err != nil {
		return err
	}

	// Also sync updated business name, phone, address to stores table for this storeId
	_, _ = db.Exec(`
		UPDATE stores
		SET name = $1, phone = $2, address = $3
		WHERE id = $4
	`, s.CompanyName, s.Phone, s.Address, s.StoreID)

	return nil
}

// HashPassword hashes plain text password with SHA-256
func HashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte("invot_salt_2026_" + password))
	return hex.EncodeToString(h.Sum(nil))
}

// GetAllUsers retrieves all registered users
func GetAllUsers(db *sql.DB) ([]models.UserDB, error) {
	rows, err := db.Query(`
		SELECT id, name, email, passwordHash, role, status, avatar, lastActive, createdAt, department, actionsCount
		FROM users
		ORDER BY createdAt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.UserDB
	for rows.Next() {
		var u models.UserDB
		var avatar, dept sql.NullString
		err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &avatar, &u.LastActive, &u.CreatedAt, &dept, &u.ActionsCount)
		if err != nil {
			return nil, err
		}
		u.Avatar = avatar.String
		u.Department = dept.String
		u.MustChangePassword = (u.PasswordHash == HashPassword("admin123"))
		u.Permissions = models.GetPermissionsForRole(u.Role)
		users = append(users, u)
	}

	if users == nil {
		users = []models.UserDB{}
	}
	return users, nil
}

// UpsertUser adds or updates a user account
func UpsertUser(db *sql.DB, u models.UserDB, password string) error {
	if password != "" {
		u.PasswordHash = HashPassword(password)
		_, err := db.Exec(`
			INSERT INTO users (id, name, email, passwordHash, role, status, avatar, lastActive, createdAt, department, actionsCount)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT(id) DO UPDATE SET
				name=EXCLUDED.name,
				email=EXCLUDED.email,
				passwordHash=EXCLUDED.passwordHash,
				role=EXCLUDED.role,
				status=EXCLUDED.status,
				avatar=EXCLUDED.avatar,
				lastActive=EXCLUDED.lastActive,
				department=EXCLUDED.department,
				actionsCount=EXCLUDED.actionsCount
		`, u.ID, u.Name, u.Email, u.PasswordHash, u.Role, u.Status, u.Avatar, u.LastActive, u.CreatedAt, u.Department, u.ActionsCount)
		return err
	}

	_, err := db.Exec(`
		INSERT INTO users (id, name, email, passwordHash, role, status, avatar, lastActive, createdAt, department, actionsCount)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT(id) DO UPDATE SET
			name=EXCLUDED.name,
			email=EXCLUDED.email,
			role=EXCLUDED.role,
			status=EXCLUDED.status,
			avatar=EXCLUDED.avatar,
			lastActive=EXCLUDED.lastActive,
			department=EXCLUDED.department,
			actionsCount=EXCLUDED.actionsCount
	`, u.ID, u.Name, u.Email, HashPassword("admin123"), u.Role, u.Status, u.Avatar, u.LastActive, u.CreatedAt, u.Department, u.ActionsCount)
	return err
}

// UpdateUserStatus updates user status (e.g. Active, Suspended)
func UpdateUserStatus(db *sql.DB, id string, status string) error {
	_, err := db.Exec("UPDATE users SET status = $1 WHERE id = $2", status, id)
	return err
}

// DeleteUserByID removes user account
func DeleteUserByID(db *sql.DB, id string) error {
	var role string
	var email string
	err := db.QueryRow("SELECT role, LOWER(email) FROM users WHERE id = $1", id).Scan(&role, &email)
	if err == nil {
		if role == "Super Admin" || email == "admin" || email == "admin@invot.io" {
			return fmt.Errorf("cannot delete Super Admin user account")
		}
	}
	_, err = db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}

// ChangeUserPassword updates a user's password and optional full name in the database
func ChangeUserPassword(db *sql.DB, userID string, newPassword string, fullName string) error {
	hash := HashPassword(newPassword)
	if fullName != "" {
		_, err := db.Exec("UPDATE users SET passwordHash = $1, name = $2 WHERE id = $3", hash, fullName, userID)
		return err
	}
	_, err := db.Exec("UPDATE users SET passwordHash = $1 WHERE id = $2", hash, userID)
	return err
}

// AuthenticateUser verifies user credentials by email or phone and returns session token
func AuthenticateUser(db *sql.DB, loginInput, password string) (*models.UserDB, string, error) {
	var u models.UserDB
	var phone sql.NullString
	hash := HashPassword(password)
	cleanLogin := strings.TrimSpace(loginInput)

	// Search user by email or phone
	err := db.QueryRow(`
		SELECT id, name, email, COALESCE(phone, ''), passwordHash, role, status, avatar, lastActive, createdAt, department, actionsCount
		FROM users
		WHERE (LOWER(email) = LOWER($1) OR (phone IS NOT NULL AND phone != '' AND phone = $1) OR (LOWER($2) = 'admin' AND LOWER(email) IN ('admin', 'admin@invot.io')))
		  AND status = 'Active'
	`, cleanLogin, cleanLogin).Scan(&u.ID, &u.Name, &u.Email, &phone, &u.PasswordHash, &u.Role, &u.Status, &u.Avatar, &u.LastActive, &u.CreatedAt, &u.Department, &u.ActionsCount)

	if err != nil {
		return nil, "", fmt.Errorf("invalid credentials or suspended account")
	}

	u.Phone = phone.String

	// If password has not been created yet or is empty or default admin123
	if u.PasswordHash == "" || u.PasswordHash == HashPassword("admin123") {
		u.MustChangePassword = true
		u.Permissions = models.GetPermissionsForRole(u.Role)
		return &u, "", fmt.Errorf("PASSWORD_NOT_CREATED")
	}

	if u.PasswordHash != hash {
		return nil, "", fmt.Errorf("invalid credentials")
	}

	u.MustChangePassword = false
	u.Permissions = models.GetPermissionsForRole(u.Role)

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	_, _ = db.Exec("UPDATE users SET lastActive = $1 WHERE id = $2", nowStr, u.ID)
	u.LastActive = nowStr

	token := fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), u.ID)
	expiresAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	_, err = db.Exec("INSERT INTO sessions (token, userId, expiresAt) VALUES ($1, $2, $3)", token, u.ID, expiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	return &u, token, nil
}

// SetUserPassword creates or sets a new password for a user identified by email or phone.
func SetUserPassword(db *sql.DB, loginInput, newPassword string) error {
	cleanLogin := strings.TrimSpace(loginInput)
	if cleanLogin == "" || len(newPassword) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}

	hash := HashPassword(newPassword)
	res, err := db.Exec(`
		UPDATE users
		SET passwordHash = $1
		WHERE (LOWER(email) = LOWER($2) OR (phone IS NOT NULL AND phone != '' AND phone = $2) OR (LOWER($3) = 'admin' AND LOWER(email) IN ('admin', 'admin@invot.io')))
	`, hash, cleanLogin, cleanLogin)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found for email or phone: %s", cleanLogin)
	}

	return nil
}

// ValidateSessionToken checks if session token is valid and returns user
func ValidateSessionToken(db *sql.DB, token string) (*models.UserDB, error) {
	if token == "" {
		return nil, fmt.Errorf("token missing")
	}

	var u models.UserDB
	var expiresAtStr string

	err := db.QueryRow(`
		SELECT s.expiresAt, u.id, u.name, u.email, u.passwordHash, u.role, u.status, u.avatar, u.lastActive, u.createdAt, u.department, u.actionsCount
		FROM sessions s
		JOIN users u ON s.userId = u.id
		WHERE s.token = $1 AND u.status = 'Active'
	`, token).Scan(&expiresAtStr, &u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.Avatar, &u.LastActive, &u.CreatedAt, &u.Department, &u.ActionsCount)

	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	u.MustChangePassword = (u.PasswordHash == HashPassword("admin123"))
	u.Permissions = models.GetPermissionsForRole(u.Role)

	return &u, nil
}

// GetAllActivities returns activity logs
func GetAllActivities(db *sql.DB) ([]models.ActivityLogDB, error) {
	rows, err := db.Query(`
		SELECT id, timestamp, userId, userName, userAvatar, type, action, details, ipAddress, severity, location
		FROM activity_logs
		ORDER BY timestamp DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.ActivityLogDB
	for rows.Next() {
		var a models.ActivityLogDB
		var details, location sql.NullString
		err := rows.Scan(&a.ID, &a.Timestamp, &a.UserID, &a.UserName, &a.UserAvatar, &a.Type, &a.Action, &details, &a.IPAddress, &a.Severity, &location)
		if err != nil {
			return nil, err
		}
		a.Details = details.String
		a.Location = location.String
		logs = append(logs, a)
	}

	if logs == nil {
		logs = []models.ActivityLogDB{}
	}
	return logs, nil
}

// LogActivity inserts a new activity log entry
func LogActivity(db *sql.DB, a models.ActivityLogDB) error {
	if a.ID == "" {
		a.ID = fmt.Sprintf("act_%d", time.Now().UnixNano())
	}
	if a.Timestamp == "" {
		a.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	}

	_, err := db.Exec(`
		INSERT INTO activity_logs (id, timestamp, userId, userName, userAvatar, type, action, details, ipAddress, severity, location)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, a.ID, a.Timestamp, a.UserID, a.UserName, a.UserAvatar, a.Type, a.Action, a.Details, a.IPAddress, a.Severity, a.Location)

	return err
}

// GetAllStores retrieves all stores/businesses.
func GetAllStores(db *sql.DB) ([]models.StoreDB, error) {
	rows, err := db.Query("SELECT id, name, code, COALESCE(phone, ''), COALESCE(address, ''), COALESCE(email, ''), status, createdAt FROM stores ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stores []models.StoreDB
	for rows.Next() {
		var s models.StoreDB
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &s.Phone, &s.Address, &s.Email, &s.Status, &s.CreatedAt); err != nil {
			return nil, err
		}
		stores = append(stores, s)
	}
	if stores == nil {
		stores = []models.StoreDB{}
	}
	return stores, nil
}

// GetStoreByID fetches a specific store by ID.
func GetStoreByID(db *sql.DB, id int) (*models.StoreDB, error) {
	var s models.StoreDB
	err := db.QueryRow("SELECT id, name, code, COALESCE(phone, ''), COALESCE(address, ''), COALESCE(email, ''), status, createdAt FROM stores WHERE id = $1", id).Scan(&s.ID, &s.Name, &s.Code, &s.Phone, &s.Address, &s.Email, &s.Status, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateOrUpdateStore creates or updates a store/business.
func CreateOrUpdateStore(db *sql.DB, s *models.StoreDB) error {
	if s.Status == "" {
		s.Status = "Active"
	}
	if s.CreatedAt == "" {
		s.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}

	if s.ID > 0 {
		_, err := db.Exec(`
			UPDATE stores
			SET name = $1, code = $2, phone = $3, address = $4, email = $5, status = $6
			WHERE id = $7
		`, s.Name, s.Code, s.Phone, s.Address, s.Email, s.Status, s.ID)
		if err != nil {
			return err
		}
		_, _ = db.Exec(`
			UPDATE settings
			SET companyName = $1, phone = $2, address = $3
			WHERE storeId = $4 OR (id = 1 AND $4 = 1)
		`, s.Name, s.Phone, s.Address, s.ID)
		return nil
	}

	if s.Code == "" {
		s.Code = fmt.Sprintf("STORE-%03d", time.Now().Unix()%1000)
	}

	err := db.QueryRow(`
		INSERT INTO stores (name, code, phone, address, email, status, createdAt)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, s.Name, s.Code, s.Phone, s.Address, s.Email, s.Status, s.CreatedAt).Scan(&s.ID)
	if err != nil {
		return err
	}

	_, _ = db.Exec(`
		UPDATE settings
		SET companyName = $1, phone = $2, address = $3
		WHERE storeId = $4 OR (id = 1 AND $4 = 1)
	`, s.Name, s.Phone, s.Address, s.ID)
	return nil
}

// DeleteStoreByID deletes a store by ID (except default store #1).
func DeleteStoreByID(db *sql.DB, id int) error {
	if id <= 1 {
		return fmt.Errorf("cannot delete default main store")
	}
	_, err := db.Exec("DELETE FROM stores WHERE id = $1", id)
	return err
}

// GetAllUsersAdmin returns all registered user accounts with phone, status, role, and assigned stores.
func GetAllUsersAdmin(db *sql.DB) ([]models.UserDB, error) {
	rows, err := db.Query(`
		SELECT u.id, u.name, u.email, COALESCE(u.phone, ''), u.role, u.status, COALESCE(u.avatar, ''), COALESCE(u.lastActive, ''), u.createdAt, COALESCE(u.department, ''), u.actionsCount, COALESCE(u.storeId, 1), COALESCE(u.storeIds, '')
		FROM users u
		ORDER BY u.createdAt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.UserDB
	for rows.Next() {
		var u models.UserDB
		var phone sql.NullString
		var rawStoreIDs string
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &phone, &u.Role, &u.Status, &u.Avatar, &u.LastActive, &u.CreatedAt, &u.Department, &u.ActionsCount, &u.StoreID, &rawStoreIDs); err != nil {
			return nil, err
		}
		u.Phone = phone.String
		u.StoreIDs = parseStoreIDs(rawStoreIDs, u.StoreID)
		u.Permissions = models.GetPermissionsForRole(u.Role)
		users = append(users, u)
	}
	return users, nil
}

// CreateUserAdmin creates a user account by email or phone with multiple store support.
func CreateUserAdmin(db *sql.DB, name, email, phone, role, status string, storeId int, storeIds []int) (*models.UserDB, error) {
	cleanEmail := strings.TrimSpace(email)
	cleanPhone := strings.TrimSpace(phone)
	if cleanEmail == "" && cleanPhone == "" {
		return nil, fmt.Errorf("either email or phone number is required")
	}
	if cleanEmail == "" {
		cleanEmail = fmt.Sprintf("%s@invot.local", strings.ReplaceAll(cleanPhone, "+", ""))
	}
	if role == "" {
		role = "Cashier"
	}
	if status == "" {
		status = "Active"
	}
	if storeId <= 0 {
		if len(storeIds) > 0 {
			storeId = storeIds[0]
		} else {
			storeId = 1
		}
	}

	rawStoreIDs := formatStoreIDs(storeIds, storeId)
	userId := fmt.Sprintf("u_%d", time.Now().UnixNano())
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	_, err := db.Exec(`
		INSERT INTO users (id, name, email, phone, passwordHash, role, status, avatar, lastActive, createdAt, department, actionsCount, storeId, storeIds)
		VALUES ($1, $2, $3, $4, '', $5, $6, '', $7, $7, '', 0, $8, $9)
	`, userId, name, cleanEmail, cleanPhone, role, status, nowStr, storeId, rawStoreIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &models.UserDB{
		ID:                 userId,
		Name:               name,
		Email:              cleanEmail,
		Phone:              cleanPhone,
		MustChangePassword: true,
		Role:               role,
		Status:             status,
		CreatedAt:          nowStr,
		LastActive:         nowStr,
		StoreID:            storeId,
		StoreIDs:           parseStoreIDs(rawStoreIDs, storeId),
		Permissions:        models.GetPermissionsForRole(role),
	}, nil
}

// SetUserStatusAdmin toggles or sets status for a user.
func SetUserStatusAdmin(db *sql.DB, userId, status string) error {
	res, err := db.Exec("UPDATE users SET status = $1 WHERE id = $2", status, userId)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// UpdateUserAdmin updates existing user details including multiple stores
func UpdateUserAdmin(db *sql.DB, u *models.UserDB) error {
	cleanEmail := strings.TrimSpace(u.Email)
	cleanPhone := strings.TrimSpace(u.Phone)
	if cleanEmail == "" && cleanPhone == "" {
		return fmt.Errorf("either email or phone is required")
	}

	if u.StoreID <= 0 {
		if len(u.StoreIDs) > 0 {
			u.StoreID = u.StoreIDs[0]
		} else {
			u.StoreID = 1
		}
	}
	rawStoreIDs := formatStoreIDs(u.StoreIDs, u.StoreID)

	_, err := db.Exec(`
		UPDATE users
		SET name = $1, email = $2, phone = $3, role = $4, status = $5, storeId = $6, storeIds = $7
		WHERE id = $8
	`, u.Name, cleanEmail, cleanPhone, u.Role, u.Status, u.StoreID, rawStoreIDs, u.ID)
	return err
}

// DeleteUserAdmin deletes a user account by ID
func DeleteUserAdmin(db *sql.DB, userId string) error {
	res, err := db.Exec("DELETE FROM users WHERE id = $1", userId)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetAllPayments returns all payment records.
func GetAllPayments(db *sql.DB) ([]models.PaymentDB, error) {
	rows, err := db.Query(`
		SELECT id, userId, COALESCE(userName, ''), COALESCE(userEmail, ''), COALESCE(userPhone, ''), planDuration, amount, paymentMethod, status, startDate, endDate, COALESCE(notes, ''), createdAt
		FROM payments
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []models.PaymentDB
	for rows.Next() {
		var p models.PaymentDB
		if err := rows.Scan(&p.ID, &p.UserID, &p.UserName, &p.UserEmail, &p.UserPhone, &p.PlanDuration, &p.Amount, &p.PaymentMethod, &p.Status, &p.StartDate, &p.EndDate, &p.Notes, &p.CreatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, nil
}

// CreatePayment records a new payment and returns created PaymentDB.
func CreatePayment(db *sql.DB, p *models.PaymentDB) (*models.PaymentDB, error) {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	if p.StartDate == "" {
		p.StartDate = now.Format("2006-01-02")
	}

	start, err := time.Parse("2006-01-02", p.StartDate)
	if err != nil {
		start = now
		p.StartDate = start.Format("2006-01-02")
	}

	var endDate time.Time
	switch strings.ToLower(strings.TrimSpace(p.PlanDuration)) {
	case "1 month", "1m":
		endDate = start.AddDate(0, 1, 0)
	case "3 months", "3m":
		endDate = start.AddDate(0, 3, 0)
	case "6 months", "6m":
		endDate = start.AddDate(0, 6, 0)
	case "1 year", "1y", "12 months":
		endDate = start.AddDate(1, 0, 0)
	default:
		endDate = start.AddDate(0, 1, 0)
	}
	p.EndDate = endDate.Format("2006-01-02")
	p.CreatedAt = nowStr
	if p.Status == "" {
		p.Status = "Paid"
	}
	if p.PaymentMethod == "" {
		p.PaymentMethod = "Cash"
	}

	var id int
	err = db.QueryRow(`
		INSERT INTO payments (userId, userName, userEmail, userPhone, planDuration, amount, paymentMethod, status, startDate, endDate, notes, createdAt)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`, p.UserID, p.UserName, p.UserEmail, p.UserPhone, p.PlanDuration, p.Amount, p.PaymentMethod, p.Status, p.StartDate, p.EndDate, p.Notes, p.CreatedAt).Scan(&id)

	if err != nil {
		return nil, fmt.Errorf("failed to record payment: %w", err)
	}

	p.ID = id
	return p, nil
}

// DeletePaymentByID deletes a payment record by ID
func DeletePaymentByID(db *sql.DB, id int) error {
	res, err := db.Exec("DELETE FROM payments WHERE id = $1", id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("payment record not found")
	}
	return nil
}

func parseStoreIDs(raw string, primaryStoreID int) []int {
	raw = strings.TrimSpace(raw)
	seen := make(map[int]bool)
	var res []int
	if primaryStoreID > 0 {
		res = append(res, primaryStoreID)
		seen[primaryStoreID] = true
	}
	if raw != "" {
		parts := strings.Split(raw, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if id, err := strconv.Atoi(p); err == nil && id > 0 {
				if !seen[id] {
					seen[id] = true
					res = append(res, id)
				}
			}
		}
	}
	return res
}

func formatStoreIDs(ids []int, primaryStoreID int) string {
	seen := make(map[int]bool)
	var strParts []string
	if primaryStoreID > 0 {
		seen[primaryStoreID] = true
		strParts = append(strParts, strconv.Itoa(primaryStoreID))
	}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			strParts = append(strParts, strconv.Itoa(id))
		}
	}
	return strings.Join(strParts, ",")
}
