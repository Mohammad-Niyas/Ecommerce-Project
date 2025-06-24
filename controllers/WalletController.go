package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type TransactionSummary struct {
	TransactionID    uint      `json:"transaction_id"`
	TransactionUID   string    `json:"transaction_uid"`
	TransactionDate  time.Time `json:"transaction_date"`
	UserName         string    `json:"user_name"`
	TransactionType  string    `json:"transaction_type"`
	Amount           float64   `json:"amount"`
}

type TransactionDetails struct {
	TransactionID    uint           `json:"transaction_id"`
	TransactionUID   string         `json:"transaction_uid"`
	TransactionDate  time.Time      `json:"transaction_date"`
	UserDetails      models.User    `json:"user_details"`
	TransactionType  string         `json:"transaction_type"`
	Amount           float64        `json:"amount"`
	Source           string         `json:"source"`
	OrderUID         *string        `json:"order_uid,omitempty"`
	OrderDetailLink  *string        `json:"order_detail_link,omitempty"`
}

func WalletManagement(c *gin.Context) {
    logger.Log.Info("Loading wallet management page")
    
    var transactions []models.WalletTransaction
    query := config.DB.
        Preload("Wallet").
        Preload("Wallet.User").
        Order("transaction_date DESC")

    if err := query.Find(&transactions).Error; err != nil {
        logger.Log.Error("Failed to fetch wallet transactions",
            zap.Error(err))
        c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to load transactions"})
        return
    }

    logger.Log.Info("Fetched wallet transactions",
        zap.Int("count", len(transactions)))

    var transactionSummaries []TransactionSummary
    var skippedTransactions int
    for _, tx := range transactions {
        if tx.Wallet.UserID == 0 || tx.Wallet.User.FirstName == "" {
            logger.Log.Warn("Skipping transaction due to invalid user data",
                zap.String("transactionUID", tx.TransactionUID),
                zap.Uint("walletID", tx.WalletID))
            skippedTransactions++
            continue
        }

        userName := tx.Wallet.User.FirstName
        if tx.Wallet.User.LastName != "" {
            userName += " " + tx.Wallet.User.LastName
        }

        transactionSummaries = append(transactionSummaries, TransactionSummary{
            TransactionID:    tx.ID,
            TransactionUID:   tx.TransactionUID,
            TransactionDate:  tx.TransactionDate,
            UserName:         userName,
            TransactionType:  tx.TransactionType,
            Amount:           tx.TransactionAmount,
        })
    }

    if skippedTransactions > 0 {
        logger.Log.Warn("Skipped transactions due to invalid user data",
            zap.Int("count", skippedTransactions))
    }

    logger.Log.Info("Successfully prepared transaction summaries",
        zap.Int("validTransactions", len(transactionSummaries)))
    
    c.HTML(http.StatusOK, "Admin_Wallet_Management.html", gin.H{
        "Transactions": transactionSummaries,
    })
}

func TransactionDetail(c *gin.Context) {
    transactionUID := c.Param("id")
    logger.Log.Info("Loading transaction details",
        zap.String("transactionUID", transactionUID))

    var transaction models.WalletTransaction
    if err := config.DB.
        Preload("Wallet").
        Preload("Wallet.User").
        First(&transaction, "transaction_uid = ?", transactionUID).Error; err != nil {
        logger.Log.Error("Failed to fetch transaction",
            zap.String("transactionUID", transactionUID),
            zap.Error(err))
        c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Transaction not found"})
        return
    }

    logger.Log.Info("Fetched transaction details",
        zap.String("transactionUID", transactionUID),
        zap.String("type", transaction.TransactionType),
        zap.Float64("amount", transaction.TransactionAmount))

    var source string
    var orderUID *string
    var orderDetailLink *string

    if transaction.OrderID != nil {
        var order models.Order
        if err := config.DB.Preload("OrderItem").First(&order, *transaction.OrderID).Error; err == nil {
            source = "Order"
            orderUIDStr := order.OrderUID
            orderUID = &orderUIDStr
            orderDetailLink = &orderUIDStr
            
            logger.Log.Debug("Found associated order",
                zap.String("transactionUID", transactionUID),
                zap.String("orderUID", orderUIDStr))
        } else {
            logger.Log.Warn("Failed to fetch order for transaction",
                zap.String("transactionUID", transactionUID),
                zap.Uint("orderID", *transaction.OrderID),
                zap.Error(err))
            source = "Order (Details unavailable)"
        }
    } else if (transaction.TransactionType == "Refund" || transaction.TransactionType == "Cancellation") && transaction.OrderItemID != nil {
        source = "Return/Cancellation"
        var order models.Order
        if err := config.DB.Preload("Order").First(&order, *transaction.OrderItemID).Error; err == nil {
            if order.OrderUID != "" {
                orderUIDStr := order.OrderUID
                orderUID = &orderUIDStr
                orderDetailLink = &orderUIDStr
                
                logger.Log.Debug("Found associated order item",
                    zap.String("transactionUID", transactionUID),
                    zap.Uint("orderItemID", *transaction.OrderItemID),
                    zap.String("orderUID", orderUIDStr))
            } else {
                logger.Log.Warn("Order UID empty for order item",
                    zap.String("transactionUID", transactionUID),
                    zap.Uint("orderItemID", *transaction.OrderItemID))
            }
        } else {
            logger.Log.Warn("Failed to fetch order item for transaction",
                zap.String("transactionUID", transactionUID),
                zap.Uint("orderItemID", *transaction.OrderItemID),
                zap.Error(err))
        }
    } else {
        source = "Manual Adjustment"
        logger.Log.Debug("Transaction has no associated order",
            zap.String("transactionUID", transactionUID))
    }

    if orderUID != nil {
        fullLink := "/admin/order/" + *orderUID 
        orderDetailLink = &fullLink
    }

    if transaction.Wallet.UserID == 0 || transaction.Wallet.User.FirstName == "" {
        logger.Log.Error("Invalid user data for transaction",
            zap.String("transactionUID", transactionUID),
            zap.Uint("walletID", transaction.WalletID))
        c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Invalid user data"})
        return
    }

    detail := TransactionDetails{
        TransactionID:    transaction.ID,
        TransactionUID:   transaction.TransactionUID,
        TransactionDate:  transaction.TransactionDate,
        UserDetails:      transaction.Wallet.User,
        TransactionType:  transaction.TransactionType,
        Amount:           transaction.TransactionAmount,
        Source:           source,
        OrderUID:         orderUID,
        OrderDetailLink:  orderDetailLink,
    }

    logger.Log.Info("Successfully prepared transaction details",
        zap.String("transactionUID", transactionUID),
        zap.String("source", source))
    
    c.HTML(http.StatusOK, "Admin_Wallet_Transactions_Deatils.html", gin.H{
        "Transaction": detail,
    })
}