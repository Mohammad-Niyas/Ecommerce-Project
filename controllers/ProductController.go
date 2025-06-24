package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"ecommerce/utils"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func renderError(c *gin.Context, status int, message string, productData gin.H) {
	logger.Log.Error("Rendering error for product addition",
		zap.Int("status", status),
		zap.String("message", message))
	var categories []models.Category
	if err := config.DB.Find(&categories).Error; err != nil {
		logger.Log.Error("Failed to load categories",
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Product_Add.html", gin.H{
			"Error": "Failed to load categories",
		})
		return
	}
	data := gin.H{
		"Error":    message,
		"Category": categories,
	}
	for k, v := range productData {
		data[k] = v
	}
	logger.Log.Info("Rendering Admin_Product_Add.html with error",
		zap.String("message", message))
	c.HTML(status, "Admin_Product_Add.html", data)
}

func validateProductForm(c *gin.Context) (map[string]interface{}, bool) {
	logger.Log.Info("Validating product form data")
	productName := c.PostForm("product_name")
	description := c.PostForm("description")
	brand := c.PostForm("brand")
	categoryIDStr := c.PostForm("category_id")
	sizes := c.PostFormArray("size[]")
	stocks := c.PostFormArray("stock[]")
	actualPrices := c.PostFormArray("actual_price[]")

	if productName == "" || categoryIDStr == "" {
		logger.Log.Error("Product name or category ID is empty")
		return nil, false
	}
	if len(sizes) == 0 || len(stocks) == 0 || len(actualPrices) == 0 {
		logger.Log.Error("No variant data provided",
			zap.Int("sizesCount", len(sizes)),
			zap.Int("stocksCount", len(stocks)),
			zap.Int("pricesCount", len(actualPrices)))
		return nil, false
	}

	catID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		logger.Log.Error("Invalid category ID",
			zap.String("categoryIDStr", categoryIDStr),
			zap.Error(err))
		return nil, false
	}

	var category models.Category
	if err := config.DB.First(&category, catID).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.Int("categoryID", catID),
			zap.Error(err))
		return nil, false
	}

	logger.Log.Info("Product form validated successfully",
		zap.String("productName", productName),
		zap.Int("categoryID", catID))
	return map[string]interface{}{
		"productName":  productName,
		"description":  description,
		"brand":        brand,
		"categoryID":   uint(catID),
		"sizes":        sizes,
		"stocks":       stocks,
		"actualPrices": actualPrices,
	}, true
}

func processImages(c *gin.Context, tx *gorm.DB, productID uint) (bool, string) {
	logger.Log.Info("Processing images for product",
		zap.Uint("productID", productID))
	form, err := c.MultipartForm()
	if err != nil {
		logger.Log.Error("Invalid file upload",
			zap.Error(err))
		return false, "Invalid file upload"
	}

	files := form.File["images[]"]
	if len(files) < 3 {
		logger.Log.Error("Insufficient images uploaded",
			zap.Int("imageCount", len(files)))
		return false, "At least 3 images are required"
	}

	cloudService, err := utils.NewCloudinaryService()
	if err != nil {
		logger.Log.Error("Failed to initialize Cloudinary",
			zap.Error(err))
		return false, "Failed to initialize Cloudinary"
	}

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			logger.Log.Error("Failed to open image file",
				zap.Int("fileIndex", i),
				zap.Error(err))
			return false, "Failed to open image file"
		}
		defer file.Close()

		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			logger.Log.Error("Failed to read image file",
				zap.Int("fileIndex", i),
				zap.Error(err))
			return false, "Failed to read image file"
		}
		file.Seek(0, 0)
		contentType := http.DetectContentType(buffer)
		if !strings.HasPrefix(contentType, "image/") {
			logger.Log.Error("Invalid file type",
				zap.Int("fileIndex", i),
				zap.String("contentType", contentType))
			return false, "Invalid file type: only images are allowed"
		}

		cloudURL, err := cloudService.UploadImage(file, "ecommerce_products")
		if err != nil {
			logger.Log.Error("Image upload failed",
				zap.Int("fileIndex", i),
				zap.Error(err))
			return false, "Image upload failed"
		}

		productImage := models.ProductImage{
			ProductID: productID,
			ImageURL:  cloudURL,
		}
		if err := tx.Create(&productImage).Error; err != nil {
			logger.Log.Error("Failed to save product image",
				zap.Int("fileIndex", i),
				zap.String("imageURL", cloudURL),
				zap.Error(err))
			return false, "Failed to save product image"
		}
	}
	logger.Log.Info("Images processed successfully",
		zap.Uint("productID", productID),
		zap.Int("imageCount", len(files)))
	return true, ""
}

func AdminProducts(c *gin.Context) {
	logger.Log.Info("Requested to show product management page")
	var products []models.Product
	var categories []models.Category

	err := config.DB.Preload("Variants").Preload("Images").Preload("Category").Find(&products).Error
	if err != nil {
		logger.Log.Error("Failed to fetch products",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch products",
		})
		return
	}

	err = config.DB.Find(&categories).Error
	if err != nil {
		logger.Log.Error("Failed to fetch categories",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch categories",
		})
		return
	}

	type ProductWithTotalStock struct {
		models.Product
		TotalStock int
	}

	var productsWithStock []ProductWithTotalStock
	for _, p := range products {
		totalStock := 0
		for _, v := range p.Variants {
			totalStock += v.StockCount
		}
		productsWithStock = append(productsWithStock, ProductWithTotalStock{
			Product:    p,
			TotalStock: totalStock,
		})
	}

	logger.Log.Info("Rendering Admin_Product_Management.html",
		zap.Int("productCount", len(productsWithStock)),
		zap.Int("categoryCount", len(categories)))
	c.HTML(http.StatusOK, "Admin_Product_Management.html", gin.H{
		"Products":   productsWithStock,
		"Categories": categories,
	})
}

func ShowAddProductPage(c *gin.Context) {
	logger.Log.Info("Requested to show add product page")
	var categories []models.Category
	if err := config.DB.Find(&categories).Error; err != nil {
		logger.Log.Error("Failed to find categories",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to find category",
		})
		return
	}
	logger.Log.Info("Rendering Admin_Product_Add.html",
		zap.Int("categoryCount", len(categories)))
	c.HTML(http.StatusOK, "Admin_Product_Add.html", gin.H{
		"Category": categories,
	})
}

func AddProduct(c *gin.Context) {
	logger.Log.Info("Requested to add new product")
	productName := strings.TrimSpace(c.PostForm("product_name"))
	description := strings.TrimSpace(c.PostForm("description"))
	brand := strings.TrimSpace(c.PostForm("brand"))
	categoryIDStr := c.PostForm("category_id")
	sizes := c.PostFormArray("size[]")
	stocks := c.PostFormArray("stock[]")
	actualPrices := c.PostFormArray("actual_price[]")

	formData := gin.H{
		"productName":  productName,
		"description":  description,
		"brand":        brand,
		"categoryID":   categoryIDStr,
		"sizes":        sizes,
		"stocks":       stocks,
		"actualPrices": actualPrices,
	}

	if productName == "" {
		logger.Log.Error("Product name is required")
		renderError(c, http.StatusBadRequest, "Product name is required", formData)
		return
	}
	if len(productName) <= 5 {
		logger.Log.Error("Product name too short",
			zap.String("productName", productName),
			zap.Int("length", len(productName)))
		renderError(c, http.StatusBadRequest, "Product name must be more than 5 characters", formData)
		return
	}
	if len(productName) > 255 {
		logger.Log.Error("Product name too long",
			zap.String("productName", productName),
			zap.Int("length", len(productName)))
		renderError(c, http.StatusBadRequest, "Product name must not exceed 255 characters", formData)
		return
	}

	if description == "" {
		logger.Log.Error("Description is required")
		renderError(c, http.StatusBadRequest, "Description is required", formData)
		return
	}
	if len(description) > 10000 {
		logger.Log.Error("Description too long",
			zap.Int("length", len(description)))
		renderError(c, http.StatusBadRequest, "Description must not exceed 10,000 characters", formData)
		return
	}

	if brand == "" {
		logger.Log.Error("Brand is required")
		renderError(c, http.StatusBadRequest, "Brand is required", formData)
		return
	}
	if len(brand) > 255 {
		logger.Log.Error("Brand too long",
			zap.String("brand", brand),
			zap.Int("length", len(brand)))
		renderError(c, http.StatusBadRequest, "Brand must not exceed 255 characters", formData)
		return
	}

	if categoryIDStr == "" {
		logger.Log.Error("Category is required")
		renderError(c, http.StatusBadRequest, "Category is required", formData)
		return
	}
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil || categoryID <= 0 {
		logger.Log.Error("Invalid category ID",
			zap.String("categoryIDStr", categoryIDStr),
			zap.Error(err))
		renderError(c, http.StatusBadRequest, "Invalid category ID", formData)
		return
	}

	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		logger.Log.Error("Selected category does not exist",
			zap.Int("categoryID", categoryID),
			zap.Error(err))
		renderError(c, http.StatusBadRequest, "Selected category does not exist", formData)
		return
	}

	if len(sizes) == 0 || len(stocks) == 0 || len(actualPrices) == 0 {
		logger.Log.Error("No variant data provided",
			zap.Int("sizesCount", len(sizes)),
			zap.Int("stocksCount", len(stocks)),
			zap.Int("pricesCount", len(actualPrices)))
		renderError(c, http.StatusBadRequest, "At least one size variant (size, stock, and price) is required", formData)
		return
	}
	if len(sizes) != len(stocks) || len(sizes) != len(actualPrices) {
		logger.Log.Error("Mismatch in variant data",
			zap.Int("sizesCount", len(sizes)),
			zap.Int("stocksCount", len(stocks)),
			zap.Int("pricesCount", len(actualPrices)))
		renderError(c, http.StatusBadRequest, "Mismatch in variant data (sizes, stocks, prices)", formData)
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.Error(tx.Error))
		renderError(c, http.StatusInternalServerError, "Failed to start transaction", formData)
		return
	}

	product := models.Product{
		ProductName: productName,
		Description: description,
		Brand:       brand,
		CategoryID:  uint(categoryID),
		IsActive:    true,
	}
	if err := tx.Create(&product).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to create product",
			zap.String("productName", productName),
			zap.Error(err))
		renderError(c, http.StatusInternalServerError, "Failed to create product", formData)
		return
	}

	success, errMsg := ProcessVariants(tx, product.ID, sizes, stocks, actualPrices)
	if !success {
		tx.Rollback()
		logger.Log.Error("Variant processing failed",
			zap.String("errorMsg", errMsg))
		renderError(c, http.StatusBadRequest, errMsg, formData)
		return
	}

	success, errMsg = processImages(c, tx, product.ID)
	if !success {
		tx.Rollback()
		logger.Log.Error("Image processing failed",
			zap.String("errorMsg", errMsg))
		renderError(c, http.StatusBadRequest, errMsg, formData)
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.Uint("productID", product.ID),
			zap.Error(err))
		renderError(c, http.StatusInternalServerError, "Failed to commit transaction", formData)
		return
	}

	logger.Log.Info("Product added successfully",
		zap.Uint("productID", product.ID),
		zap.String("productName", productName))
	c.Redirect(http.StatusSeeOther, "/admin/products?message=Product added successfully")
}

func ProcessVariants(tx *gorm.DB, productID uint, sizes, stocks, actualPrices []string) (bool, string) {
	logger.Log.Info("Processing variants for product",
		zap.Uint("productID", productID))
	if len(sizes) != len(stocks) || len(sizes) != len(actualPrices) {
		logger.Log.Error("Mismatch in variant data",
			zap.Int("sizesCount", len(sizes)),
			zap.Int("stocksCount", len(stocks)),
			zap.Int("pricesCount", len(actualPrices)))
		return false, "Mismatch in variant data"
	}

	sizeSet := make(map[string]bool)
	for i := 0; i < len(sizes); i++ {
		size := strings.TrimSpace(sizes[i])
		if size == "" {
			logger.Log.Error("Empty size provided",
				zap.Int("index", i))
			return false, "Size cannot be empty"
		}
		if sizeSet[size] {
			logger.Log.Error("Duplicate size found",
				zap.String("size", size))
			return false, "Duplicate sizes are not allowed"
		}
		sizeSet[size] = true

		stock, err := strconv.Atoi(stocks[i])
		if err != nil || stock < 0 {
			logger.Log.Error("Invalid stock value",
				zap.String("stock", stocks[i]),
				zap.Error(err))
			return false, "Stock must be a non-negative number"
		}

		actualPrice, err := strconv.ParseFloat(actualPrices[i], 64)
		if err != nil || actualPrice <= 0 {
			logger.Log.Error("Invalid actual price",
				zap.String("actualPrice", actualPrices[i]),
				zap.Error(err))
			return false, "Actual price must be a positive number"
		}

		variant := models.ProductVariant{
			ProductID:   productID,
			Size:        size,
			StockCount:  stock,
			ActualPrice: actualPrice,
			IsActive:    true,
		}

		if err := tx.Create(&variant).Error; err != nil {
			logger.Log.Error("Failed to create product variant",
				zap.String("size", size),
				zap.Error(err))
			return false, "Failed to create product variant: " + err.Error()
		}
	}
	logger.Log.Info("Variants processed successfully",
		zap.Uint("productID", productID),
		zap.Int("variantCount", len(sizes)))
	return true, ""
}

func ToggleProductStatus(c *gin.Context) {
	logger.Log.Info("Requested to toggle product status")
	productID := c.Param("id")

	var product models.Product
	if err := config.DB.First(&product, productID).Error; err != nil {
		logger.Log.Error("Product not found",
			zap.String("productID", productID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Product not found",
		})
		return
	}

	var category models.Category
	if err := config.DB.First(&category, product.CategoryID).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.Uint("categoryID", product.CategoryID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Category not found",
		})
		return
	}

	if !category.List {
		logger.Log.Error("Parent category is unlisted",
			zap.Uint("categoryID", product.CategoryID))
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "Cannot toggle product status because the parent category is unlisted",
		})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("productID", productID),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction: " + tx.Error.Error(),
		})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error("Panic occurred, rolling back transaction",
				zap.String("productID", productID),
				zap.Any("panic", r))
			tx.Rollback()
		}
	}()

	newStatus := !product.IsActive

	if err := tx.Model(&models.Product{}).
		Where("id = ?", productID).
		Update("is_active", newStatus).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update product status",
			zap.String("productID", productID),
			zap.Bool("newStatus", newStatus),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update product status: " + err.Error(),
		})
		return
	}

	if err := tx.Model(&models.ProductVariant{}).
		Where("product_id = ?", productID).
		Update("is_active", newStatus).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update product variants status",
			zap.String("productID", productID),
			zap.Bool("newStatus", newStatus),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update product variants status: " + err.Error(),
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("productID", productID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction: " + err.Error(),
		})
		return
	}

	logger.Log.Info("Product status updated successfully",
		zap.String("productID", productID),
		zap.Bool("newStatus", newStatus))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"is_active": newStatus,
	})
}

func ProductDetailPage(c *gin.Context) {
	logger.Log.Info("Requested to show product detail page")
	productIDStr := c.Param("id")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		logger.Log.Error("Invalid product ID",
			zap.String("productIDStr", productIDStr),
			zap.Error(err))
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"Error": "Invalid product ID"})
		return
	}

	var product models.Product
	err = config.DB.
		Preload("Category").
		Preload("Variants").
		Preload("Images").
		Preload("ProductOffers").
		Preload("Category.CategoryOffers").
		First(&product, productID).Error
	if err != nil {
		logger.Log.Error("Product not found",
			zap.Int("productID", productID),
			zap.Error(err))
		c.HTML(http.StatusNotFound, "error.html", gin.H{"Error": "Product not found"})
		return
	}

	for i, variant := range product.Variants {
		sellingPrice, _ := CalculateSellingPrice(variant, config.DB)
		product.Variants[i].SellingPrice = sellingPrice
		if err := config.DB.Save(&product.Variants[i]).Error; err != nil {
			logger.Log.Error("Failed to update variant",
				zap.Uint("variantID", variant.ID),
				zap.Error(err))
		}
	}

	totalStock := 0
	for _, variant := range product.Variants {
		totalStock += variant.StockCount
	}

	data := gin.H{
		"Product": struct {
			ID             uint
			ProductName    string
			Brand          string
			Category       models.Category
			Description    string
			IsActive       bool
			Images         []models.ProductImage
			Variants       []models.ProductVariant
			TotalStock     int
			ProductOffers  []models.ProductOffer
			CategoryOffers []models.CategoryOffer
		}{
			ID:             product.ID,
			ProductName:    product.ProductName,
			Brand:          product.Brand,
			Category:       product.Category,
			Description:    product.Description,
			IsActive:       product.IsActive,
			Images:         product.Images,
			Variants:       product.Variants,
			TotalStock:     totalStock,
			ProductOffers:  product.ProductOffers,
			CategoryOffers: product.Category.CategoryOffers,
		},
	}

	if len(product.ProductOffers) == 0 {
		logger.Log.Warn("No product offer found",
			zap.Uint("productID", product.ID))
		data["NoProductOffer"] = true
	}

	logger.Log.Info("Rendering Admin_Product_Detail.html",
		zap.Uint("productID", product.ID),
		zap.String("productName", product.ProductName))
	c.HTML(http.StatusOK, "Admin_Product_Detail.html", data)
}

func ToggleVariantStatus(c *gin.Context) {
	logger.Log.Info("Requested to toggle variant status")
	variantID := c.Param("id")
	var input struct {
		IsActive bool `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Log.Error("Invalid input",
			zap.String("variantID", variantID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input"})
		return
	}

	var variant models.ProductVariant
	if err := config.DB.First(&variant, variantID).Error; err != nil {
		logger.Log.Error("Variant not found",
			zap.String("variantID", variantID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Variant not found"})
		return
	}

	variant.IsActive = input.IsActive
	if err := config.DB.Save(&variant).Error; err != nil {
		logger.Log.Error("Failed to update variant",
			zap.String("variantID", variantID),
			zap.Bool("newStatus", input.IsActive),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update variant"})
		return
	}

	logger.Log.Info("Variant status updated successfully",
		zap.String("variantID", variantID),
		zap.Bool("newStatus", input.IsActive))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "success"})
}

func AdminEditProductPage(c *gin.Context) {
	logger.Log.Info("Requested to show edit product page")
	productID := c.Param("id")
	if productID == "" {
		logger.Log.Error("Invalid product ID")
		c.Redirect(http.StatusSeeOther, "/admin/products?error=Invalid product ID")
		return
	}

	id, err := strconv.Atoi(productID)
	if err != nil {
		logger.Log.Error("Invalid product ID format",
			zap.String("productID", productID),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/products?error=Invalid product ID format")
		return
	}

	var product models.Product
	if err := config.DB.Preload("Variants").Preload("Images").First(&product, id).Error; err != nil {
		logger.Log.Error("Product not found",
			zap.Int("productID", id),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/products?error=Product not found")
		return
	}

	var categories []models.Category
	if err := config.DB.Find(&categories).Error; err != nil {
		logger.Log.Error("Failed to load categories",
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Product_Edit.html", gin.H{
			"Error": "Failed to load categories",
		})
		return
	}

	logger.Log.Info("Rendering Admin_Product_Edit.html",
		zap.Uint("productID", product.ID),
		zap.String("productName", product.ProductName))
	c.HTML(http.StatusOK, "Admin_Product_Edit.html", gin.H{
		"Product":  product,
		"Category": categories,
	})
}

func AdminEditProduct(c *gin.Context) {
	logger.Log.Info("Requested to edit product")
	productID := c.Param("id")
	if productID == "" {
		logger.Log.Error("Invalid product ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	id, err := strconv.Atoi(productID)
	if err != nil {
		logger.Log.Error("Invalid product ID format",
			zap.String("productID", productID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID format"})
		return
	}

	var product models.Product
	if err := config.DB.Preload("Variants").Preload("Images").First(&product, id).Error; err != nil {
		logger.Log.Error("Product not found",
			zap.Int("productID", id),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	productName := strings.TrimSpace(c.PostForm("product_name"))
	description := strings.TrimSpace(c.PostForm("description"))
	brand := strings.TrimSpace(c.PostForm("brand"))
	categoryIDStr := c.PostForm("category_id")
	sizes := c.PostFormArray("size[]")
	stocks := c.PostFormArray("stock[]")
	actualPrices := c.PostFormArray("actual_price[]")
	deleteImageURLs := c.PostFormArray("delete_image[]")

	if productName == "" {
		logger.Log.Error("Product name is required",
			zap.Uint("productID", product.ID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product name is required"})
		return
	}
	if len(productName) <= 5 {
		logger.Log.Error("Product name too short",
			zap.Uint("productID", product.ID),
			zap.String("productName", productName),
			zap.Int("length", len(productName)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product name must be more than 5 characters"})
		return
	}
	if len(productName) > 255 {
		logger.Log.Error("Product name too long",
			zap.Uint("productID", product.ID),
			zap.String("productName", productName),
			zap.Int("length", len(productName)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product name must not exceed 255 characters"})
		return
	}

	if description == "" {
		logger.Log.Error("Description is required",
			zap.Uint("productID", product.ID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Description is required"})
		return
	}
	if len(description) > 10000 {
		logger.Log.Error("Description too long",
			zap.Uint("productID", product.ID),
			zap.Int("length", len(description)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Description must not exceed 10,000 characters"})
		return
	}

	if brand == "" {
		logger.Log.Error("Brand is required",
			zap.Uint("productID", product.ID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Brand is required"})
		return
	}
	if len(brand) > 255 {
		logger.Log.Error("Brand too long",
			zap.Uint("productID", product.ID),
			zap.String("brand", brand),
			zap.Int("length", len(brand)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Brand must not exceed 255 characters"})
		return
	}

	if categoryIDStr == "" {
		logger.Log.Error("Category is required",
			zap.Uint("productID", product.ID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Category is required"})
		return
	}
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil || categoryID <= 0 {
		logger.Log.Error("Invalid category ID",
			zap.Uint("productID", product.ID),
			zap.String("categoryIDStr", categoryIDStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		logger.Log.Error("Selected category does not exist",
			zap.Uint("productID", product.ID),
			zap.Int("categoryID", categoryID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected category does not exist"})
		return
	}

	if len(sizes) == 0 || len(stocks) == 0 || len(actualPrices) == 0 {
		logger.Log.Error("No variant data provided",
			zap.Uint("productID", product.ID),
			zap.Int("sizesCount", len(sizes)),
			zap.Int("stocksCount", len(stocks)),
			zap.Int("pricesCount", len(actualPrices)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one size variant (size, stock, and price) is required"})
		return
	}
	if len(sizes) != len(stocks) || len(sizes) != len(actualPrices) {
		logger.Log.Error("Mismatch in variant data",
			zap.Uint("productID", product.ID),
			zap.Int("sizesCount", len(sizes)),
			zap.Int("stocksCount", len(stocks)),
			zap.Int("pricesCount", len(actualPrices)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mismatch in variant data (sizes, stocks, prices)"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.Uint("productID", product.ID),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	product.ProductName = productName
	product.Description = description
	product.Brand = brand
	product.CategoryID = uint(categoryID)
	if err := tx.Save(&product).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update product",
			zap.Uint("productID", product.ID),
			zap.String("productName", productName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	existingVariants := make(map[string]models.ProductVariant)
	for _, v := range product.Variants {
		existingVariants[v.Size] = v
	}

	var variantsToDelete []uint
	for i, size := range sizes {
		stock, err := strconv.Atoi(stocks[i])
		if err != nil {
			tx.Rollback()
			logger.Log.Error("Invalid stock value",
				zap.Uint("productID", product.ID),
				zap.String("stock", stocks[i]),
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stock value"})
			return
		}
		actualPrice, err := strconv.ParseFloat(actualPrices[i], 64)
		if err != nil {
			tx.Rollback()
			logger.Log.Error("Invalid price value",
				zap.Uint("productID", product.ID),
				zap.String("actualPrice", actualPrices[i]),
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price value"})
			return
		}

		if stock < 0 {
			tx.Rollback()
			logger.Log.Error("Stock cannot be negative",
				zap.Uint("productID", product.ID),
				zap.Int("stock", stock))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Stock cannot be negative"})
			return
		}
		if actualPrice <= 0 {
			tx.Rollback()
			logger.Log.Error("Price must be positive",
				zap.Uint("productID", product.ID),
				zap.Float64("actualPrice", actualPrice))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Price must be positive"})
			return
		}

		if existingVariant, exists := existingVariants[size]; exists {
			existingVariant.StockCount = stock
			existingVariant.ActualPrice = actualPrice
			existingVariant.SellingPrice, _ = CalculateSellingPrice(existingVariant, tx)
			if err := tx.Save(&existingVariant).Error; err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to update variant",
					zap.Uint("productID", product.ID),
					zap.String("size", size),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update variant"})
				return
			}
			delete(existingVariants, size)
		} else {
			variant := models.ProductVariant{
				ProductID:   product.ID,
				Size:        size,
				StockCount:  stock,
				ActualPrice: actualPrice,
				IsActive:    true,
			}
			variant.SellingPrice, _ = CalculateSellingPrice(variant, tx)
			if err := tx.Create(&variant).Error; err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to create variant",
					zap.Uint("productID", product.ID),
					zap.String("size", size),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create variant"})
				return
			}
		}
	}

	for _, variant := range existingVariants {
		variantsToDelete = append(variantsToDelete, variant.ID)
		variant.IsActive = false
		if err := tx.Save(&variant).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to mark variant as inactive",
				zap.Uint("variantID", variant.ID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update variant"})
			return
		}

		if err := tx.Model(&models.CartItem{}).
			Where("product_variant_id = ?", variant.ID).
			Update("is_valid", false).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to mark cart items as invalid",
				zap.Uint("variantID", variant.ID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart items"})
			return
		}
	}

	cloudService, err := utils.NewCloudinaryService()
	if err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to initialize Cloudinary",
			zap.Uint("productID", product.ID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Cloudinary"})
		return
	}

	for _, imageURL := range deleteImageURLs {
		if imageURL == "" {
			continue
		}
		publicID := utils.ExtractPublicIDFromURL(imageURL)
		if publicID != "" {
			if err := cloudService.DeleteImage(publicID); err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to delete image from Cloudinary",
					zap.String("imageURL", imageURL),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image from Cloudinary"})
				return
			}
		}
		if err := tx.Where("product_id = ? AND image_url = ?", product.ID, imageURL).Delete(&models.ProductImage{}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to delete image from database",
				zap.String("imageURL", imageURL),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image from database"})
			return
		}
	}

	form, err := c.MultipartForm()
	if err == nil {
		files := form.File["new_images[]"]
		for i, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to open image file",
					zap.Int("fileIndex", i),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open image file"})
				return
			}
			defer file.Close()

			buffer := make([]byte, 512)
			_, err = file.Read(buffer)
			if err != nil && err.Error() != "EOF" {
				tx.Rollback()
				logger.Log.Error("Failed to read image file",
					zap.Int("fileIndex", i),
					zap.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read image file"})
				return
			}
			file.Seek(0, 0)
			contentType := http.DetectContentType(buffer)
			if !strings.HasPrefix(contentType, "image/") {
				tx.Rollback()
				logger.Log.Error("Invalid file type",
					zap.Int("fileIndex", i),
					zap.String("contentType", contentType))
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type: only images are allowed"})
				return
			}

			cloudURL, err := cloudService.UploadImage(file, "ecommerce_products")
			if err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to upload image to Cloudinary",
					zap.Int("fileIndex", i),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
				return
			}

			productImage := models.ProductImage{
				ProductID: product.ID,
				ImageURL:  cloudURL,
			}
			if err := tx.Create(&productImage).Error; err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to save product image",
					zap.Int("fileIndex", i),
					zap.String("imageURL", cloudURL),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save product image"})
				return
			}
		}
	}

	var imageCount int64
	if err := tx.Model(&models.ProductImage{}).Where("product_id = ?", product.ID).Count(&imageCount).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to count images",
			zap.Uint("productID", product.ID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count images"})
		return
	}
	if imageCount < 3 {
		tx.Rollback()
		logger.Log.Error("Insufficient images",
			zap.Uint("productID", product.ID),
			zap.Int64("imageCount", imageCount))
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.Uint("productID", product.ID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	logger.Log.Info("Product updated successfully",
		zap.Uint("productID", product.ID),
		zap.String("productName", productName))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Product updated successfully"})
}

func GetOffer(c *gin.Context) {
	logger.Log.Info("Requested to get product offer")
	offerIDStr := c.Param("id")
	offerID, err := strconv.Atoi(offerIDStr)
	if err != nil {
		logger.Log.Error("Invalid offer ID",
			zap.String("offerIDStr", offerIDStr),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid offer ID"})
		return
	}

	var offer models.ProductOffer
	if err := config.DB.First(&offer, offerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Log.Error("Offer not found",
				zap.Int("offerID", offerID))
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Offer not found"})
			return
		}
		logger.Log.Error("Failed to load offer details",
			zap.Int("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load offer details: " + err.Error()})
		return
	}

	logger.Log.Info("Offer retrieved successfully",
		zap.Uint("offerID", offer.ID),
		zap.String("offerName", offer.OfferName))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"offer":   offer,
	})
}

func AddOffer(c *gin.Context) {
	logger.Log.Info("Requested to add product offer")
	var input struct {
		OfferName       string  `json:"offer_name"`
		OfferDetails    string  `json:"offer_details"`
		OfferPercentage float64 `json:"offer_percentage"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		ProductID       uint    `json:"product_id"`
		Status          string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Log.Error("Invalid input",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: " + err.Error()})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		logger.Log.Error("Invalid start date",
			zap.String("startDate", input.StartDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid start date"})
		return
	}
	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		logger.Log.Error("Invalid end date",
			zap.String("endDate", input.EndDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid end date"})
		return
	}

	offer := models.ProductOffer{
		OfferName:       input.OfferName,
		OfferDetails:    input.OfferDetails,
		OfferPercentage: input.OfferPercentage,
		StartDate:       startDate,
		EndDate:         endDate,
		ProductID:       input.ProductID,
		Status:          input.Status,
	}
	if offer.Status == "" {
		offer.Status = "Active"
	}

	if err := config.DB.Create(&offer).Error; err != nil {
		logger.Log.Error("Failed to add offer",
			zap.String("offerName", input.OfferName),
			zap.Uint("productID", input.ProductID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to add offer: " + err.Error()})
		return
	}

	logger.Log.Info("Offer added successfully",
		zap.Uint("offerID", offer.ID),
		zap.String("offerName", offer.OfferName),
		zap.Uint("productID", offer.ProductID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Offer added successfully"})
}

func EditOffer(c *gin.Context) {
	logger.Log.Info("Requested to edit product offer")
	offerID := c.Param("id")
	var input struct {
		OfferName       string  `json:"offer_name"`
		OfferDetails    string  `json:"offer_details"`
		OfferPercentage float64 `json:"offer_percentage"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		ProductID       uint    `json:"product_id"`
		Status          string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Log.Error("Invalid input",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid input: " + err.Error()})
		return
	}

	if input.OfferName == "" {
		logger.Log.Error("Offer name is required",
			zap.String("offerID", offerID))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Offer name is required"})
		return
	}
	if input.OfferPercentage <= 0 || input.OfferPercentage > 100 {
		logger.Log.Error("Invalid offer percentage",
			zap.String("offerID", offerID),
			zap.Float64("offerPercentage", input.OfferPercentage))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Offer percentage must be between 0 and 100"})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		logger.Log.Error("Invalid start date format",
			zap.String("offerID", offerID),
			zap.String("startDate", input.StartDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid start date format (use YYYY-MM-DD)"})
		return
	}
	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		logger.Log.Error("Invalid end date format",
			zap.String("offerID", offerID),
			zap.String("endDate", input.EndDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid end date format (use YYYY-MM-DD)"})
		return
	}
	if endDate.Before(startDate) {
		logger.Log.Error("End date before start date",
			zap.String("offerID", offerID),
			zap.String("startDate", input.StartDate),
			zap.String("endDate", input.EndDate))
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "End date must be after start date"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("offerID", offerID),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to start transaction"})
		return
	}

	var offer models.ProductOffer
	if err := tx.First(&offer, offerID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Log.Error("Offer not found",
				zap.String("offerID", offerID))
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Offer not found"})
		} else {
			logger.Log.Error("Failed to fetch offer",
				zap.String("offerID", offerID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch offer"})
		}
		return
	}

	offer.OfferName = input.OfferName
	offer.OfferDetails = input.OfferDetails
	offer.OfferPercentage = input.OfferPercentage
	offer.StartDate = startDate
	offer.EndDate = endDate
	offer.ProductID = input.ProductID
	if input.Status != "" {
		offer.Status = input.Status
	}

	if err := tx.Save(&offer).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update offer",
			zap.String("offerID", offerID),
			zap.String("offerName", input.OfferName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update offer"})
		return
	}

	var product models.Product
	if err := tx.Preload("Variants").First(&product, input.ProductID).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to fetch product",
			zap.Uint("productID", input.ProductID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch product"})
		return
	}

	for i, variant := range product.Variants {
		sellingPrice, _ := CalculateSellingPrice(variant, tx)
		product.Variants[i].SellingPrice = sellingPrice
		if err := tx.Save(&product.Variants[i]).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to update variant",
				zap.Uint("variantID", variant.ID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update variant"})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to commit transaction"})
		return
	}

	logger.Log.Info("Offer updated successfully",
		zap.String("offerID", offerID),
		zap.String("offerName", input.OfferName),
		zap.Uint("productID", input.ProductID))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Offer updated successfully"})
}
