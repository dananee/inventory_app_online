package models

// CategoryDB represents a product category linked with a business ID.
type CategoryDB struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	BusinessID int    `json:"businessId"`
}

// Product represents an item in our shop inventory.
type Product struct {
	ID              int     `json:"id"`
	LocalID         int     `json:"localId,omitempty"`
	Barcode         string  `json:"barcode,omitempty"`
	Name            string  `json:"name"`
	CostPrice       float64 `json:"costPrice"`
	SellingPrice    float64 `json:"sellingPrice"`
	Quantity        int     `json:"quantity"`
	UnitMeasurement string  `json:"unitMeasurement"` // e.g., pcs, L, mL, kg, g, box, m3
	ProductType     string  `json:"productType"`     // e.g., Solid, Liquid, Gas, Perishable, General
	Category        string  `json:"category"`        // e.g., General, Electronics, Beverages, Food & Snacks, Hardware, Supplies
	CategoryID      *int    `json:"categoryId,omitempty"`
	SupplierID      int     `json:"supplierId"`
	StoreID         int     `json:"storeId,omitempty"`
	UpdatedAt       string  `json:"updatedAt"`
	ImageUrl        string  `json:"imageUrl,omitempty"` // Optional product image URL
}

// Supplier represents a product provider.
type Supplier struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
	StoreID int    `json:"storeId,omitempty"`
}

// Receipt represents a logged sale transaction.
type Receipt struct {
	ReceiptID     string        `json:"receiptId"`
	Timestamp     string        `json:"timestamp"`
	Subtotal      float64       `json:"subtotal"`
	TaxTotal      float64       `json:"taxTotal"`
	GrandTotal    float64       `json:"grandTotal"`
	PaymentMethod string        `json:"paymentMethod"`
	StoreID       int           `json:"storeId,omitempty"`
	Items         []ReceiptItem `json:"items,omitempty"`
}

// ReceiptItem represents an item within a receipt.
type ReceiptItem struct {
	ProductID int     `json:"productId"`
	Name      string  `json:"name,omitempty"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"costPrice"`
}

// NewTransactionRequest represents the incoming API payload for a new receipt
type NewTransactionRequest struct {
	ReceiptID     string        `json:"receiptId"`
	Timestamp     string        `json:"timestamp"`
	Subtotal      float64       `json:"subtotal"`
	TaxTotal      float64       `json:"taxTotal"`
	GrandTotal    float64       `json:"grandTotal"`
	PaymentMethod string        `json:"paymentMethod"`
	StoreID       int           `json:"storeId,omitempty"`
	Items         []ReceiptItem `json:"items"`
}

// PaymentDB represents a subscription or license payment
type PaymentDB struct {
	ID            int     `json:"id"`
	UserID        string  `json:"userId"`
	UserName      string  `json:"userName"`
	UserEmail     string  `json:"userEmail"`
	UserPhone     string  `json:"userPhone"`
	PlanDuration  string  `json:"planDuration"` // "1 Month", "3 Months", "6 Months", "1 Year"
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"paymentMethod"` // "Cash", "Card", "Bank Transfer", "Check"
	Status        string  `json:"status"`        // "Paid", "Pending", "Expired"
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	Notes         string  `json:"notes,omitempty"`
	CreatedAt     string  `json:"createdAt"`
}

// DashboardAnalytics holds all aggregate statistics for UI consumption
type DashboardAnalytics struct {
	DatabaseConnected  bool              `json:"databaseConnected"`
	TotalProducts      int               `json:"totalProducts"`
	LowStockCount      int               `json:"lowStockCount"`
	TotalTransactions  int               `json:"totalTransactions"`
	TotalRevenue       float64           `json:"totalRevenue"`
	TotalProfit        float64           `json:"totalProfit"`
	RecentTransactions []Receipt         `json:"recentTransactions"`
	RevenueByDate      []RevenuePoint    `json:"revenueByDate"`
	ProfitByDate       []ProfitPoint     `json:"profitByDate"`
	TopProducts        []ProductVelocity `json:"topProducts"`
}

type RevenuePoint struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
}

type ProfitPoint struct {
	Date   string  `json:"date"`
	Profit float64 `json:"profit"`
}

type ProductVelocity struct {
	ProductID int    `json:"productId"`
	Name      string `json:"name"`
	UnitsSold int    `json:"unitsSold"`
}

// Settings represents global or store-specific application configuration
type Settings struct {
	StoreID              int     `json:"storeId,omitempty"`
	CompanyName          string  `json:"companyName"`
	Phone                string  `json:"phone"`
	Address              string  `json:"address"`
	Logo                 string  `json:"logo"`
	ICE                  string  `json:"ice"`
	TaxPercentage        float64 `json:"taxPercentage"`
	DisplayByID          bool    `json:"displayById"`
	ActivationPin        string  `json:"activationPin"`
	LicenseActivated     bool    `json:"licenseActivated"`
	ReceiptFooterMessage string  `json:"receiptFooterMessage"`
}

// Permission represents a granular system permission string
type Permission string

const (
	PermUsersRead       Permission = "users:read"
	PermUsersWrite      Permission = "users:write"
	PermUsersDelete     Permission = "users:delete"
	PermInventoryRead   Permission = "inventory:read"
	PermInventoryWrite  Permission = "inventory:write"
	PermInventoryDelete Permission = "inventory:delete"
	PermActivityRead    Permission = "activity:read"
	PermActivityWrite   Permission = "activity:write"
	PermSettingsRead    Permission = "settings:read"
	PermSettingsWrite   Permission = "settings:write"
)

var RolePermissions = map[string][]Permission{
	"Super Admin": {
		PermUsersRead, PermUsersWrite, PermUsersDelete,
		PermInventoryRead, PermInventoryWrite, PermInventoryDelete,
		PermActivityRead, PermActivityWrite,
		PermSettingsRead, PermSettingsWrite,
	},
	"Store Manager": {
		PermUsersRead, PermUsersWrite, PermUsersDelete,
		PermInventoryRead, PermInventoryWrite, PermInventoryDelete,
		PermActivityRead, PermActivityWrite,
		PermSettingsRead, PermSettingsWrite,
	},
	"Inventory Manager": {
		PermInventoryRead, PermInventoryWrite, PermInventoryDelete,
		PermActivityRead, PermActivityWrite,
		PermUsersRead,
	},
	"Cashier": {
		PermInventoryRead, PermInventoryWrite,
		PermActivityRead, PermActivityWrite,
		PermSettingsRead,
	},
	"Stock Clerk": {
		PermInventoryRead, PermInventoryWrite,
		PermActivityRead, PermActivityWrite,
		PermSettingsRead,
	},
	"Auditor": {
		PermUsersRead, PermInventoryRead,
		PermActivityRead, PermSettingsRead,
	},
	"Support Agent": {
		PermUsersRead, PermInventoryRead,
		PermActivityRead, PermActivityWrite,
	},
}

func GetPermissionsForRole(role string) []Permission {
	if perms, ok := RolePermissions[role]; ok {
		return perms
	}
	return []Permission{}
}

func HasPermission(role string, perm Permission) bool {
	perms := GetPermissionsForRole(role)
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// UserDB represents an administrator or staff user
type UserDB struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Email              string       `json:"email"`
	Phone              string       `json:"phone,omitempty"`
	PasswordHash       string       `json:"-"`
	MustChangePassword bool         `json:"mustChangePassword"`
	Role               string       `json:"role"`
	Status             string       `json:"status"`
	Avatar             string       `json:"avatar"`
	LastActive         string       `json:"lastActive"`
	CreatedAt          string       `json:"createdAt"`
	Department         string       `json:"department"`
	ActionsCount       int          `json:"actionsCount"`
	StoreID            int          `json:"storeId,omitempty"`
	StoreIDs           []int        `json:"storeIds,omitempty"`
	Permissions        []Permission `json:"permissions"`
}

// ActivityLogDB represents a recorded event
type ActivityLogDB struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	UserID     string `json:"userId"`
	UserName   string `json:"userName"`
	UserAvatar string `json:"userAvatar"`
	Type       string `json:"type"`
	Action     string `json:"action"`
	Details    string `json:"details"`
	IPAddress  string `json:"ipAddress"`
	Severity   string `json:"severity"`
	Location   string `json:"location"`
}

// StoreDB represents a store / business location.
type StoreDB struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Phone     string `json:"phone,omitempty"`
	Address   string `json:"address,omitempty"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// OpenFoodFactsResponse represents the Open Food Facts API v2 response schema
type OpenFoodFactsResponse struct {
	Code          string          `json:"code"`
	Status        int             `json:"status"`
	StatusVerbose string          `json:"status_verbose"`
	Product       *OFFProductItem `json:"product"`
}

type OFFProductItem struct {
	ProductName   string `json:"product_name"`
	ProductNameEn string `json:"product_name_en"`
	Brands        string `json:"brands"`
	Categories    string `json:"categories"`
	Quantity      string `json:"quantity"`
	ImageFrontURL string `json:"image_front_url"`
	ServingSize   string `json:"serving_size"`
}

// AddByBarcodeRequest represents payload options when adding products via barcode scan
type AddByBarcodeRequest struct {
	Barcode         string  `json:"barcode"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	CostPrice       float64 `json:"costPrice"`
	SellingPrice    float64 `json:"sellingPrice"`
	Quantity        int     `json:"quantity"`
	UnitMeasurement string  `json:"unitMeasurement"`
	ProductType     string  `json:"productType"`
	Category        string  `json:"category"`
	SupplierID      int     `json:"supplierId"`
}
