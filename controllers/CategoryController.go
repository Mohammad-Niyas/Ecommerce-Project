package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CategoryManagementPage(c *gin.Context) {
	var category []models.Category
	config.DB.Find(&category)
	c.HTML(http.StatusOK, "Admin_Category_Management.html", gin.H{
		"category" : category,
	})
}


func AddCategoryPage(c *gin.Context){
	c.HTML(http.StatusOK,"Admin_Category_Add.html",nil)
}

func AddCategory(c *gin.Context) {
    categoryName := strings.TrimSpace(c.PostForm("categoryName"))
    description := strings.TrimSpace(c.PostForm("description"))


    if categoryName == "" {
        c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
            "error":        "Category name is required and cannot be empty or just spaces",
            "categoryName": c.PostForm("categoryName"), 
            "description":  c.PostForm("description"),
        })
        return
    }
    if len(categoryName) > 255 {
        c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
            "error":        "Category name must not exceed 255 characters",
            "categoryName": c.PostForm("categoryName"),
            "description":  c.PostForm("description"),
        })
        return
    }

    if description == "" {
        c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
            "error":        "Description is required and cannot be empty or just spaces",
            "categoryName": c.PostForm("categoryName"),
            "description":  c.PostForm("description"),
        })
        return
    }
    if len(description) > 255 {
        c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
            "error":        "Description must not exceed 255 characters",
            "categoryName": c.PostForm("categoryName"),
            "description":  c.PostForm("description"),
        })
        return
    }

    var existingCategory models.Category
    if err := config.DB.Where("category_name = ?", categoryName).First(&existingCategory).Error; err == nil {
        c.HTML(http.StatusBadRequest, "Admin_Category_Add.html", gin.H{
            "error":        "Category with this name already exists",
            "categoryName": c.PostForm("categoryName"),
            "description":  c.PostForm("description"),
        })
        return
    } else if err != gorm.ErrRecordNotFound {
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
        c.HTML(http.StatusInternalServerError, "Admin_Category_Add.html", gin.H{
            "error":        "Failed to add category: " + err.Error(),
            "categoryName": c.PostForm("categoryName"),
            "description":  c.PostForm("description"),
        })
        return
    }

    c.Redirect(http.StatusSeeOther, "/admin/categories?success=Category added successfully")
}

func EditCategoryPage(c *gin.Context){
	id := c.Param("id")

	var category models.Category
	if err:= config.DB.First(&category,id).Error;err!=nil{
		c.HTML(http.StatusNotFound,"Admin_Category_Management.html",gin.H{
			"error" : "Category not found",
		})
		return
	}

	c.HTML(http.StatusOK,"Admin_Category_Edit.html",gin.H{
		"category" : category,
	})
}

func EditCategory(c *gin.Context) {
    id := c.Param("id")

    var input struct {
        CategoryName string `form:"categoryName"`
        Description  string `form:"description"`
    }

    if err := c.ShouldBind(&input); err != nil {
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
        c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
            "error":        "Category name is required",
            "categoryName": input.CategoryName,
            "description":  input.Description,
            "category":     models.Category{
                Model: gorm.Model{ID: uintFromString(id)},
            },
        })
        return
    }
    if len(input.CategoryName) > 255 {
        c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
            "error":        "Category name must not exceed 255 characters",
            "categoryName": input.CategoryName,
            "description":  input.Description,
            "category":     models.Category{
                Model: gorm.Model{ID: uintFromString(id)},
            },
        })
        return
    }
    if len(input.Description) > 255 {
        c.HTML(http.StatusBadRequest, "Admin_Category_Edit.html", gin.H{
            "error":        "Description must not exceed 255 characters",
            "categoryName": input.CategoryName,
            "description":  input.Description,
            "category":     models.Category{
                Model: gorm.Model{ID: uintFromString(id)},
            },
        })
        return
    }

    var category models.Category
    if err := config.DB.First(&category, id).Error; err != nil {
        c.HTML(http.StatusNotFound, "Admin_Category_Edit.html", gin.H{
            "error": "Category not found",
        })
        return
    }

    var existingCategory models.Category
    if err := config.DB.Where("category_name = ? AND id != ?", input.CategoryName, id).First(&existingCategory).Error; err == nil {
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
        c.HTML(http.StatusInternalServerError, "Admin_Category_Edit.html", gin.H{
            "error":        "Failed to edit category: " + err.Error(),
            "categoryName": input.CategoryName,
            "description":  input.Description,
            "category":     category,
        })
        return
    }

    c.Redirect(http.StatusFound, "/admin/categories?success=Category updated successfully")
}

func uintFromString(id string) uint {
    val, _ := strconv.ParseUint(id, 10, 32)
    return uint(val)
}

func ToggleCategoryStatus(c *gin.Context) {
    id := c.Param("id")

    var category models.Category
    if err := config.DB.First(&category, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Category not found",
        })
        return
    }
    
    newStatus := !category.List
    tx := config.DB.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    if !newStatus {
        if err := tx.Model(&models.Product{}).
            Where("category_id = ?", id).
            Update("is_active", false).Error; err != nil {
            tx.Rollback()
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
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Failed to unlist product variants: " + err.Error(),
            })
            return
        }

        if err := tx.Model(&models.CategoryOffer{}).
            Where("category_id = ?", id).
            Update("offer_status", "Inactive").Error; err != nil {
            tx.Rollback()
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
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to update category status: " + err.Error(),
        })
        return
    }

    if err := tx.Commit().Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to commit transaction: " + err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Category status updated successfully",
        "status":  newStatus,
    })
}

func CategoryDetailsPage(c *gin.Context) {
    id := c.Param("id")

    var category models.Category
    if err := config.DB.Preload("CategoryOffers").First(&category, id).Error; err != nil {
        c.HTML(http.StatusNotFound, "Admin_Category_Management.html", gin.H{
            "error": "Category not found",
        })
        return
    }

    c.HTML(http.StatusOK, "Admin_Category_Details.html", gin.H{
        "category": category,
    })
}

func AddCategoryOffer(c *gin.Context) {
	var input struct {
		CategoryID         string  `json:"categoryId"`
		OfferName          string  `json:"offerName"`
		OfferDescription   string  `json:"offerDescription"`
		DiscountPercentage float64 `json:"discountPercentage"`
		StartDate          string  `json:"startDate"`
		EndDate            string  `json:"endDate"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid input: " + err.Error()})
		return
	}

	categoryID, err := strconv.ParseUint(input.CategoryID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid category ID: " + err.Error()})
		return
	}

	var category models.Category
	if err := config.DB.First(&category, uint(categoryID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Category not found"})
		return
	}

	if input.OfferName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer name is required"})
		return
	}

	if input.OfferDescription == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer description is required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid start date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid end date format. Use YYYY-MM-DD"})
		return
	}

	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "End date cannot be before start date"})
		return
	}

	if input.DiscountPercentage <= 0 || input.DiscountPercentage > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Discount percentage must be between 0 and 100"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
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
		log.Printf("Failed to create offer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to add offer: " + err.Error()})
		return
	}

	if err := updateCategorySellingPrices(uint(categoryID), tx); err != nil {
		tx.Rollback()
		log.Printf("Failed to update selling prices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update selling prices: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Offer added successfully"})
}

func EditCategoryOffer(c *gin.Context) {
	offerID := c.Param("id")
	log.Printf("Editing offer ID: %s", offerID)

	var input struct {
		CategoryID         string  `json:"categoryId"`
		OfferName          string  `json:"offerName"`
		OfferDescription   string  `json:"offerDescription"`
		DiscountPercentage float64 `json:"discountPercentage"`
		StartDate          string  `json:"startDate"`
		EndDate            string  `json:"endDate"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid input: " + err.Error()})
		return
	}
	log.Printf("Input data: %+v", input)

	categoryID, err := strconv.ParseUint(input.CategoryID, 10, 32)
	if err != nil {
		log.Printf("Invalid category ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid category ID: " + err.Error()})
		return
	}

	var offer models.CategoryOffer
	if err := config.DB.First(&offer, offerID).Error; err != nil {
		log.Printf("Offer not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Offer not found"})
		return
	}

	if offer.CategoryID != uint(categoryID) {
		log.Printf("Category mismatch: offer.CategoryID=%d, input.CategoryID=%d", offer.CategoryID, categoryID)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Category mismatch"})
		return
	}

	if input.OfferName == "" {
		log.Printf("Offer name is empty")
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer name is required"})
		return
	}

	if input.OfferDescription == "" {
		log.Printf("Offer description is empty")
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Offer description is required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		log.Printf("Invalid start date: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid start date format. Use YYYY-MM-DD"})
		return
	}

	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		log.Printf("Invalid end date: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid end date format. Use YYYY-MM-DD"})
		return
	}

	if endDate.Before(startDate) {
		log.Printf("End date before start date: start=%s, end=%s", input.StartDate, input.EndDate)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "End date cannot be before start date"})
		return
	}

	if input.DiscountPercentage <= 0 || input.DiscountPercentage > 100 {
		log.Printf("Invalid discount percentage: %f", input.DiscountPercentage)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Discount percentage must be between 0 and 100"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
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
		log.Printf("Failed to update offer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update offer: " + err.Error()})
		return
	}

	if err := updateCategorySellingPrices(uint(categoryID), tx); err != nil {
		tx.Rollback()
		log.Printf("Failed to update selling prices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update selling prices: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction: " + err.Error()})
		return
	}

	log.Printf("Offer %s updated successfully", offerID)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Offer updated successfully"})
}

func ToggleCategoryOfferStatus(c *gin.Context) {
	offerID := c.Param("id")

	var offer models.CategoryOffer
	if err := config.DB.First(&offer, offerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Offer not found"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction: " + tx.Error.Error()})
		return
	}

	if offer.OfferStatus == "Active" {
		offer.OfferStatus = "Inactive"
	} else {
		offer.OfferStatus = "Active"
	}

	if err := tx.Save(&offer).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update offer status: " + err.Error()})
		return
	}

	if err := updateCategorySellingPrices(offer.CategoryID, tx); err != nil {
		tx.Rollback()
		log.Printf("Failed to update selling prices: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update selling prices: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Offer status updated successfully"})
}

func updateCategorySellingPrices(categoryID uint, db *gorm.DB) error {
	var products []models.Product
	if err := db.Preload("Variants").Where("category_id = ?", categoryID).Find(&products).Error; err != nil {
		return fmt.Errorf("failed to fetch products: %v", err)
	}

	for _, product := range products {
		for i, variant := range product.Variants {
			sellingPrice, _ := CalculateSellingPrice(variant, db)
			product.Variants[i].SellingPrice = sellingPrice
			if db.Error != nil {
				return fmt.Errorf("transaction aborted before saving variant %d: %v", variant.ID, db.Error)
			}
			if err := db.Save(&product.Variants[i]).Error; err != nil {
				return fmt.Errorf("failed to update variant %d: %v", variant.ID, err)
			}
		}
	}
	return nil
}