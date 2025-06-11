package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	var transactions []models.WalletTransaction
	query := config.DB.
		Preload("Wallet").
		Preload("Wallet.User").
		Order("transaction_date DESC")

	if err := query.Find(&transactions).Error; err != nil {
		log.Printf("Failed to fetch wallet transactions: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to load transactions"})
		return
	}

	var transactionSummaries []TransactionSummary
	for _, tx := range transactions {
		if tx.Wallet.UserID == 0 || tx.Wallet.User.FirstName == "" {
			log.Printf("Skipping transaction %s: Invalid or missing user data", tx.TransactionUID)
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

	c.HTML(http.StatusOK, "Admin_Wallet_Management.html", gin.H{
		"Transactions": transactionSummaries,
	})
}

func TransactionDetail(c *gin.Context) {
	transactionUID := c.Param("id")
	var transaction models.WalletTransaction
	if err := config.DB.
		Preload("Wallet").
		Preload("Wallet.User").
		First(&transaction, "transaction_uid = ?", transactionUID).Error; err != nil {
		log.Printf("Failed to fetch transaction %s: %v", transactionUID, err)
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Transaction not found"})
		return
	}

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
		} else {
			log.Printf("Failed to fetch order for transaction %s: %v", transactionUID, err)
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
			} else {
				log.Printf("Order not found for order item %d in transaction %s", *transaction.OrderItemID, transactionUID)
			}
		} else {
			log.Printf("Failed to fetch order item %d for transaction %s: %v", *transaction.OrderItemID, transactionUID, err)
		}
	} else {
		source = "Manual Adjustment"
	}

	if orderUID != nil {
		fullLink := "/admin/order/" + *orderUID 
		orderDetailLink = &fullLink
	}

	if transaction.Wallet.UserID == 0 || transaction.Wallet.User.FirstName == "" {
		log.Printf("Invalid user data for transaction %s", transactionUID)
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

	c.HTML(http.StatusOK, "Admin_Wallet_Transactions_Deatils.html", gin.H{
		"Transaction": detail,
	})
}