package controllers

import (
	"ecommerce/config"
	"ecommerce/middleware"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func AdminLoginPage(c *gin.Context) {
	logger.Log.Info("Requested to show admin login page")
	if middleware.ValidateAdminToken(c) {
		logger.Log.Info("Admin token valid, redirecting to dashboard")
		c.Redirect(http.StatusSeeOther, "/admin/dashboard")
		return
	}
	logger.Log.Info("Rendering admin login page")
	c.HTML(http.StatusOK, "Admin_Login.html", gin.H{})
}

func AdminLogin(c *gin.Context) {
	logger.Log.Info("Admin login attempt initiated")
	email := c.PostForm("email")
	password := c.PostForm("password")

	var admin models.Admin

	if err := config.DB.Where("email = ?", email).First(&admin).Error; err != nil {
		logger.Log.Error("Failed to find admin", zap.String("email", email), zap.Error(err))
		c.HTML(http.StatusUnauthorized, "Admin_Login.html", gin.H{"error": "Admin not Found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)); err != nil {
		logger.Log.Error("Invalid password for admin", zap.String("email", email), zap.Error(err))
		c.HTML(http.StatusUnauthorized, "Admin_Login.html", gin.H{"error": "Invalid password"})
		return
	}

	token, err := middleware.GenerateToken(admin.ID, admin.Email, "Admin")
	if err != nil {
		logger.Log.Error("Failed to generate token for admin", zap.String("email", email), zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Login.html", gin.H{"error": "Could not generate token"})
		return
	}

	logger.Log.Info("Admin login successful, setting cookie and redirecting", zap.String("email", email))
	c.SetCookie("jwtTokensAdmin", token, 3600, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/admin/dashboard")
}

type DashboardData struct {
	TotalRevenue      float64
	TotalOrders       int
	AvgOrderValue     float64
	TotalDiscount     float64
	TotalAmount       float64
	AggregatedRevenue map[string]float64
	AggregatedOrders  map[string]int
	Orders            []OrderSummary
	Period            string
	TopProducts       []ProductSales
	TopCategories     []CategorySales
	TopBrands         []BrandSales
}

type OrderSummary struct {
	OrderUID   string
	UserName   string
	OrderDate  time.Time
	Total      float64
	Discount   float64
	Status     string
}

type ProductSales struct {
	ProductName string
	TotalSold   int
	Revenue     float64
}

type CategorySales struct {
	CategoryName string
	TotalSold    int
	Revenue      float64
}

type BrandSales struct {
	BrandName string
	TotalSold int
	Revenue   float64
}

func AdminDashboard(c *gin.Context) {
	logger.Log.Info("Requested to show admin dashboard")
	startDateStr := c.DefaultQuery("start", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end", time.Now().Format("2006-01-02"))
	period := c.DefaultQuery("period", "daily")

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		logger.Log.Error("Invalid start date, using default",
			zap.String("startDate", startDateStr),
			zap.Error(err))
		startDate = time.Now().AddDate(0, 0, -30)
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		logger.Log.Error("Invalid end date, using default",
			zap.String("endDate", endDateStr),
			zap.Error(err))
		endDate = time.Now()
	}

	if startDate.After(endDate) {
		logger.Log.Info("Swapped startDate and endDate to ensure startDate <= endDate",
			zap.String("startDate", startDate.Format("2006-01-02")),
			zap.String("endDate", endDate.Format("2006-01-02")))
		startDate, endDate = endDate, startDate
	}

	data := fetchDashboardData(startDate, endDate, period)

	var totalUsers int64
	if err := config.DB.Model(&models.User{}).Count(&totalUsers).Error; err != nil {
		logger.Log.Error("Failed to fetch total users", zap.Error(err))
	}

	var totalOrders int64
	var deliveredOrders []models.Order
	if err := config.DB.
		Preload("OrderItem").
		Preload("PaymentDetails").
		Joins("JOIN payment_details ON payment_details.order_id = orders.id").
		Where("payment_details.payment_status = ?", "Completed").
		Find(&deliveredOrders).Error; err != nil {
		logger.Log.Error("Failed to fetch orders for total count", zap.Error(err))
	}

	totalOrders = 0
	var totalRevenue float64
	for _, order := range deliveredOrders {
		isDelivered := true
		for _, item := range order.OrderItem {
			if item.OrderStatus != "Delivered" {
				isDelivered = false
				break
			}
		}
		if isDelivered {
			totalOrders++
			totalRevenue += order.TotalAmount
		}
	}

	logger.Log.Info("Fetched dashboard data successfully",
		zap.Int64("totalUsers", totalUsers),
		zap.Int64("totalOrders", totalOrders),
		zap.Float64("totalRevenue", totalRevenue))

	aggregatedRevenueJSON, _ := json.Marshal(data.AggregatedRevenue)
	aggregatedOrdersJSON, _ := json.Marshal(data.AggregatedOrders)
	topProductsJSON, _ := json.Marshal(data.TopProducts)
	topCategoriesJSON, _ := json.Marshal(data.TopCategories)
	topBrandsJSON, _ := json.Marshal(data.TopBrands)

	c.HTML(http.StatusOK, "Admin_Dashboard.html", gin.H{
		"Data":                  data,
		"StartDate":             startDate.Format("2006-01-02"),
		"EndDate":               endDate.Format("2006-01-02"),
		"TotalUsers":            totalUsers,
		"TotalOrders":           totalOrders,
		"TotalRevenue":          totalRevenue,
		"AggregatedRevenueJSON": string(aggregatedRevenueJSON),
		"AggregatedOrdersJSON":  string(aggregatedOrdersJSON),
		"TopProductsJSON":       string(topProductsJSON),
		"TopCategoriesJSON":     string(topCategoriesJSON),
		"TopBrandsJSON":         string(topBrandsJSON),
	})
}

func GetDashboardData(c *gin.Context) {
	logger.Log.Info("Requested to get dashboard data")
	startDateStr := c.Query("start")
	endDateStr := c.Query("end")
	period := c.Query("period")
	if period == "" {
		period = "daily"
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		logger.Log.Error("Invalid start date",
			zap.String("startDate", startDateStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		logger.Log.Error("Invalid end date",
			zap.String("endDate", endDateStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date"})
		return
	}

	if startDate.After(endDate) {
		logger.Log.Info("Swapped startDate and endDate to ensure startDate <= endDate",
			zap.String("startDate", startDate.Format("2006-01-02")),
			zap.String("endDate", endDate.Format("2006-01-02")))
		startDate, endDate = endDate, startDate
	}

	data := fetchDashboardData(startDate, endDate, period)

	logger.Log.Info("Dashboard data fetched successfully",
		zap.Int("totalOrders", data.TotalOrders),
		zap.Float64("totalRevenue", data.TotalRevenue))

	c.JSON(http.StatusOK, gin.H{
		"TotalRevenue":      data.TotalRevenue,
		"TotalOrders":       data.TotalOrders,
		"AvgOrderValue":     data.AvgOrderValue,
		"TotalDiscount":     data.TotalDiscount,
		"TotalAmount":       data.TotalAmount,
		"AggregatedRevenue": data.AggregatedRevenue,
		"AggregatedOrders":  data.AggregatedOrders,
		"Period":            data.Period,
		"TopProducts":       data.TopProducts,
		"TopCategories":     data.TopCategories,
		"TopBrands":         data.TopBrands,
	})
}

func fetchDashboardData(startDate, endDate time.Time, period string) DashboardData {
	logger.Log.Info("Fetching dashboard data",
		zap.String("startDate", startDate.Format("2006-01-02")),
		zap.String("endDate", endDate.Format("2006-01-02")),
		zap.String("period", period))

	var data DashboardData
	data.AggregatedRevenue = make(map[string]float64)
	data.AggregatedOrders = make(map[string]int)
	data.Period = period

	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	var orders []models.Order
	query := config.DB.
		Preload("OrderItem").
		Preload("OrderItem.Product").
		Preload("OrderItem.Product.Category").
		Preload("User").
		Preload("PaymentDetails").
		Where("order_date BETWEEN ? AND ?", startDate, endDate).
		Order("order_date DESC")

	if err := query.Find(&orders).Error; err != nil {
		logger.Log.Error("Failed to fetch orders", zap.Error(err))
		return data
	}

	logger.Log.Info("Fetched orders from database", zap.Int("orderCount", len(orders)))
	if len(orders) == 0 {
		logger.Log.Warn("No orders found",
			zap.String("startDate", startDate.Format("2006-01-02")),
			zap.String("endDate", endDate.Format("2006-01-02")))
	}

	totalRevenue := 0.0
	totalAmount := 0.0
	totalDiscount := 0.0
	orderCount := 0
	productMap := make(map[uint]ProductSales)
	categoryMap := make(map[uint]CategorySales)
	brandMap := make(map[string]BrandSales)

	for _, order := range orders {
		paymentStatus := order.PaymentDetails.PaymentStatus
		logger.Log.Info("Processing order",
			zap.String("orderUID", order.OrderUID),
			zap.String("paymentStatus", paymentStatus),
			zap.String("orderDate", order.OrderDate.Format("2006-01-02")))

		if order.PaymentDetails.ID == 0 || order.PaymentDetails.PaymentStatus != "Completed" {
			logger.Log.Info("Skipping order: invalid payment status",
				zap.String("orderUID", order.OrderUID),
				zap.String("paymentStatus", paymentStatus),
				zap.Uint("paymentID", order.PaymentDetails.ID))
			continue
		}

		isDelivered := true
		for _, item := range order.OrderItem {
			if item.OrderStatus != "Delivered" {
				logger.Log.Info("Order item not delivered",
					zap.String("orderUID", order.OrderUID),
					zap.Uint("itemID", item.ID),
					zap.String("status", item.OrderStatus))
				isDelivered = false
				break
			}
		}
		if !isDelivered {
			logger.Log.Info("Skipping order: not all items delivered",
				zap.String("orderUID", order.OrderUID))
			continue
		}

		dateKey := getDateKey(order.OrderDate, period)
		logger.Log.Info("Order date key",
			zap.String("orderUID", order.OrderUID),
			zap.String("dateKey", dateKey))

		revenueForOrder := order.TotalAmount
		discountForOrder := order.TotalDiscount + order.CouponDiscount
		amountForOrder := order.TotalAmount

		userName := order.User.FirstName
		if order.User.LastName != "" {
			userName += " " + order.User.LastName
		}
		data.Orders = append(data.Orders, OrderSummary{
			OrderUID:   order.OrderUID,
			UserName:   userName,
			OrderDate:  order.OrderDate,
			Total:      revenueForOrder,
			Discount:   discountForOrder,
			Status:     "Delivered",
		})

		for _, item := range order.OrderItem {
			if prod, ok := productMap[item.ProductID]; ok {
				prod.TotalSold += item.Quantity
				prod.Revenue += item.Total
				productMap[item.ProductID] = prod
			} else {
				productMap[item.ProductID] = ProductSales{
					ProductName: item.Product.ProductName,
					TotalSold:   item.Quantity,
					Revenue:     item.Total,
				}
			}

			if cat, ok := categoryMap[item.Product.CategoryID]; ok {
				cat.TotalSold += item.Quantity
				cat.Revenue += item.Total
				categoryMap[item.Product.CategoryID] = cat
			} else {
				categoryMap[item.Product.CategoryID] = CategorySales{
					CategoryName: item.Product.Category.CategoryName,
					TotalSold:   item.Quantity,
					Revenue:     item.Total,
				}
			}

			brand := item.Product.Brand
			if brandSales, ok := brandMap[brand]; ok {
				brandSales.TotalSold += item.Quantity
				brandSales.Revenue += item.Total
				brandMap[brand] = brandSales
			} else {
				brandMap[brand] = BrandSales{
					BrandName: brand,
					TotalSold: item.Quantity,
					Revenue:   item.Total,
				}
			}
		}

		totalRevenue += revenueForOrder
		totalAmount += amountForOrder
		totalDiscount += discountForOrder
		data.AggregatedRevenue[dateKey] += revenueForOrder
		data.AggregatedOrders[dateKey]++
		orderCount++
	}

	data.TotalRevenue = totalRevenue
	data.TotalAmount = totalAmount
	data.TotalDiscount = totalDiscount
	data.TotalOrders = orderCount
	if orderCount > 0 {
		data.AvgOrderValue = totalAmount / float64(orderCount)
	}

	data.TopProducts = sortAndLimit(productMap, 10)
	data.TopCategories = sortAndLimitCategories(categoryMap, 10)
	data.TopBrands = sortAndLimitBrands(brandMap, 10)

	logger.Log.Info("Dashboard data processed",
		zap.Int("totalOrders", data.TotalOrders),
		zap.Float64("totalRevenue", data.TotalRevenue),
		zap.String("period", period),
		zap.String("startDate", startDate.Format("2006-01-02")),
		zap.String("endDate", endDate.Format("2006-01-02")))

	return data
}

func getDateKey(date time.Time, period string) string {
	switch period {
	case "daily":
		return date.Format("2006-01-02")
	case "weekly":
		year, week := date.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "monthly":
		return date.Format("2006-01")
	case "yearly":
		return date.Format("2006")
	case "custom":
		return date.Format("2006-01-02")
	default:
		return date.Format("2006-01-02")
	}
}

func sortAndLimit(productMap map[uint]ProductSales, limit int) []ProductSales {
	var products []ProductSales
	for _, prod := range productMap {
		products = append(products, prod)
	}
	sort.Slice(products, func(i, j int) bool {
		return products[i].TotalSold > products[j].TotalSold
	})
	if len(products) > limit {
		return products[:limit]
	}
	return products
}

func sortAndLimitCategories(categoryMap map[uint]CategorySales, limit int) []CategorySales {
	var categories []CategorySales
	for _, cat := range categoryMap {
		categories = append(categories, cat)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].TotalSold > categories[j].TotalSold
	})
	if len(categories) > limit {
		return categories[:limit]
	}
	return categories
}

func sortAndLimitBrands(brandMap map[string]BrandSales, limit int) []BrandSales {
	var brands []BrandSales
	for _, brand := range brandMap {
		brands = append(brands, brand)
	}
	sort.Slice(brands, func(i, j int) bool {
		return brands[i].TotalSold > brands[j].TotalSold
	})
	if len(brands) > limit {
		return brands[:limit]
	}
	return brands
}

func DownloadPDF(c *gin.Context) {
	logger.Log.Info("Requested to download PDF report")
	startDateStr := c.Query("start")
	endDateStr := c.Query("end")
	period := c.Query("period")
	if period == "" {
		period = "daily"
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		logger.Log.Error("Invalid start date",
			zap.String("startDate", startDateStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		logger.Log.Error("Invalid end date",
			zap.String("endDate", endDateStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date"})
		return
	}

	if startDate.After(endDate) {
		logger.Log.Info("Swapped startDate and endDate to ensure startDate <= endDate",
			zap.String("startDate", startDate.Format("2006-01-02")),
			zap.String("endDate", endDate.Format("2006-01-02")))
		startDate, endDate = endDate, startDate
	}

	data := fetchDashboardData(startDate, endDate, period)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "VogueLuxe Sales Report")

	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Report Period: %s to %s (%s)", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), period))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Summary")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 12)
	writeMetric(pdf, "Total Revenue", fmt.Sprintf("%.2f", data.TotalRevenue))
	writeMetric(pdf, "Total Delivered Orders", fmt.Sprintf("%d", data.TotalOrders))
	writeMetric(pdf, "Average Order Value", fmt.Sprintf("%.2f", data.AvgOrderValue))
	writeMetric(pdf, "Total Discount", fmt.Sprintf("%.2f", data.TotalDiscount))
	writeMetric(pdf, "Total Amount", fmt.Sprintf("%.2f", data.TotalAmount))

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Order Details")
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(50, 10, "Order ID", "1", 0, "", false, 0, "")
	pdf.CellFormat(40, 10, "Customer", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Date", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Amount  ", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Discount  ", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Status", "1", 1, "", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, order := range data.Orders {
		var shortOrderID string
		shortOrderID = order.OrderUID[:3] + order.OrderUID[len(order.OrderUID)-6:]
		pdf.CellFormat(50, 10, shortOrderID, "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 10, order.UserName, "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, order.OrderDate.Format("2006-01-02"), "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, fmt.Sprintf("%.2f", order.Total), "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, fmt.Sprintf("%.2f", order.Discount), "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, order.Status, "1", 1, "", false, 0, "")
	}

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Top 10 Best-Selling Products")
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(60, 10, "Product Name", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Units Sold", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Revenue ", "1", 1, "", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, product := range data.TopProducts {
		pdf.CellFormat(60, 10, product.ProductName, "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, fmt.Sprintf("%d", product.TotalSold), "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", product.Revenue), "1", 1, "", false, 0, "")
	}

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Top 10 Best-Selling Categories")
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(60, 10, "Category Name", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Units Sold", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Revenue ", "1", 1, "", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, category := range data.TopCategories {
		pdf.CellFormat(60, 10, category.CategoryName, "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, fmt.Sprintf("%d", category.TotalSold), "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", category.Revenue), "1", 1, "", false, 0, "")
	}

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Top 10 Best-Selling Brands")
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(60, 10, "Brand Name", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Units Sold", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Revenue ", "1", 1, "", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, brand := range data.TopBrands {
		pdf.CellFormat(60, 10, brand.BrandName, "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, fmt.Sprintf("%d", brand.TotalSold), "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", brand.Revenue), "1", 1, "", false, 0, "")
	}

	c.Header("Content-Disposition", "attachment; filename=sales_report.pdf")
	c.Header("Content-Type", "application/pdf")
	if err := pdf.Output(c.Writer); err != nil {
		logger.Log.Error("Failed to generate PDF", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}

	logger.Log.Info("PDF report generated successfully",
		zap.String("startDate", startDate.Format("2006-01-02")),
		zap.String("endDate", endDate.Format("2006-01-02")))
}

func DownloadExcel(c *gin.Context) {
	logger.Log.Info("Requested to download Excel report")
	startDateStr := c.Query("start")
	endDateStr := c.Query("end")
	period := c.Query("period")
	if period == "" {
		period = "daily"
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		logger.Log.Error("Invalid start date",
			zap.String("startDate", startDateStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date"})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		logger.Log.Error("Invalid end date",
			zap.String("endDate", endDateStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date"})
		return
	}

	if startDate.After(endDate) {
		logger.Log.Info("Swapped startDate and endDate to ensure startDate <= endDate",
			zap.String("startDate", startDate.Format("2006-01-02")),
			zap.String("endDate", endDate.Format("2006-01-02")))
		startDate, endDate = endDate, startDate
	}

	data := fetchDashboardData(startDate, endDate, period)

	c.Header("Content-Disposition", "attachment; filename=sales_report.csv")
	c.Header("Content-Type", "text/csv")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"VogueLuxe Sales Report"})
	writer.Write([]string{fmt.Sprintf("Period: %s to %s (%s)", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), period)})
	writer.Write([]string{""})

	writer.Write([]string{"Summary"})
	writer.Write([]string{"Metric", "Value"})
	writer.Write([]string{"Total Revenue", fmt.Sprintf("₹%.2f", data.TotalRevenue)})
	writer.Write([]string{"Total Delivered Orders", fmt.Sprintf("%d", data.TotalOrders)})
	writer.Write([]string{"Average Order Value", fmt.Sprintf("₹%.2f", data.AvgOrderValue)})
	writer.Write([]string{"Total Discount", fmt.Sprintf("₹%.2f", data.TotalDiscount)})
	writer.Write([]string{"Total Amount", fmt.Sprintf("₹%.2f", data.TotalAmount)})
	writer.Write([]string{""})

	writer.Write([]string{"Order Details"})
	writer.Write([]string{"Order ID", "Customer", "Date", "Amount", "Discount", "Status"})
	for _, order := range data.Orders {
		writer.Write([]string{
			order.OrderUID,
			order.UserName,
			order.OrderDate.Format("2006-01-02"),
			fmt.Sprintf("%.2f", order.Total),
			fmt.Sprintf("%.2f", order.Discount),
			order.Status,
		})
	}

	writer.Write([]string{""})
	writer.Write([]string{"Top 10 Best-Selling Products"})
	writer.Write([]string{"Product Name", "Units Sold", "Revenue "})
	for _, product := range data.TopProducts {
		writer.Write([]string{
			product.ProductName,
			fmt.Sprintf("%d", product.TotalSold),
			fmt.Sprintf("%.2f", product.Revenue),
		})
	}

	writer.Write([]string{""})
	writer.Write([]string{"Top 10 Best-Selling Categories"})
	writer.Write([]string{"Category Name", "Units Sold", "Revenue"})
	for _, category := range data.TopCategories {
		writer.Write([]string{
			category.CategoryName,
			fmt.Sprintf("%d", category.TotalSold),
			fmt.Sprintf("%.2f", category.Revenue),
		})
	}

	writer.Write([]string{""})
	writer.Write([]string{"Top 10 Best-Selling Brands"})
	writer.Write([]string{"Brand Name", "Units Sold", "Revenue"})
	for _, brand := range data.TopBrands {
		writer.Write([]string{
			brand.BrandName,
			fmt.Sprintf("%d", brand.TotalSold),
			fmt.Sprintf("%.2f", brand.Revenue),
		})
	}

	logger.Log.Info("Excel report generated successfully",
		zap.String("startDate", startDate.Format("2006-01-02")),
		zap.String("endDate", endDate.Format("2006-01-02")))
}

func writeMetric(pdf *gofpdf.Fpdf, label, value string) {
	pdf.Cell(60, 10, label)
	pdf.Cell(40, 10, value)
	pdf.Ln(8)
}

func (o OrderSummary) MarshalJSON() ([]byte, error) {
	type Alias OrderSummary
	return json.Marshal(&struct {
		*Alias
		OrderDate string `json:"OrderDate"`
	}{
		Alias:     (*Alias)(&o),
		OrderDate: o.OrderDate.Format("2006-01-02"),
	})
}

func AdminUserManagement(c *gin.Context) {
	logger.Log.Info("Requested to show user management page")
	var users []models.User

	if err := config.DB.Preload("UserDetails").Find(&users).Error; err != nil {
		logger.Log.Error("Failed to fetch users", zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_User_Management.html", gin.H{
			"error": "Failed to fetch users",
		})
		return
	}

	for _, user := range users {
		logger.Log.Info("User details",
			zap.Uint("userID", user.ID),
			zap.String("firstName", user.FirstName),
			zap.String("lastName", user.LastName),
			zap.String("email", user.Email),
			zap.Bool("isActive", user.UserDetails.IsActive))
	}

	logger.Log.Info("User management page loaded successfully", zap.Int("userCount", len(users)))
	c.HTML(http.StatusOK, "Admin_User_Management.html", gin.H{
		"users": users,
	})
}

func AdminBlockUser(c *gin.Context) {
	userID := c.Param("id")
	logger.Log.Info("Requested to block user", zap.String("userID", userID))

	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
		logger.Log.Error("User not found", zap.String("userID", userID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.UserDetails.IsActive = false
	if err := config.DB.Save(&user.UserDetails).Error; err != nil {
		logger.Log.Error("Failed to block user", zap.String("userID", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block user"})
		return
	}

	logger.Log.Info("User blocked successfully", zap.String("userID", userID))
	c.JSON(http.StatusOK, gin.H{
		"status":  "Success",
		"message": "User blocked successfully",
	})
}

func AdminUnblockUser(c *gin.Context) {
	userID := c.Param("id")
	logger.Log.Info("Requested to unblock user", zap.String("userID", userID))

	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
		logger.Log.Error("User not found", zap.String("userID", userID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.UserDetails.IsActive = true
	if err := config.DB.Save(&user.UserDetails).Error; err != nil {
		logger.Log.Error("Failed to unblock user", zap.String("userID", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unblock user"})
		return
	}

	logger.Log.Info("User unblocked successfully", zap.String("userID", userID))
	c.JSON(http.StatusOK, gin.H{
		"status":  "Success",
		"message": "User unblocked successfully",
	})
}

func AdminLogout(c *gin.Context) {
	logger.Log.Info("Admin logout requested")
	c.SetCookie("jwtTokensAdmin", "", -1, "/", "", false, true)
	logger.Log.Info("Admin logged out successfully")
	c.Redirect(http.StatusSeeOther, "/admin/login")
}