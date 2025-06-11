package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func AdminCouponManagement(c *gin.Context) {
    var coupons []models.Coupon
    if err := config.DB.Find(&coupons).Error; err != nil {
        log.Printf("Error fetching coupons: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch coupons"})
        return
    }

    data := gin.H{
        "Coupons": coupons,
        "Message": c.Query("message"),
        "Error":   c.Query("error"), 
    }

    log.Println("Rendering Admin Coupon Management page")
    c.HTML(http.StatusOK, "Admin_Coupon_Management.html", data)
}

func AdminCreateCoupon(c *gin.Context) {
    couponCode := strings.TrimSpace(c.PostForm("coupon_code"))
    discountStr := c.PostForm("discount")
    minAmountStr := c.PostForm("min_amount")
    maxAmountStr := c.PostForm("max_amount")
    usageLimitStr := c.PostForm("usage_limit")
    expirationDateStr := c.PostForm("expiration_date")

    if couponCode == "" {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code is required")
        return
    }
    if len(couponCode) < 4 || len(couponCode) > 50 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code must be between 4 and 50 characters")
        return
    }
    if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(couponCode) {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code must be alphanumeric (no spaces or special characters)")
        return
    }

    discount, err := strconv.ParseFloat(discountStr, 64)
    if err != nil || discount <= 0 || discount > 100 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Discount must be a valid percentage between 0.01 and 100")
        return
    }

    minAmount, err := strconv.ParseFloat(minAmountStr, 64)
    if err != nil || minAmount < 0 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Minimum amount must be a non-negative number")
        return
    }
    if minAmount > 1000000 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Minimum amount must not exceed 1,000,000")
        return
    }

    maxAmount, err := strconv.ParseFloat(maxAmountStr, 64)
    if err != nil && maxAmountStr != "" {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must be a valid number or left empty")
        return
    }
    if maxAmount < 0 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must be non-negative")
        return
    }
    if maxAmount > 1000000 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must not exceed 1,000,000")
        return
    }
    if maxAmount > 0 && maxAmount < minAmount {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must be greater than or equal to minimum amount")
        return
    }

    usageLimit, err := strconv.Atoi(usageLimitStr)
    if err != nil || usageLimit <= 0 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Usage limit must be a positive integer")
        return
    }
    if usageLimit > 10000 {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Usage limit must not exceed 10,000")
        return
    }

    expirationDate, err := time.Parse("2006-01-02", expirationDateStr)
    if err != nil {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Invalid expiration date format (use YYYY-MM-DD)")
        return
    }
	
    if expirationDate.Before(time.Now().AddDate(0, 0, 1)) {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Expiration date must be in the future (at least tomorrow)")
        return
    }

    var existingCoupon models.Coupon
    if err := config.DB.Where("coupon_code = ?", couponCode).First(&existingCoupon).Error; err == nil {
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code already exists")
        return
    }

    coupon := models.Coupon{
        CouponCode:     couponCode,
        Discount:       discount,
        MinAmount:      minAmount,
        MaxAmount:      maxAmount,
        UsageLimit:     usageLimit,
        IsActive:       true,
        ExpirationDate: expirationDate,
    }

    if err := config.DB.Create(&coupon).Error; err != nil {
        log.Printf("Error creating coupon: %v", err)
        c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Failed to create coupon")
        return
    }

    c.Redirect(http.StatusSeeOther, "/admin/coupon/management?message=Coupon created successfully")
}

func AdminEditCoupon(c *gin.Context) {
	couponID := c.Param("id")

	var coupon models.Coupon
	if err := config.DB.First(&coupon, couponID).Error; err != nil {
		log.Printf("Coupon %s not found: %v", couponID, err)
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon not found")
		return
	}

	data := gin.H{
		"Coupon": coupon,
		"Message": c.Query("message"),
		"Error":   c.Query("error"),
	}

	log.Printf("Rendering edit page for coupon %s", coupon.CouponCode)
	c.HTML(http.StatusOK, "Admin_Coupon_Edit.html", data)
}

func AdminUpdateCoupon(c *gin.Context) {
	couponID := c.Param("id")

	var coupon models.Coupon
	if err := config.DB.First(&coupon, couponID).Error; err != nil {
		log.Printf("Coupon %s not found: %v", couponID, err)
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon not found")
		return
	}

	couponCode := strings.TrimSpace(c.PostForm("coupon_code"))
	discountStr := c.PostForm("discount")
	minAmountStr := c.PostForm("min_amount")
	maxAmountStr := c.PostForm("max_amount")
	usageLimitStr := c.PostForm("usage_limit")
	expirationDateStr := c.PostForm("expiration_date")

	if couponCode == "" {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code is required")
		return
	}
	if len(couponCode) < 4 || len(couponCode) > 50 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code must be between 4 and 50 characters")
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(couponCode) {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code must be alphanumeric (no spaces or special characters)")
		return
	}

	if couponCode != coupon.CouponCode {
		var existingCoupon models.Coupon
		if err := config.DB.Where("coupon_code = ?", couponCode).First(&existingCoupon).Error; err == nil {
			c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code already exists")
			return
		}
	}

	discount, err := strconv.ParseFloat(discountStr, 64)
	if err != nil || discount <= 0 || discount > 100 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Discount must be a valid percentage between 0.01 and 100")
		return
	}

	minAmount, err := strconv.ParseFloat(minAmountStr, 64)
	if err != nil || minAmount < 0 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Minimum amount must be a non-negative number")
		return
	}
	if minAmount > 1000000 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Minimum amount must not exceed 1,000,000")
		return
	}

	maxAmount, err := strconv.ParseFloat(maxAmountStr, 64)
	if err != nil && maxAmountStr != "" {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must be a valid number or left empty")
		return
	}
	if maxAmount < 0 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must be non-negative")
		return
	}
	if maxAmount > 1000000 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must not exceed 1,000,000")
		return
	}
	if maxAmount > 0 && maxAmount < minAmount {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must be greater than or equal to minimum amount")
		return
	}

	usageLimit, err := strconv.Atoi(usageLimitStr)
	if err != nil || usageLimit <= 0 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Usage limit must be a positive integer")
		return
	}
	if usageLimit > 10000 {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Usage limit must not exceed 10,000")
		return
	}
	if usageLimit < coupon.UsedCount {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Usage limit cannot be less than current used count ("+strconv.Itoa(coupon.UsedCount)+")")
		return
	}

	expirationDate, err := time.Parse("2006-01-02", expirationDateStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Invalid expiration date format (use YYYY-MM-DD)")
		return
	}
	if expirationDate.Before(time.Now().AddDate(0, 0, 1)) {
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Expiration date must be in the future (at least tomorrow)")
		return
	}

	coupon.CouponCode = couponCode
	coupon.Discount = discount
	coupon.MinAmount = minAmount
	coupon.MaxAmount = maxAmount
	coupon.UsageLimit = usageLimit
	coupon.ExpirationDate = expirationDate

	if err := config.DB.Save(&coupon).Error; err != nil {
		log.Printf("Error updating coupon %s: %v", couponID, err)
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Failed to update coupon")
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/coupon/management?message=Coupon updated successfully")
}

func AdminToggleCouponStatus(c *gin.Context) {
    couponID := c.Param("id")

    var coupon models.Coupon
    if err := config.DB.First(&coupon, couponID).Error; err != nil {
        log.Printf("Coupon %s not found: %v", couponID, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Coupon not found"})
        return
    }

    coupon.IsActive = !coupon.IsActive
    if err := config.DB.Save(&coupon).Error; err != nil {
        log.Printf("Error updating coupon status: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update coupon status"})
        return
    }

    status := "deactivated"
    if coupon.IsActive {
        status = "activated"
    }
    c.Redirect(http.StatusSeeOther, "/admin/coupon/management?message=Coupon "+status+" successfully")
}