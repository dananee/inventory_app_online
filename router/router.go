package router

import (
	"net/http"

	"inventory_app/internal/handlers"
	"inventory_app/internal/models"
)

// SetupRoutes registers all HTTP routes against the provided ServeMux.
func SetupRoutes(mux *http.ServeMux, ctx *handlers.AppContext) {
	// ───── Static file server for uploads directory ─────
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// ───── Server Info ─────
	mux.HandleFunc("/api/server/ip", ctx.HandleGetIPs)

	// ───── Auth ─────
	mux.HandleFunc("/api/auth/login", ctx.HandleLogin)
	mux.HandleFunc("/api/auth/set-password", ctx.HandleSetPassword)
	mux.HandleFunc("/api/auth/me", ctx.HandleGetMe)
	mux.HandleFunc("/api/auth/change-password", ctx.HandleChangePassword)

	// ───── License ─────
	mux.HandleFunc("/api/license/activate", ctx.HandleActivateLicense)
	mux.HandleFunc("/api/license/verify", ctx.HandleVerifyLicense)

	// ───── Inventory ─────
	mux.HandleFunc("/api/inventory/sync", ctx.HandleMobileSync)
	mux.HandleFunc("/api/inventory/delete", ctx.RequirePermission(models.PermInventoryDelete, ctx.HandleDeleteProduct))
	mux.HandleFunc("/api/inventory/add", ctx.RequirePermission(models.PermInventoryWrite, ctx.HandleAddProduct))
	mux.HandleFunc("/api/inventory/upload", ctx.RequirePermission(models.PermInventoryWrite, ctx.HandleImageUpload))
	mux.HandleFunc("/api/inventory/scan-barcode", ctx.HandleScanBarcode)
	mux.HandleFunc("/api/inventory/add-by-barcode", ctx.HandleAddByBarcode)

	// ───── Transactions ─────
	mux.HandleFunc("/api/transactions/new", ctx.HandleNewTransaction)
	mux.HandleFunc("/api/transactions/pdf", ctx.HandleExportPDF)

	// ───── Receipts ─────
	mux.HandleFunc("/api/receipts", ctx.HandleGetReceipts)
	mux.HandleFunc("/api/receipts/delete", ctx.RequirePermission(models.PermInventoryDelete, ctx.HandleDeleteReceipt))

	// ───── Analytics ─────
	mux.HandleFunc("/api/analytics/dashboard", ctx.HandleAnalyticsQuery)

	// ───── Settings ─────
	mux.HandleFunc("/api/settings", ctx.HandleGetSettings)
	mux.HandleFunc("/api/settings/update", ctx.RequirePermission(models.PermSettingsWrite, ctx.HandleUpdateSettings))

	// ───── Suppliers ─────
	mux.HandleFunc("/api/suppliers", ctx.HandleGetSuppliers)
	mux.HandleFunc("/api/suppliers/update", ctx.RequirePermission(models.PermInventoryWrite, ctx.HandleUpdateSupplier))
	mux.HandleFunc("/api/suppliers/delete", ctx.RequirePermission(models.PermInventoryDelete, ctx.HandleDeleteSupplier))

	// ───── Users ─────
	mux.HandleFunc("/api/users", ctx.RequirePermission(models.PermUsersRead, ctx.HandleGetUsers))
	mux.HandleFunc("/api/users/add", ctx.RequirePermission(models.PermUsersWrite, ctx.HandleAddUser))
	mux.HandleFunc("/api/users/status", ctx.RequirePermission(models.PermUsersWrite, ctx.HandleUpdateUserStatus))
	mux.HandleFunc("/api/users/delete", ctx.RequirePermission(models.PermUsersDelete, ctx.HandleDeleteUser))

	// ───── Activities ─────
	mux.HandleFunc("/api/activities", ctx.RequirePermission(models.PermActivityRead, ctx.HandleGetActivities))
	mux.HandleFunc("/api/activities/log", ctx.RequirePermission(models.PermActivityWrite, ctx.HandleLogActivity))

	// ───── Stores ─────
	mux.HandleFunc("/api/stores", ctx.HandleGetStores)
	mux.HandleFunc("/api/stores/update", ctx.RequirePermission(models.PermSettingsWrite, ctx.HandleUpdateStore))
	mux.HandleFunc("/api/stores/delete", ctx.RequirePermission(models.PermSettingsWrite, ctx.HandleDeleteStore))

	// ───── Categories ─────
	mux.HandleFunc("/api/categories", ctx.HandleCategories)

	// ───── Admin Panel ─────
	mux.HandleFunc("/api/admin/users", ctx.HandleAdminGetUsers)
	mux.HandleFunc("/api/admin/users/create", ctx.HandleAdminCreateUser)
	mux.HandleFunc("/api/admin/users/status", ctx.HandleAdminSetUserStatus)
	mux.HandleFunc("/api/admin/users/reset-password", ctx.HandleAdminResetPassword)
	mux.HandleFunc("/api/admin/users/update", ctx.HandleAdminUpdateUser)
	mux.HandleFunc("/api/admin/users/delete", ctx.HandleAdminDeleteUser)
	mux.HandleFunc("/api/admin/payments", ctx.HandleAdminGetPayments)
	mux.HandleFunc("/api/admin/payments/create", ctx.HandleAdminCreatePayment)
	mux.HandleFunc("/api/admin/payments/delete", ctx.HandleAdminDeletePayment)
	mux.HandleFunc("/api/admin/summary", ctx.HandleAdminSummary)
}
