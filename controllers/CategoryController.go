package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CategoryManagementPage(c *gin.Context) {
	logger.Log.Info("Requested to show category management page")
	var category []models.Category
	if err := config.DB.Find(&category).Error; err != nil {
		logger.Log.Error("Failed to fetch categories", zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Category_Management.html", gin.H{
			"error": "Failed to fetch categories: " + err.Error(),
		})
		return
	}

	logger.Log.Info("Fetched categories successfully", zap.Int("categoryCount", len(category)))
	c.HTML(http.StatusOK, "Admin_Category_Management.html", gin.H{
		"category": category,
	})
}

func AddCategoryPage(c *gin.Context) {
	logger.Log.Info("Requested to show add category page")
	c.HTML(http.StatusOK, "Admin_Category_Add.html", nil)
}

func AddCategory(c *gin.Context) {
	logger.Log.Info("Requested to add new category")
	categoryName := strings.TrimSpace(c.PostForm("categoryName"))
	description := strings.TrimSpace(c.PostForm("description"))

	if categoryName == "" {
		logger.Log.Error("Category name is empty")
		c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
			"error":        "Category name is required and cannot be empty or just spaces",
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	}
	if len(categoryName) > 255 {
		logger.Log.Error("Category name exceeds 255 characters", zap.String("categoryName", categoryName))
		c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
			"error":        "Category name must not exceed 255 characters",
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	}

	if description == "" {
		logger.Log.Error("Description is empty")
		c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
			"error":        "Description is required and cannot be empty or just spaces",
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	}
	if len(description) > 255 {
		logger.Log.Error("Description exceeds 255 characters", zap.String("description", description))
		c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
			"error":        "Description must not exceed 255 characters",
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	}

	var existingCategory models.Category
	if err := config.DB.Where("category_name = ?", categoryName).First(&existingCategory).Error; err == nil {
		logger.Log.Error("Category already exists", zap.String("categoryName", categoryName))
		c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
			"error":        "Category with this name already exists",
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	} else if err != gorm.ErrRecordNotFound {
		logger.Log.Error("Database error while checking existing category",
			zap.String("categoryName", categoryName),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Category_Add.html", gin.H{
			"error":        "Database error: " + err.Error(),
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	}

	newCategory := models.Category{
		CategoryName: categoryName,
		Description:  description,
		List:         true,
	}

	if err := config.DB.Create(&newCategory).Error; err != nil {
		logger.Log.Error("Failed to create category",
			zap.String("categoryName", categoryName),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Category_Add.html", gin.H{
			"error":        "Failed to add category: " + err.Error(),
			"categoryName": c.PostForm("categoryName"),
			"description":  c.PostForm("description"),
		})
		return
	}

	logger.Log.Info("Category added successfully",
		zap.String("categoryName", categoryName),
		zap.Uint("categoryID", newCategory.ID))
	c.Redirect(http.StatusSeeOther, "/admin/categories?success=Category added successfully")
}

func EditCategoryPage(c *gin.Context) {
	logger.Log.Info("Requested to show edit category page")
	id := c.Param("id")

	var category models.Category
	if err := config.DB.First(&category, id).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.String("categoryID", id),
			zap.Error(err))
		c.HTML(http.StatusNotFound, "Admin_Category_Management.html", gin.H{
			"error": "Category not found",
		})
		return
	}

	logger.Log.Info("Category loaded for editing",
		zap.String("categoryID", id),
		zap.String("categoryName", category.CategoryName))
	c.HTML(http.StatusOK, "Admin_Category_Edit.html", gin.H{
		"category": category,
	})
}

func EditCategory(c *gin.Context) {
	logger.Log.Info("Requested to edit category")
	id := c.Param("id")

	var input struct {
		CategoryName string `form:"categoryName"`
		Description  string `form:"description"`
	}

	if err := c.ShouldBind(&input); err != nil {
		logger.Log.Error("Invalid input",
			zap.String("categoryID", id),
			zap.Error(err))
		c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
			"error":        "Invalid input: " + err.Error(),
			"categoryName": input.CategoryName,
			"description":  input.Description,
		})
		return
	}

	input.CategoryName = strings.TrimSpace(input.CategoryName)
	input.Description = strings.TrimSpace(input.Description)

	if input.CategoryName == "" {
		logger.Log.Error("Category name is required",
			zap.String("categoryID", id))
		c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
			"error":        "Category name is required",
			"categoryName": input.CategoryName,
			"description":  input.Description,
			"category":     models.Category{Model: gorm.Model{ID: uintFromString(id)}},
		})
		return
	}
	if len(input.CategoryName) > 255 {
		logger.Log.Error("Category name exceeds 255 characters",
			zap.String("categoryID", id),
			zap.String("categoryName", input.CategoryName))
		c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
			"error":        "Category name must not exceed 255 characters",
			"categoryName": input.CategoryName,
			"description":  input.Description,
			"category":     models.Category{Model: gorm.Model{ID: uintFromString(id)}},
		})
		return
	}
	if len(input.Description) > 255 {
		logger.Log.Error("Description exceeds 255 characters",
			zap.String("categoryID", id),
			zap.String("description", input.Description))
		c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
			"error":        "Description must not exceed 255 characters",
			"categoryName": input.CategoryName,
			"description":  input.Description,
			"category":     models.Category{Model: gorm.Model{ID: uintFromString(id)}},
		})
		return
	}

	var category models.Category
	if err := config.DB.First(&category, id).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.String("categoryID", id),
			zap.Error(err))
		c.HTML(http.StatusNotFound, "Admin_Category_Edit.html", gin.H{
			"error": "Category not found",
		})
		return
	}

	var existingCategory models.Category
	if err := config.DB.Where("category_name = ? AND id != ?", input.CategoryName, id).First(&existingCategory).Error; err == nil {
		logger.Log.Error("Category with name already exists",
			zap.String("categoryID", id),
			zap.String("categoryName", input.CategoryName))
		c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
			"error":        "Category with this name already exists",
			"categoryName": input.CategoryName,
			"description":  input.Description,
			"category":     category,
		})
		return
	}

	if input.CategoryName != "" {
		category.CategoryName = input.CategoryName
	}
	if input.Description != "" {
		category.Description = input.Description
	}

	if err := config.DB.Save(&category).Error; err != nil {
		logger.Log.Error("Failed to update category",
			zap.String("categoryID", id),
			zap.String("categoryName", input.CategoryName),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "Admin_Category_Edit.html", gin.H{
			"error":        "Failed to edit category: " + err.Error(),
			"categoryName": input.CategoryName,
			"description":  input.Description,
			"category":     category,
		})
		return
	}

	logger.Log.Info("Category updated successfully",
		zap.String("categoryID", id),
		zap.String("categoryName", category.CategoryName))
	c.Redirect(http.StatusFound, "/admin/categories?success=Category updated successfully")
}

func uintFromString(id string) uint {
	val, _ := strconv.ParseUint(id, 10, 32)
	return uint(val)
}

func ToggleCategoryStatus(c *gin.Context) {
	logger.Log.Info("Requested to toggle category status")
	id := c.Param("id")

	var category models.Category
	if err := config.DB.First(&category, id).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.String("categoryID", id),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Category not found",
		})
		return
	}

	newStatus := !category.List
	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("categoryID", id),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start transaction: " + tx.Error.Error(),
		})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error("Panic occurred, rolling back transaction",
				zap.String("categoryID", id),
				zap.Any("panic", r))
			tx.Rollback()
		}
	}()

	if !newStatus {
		if err := tx.Model(&models.Product{}).
			Where("category_id = ?", id).
			Update("is_active", false).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to unlist products",
				zap.String("categoryID", id),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to unlist products: " + err.Error(),
			})
			return
		}

		if err := tx.Exec(`
			UPDATE product_variants pv
			SET is_active = false
			FROM products p
			WHERE p.id = pv.product_id
			AND p.category_id = ?`, id).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to unlist product variants",
				zap.String("categoryID", id),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to unlist product variants: " + err.Error(),
			})
			return
		}

		if err := tx.Model(&models.CategoryOffer{}).
			Where("category_id = ?", id).
			Update("offer_status", "Inactive").Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to deactivate category offers",
				zap.String("categoryID", id),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to deactivate category offers: " + err.Error(),
			})
			return
		}
	} else {
		if err := tx.Model(&models.Product{}).
			Where("category_id = ?", id).
			Update("is_active", true).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to list products",
				zap.String("categoryID", id),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to list products: " + err.Error(),
			})
			return
		}

		if err := tx.Exec(`
			UPDATE product_variants pv
			SET is_active = true
			FROM products p
			WHERE p.id = pv.product_id
			AND p.category_id = ?`, id).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to list product variants",
				zap.String("categoryID", id),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to list product variants: " + err.Error(),
			})
			return
		}
	}

	if err := tx.Model(&category).
		Where("id = ?", id).
		Update("list", newStatus).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update category status",
			zap.String("categoryID", id),
			zap.Bool("newStatus", newStatus),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update category status: " + err.Error(),
		})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("categoryID", id),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to commit transaction: " + err.Error(),
		})
		return
	}

	logger.Log.Info("Category status updated successfully",
		zap.String("categoryID", id),
		zap.Bool("newStatus", newStatus))
	c.JSON(http.StatusOK, gin.H{
		"message": "Category status updated successfully",
		"status":  newStatus,
	})
}

func CategoryDetailsPage(c *gin.Context) {
	logger.Log.Info("Requested to show category details page")
	id := c.Param("id")

	var category models.Category
	if err := config.DB.Preload("CategoryOffers").First(&category, id).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.String("categoryID", id),
			zap.Error(err))
		c.HTML(http.StatusNotFound, "Admin_Category_Management.html", gin.H{
			"error": "Category not found",
		})
		return
	}

	logger.Log.Info("Category details loaded successfully",
		zap.String("categoryID", id),
		zap.String("categoryName", category.CategoryName))
	c.HTML(http.StatusOK, "Admin_Category_Details.html", gin.H{
		"category": category,
	})
}

func AddCategoryOffer(c *gin.Context) {
	logger.Log.Info("Requested to add category offer")
	var input struct {
		CategoryID         string  `json:"categoryId"`
		OfferName          string  `json:"offerName"`
		OfferDescription   string  `json:"offerDescription"`
		DiscountPercentage float64 `json:"discountPercentage"`
		StartDate          string  `json:"startDate"`
		EndDate            string  `json:"endDate"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Log.Error("Failed to bind JSON",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid input: " + err.Error()})
		return
	}
	logger.Log.Info("Received input data",
		zap.String("categoryID", input.CategoryID),
		zap.String("offerName", input.OfferName),
		zap.Float64("discountPercentage", input.DiscountPercentage))

	categoryID, err := strconv.ParseUint(input.CategoryID, 10, 32)
	if err != nil {
		logger.Log.Error("Invalid category ID",
			zap.String("categoryID", input.CategoryID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid category ID: " + err.Error()})
		return
	}

	var category models.Category
	if err := config.DB.First(&category, uint(categoryID)).Error; err != nil {
		logger.Log.Error("Category not found",
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Category not found"})
		return
	}

	if input.OfferName == "" {
		logger.Log.Error("Offer name is required",
			zap.Uint("categoryID", uint(categoryID)))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer name is required"})
		return
	}

	if input.OfferDescription == "" {
		logger.Log.Error("Offer description is required",
			zap.Uint("categoryID", uint(categoryID)))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer description is required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		logger.Log.Error("Invalid start date format",
			zap.String("startDate", input.StartDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid start date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		logger.Log.Error("Invalid end date format",
			zap.String("endDate", input.EndDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid end date format. Use YYYY-MM-DD"})
		return
	}

	if endDate.Before(startDate) {
		logger.Log.Error("End date before start date",
			zap.String("startDate", input.StartDate),
			zap.String("endDate", input.EndDate))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "End date cannot be before start date"})
		return
	}

	if input.DiscountPercentage <= 0 || input.DiscountPercentage > 100 {
		logger.Log.Error("Invalid discount percentage",
			zap.Float64("discountPercentage", input.DiscountPercentage))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Discount percentage must be between 0 and 100"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction: " + tx.Error.Error()})
		return
	}

	newOffer := models.CategoryOffer{
		CategoryOfferName:       input.OfferName,
		CategoryOfferPercentage: input.DiscountPercentage,
		OfferDescription:        input.OfferDescription,
		CategoryID:              uint(categoryID),
		OfferStatus:             "Active",
		StartDate:               startDate,
		EndDate:                 endDate,
	}

	if err := tx.Create(&newOffer).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to create offer",
			zap.Uint("categoryID", uint(categoryID)),
			zap.String("offerName", input.OfferName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to add offer: " + err.Error()})
		return
	}

	if err := updateCategorySellingPrices(uint(categoryID), tx); err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update selling prices",
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update selling prices: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction: " + err.Error()})
		return
	}

	logger.Log.Info("Category offer added successfully",
		zap.Uint("categoryID", uint(categoryID)),
		zap.String("offerName", input.OfferName),
		zap.Uint("offerID", newOffer.ID))
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Offer added successfully"})
}

func EditCategoryOffer(c *gin.Context) {
	logger.Log.Info("Requested to edit category offer")
	offerID := c.Param("id")

	var input struct {
		CategoryID         string  `json:"categoryId"`
		OfferName          string  `json:"offerName"`
		OfferDescription   string  `json:"offerDescription"`
		DiscountPercentage float64 `json:"discountPercentage"`
		StartDate          string  `json:"startDate"`
		EndDate            string  `json:"endDate"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		logger.Log.Error("Failed to bind JSON",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid input: " + err.Error()})
		return
	}
	logger.Log.Info("Received input data",
		zap.String("offerID", offerID),
		zap.String("categoryID", input.CategoryID),
		zap.String("offerName", input.OfferName),
		zap.Float64("discountPercentage", input.DiscountPercentage))

	categoryID, err := strconv.ParseUint(input.CategoryID, 10, 32)
	if err != nil {
		logger.Log.Error("Invalid category ID",
			zap.String("offerID", offerID),
			zap.String("categoryID", input.CategoryID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid category ID: " + err.Error()})
		return
	}

	var offer models.CategoryOffer
	if err := config.DB.First(&offer, offerID).Error; err != nil {
		logger.Log.Error("Offer not found",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Offer not found"})
		return
	}

	if offer.CategoryID != uint(categoryID) {
		logger.Log.Error("Category mismatch",
			zap.String("offerID", offerID),
			zap.Uint("offerCategoryID", offer.CategoryID),
			zap.Uint("inputCategoryID", uint(categoryID)))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Category mismatch"})
		return
	}

	if input.OfferName == "" {
		logger.Log.Error("Offer name is required",
			zap.String("offerID", offerID))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer name is required"})
		return
	}

	if input.OfferDescription == "" {
		logger.Log.Error("Offer description is required",
			zap.String("offerID", offerID))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer description is required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		logger.Log.Error("Invalid start date format",
			zap.String("offerID", offerID),
			zap.String("startDate", input.StartDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid start date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		logger.Log.Error("Invalid end date format",
			zap.String("offerID", offerID),
			zap.String("endDate", input.EndDate),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid end date format. Use YYYY-MM-DD"})
		return
	}

	if endDate.Before(startDate) {
		logger.Log.Error("End date before start date",
			zap.String("offerID", offerID),
			zap.String("startDate", input.StartDate),
			zap.String("endDate", input.EndDate))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "End date cannot be before start date"})
		return
	}

	if input.DiscountPercentage <= 0 || input.DiscountPercentage > 100 {
		logger.Log.Error("Invalid discount percentage",
			zap.String("offerID", offerID),
			zap.Float64("discountPercentage", input.DiscountPercentage))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Discount percentage must be between 0 and 100"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("offerID", offerID),
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction: " + tx.Error.Error()})
		return
	}

	offer.CategoryOfferName = input.OfferName
	offer.CategoryOfferPercentage = input.DiscountPercentage
	offer.OfferDescription = input.OfferDescription
	offer.StartDate = startDate
	offer.EndDate = endDate

	if err := tx.Save(&offer).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update offer",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update offer: " + err.Error()})
		return
	}

	if err := updateCategorySellingPrices(uint(categoryID), tx); err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update selling prices",
			zap.String("offerID", offerID),
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update selling prices: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("offerID", offerID),
			zap.Uint("categoryID", uint(categoryID)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction: " + err.Error()})
		return
	}

	logger.Log.Info("Offer updated successfully",
		zap.String("offerID", offerID),
		zap.String("offerName", input.OfferName))
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Offer updated successfully"})
}

func ToggleCategoryOfferStatus(c *gin.Context) {
	logger.Log.Info("Requested to toggle category offer status")
	offerID := c.Param("id")

	var offer models.CategoryOffer
	if err := config.DB.First(&offer, offerID).Error; err != nil {
		logger.Log.Error("Offer not found",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Offer not found"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("offerID", offerID),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction: " + tx.Error.Error()})
		return
	}

	newStatus := "Inactive"
	if offer.OfferStatus == "Active" {
		offer.OfferStatus = "Inactive"
	} else {
		offer.OfferStatus = "Active"
		newStatus = "Active"
	}

	if err := tx.Save(&offer).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update offer status",
			zap.String("offerID", offerID),
			zap.String("newStatus", newStatus),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update offer status: " + err.Error()})
		return
	}

	if err := updateCategorySellingPrices(offer.CategoryID, tx); err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update selling prices",
			zap.String("offerID", offerID),
			zap.Uint("categoryID", offer.CategoryID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update selling prices: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("offerID", offerID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction: " + err.Error()})
		return
	}

	logger.Log.Info("Offer status updated successfully",
		zap.String("offerID", offerID),
		zap.String("newStatus", newStatus))
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Offer status updated successfully"})
}

func updateCategorySellingPrices(categoryID uint, db *gorm.DB) error {
	logger.Log.Info("Updating selling prices for category",
		zap.Uint("categoryID", categoryID))
	var products []models.Product
	if err := db.Preload("Variants").Where("category_id = ?", categoryID).Find(&products).Error; err != nil {
		logger.Log.Error("Failed to fetch products",
			zap.Uint("categoryID", categoryID),
			zap.Error(err))
		return fmt.Errorf("failed to fetch products: %v", err)
	}

	for _, product := range products {
		for i, variant := range product.Variants {
			sellingPrice, _ := CalculateSellingPrice(variant, db)
			product.Variants[i].SellingPrice = sellingPrice
			if db.Error != nil {
				logger.Log.Error("Transaction aborted before saving variant",
					zap.Uint("variantID", variant.ID),
					zap.Error(db.Error))
				return fmt.Errorf("transaction aborted before saving variant %d: %v", variant.ID, db.Error)
			}
			if err := db.Save(&product.Variants[i]).Error; err != nil {
				logger.Log.Error("Failed to update variant",
					zap.Uint("variantID", variant.ID),
					zap.Error(err))
				return fmt.Errorf("failed to update variant %d: %v", variant.ID, err)
			}
		}
	}
	logger.Log.Info("Selling prices updated successfully",
		zap.Uint("categoryID", categoryID),
		zap.Int("productCount", len(products)))
	return nil
}