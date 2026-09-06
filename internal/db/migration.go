package db

import (
	"database/sql"
	"inventory_app/internal/models"
	"strings"
	"time"
)

// ProcessBulkDimensions handles Phase 1
func ProcessBulkDimensions(db *sql.DB, storeID int, req models.DimensionsMigrationRequest) (models.DimensionsMigrationResponse, error) {
	if storeID <= 0 {
		storeID = 1
	}

	res := models.DimensionsMigrationResponse{
		Categories: []models.MigrationMapping{},
		Suppliers:  []models.MigrationMapping{},
	}

	tx, err := db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// 1. Process Categories
	for _, cat := range req.Categories {
		if cat.LocalID <= 0 {
			continue
		}
		catName := strings.TrimSpace(cat.Name)
		if catName == "" {
			continue
		}

		businessID := cat.BusinessID
		if businessID <= 0 {
			businessID = 1
		}

		var cloudID int
		// UPSERT strategy
		err := tx.QueryRow(`
			INSERT INTO categories (name, businessId) 
			VALUES ($1, $2)
			ON CONFLICT (name, businessId) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, catName, businessID).Scan(&cloudID)
		
		if err == nil {
			res.Categories = append(res.Categories, models.MigrationMapping{
				LocalID: cat.LocalID,
				CloudID: cloudID,
			})
		}
	}

	// 2. Process Suppliers
	for _, sup := range req.Suppliers {
		if sup.LocalID <= 0 {
			continue
		}
		supName := strings.TrimSpace(sup.Name)
		if supName == "" {
			continue
		}
		
		var cloudID int
		// We try to match by name, if exists update, otherwise insert
		err := tx.QueryRow("SELECT id FROM suppliers WHERE LOWER(name) = LOWER($1)", supName).Scan(&cloudID)
		if err == sql.ErrNoRows {
			errIns := tx.QueryRow(`
				INSERT INTO suppliers (name, phone, email, address, storeId)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, supName, sup.Phone, sup.Email, sup.Address, storeID).Scan(&cloudID)
			if errIns == nil {
				res.Suppliers = append(res.Suppliers, models.MigrationMapping{LocalID: sup.LocalID, CloudID: cloudID})
			}
		} else if err == nil {
			// update existing
			_, _ = tx.Exec(`
				UPDATE suppliers SET phone=$1, email=$2, address=$3, storeId=$4 WHERE id=$5
			`, sup.Phone, sup.Email, sup.Address, storeID, cloudID)
			res.Suppliers = append(res.Suppliers, models.MigrationMapping{LocalID: sup.LocalID, CloudID: cloudID})
		}
	}

	err = tx.Commit()
	return res, err
}

// ProcessBulkProducts handles Phase 2
func ProcessBulkProducts(db *sql.DB, storeID int, req models.ProductsMigrationRequest) (models.ProductsMigrationResponse, error) {
	if storeID <= 0 {
		storeID = 1
	}

	res := models.ProductsMigrationResponse{
		Products: []models.MigrationMapping{},
	}

	tx, err := db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	for _, p := range req.Products {
		if p.LocalID <= 0 {
			continue
		}

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

		var cloudID int
		
		// If barcode is provided, use it as unique constraint matching (like ON CONFLICT)
		if p.Barcode != "" {
			var existingID int
			err := tx.QueryRow("SELECT id FROM products WHERE barcode = $1", p.Barcode).Scan(&existingID)
			if err == nil {
				// Update existing
				_, errUpd := tx.Exec(`
					UPDATE products SET name=$1, costPrice=$2, sellingPrice=$3, quantity=$4, unitMeasurement=$5, 
					productType=$6, category=$7, supplierId=$8, updatedAt=$9, imageUrl=$10, categoryId=$11, storeId=$12
					WHERE id=$13
				`, p.Name, p.CostPrice, p.SellingPrice, p.Quantity, p.UnitMeasurement, p.ProductType, p.Category, 
				supplierID, p.UpdatedAt, p.ImageUrl, categoryID, storeID, existingID)
				
				if errUpd == nil {
					res.Products = append(res.Products, models.MigrationMapping{LocalID: p.LocalID, CloudID: existingID})
				}
				continue
			}
		}

		// Insert new product
		errIns := tx.QueryRow(`
			INSERT INTO products (barcode, name, costPrice, sellingPrice, quantity, unitMeasurement, 
			productType, category, supplierId, updatedAt, imageUrl, categoryId, storeId) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id
		`, p.Barcode, p.Name, p.CostPrice, p.SellingPrice, p.Quantity, p.UnitMeasurement, 
		p.ProductType, p.Category, supplierID, p.UpdatedAt, p.ImageUrl, categoryID, storeID).Scan(&cloudID)
		
		if errIns == nil {
			res.Products = append(res.Products, models.MigrationMapping{LocalID: p.LocalID, CloudID: cloudID})
		}
	}

	err = tx.Commit()
	return res, err
}

// ProcessBulkReceipts handles Phase 3
func ProcessBulkReceipts(db *sql.DB, storeID int, req models.ReceiptsMigrationRequest) error {
	if storeID <= 0 {
		storeID = 1
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, r := range req.Receipts {
		// Insert receipt
		res, err := tx.Exec(`
			INSERT INTO receipts (receiptId, timestamp, subtotal, taxTotal, grandTotal, paymentMethod, storeId)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (receiptId) DO NOTHING
		`, r.ReceiptID, r.Timestamp, r.Subtotal, r.TaxTotal, r.GrandTotal, r.PaymentMethod, storeID)
		
		if err != nil {
			return err
		}

		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			// already exists, skip items
			continue
		}

		// Insert receipt items (DO NOT decrement stock, as local DB already handled local stock decrement)
		itemStmt, err := tx.Prepare(`
			INSERT INTO receipt_items (receiptId, product_id, quantity, price, costPrice)
			VALUES ($1, $2, $3, $4, $5)
		`)
		if err != nil {
			return err
		}

		for _, item := range r.Items {
			if item.CostPrice == 0 {
				_ = tx.QueryRow("SELECT costPrice FROM products WHERE id = $1", item.ProductID).Scan(&item.CostPrice)
			}
			_, _ = itemStmt.Exec(r.ReceiptID, item.ProductID, item.Quantity, item.Price, item.CostPrice)
		}
		itemStmt.Close()
	}

	return tx.Commit()
}
