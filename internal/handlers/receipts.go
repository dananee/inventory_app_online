package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"

	"inventory_app/internal/db"
	"inventory_app/internal/middleware"
	"inventory_app/internal/models"
)

// HandleGetReceipts returns all historical receipts at /api/receipts
func (ctx *AppContext) HandleGetReceipts(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	storeIDStr := r.URL.Query().Get("store_id")
	if storeIDStr == "" {
		storeIDStr = r.URL.Query().Get("storeId")
	}
	storeID, _ := strconv.Atoi(storeIDStr)

	receipts, err := db.GetReceiptsByStore(ctx.DB, storeID)
	if err != nil {
		log.Printf("Error fetching receipts: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch receipt history"})
		return
	}

	middleware.WriteJSON(w, http.StatusOK, receipts)
}

// HandleDeleteReceipt removes a receipt and its items at /api/receipts/delete
func (ctx *AppContext) HandleDeleteReceipt(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		middleware.WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Accept both ?receiptId= (Flutter client) and ?id= (legacy)
	receiptID := r.URL.Query().Get("receiptId")
	fmt.Printf("Receipt ID %s", receiptID)
	if receiptID == "" {
		receiptID = r.URL.Query().Get("id")
	}
	if receiptID == "" {
		middleware.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "Receipt ID required"})
		return
	}

	// Resolve the authenticated user so we can attribute the log entry
	var deletedByID, deletedByName string
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if user, err := db.ValidateSessionToken(ctx.DB, token); err == nil {
		deletedByID = user.ID
		deletedByName = user.Name
	}

	if err := db.DeleteReceiptByID(ctx.DB, receiptID); err != nil {
		log.Printf("Error deleting receipt: %v", err)
		middleware.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete receipt"})
		return
	}

	// Write an immutable audit log entry server-side so it can never be missed
	_ = db.LogActivity(ctx.DB, models.ActivityLogDB{
		UserID:   deletedByID,
		UserName: deletedByName,
		Type:     "TICKET",
		Action:   "DELETE_TICKET",
		Details:  fmt.Sprintf("Receipt %s was permanently deleted by %s", receiptID, deletedByName),
		Severity: "warning",
	})

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// HandleExportPDF generates a PDF for a specific receipt at /api/transactions/pdf
func (ctx *AppContext) HandleExportPDF(w http.ResponseWriter, r *http.Request) {
	if middleware.EnableCORS(w, r) {
		return
	}

	receiptID := r.URL.Query().Get("id")
	lang := r.URL.Query().Get("lang")
	customerName := r.URL.Query().Get("customer")
	if lang == "" {
		lang = "en"
	}
	if receiptID == "" {
		http.Error(w, "Receipt ID is required", http.StatusBadRequest)
		return
	}

	var receipt models.Receipt
	err := ctx.DB.QueryRow("SELECT receiptId, timestamp, subtotal, taxTotal, grandTotal, paymentMethod FROM receipts WHERE receiptId = $1", receiptID).
		Scan(&receipt.ReceiptID, &receipt.Timestamp, &receipt.Subtotal, &receipt.TaxTotal, &receipt.GrandTotal, &receipt.PaymentMethod)
	if err != nil {
		http.Error(w, "Receipt not found", http.StatusNotFound)
		return
	}

	items, err := db.GetReceiptItems(ctx.DB, receiptID)
	if err != nil {
		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
		return
	}
	receipt.Items = items

	settings, _ := db.GetSettings(ctx.DB)

	type Labels struct {
		Invoice, Date, Customer, Product, Qty, Price, Total, Subtotal, Tax, Grand, Thanks string
	}
	translations := map[string]Labels{
		"en": {"Invoice:", "Date:", "Customer:", "Product", "Qty", "Price", "Total", "Subtotal", "Tax", "GRAND TOTAL", "Thank you for your visit!"},
		"fr": {"Facture :", "Date :", "Client :", "Produit", "Qté", "Prix", "Total", "Sous-total", "Taxe", "TOTAL GÉNÉRAL", "Merci de votre visite !"},
		"ar": {"Invoice:", "Date:", "Customer:", "Product", "Qty", "Price", "Total", "Subtotal", "Tax", "GRAND TOTAL", "Thank you for your visit!"},
	}
	l := translations[lang]
	if l.Invoice == "" {
		l = translations["en"]
	}

	formattedDate := receipt.Timestamp
	if t, err := time.Parse(time.RFC3339, receipt.Timestamp); err == nil {
		formattedDate = t.Format("02-01-2006")
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()

	if settings.Logo != "" {
		parts := strings.Split(settings.Logo, ",")
		if len(parts) > 1 {
			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err == nil {
				imgType := "PNG"
				if strings.Contains(parts[0], "jpeg") || strings.Contains(parts[0], "jpg") {
					imgType = "JPG"
				}
				reader := bytes.NewReader(decoded)
				pdf.RegisterImageOptionsReader("logo", gofpdf.ImageOptions{ImageType: imgType}, reader)
				pdf.ImageOptions("logo", 10, 10, 30, 0, false, gofpdf.ImageOptions{ImageType: imgType}, 0, "")
				pdf.Ln(25)
			}
		}
	}

	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, tr(settings.CompanyName), "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	if settings.Phone != "" {
		pdf.CellFormat(0, 5, tr("Phone: "+settings.Phone), "", 1, "C", false, 0, "")
	}
	if settings.Address != "" {
		pdf.MultiCell(0, 5, tr(settings.Address), "", "C", false)
	}
	if settings.ICE != "" {
		pdf.CellFormat(0, 5, tr("ICE: "+settings.ICE), "", 1, "C", false, 0, "")
	}
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 10, tr(l.Invoice+" "+receipt.ReceiptID))
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 10)
	if customerName != "" {
		pdf.Cell(0, 10, tr(l.Customer+" "+customerName))
		pdf.Ln(6)
	}
	pdf.Cell(0, 10, tr(l.Date+" "+formattedDate))
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(100, 8, tr(l.Product), "1", 0, "L", false, 0, "")
	pdf.CellFormat(30, 8, tr(l.Qty), "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, tr(l.Price), "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, tr(l.Total), "1", 1, "R", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, item := range receipt.Items {
		name := item.Name
		if settings.DisplayByID {
			name = fmt.Sprintf("ID: %d", item.ProductID)
		}
		if name == "" {
			name = "Unknown Product"
		}
		pdf.CellFormat(100, 8, tr(name), "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 8, strconv.Itoa(item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", item.Price), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", float64(item.Quantity)*item.Price), "1", 1, "R", false, 0, "")
	}

	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(160, 8, tr(l.Subtotal), "", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, fmt.Sprintf("%.2f DH", receipt.Subtotal), "1", 1, "R", false, 0, "")
	if receipt.TaxTotal > 0 {
		taxLabel := l.Tax
		if settings.TaxPercentage > 0 {
			taxLabel = fmt.Sprintf("%s (%.1f%%)", l.Tax, settings.TaxPercentage)
		}
		pdf.CellFormat(160, 8, tr(taxLabel), "", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f DH", receipt.TaxTotal), "1", 1, "R", false, 0, "")
	}
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(160, 10, tr(l.Grand), "", 0, "R", false, 0, "")
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f DH", receipt.GrandTotal), "1", 1, "R", false, 0, "")

	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 10, tr(l.Thanks), "", 1, "C", false, 0, "")

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=Invoice_"+receiptID+".pdf")
	err = pdf.Output(w)
	if err != nil {
		log.Printf("Error generating PDF output: %v", err)
	}
}
