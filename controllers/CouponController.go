package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func AdminCouponManagement(c *gin.Context) {
	logger.Log.Info("Requested to show coupon management page")
	var coupons []models.Coupon
	if err := config.DB.Find(&coupons).Error; err != nil {
		logger.Log.Error("Failed to fetch coupons", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch coupons"})
		return
	}

	data := gin.H{
		"Coupons": coupons,
		"Message": c.Query("message"),
		"Error":   c.Query("error"),
	}

	logger.Log.Info("Rendering Admin Coupon Management page", zap.Int("couponCount", len(coupons)))
	c.HTML(http.StatusOK, "Admin_Coupon_Management.html", data)
}

func AdminCreateCoupon(c *gin.Context) {
	logger.Log.Info("Requested to create new coupon")
	couponCode := strings.TrimSpace(c.PostForm("coupon_code"))
	discountStr := c.PostForm("discount")
	minAmountStr := c.PostForm("min_amount")
	maxAmountStr := c.PostForm("max_amount")
	usageLimitStr := c.PostForm("usage_limit")
	expirationDateStr := c.PostForm("expiration_date")

	if couponCode == "" {
		logger.Log.Error("Coupon code is required")
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code is required")
		return
	}
	if len(couponCode) < 4 || len(couponCode) > 50 {
		logger.Log.Error("Coupon code length invalid",
			zap.String("couponCode", couponCode),
			zap.Int("length", len(couponCode)))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code must be between 4 and 50 characters")
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(couponCode) {
		logger.Log.Error("Coupon code must be alphanumeric",
			zap.String("couponCode", couponCode))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon code must be alphanumeric (no spaces or special characters)")
		return
	}

	discount, err := strconv.ParseFloat(discountStr, 64)
	if err != nil || discount <= 0 || discount > 100 {
		logger.Log.Error("Invalid discount percentage",
			zap.String("discountStr", discountStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Discount must be a valid percentage between 0.01 and 100")
		return
	}

	minAmount, err := strconv.ParseFloat(minAmountStr, 64)
	if err != nil || minAmount < 0 {
		logger.Log.Error("Invalid minimum amount",
			zap.String("minAmountStr", minAmountStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Minimum amount must be a non-negative number")
		return
	}
	if minAmount > 1000000 {
		logger.Log.Error("Minimum amount exceeds limit",
			zap.Float64("minAmount", minAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Minimum amount must not exceed 1,000,000")
		return
	}

	maxAmount, err := strconv.ParseFloat(maxAmountStr, 64)
	if err != nil && maxAmountStr != "" {
		logger.Log.Error("Invalid maximum amount",
			zap.String("maxAmountStr", maxAmountStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must be a valid number or left empty")
		return
	}
	if maxAmount < 0 {
		logger.Log.Error("Maximum amount is negative",
			zap.Float64("maxAmount", maxAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must be non-negative")
		return
	}
	if maxAmount > 1000000 {
		logger.Log.Error("Maximum amount exceeds limit",
			zap.Float64("maxAmount", maxAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must not exceed 1,000,000")
		return
	}
	if maxAmount > 0 && maxAmount < minAmount {
		logger.Log.Error("Maximum amount less than minimum amount",
			zap.Float64("maxAmount", maxAmount),
			zap.Float64("minAmount", minAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Maximum amount must be greater than or equal to minimum amount")
		return
	}

	usageLimit, err := strconv.Atoi(usageLimitStr)
	if err != nil || usageLimit <= 0 {
		logger.Log.Error("Invalid usage limit",
			zap.String("usageLimitStr", usageLimitStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Usage limit must be a positive integer")
		return
	}
	if usageLimit > 10000 {
		logger.Log.Error("Usage limit exceeds limit",
			zap.Int("usageLimit", usageLimit))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Usage limit must not exceed 10,000")
		return
	}

	expirationDate, err := time.Parse("2006-01-02", expirationDateStr)
	if err != nil {
		logger.Log.Error("Invalid expiration date format",
			zap.String("expirationDateStr", expirationDateStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Invalid expiration date format (use YYYY-MM-DD)")
		return
	}

	if expirationDate.Before(time.Now().AddDate(0, 0, 1)) {
		logger.Log.Error("Expiration date not in future",
			zap.String("expirationDate", expirationDateStr))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Expiration date must be in the future (at least tomorrow)")
		return
	}

	var existingCoupon models.Coupon
	if err := config.DB.Where("coupon_code = ?", couponCode).First(&existingCoupon).Error; err == nil {
		logger.Log.Error("Coupon code already exists",
			zap.String("couponCode", couponCode))
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
		logger.Log.Error("Failed to create coupon",
			zap.String("couponCode", couponCode),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Failed to create coupon")
		return
	}

	logger.Log.Info("Coupon created successfully",
		zap.String("couponCode", couponCode),
		zap.Uint("couponID", coupon.ID))
	c.Redirect(http.StatusSeeOther, "/admin/coupon/management?message=Coupon created successfully")
}

func AdminEditCoupon(c *gin.Context) {
	logger.Log.Info("Requested to show edit coupon page")
	couponID := c.Param("id")

	var coupon models.Coupon
	if err := config.DB.First(&coupon, couponID).Error; err != nil {
		logger.Log.Error("Coupon not found",
			zap.String("couponID", couponID),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/management?error=Coupon not found")
		return
	}

	data := gin.H{
		"Coupon":  coupon,
		"Message": c.Query("message"),
		"Error":   c.Query("error"),
	}

	logger.Log.Info("Rendering edit page for coupon",
		zap.String("couponID", couponID),
		zap.String("couponCode", coupon.CouponCode))
	c.HTML(http.StatusOK, "Admin_Coupon_Edit.html", data)
}

func AdminUpdateCoupon(c *gin.Context) {
	logger.Log.Info("Requested to update coupon")
	couponID := c.Param("id")

	var coupon models.Coupon
	if err := config.DB.First(&coupon, couponID).Error; err != nil {
		logger.Log.Error("Coupon not found",
			zap.String("couponID", couponID),
			zap.Error(err))
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
		logger.Log.Error("Coupon code is required",
			zap.String("couponID", couponID))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code is required")
		return
	}
	if len(couponCode) < 4 || len(couponCode) > 50 {
		logger.Log.Error("Coupon code length invalid",
			zap.String("couponID", couponID),
			zap.String("couponCode", couponCode),
			zap.Int("length", len(couponCode)))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code must be between 4 and 50 characters")
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(couponCode) {
		logger.Log.Error("Coupon code must be alphanumeric",
			zap.String("couponID", couponID),
			zap.String("couponCode", couponCode))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code must be alphanumeric (no spaces or special characters)")
		return
	}

	if couponCode != coupon.CouponCode {
		var existingCoupon models.Coupon
		if err := config.DB.Where("coupon_code = ?", couponCode).First(&existingCoupon).Error; err == nil {
			logger.Log.Error("Coupon code already exists",
				zap.String("couponID", couponID),
				zap.String("couponCode", couponCode))
			c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Coupon code already exists")
			return
		}
	}

	discount, err := strconv.ParseFloat(discountStr, 64)
	if err != nil || discount <= 0 || discount > 100 {
		logger.Log.Error("Invalid discount percentage",
			zap.String("couponID", couponID),
			zap.String("discountStr", discountStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Discount must be a valid percentage between 0.01 and 100")
		return
	}

	minAmount, err := strconv.ParseFloat(minAmountStr, 64)
	if err != nil || minAmount < 0 {
		logger.Log.Error("Invalid minimum amount",
			zap.String("couponID", couponID),
			zap.String("minAmountStr", minAmountStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Minimum amount must be a non-negative number")
		return
	}
	if minAmount > 1000000 {
		logger.Log.Error("Minimum amount exceeds limit",
			zap.String("couponID", couponID),
			zap.Float64("minAmount", minAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Minimum amount must not exceed 1,000,000")
		return
	}

	maxAmount, err := strconv.ParseFloat(maxAmountStr, 64)
	if err != nil && maxAmountStr != "" {
		logger.Log.Error("Invalid maximum amount",
			zap.String("couponID", couponID),
			zap.String("maxAmountStr", maxAmountStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must be a valid number or left empty")
		return
	}
	if maxAmount < 0 {
		logger.Log.Error("Maximum amount is negative",
			zap.String("couponID", couponID),
			zap.Float64("maxAmount", maxAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must be non-negative")
		return
	}
	if maxAmount > 1000000 {
		logger.Log.Error("Maximum amount exceeds limit",
			zap.String("couponID", couponID),
			zap.Float64("maxAmount", maxAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must not exceed 1,000,000")
		return
	}
	if maxAmount > 0 && maxAmount < minAmount {
		logger.Log.Error("Maximum amount less than minimum amount",
			zap.String("couponID", couponID),
			zap.Float64("maxAmount", maxAmount),
			zap.Float64("minAmount", minAmount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Maximum amount must be greater than or equal to minimum amount")
		return
	}

	usageLimit, err := strconv.Atoi(usageLimitStr)
	if err != nil || usageLimit <= 0 {
		logger.Log.Error("Invalid usage limit",
			zap.String("couponID", couponID),
			zap.String("usageLimitStr", usageLimitStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Usage limit must be a positive integer")
		return
	}
	if usageLimit > 10000 {
		logger.Log.Error("Usage limit exceeds limit",
			zap.String("couponID", couponID),
			zap.Int("usageLimit", usageLimit))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Usage limit must not exceed 10,000")
		return
	}
	if usageLimit < coupon.UsedCount {
		logger.Log.Error("Usage limit less than used count",
			zap.String("couponID", couponID),
			zap.Int("usageLimit", usageLimit),
			zap.Int("usedCount", coupon.UsedCount))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Usage limit cannot be less than current used count ("+strconv.Itoa(coupon.UsedCount)+")")
		return
	}

	expirationDate, err := time.Parse("2006-01-02", expirationDateStr)
	if err != nil {
		logger.Log.Error("Invalid expiration date format",
			zap.String("couponID", couponID),
			zap.String("expirationDateStr", expirationDateStr),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Invalid expiration date format (use YYYY-MM-DD)")
		return
	}
	if expirationDate.Before(time.Now().AddDate(0, 0, 1)) {
		logger.Log.Error("Expiration date not in future",
			zap.String("couponID", couponID),
			zap.String("expirationDate", expirationDateStr))
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
		logger.Log.Error("Failed to update coupon",
			zap.String("couponID", couponID),
			zap.String("couponCode", couponCode),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/admin/coupon/edit/"+couponID+"?error=Failed to update coupon")
		return
	}

	logger.Log.Info("Coupon updated successfully",
		zap.String("couponID", couponID),
		zap.String("couponCode", couponCode))
	c.Redirect(http.StatusSeeOther, "/admin/coupon/management?message=Coupon updated successfully")
}

func AdminToggleCouponStatus(c *gin.Context) {
	logger.Log.Info("Requested to toggle coupon status")
	couponID := c.Param("id")

	var coupon models.Coupon
	if err := config.DB.First(&coupon, couponID).Error; err != nil {
		logger.Log.Error("Coupon not found",
			zap.String("couponID", couponID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Coupon not found"})
		return
	}

	coupon.IsActive = !coupon.IsActive
	if err := config.DB.Save(&coupon).Error; err != nil {
		logger.Log.Error("Failed to update coupon status",
			zap.String("couponID", couponID),
			zap.Bool("newStatus", coupon.IsActive),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update coupon status"})
		return
	}

	status := "deactivated"
	if coupon.IsActive {
		status = "activated"
	}
	logger.Log.Info("Coupon status updated successfully",
		zap.String("couponID", couponID),
		zap.String("couponCode", coupon.CouponCode),
		zap.Bool("newStatus", coupon.IsActive))
	c.Redirect(http.StatusSeeOther, "/admin/coupon/management?message=Coupon "+status+" successfully")
}