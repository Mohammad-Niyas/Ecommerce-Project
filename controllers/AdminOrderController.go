package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"fmt"
	"html"
	"log"
	"net/http"
	"sort"
	"time"
    "math/rand"

	"github.com/gin-gonic/gin"
)

func AdminOrders(c *gin.Context) {
    var orders []models.Order
    if err := config.DB.
        Preload("OrderItem").
        Preload("ShippingAddress").
        Preload("User").
        Find(&orders).Error; err != nil {
        log.Printf("Failed to fetch orders: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch orders"})
        return
    }

    log.Printf("Fetched %d orders from the database", len(orders))

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
        log.Printf("Processing OrderUID=%s, UserID=%d, OrderDate=%s", order.OrderUID, order.UserID, order.OrderDate.Format("02 Jan 2006"))

        customerName := ""
        if order.User.ID != 0 {
            customerName = fmt.Sprintf("%s %s", order.User.FirstName, order.User.LastName)
        } else if order.ShippingAddress.ID != 0 {
            customerName = fmt.Sprintf("%s %s", order.ShippingAddress.FirstName, order.ShippingAddress.LastName)
            log.Printf("Order %s: Using ShippingAddress for customer name (%s) as User is missing", order.OrderUID, customerName)
        } else {
            customerName = "Unknown Customer"
            log.Printf("Order %s: No User or ShippingAddress data available", order.OrderUID)
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
            log.Printf("Order %s has no items, defaulting status to 'Pending'", order.OrderUID)
        }

        displayOrder := OrderDisplay{
            OrderUID:     order.OrderUID,
            CustomerName: customerName,
            OrderDate:    order.OrderDate.Format("02 Jan 2006"),
            OrderStatus:  orderStatus,
        }
        orderDisplays = append(orderDisplays, displayOrder)
        log.Printf("Added order: OrderUID=%s, Customer=%s, Status=%s, Date=%s", displayOrder.OrderUID, displayOrder.CustomerName, displayOrder.OrderStatus, displayOrder.OrderDate)
    }

    log.Printf("Prepared %d orders for display (sorted by date, recent first)", len(orderDisplays))
    c.HTML(http.StatusOK, "Admin_Order_Management.html", gin.H{
        "OrderItems": orderDisplays,
    })
}

func AdminOrderDetails(c *gin.Context) {
    orderID := html.EscapeString(c.Query("order_id"))
    if orderID == "" {
        log.Println("Order ID not provided in AdminOrderDetails")
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
        log.Printf("Order %s not found: %v", orderID, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
        return
    }

    var returnRequests []models.ReturnRequest
    if err := config.DB.
        Preload("OrderItem").
        Preload("OrderItem.Product").
        Where("order_item_id IN ?", getOrderItemIDs(order.OrderItem)).
        Find(&returnRequests).Error; err != nil {
        log.Printf("Failed to load return requests for order %s: %v", order.OrderUID, err)
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
        log.Printf("Order %s has no items, defaulting ExpectedDelivery to %s", order.OrderUID, expectedDelivery.Format(time.RFC3339))
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
    orderID := c.PostForm("order_id")
    itemID := c.PostForm("item_id")
    newStatus := c.PostForm("status")

    if orderID == "" || itemID == "" || newStatus == "" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Missing order ID, item ID, or status"})
        return
    }

    var order models.Order
    if err := config.DB.Where("order_uid = ?", orderID).First(&order).Error; err != nil {
        log.Printf("Order %s not found: %v", orderID, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order not found"})
        return
    }

    var orderItem models.OrderItem
    if err := config.DB.
        Where("order_id = ? AND id = ?", order.ID, itemID).
        First(&orderItem).Error; err != nil {
        log.Printf("Order item %s not found for order %s: %v", itemID, orderID, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order item not found"})
        return
    }

    if orderItem.OrderStatus == "Cancelled" || orderItem.OrderStatus == "Delivered" {
        c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Cannot update status: Order is already Cancelled or Delivered"})
        return
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
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
        log.Printf("Failed to update order item %s: %v", itemID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update order item"})
        return
    }

    if newStatus=="Delivered"{
        var PaymentDetails models.PaymentDetails
        if err:=tx.Where("order_id = ? ",order.ID).First(&PaymentDetails).Error;err!=nil{
            tx.Rollback()
            log.Printf("Payment details not found for order %s: %v", orderID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to fetch payment details"})
            return
        }

        if PaymentDetails.PaymentMethod == "cod"  && PaymentDetails.PaymentStatus != "Completed"{
            PaymentDetails.PaymentStatus = "Completed"
            PaymentDetails.PaymentDate = time.Now()
            if err := tx.Save(&PaymentDetails).Error; err != nil {
                tx.Rollback()
                log.Printf("Failed to update payment status for order %s: %v", orderID, err)
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update payment status"})
                return
            }
        }
    }

    if err := tx.Commit().Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Order item status updated"})
}

func AdminReturnAction(c *gin.Context) {
    orderID := c.PostForm("order_id")
    requestID := c.PostForm("request_id")
    action := c.PostForm("action")

    if orderID == "" || requestID == "" || action == "" {
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
        log.Printf("Return request %s not found: %v", requestID, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Return request not found"})
        return
    }
    if returnRequest.Status != "pending" {
        c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Return request already processed"})
        return
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        log.Printf("Failed to start transaction in AdminReturnAction: %v", tx.Error)
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
                log.Printf("Failed to create wallet for user %d: %v", returnRequest.UserID, err)
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create wallet"})
                return
            }
        }

        refundAmount := returnRequest.OrderItem.Total
        wallet.Balance += refundAmount
        if err := tx.Save(&wallet).Error; err != nil {
            tx.Rollback()
            log.Printf("Failed to update wallet balance for user %d: %v", returnRequest.UserID, err)
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
            log.Printf("Failed to create wallet transaction for user %d: %v", returnRequest.UserID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to record wallet transaction"})
            return
        }

        var productVariant models.ProductVariant
        if err := tx.Where("id = ?", returnRequest.ProductVariantID).First(&productVariant).Error; err != nil {
            tx.Rollback()
            log.Printf("Product variant %d not found for return request %s: %v", returnRequest.ProductVariantID, requestID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Product variant not found"})
            return
        }

        productVariant.StockCount += returnRequest.OrderItem.Quantity
        if err := tx.Save(&productVariant).Error; err != nil {
            tx.Rollback()
            log.Printf("Failed to update stock for product variant %d: %v", productVariant.ID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to restock product"})
            return
        }

    case "cancel":
        returnRequest.Status = "cancelled"

    default:
        tx.Rollback()
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid action"})
        return
    }

    if err := tx.Save(&returnRequest).Error; err != nil {
        tx.Rollback()
        log.Printf("Failed to update return request %s: %v", requestID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update return request"})
        return
    }

    if err := tx.Save(&returnRequest.OrderItem).Error; err != nil {
        tx.Rollback()
        log.Printf("Failed to update order item %d: %v", returnRequest.OrderItemID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update order item"})
        return
    }

    if err := tx.Commit().Error; err != nil {
        log.Printf("Failed to commit transaction in AdminReturnAction: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to process transaction"})
        return
    }

    log.Printf("Return request %s %s successfully for order %s", requestID, action, orderID)
    c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Return request %s successfully", action)})
}

func init() {
    rand.Seed(time.Now().UnixNano())
}