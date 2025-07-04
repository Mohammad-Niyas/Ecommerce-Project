package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"fmt"
	"html"
	"math/rand"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func AdminOrders(c *gin.Context) {
	logger.Log.Info("Requested to show admin orders page")
	var orders []models.Order
	if err := config.DB.
		Preload("OrderItem").
		Preload("ShippingAddress").
		Preload("User").
		Find(&orders).Error; err != nil {
		logger.Log.Error("Failed to fetch orders", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch orders"})
		return
	}

	logger.Log.Info("Fetched orders from database", zap.Int("orderCount", len(orders)))

	sort.Slice(orders, func(i, j int) bool {
		return orders[i].OrderDate.After(orders[j].OrderDate)
	})

	type OrderDisplay struct {
		OrderUID     string
		CustomerName string
		OrderDate    string
		OrderStatus  string
	}

	var orderDisplays []OrderDisplay
	for _, order := range orders {
		logger.Log.Info("Processing order",
			zap.String("orderUID", order.OrderUID),
			zap.Uint("userID", order.UserID),
			zap.String("orderDate", order.OrderDate.Format("02 Jan 2006")))

		customerName := ""
		if order.User.ID != 0 {
			customerName = fmt.Sprintf("%s %s", order.User.FirstName, order.User.LastName)
		} else if order.ShippingAddress.ID != 0 {
			customerName = fmt.Sprintf("%s %s", order.ShippingAddress.FirstName, order.ShippingAddress.LastName)
			logger.Log.Info("Using shipping address for customer name",
				zap.String("orderUID", order.OrderUID),
				zap.String("customerName", customerName))
		} else {
			customerName = "Unknown Customer"
			logger.Log.Warn("No user or shipping address data available",
				zap.String("orderUID", order.OrderUID))
		}

		orderStatus := "Pending"
		if len(order.OrderItem) > 0 {
			orderStatus = order.OrderItem[0].OrderStatus
			for _, item := range order.OrderItem {
				switch item.OrderStatus {
				case "Cancelled":
					orderStatus = "Cancelled"
					break
				case "Delivered":
					orderStatus = "Delivered"
					break
				case "Out for Delivery":
					if orderStatus != "Delivered" && orderStatus != "Cancelled" {
						orderStatus = "Out for Delivery"
					}
				case "Shipped":
					if orderStatus != "Delivered" && orderStatus != "Cancelled" && orderStatus != "Out for Delivery" {
						orderStatus = "Shipped"
					}
				}
			}
		} else {
			logger.Log.Info("Order has no items, defaulting status to 'Pending'",
				zap.String("orderUID", order.OrderUID))
		}

		displayOrder := OrderDisplay{
			OrderUID:     order.OrderUID,
			CustomerName: customerName,
			OrderDate:    order.OrderDate.Format("02 Jan 2006"),
			OrderStatus:  orderStatus,
		}
		orderDisplays = append(orderDisplays, displayOrder)
		logger.Log.Info("Added order for display",
			zap.String("orderUID", displayOrder.OrderUID),
			zap.String("customerName", displayOrder.CustomerName),
			zap.String("status", displayOrder.OrderStatus),
			zap.String("date", displayOrder.OrderDate))
	}

	logger.Log.Info("Prepared orders for display",
		zap.Int("orderCount", len(orderDisplays)))
	c.HTML(http.StatusOK, "Admin_Order_Management.html", gin.H{
		"OrderItems": orderDisplays,
	})
}

func AdminOrderDetails(c *gin.Context) {
	logger.Log.Info("Requested order details")
	orderID := html.EscapeString(c.Query("order_id"))
	if orderID == "" {
		logger.Log.Error("Order ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Order ID not provided"})
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("ShippingAddress").
		Preload("OrderItem").
		Preload("OrderItem.Product").
		Preload("OrderItem.Product.Images").
		Preload("PaymentDetails").
		Preload("Coupon").
		Where("order_uid = ?", orderID).
		First(&order).Error; err != nil {
		logger.Log.Error("Order not found",
			zap.String("orderUID", orderID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
		return
	}

	var returnRequests []models.ReturnRequest
	if err := config.DB.
		Preload("OrderItem").
		Preload("OrderItem.Product").
		Where("order_item_id IN ?", getOrderItemIDs(order.OrderItem)).
		Find(&returnRequests).Error; err != nil {
		logger.Log.Error("Failed to load return requests",
			zap.String("orderUID", order.OrderUID),
			zap.Error(err))
	}

	var expectedDelivery time.Time
	if len(order.OrderItem) > 0 {
		expectedDelivery = order.OrderItem[0].ExpectedDeliveryDate
		for _, item := range order.OrderItem {
			if item.ExpectedDeliveryDate.After(expectedDelivery) {
				expectedDelivery = item.ExpectedDeliveryDate
			}
		}
	} else {
		expectedDelivery = order.OrderDate.AddDate(0, 0, 5)
		logger.Log.Info("Order has no items, defaulting expected delivery",
			zap.String("orderUID", order.OrderUID),
			zap.String("expectedDelivery", expectedDelivery.Format(time.RFC3339)))
	}

	overallStatus := "Processing"
	hasProcessing := false
	hasShipped := false
	hasOutForDelivery := false
	allDelivered := true
	allCancelled := true

	if len(order.OrderItem) > 0 {
		for _, item := range order.OrderItem {
			switch item.OrderStatus {
			case "Processing":
				hasProcessing = true
				allDelivered = false
				allCancelled = false
			case "Shipped":
				hasShipped = true
				allDelivered = false
				allCancelled = false
			case "Out for Delivery":
				hasOutForDelivery = true
				allDelivered = false
				allCancelled = false
			case "Delivered":
				allCancelled = false
			case "Cancelled":
				allDelivered = false
			default:
				allDelivered = false
				allCancelled = false
			}
		}

		if allCancelled {
			overallStatus = "Cancelled"
		} else if allDelivered {
			overallStatus = "Delivered"
		} else if hasOutForDelivery {
			overallStatus = "Out for Delivery"
		} else if hasShipped {
			overallStatus = "Shipped"
		} else if hasProcessing {
			overallStatus = "Processing"
		}
	} else {
		overallStatus = "Failed"
	}

	data := gin.H{
		"Order":            order,
		"OrderUID":         order.OrderUID,
		"ShippingAddress":  order.ShippingAddress,
		"SubTotal":         order.SubTotal,
		"TotalDiscount":    order.TotalDiscount,
		"CouponDiscount":   order.CouponDiscount,
		"ShippingCharge":   order.ShippingCharge,
		"Tax":              order.Tax,
		"TotalAmount":      order.TotalAmount,
		"Coupon":           order.Coupon,
		"ExpectedDelivery": expectedDelivery.Format("02 Jan 2006"),
		"OrderItems":       order.OrderItem,
		"PaymentDetails":   order.PaymentDetails,
		"OverallStatus":    overallStatus,
		"ReturnRequests":   returnRequests,
	}

	logger.Log.Info("Order details loaded successfully",
		zap.String("orderUID", order.OrderUID))
	c.HTML(http.StatusOK, "Admin_Order_Details.html", data)
}

func GetOrderItemIDs(items []models.OrderItem) []uint {
	var ids []uint
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func AdminUpdateOrderStatus(c *gin.Context) {
	logger.Log.Info("Requested to update order status")
	orderID := c.PostForm("order_id")
	itemID := c.PostForm("item_id")
	newStatus := c.PostForm("status")

	if orderID == "" || itemID == "" || newStatus == "" {
		logger.Log.Error("Missing required fields",
			zap.String("orderID", orderID),
			zap.String("itemID", itemID),
			zap.String("status", newStatus))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Missing order ID, item ID, or status"})
		return
	}

	var order models.Order
	if err := config.DB.Where("order_uid = ?", orderID).First(&order).Error; err != nil {
		logger.Log.Error("Order not found",
			zap.String("orderUID", orderID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order not found"})
		return
	}

	var orderItem models.OrderItem
	if err := config.DB.
		Where("order_id = ? AND id = ?", order.ID, itemID).
		First(&orderItem).Error; err != nil {
		logger.Log.Error("Order item not found",
			zap.String("itemID", itemID),
			zap.String("orderUID", orderID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order item not found"})
		return
	}

	if orderItem.OrderStatus == "Cancelled" || orderItem.OrderStatus == "Delivered" {
		logger.Log.Error("Cannot update status: order item already finalized",
			zap.String("itemID", itemID),
			zap.String("orderUID", orderID),
			zap.String("currentStatus", orderItem.OrderStatus))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Cannot update status: Order is already Cancelled or Delivered"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("orderUID", orderID),
			zap.String("itemID", itemID),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction"})
		return
	}

	orderItem.OrderStatus = newStatus
	switch newStatus {
	case "Out for Delivery":
		orderItem.OutForDeliveryDate = time.Now()
	case "Shipped":
		orderItem.ShippedDate = time.Now()
	case "Delivered":
		orderItem.DeliveryDate = time.Now()
	case "Cancelled":
		orderItem.CancelDate = time.Now()
	}
	if err := tx.Save(&orderItem).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update order item",
			zap.String("itemID", itemID),
			zap.String("orderUID", orderID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update order item"})
		return
	}

	if newStatus == "Delivered" {
		var PaymentDetails models.PaymentDetails
		if err := tx.Where("order_id = ?", order.ID).First(&PaymentDetails).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Payment details not found",
				zap.String("orderUID", orderID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to fetch payment details"})
			return
		}

		if PaymentDetails.PaymentMethod == "cod" && PaymentDetails.PaymentStatus != "Completed" {
			PaymentDetails.PaymentStatus = "Completed"
			PaymentDetails.PaymentDate = time.Now()
			if err := tx.Save(&PaymentDetails).Error; err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to update payment status",
					zap.String("orderUID", orderID),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update payment status"})
				return
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("orderUID", orderID),
			zap.String("itemID", itemID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction"})
		return
	}

	logger.Log.Info("Order item status updated successfully",
		zap.String("orderUID", orderID),
		zap.String("itemID", itemID),
		zap.String("newStatus", newStatus))
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Order item status updated"})
}

func AdminReturnAction(c *gin.Context) {
	logger.Log.Info("Requested to process return action")
	orderID := c.PostForm("order_id")
	requestID := c.PostForm("request_id")
	action := c.PostForm("action")

	if orderID == "" || requestID == "" || action == "" {
		logger.Log.Error("Missing required fields",
			zap.String("orderID", orderID),
			zap.String("requestID", requestID),
			zap.String("action", action))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Missing required fields"})
		return
	}

	var returnRequest models.ReturnRequest
	if err := config.DB.
		Preload("OrderItem").
		Preload("OrderItem.Product").
		Preload("ProductVariant").
		Where("id = ?", requestID).
		First(&returnRequest).Error; err != nil {
		logger.Log.Error("Return request not found",
			zap.String("requestID", requestID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Return request not found"})
		return
	}
	if returnRequest.Status != "pending" {
		logger.Log.Error("Return request already processed",
			zap.String("requestID", requestID),
			zap.String("currentStatus", returnRequest.Status))
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Return request already processed"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		logger.Log.Error("Failed to start transaction",
			zap.String("requestID", requestID),
			zap.String("orderID", orderID),
			zap.Error(tx.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to process request"})
		return
	}

	switch action {
	case "approve":
		returnRequest.Status = "approved"
		returnRequest.OrderItem.OrderStatus = "Refunded"
		returnRequest.OrderItem.ReturnDate = time.Now()

		var wallet models.Wallet
		if err := tx.Where("user_id = ?", returnRequest.UserID).First(&wallet).Error; err != nil {
			wallet = models.Wallet{
				UserID:  returnRequest.UserID,
				Balance: 0.0,
			}
			if err := tx.Create(&wallet).Error; err != nil {
				tx.Rollback()
				logger.Log.Error("Failed to create wallet",
					zap.Uint("userID", returnRequest.UserID),
					zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create wallet"})
				return
			}
		}

		refundAmount := returnRequest.OrderItem.Total
		wallet.Balance += refundAmount
		if err := tx.Save(&wallet).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to update wallet balance",
				zap.Uint("userID", returnRequest.UserID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update wallet"})
			return
		}

		walletTransaction := models.WalletTransaction{
			WalletID:          wallet.ID,
			TransactionUID:    "WT" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", rand.Intn(1000000)),
			TransactionAmount: refundAmount,
			TransactionType:   "credit",
			TransactionStatus: "Completed",
			TransactionDate:   time.Now(),
			Description:       fmt.Sprintf("Refund for Order Item %d (Order %s)", returnRequest.OrderItemID, orderID),
		}
		if err := tx.Create(&walletTransaction).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to create wallet transaction",
				zap.Uint("userID", returnRequest.UserID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to record wallet transaction"})
			return
		}

		var productVariant models.ProductVariant
		if err := tx.Where("id = ?", returnRequest.ProductVariantID).First(&productVariant).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Product variant not found",
				zap.Uint("variantID", returnRequest.ProductVariantID),
				zap.String("requestID", requestID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Product variant not found"})
			return
		}

		productVariant.StockCount += returnRequest.OrderItem.Quantity
		if err := tx.Save(&productVariant).Error; err != nil {
			tx.Rollback()
			logger.Log.Error("Failed to update stock for product variant",
				zap.Uint("variantID", productVariant.ID),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to restock product"})
			return
		}

	case "cancel":
		returnRequest.Status = "cancelled"

	default:
		tx.Rollback()
		logger.Log.Error("Invalid action",
			zap.String("action", action),
			zap.String("requestID", requestID))
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid action"})
		return
	}

	if err := tx.Save(&returnRequest).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update return request",
			zap.String("requestID", requestID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update return request"})
		return
	}

	if err := tx.Save(&returnRequest.OrderItem).Error; err != nil {
		tx.Rollback()
		logger.Log.Error("Failed to update order item",
			zap.Uint("itemID", returnRequest.OrderItemID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update order item"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		logger.Log.Error("Failed to commit transaction",
			zap.String("requestID", requestID),
			zap.String("orderID", orderID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to process transaction"})
		return
	}

	logger.Log.Info("Return request processed successfully",
		zap.String("requestID", requestID),
		zap.String("orderID", orderID),
		zap.String("action", action))
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Return request %s successfully", action)})
}

func init() {
	rand.Seed(time.Now().UnixNano())
}