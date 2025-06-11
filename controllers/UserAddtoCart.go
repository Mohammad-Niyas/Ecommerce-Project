package controllers

import (
	"ecommerce/config"
	"ecommerce/models"
	"ecommerce/utils"
	"fmt"
	"html"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
	"github.com/razorpay/razorpay-go"
	"gorm.io/gorm"
)

func CalculateSellingPrice(variant models.ProductVariant, db *gorm.DB) (float64, float64) {
	var product models.Product
	if err := db.Preload("ProductOffers").Preload("Category.CategoryOffers").First(&product, variant.ProductID).Error; err != nil {
		log.Printf("Failed to fetch product for variant %d: %v", variant.ProductID, err)
		return variant.ActualPrice, 0.0 
	}

	currentTime := time.Now()
	
	var categoryOfferPercentage float64
	for _, offer := range product.Category.CategoryOffers {
		if offer.OfferStatus == "Active" && offer.StartDate.Before(currentTime) && offer.EndDate.After(currentTime) {
			categoryOfferPercentage = offer.CategoryOfferPercentage
			break
		}
	}

	var productOfferPercentage float64
	for _, offer := range product.ProductOffers {
		if offer.Status == "Active" && offer.StartDate.Before(currentTime) && offer.EndDate.After(currentTime) {
			productOfferPercentage = offer.OfferPercentage
			break
		}
	}

	highestOffer := math.Max(categoryOfferPercentage, productOfferPercentage)

	var sellingPrice float64
	if highestOffer > 0 {
		sellingPrice = variant.ActualPrice * (1 - highestOffer/100)
	} else {
		sellingPrice = variant.ActualPrice
	}

	if sellingPrice < 0 {
		sellingPrice = variant.ActualPrice
		highestOffer = 0.0
	}

	log.Printf("Calculated for product %d: sellingPrice=%.2f, discount=%.2f", product.ID, sellingPrice, highestOffer)
	return sellingPrice, highestOffer 
}

func AddToWishlist(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        fmt.Println("UserID not found in context")
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        fmt.Println("Invalid userID type:", userID)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
        return
    }
    fmt.Println("AddToWishlist - UserID:", userIDUint)

    var request struct {
        ProductID uint `json:"product_id" binding:"required"`
        VariantID uint `json:"variant_id"` 
    }
    if err := c.BindJSON(&request); err != nil {
        fmt.Println("Invalid request body:", err)
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid request body"})
        return
    }

    var product models.Product
    if err := config.DB.Where("id = ? AND is_active = ?", request.ProductID, true).First(&product).Error; err != nil {
        fmt.Println("Product not found:", err)
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Product not found or inactive"})
        return
    }

    var wishlist models.Wishlist
    if err := config.DB.Where("user_id = ?", userIDUint).FirstOrCreate(&wishlist, models.Wishlist{UserID: userIDUint}).Error; err != nil {
        fmt.Println("Failed to fetch or create wishlist:", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch or create wishlist"})
        return
    }

    var existingItem models.WishlistItem
    query := config.DB.Where("wishlist_id = ? AND product_id = ?", wishlist.ID, request.ProductID)
    if request.VariantID != 0 {
        query = query.Where("product_variant_id = ?", request.VariantID)
    } else {
        query = query.Where("product_variant_id IS NULL")
    }
    if err := query.First(&existingItem).Error; err == nil {
        fmt.Println("Item already in wishlist: ProductID:", request.ProductID, "VariantID:", request.VariantID)
        c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Item already in wishlist"})
        return
    } else if err != gorm.ErrRecordNotFound {
        fmt.Println("Error checking existing wishlist item:", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to check wishlist"})
        return
    }

    var variantID *uint
    if request.VariantID != 0 {
        var variant models.ProductVariant
        if err := config.DB.Where("id = ? AND product_id = ? AND is_active = ?", request.VariantID, request.ProductID, true).First(&variant).Error; err != nil {
            fmt.Println("Variant not found or inactive:", err)
            c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Variant not found or inactive"})
            return
        }
        variantID = &request.VariantID
    } else {
        variantID = nil 
    }

    wishlistItem := models.WishlistItem{
        WishlistID:       wishlist.ID,
        ProductID:        request.ProductID,
        ProductVariantID: variantID,
    }
    if err := config.DB.Create(&wishlistItem).Error; err != nil {
        fmt.Println("Failed to add to wishlist:", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to add to wishlist"})
        return
    }

    fmt.Println("Item added to wishlist successfully for user:", userIDUint, "ProductID:", request.ProductID, "VariantID:", request.VariantID)
    c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Item added to wishlist"})
}

func RemoveFromWishlist(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
		return
	}

	wishlistItemID := c.Param("id")
	if wishlistItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Wishlist item ID required"})
		return
	}

	var wishlist models.Wishlist
	if err := config.DB.Where("user_id = ?", userIDUint).First(&wishlist).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Wishlist not found"})
		return
	}

	result := config.DB.Where("id = ? AND wishlist_id = ?", wishlistItemID, wishlist.ID).Delete(&models.WishlistItem{})
	if result.Error != nil {
		fmt.Println("Failed to remove wishlist item:", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to remove item"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Wishlist item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Item removed from wishlist"})
}

func MoveToCartFromWishlist(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
		return
	}

	wishlistItemID := c.Param("id")
	if wishlistItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Wishlist item ID required"})
		return
	}

	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to start transaction"})
		return
	}

	var wishlistItem models.WishlistItem
	if err := tx.
		Preload("Product").
		Preload("ProductVariant").
		Joins("JOIN wishlists ON wishlists.id = wishlist_items.wishlist_id").
		Where("wishlist_items.id = ? AND wishlists.user_id = ?", wishlistItemID, userIDUint).
		First(&wishlistItem).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Wishlist item not found"})
		return
	}

	variant := wishlistItem.ProductVariant
	if wishlistItem.ProductVariantID == nil || *wishlistItem.ProductVariantID == 0 || !variant.IsActive || variant.StockCount <= 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "No available variant found"})
		return
	}

	var cart models.Cart
	if err := tx.Where("user_id = ?", userIDUint).FirstOrCreate(&cart, models.Cart{UserID: userIDUint}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch or create cart"})
		return
	}

	var cartItem models.CartItem
	const maxQuantity = 4
	if err := tx.Where("cart_id = ? AND product_id = ? AND product_variant_id = ?", cart.ID, wishlistItem.ProductID, variant.ID).
		First(&cartItem).Error; err == nil {
		newQuantity := cartItem.Quantity + 1
		if newQuantity > maxQuantity || newQuantity > variant.StockCount {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": fmt.Sprintf("Quantity exceeds limit (%d) or stock (%d)", maxQuantity, variant.StockCount)})
			return
		}
		cartItem.Quantity = newQuantity
		if err := tx.Save(&cartItem).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update cart"})
			return
		}
	} else {
		if variant.StockCount < 1 {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Insufficient stock"})
			return
		}
		cartItem = models.CartItem{
			CartID:           cart.ID,
			ProductID:        wishlistItem.ProductID,
			ProductVariantID: variant.ID,
			Quantity:         1,
		}
		if err := tx.Create(&cartItem).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to add to cart"})
			return
		}
	}

	if err := tx.Delete(&wishlistItem).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to remove from wishlist"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Item moved to cart and removed from wishlist"})
}

func WishlistPage(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
		return
	}

	var user models.User
    if err := config.DB.Preload("UserDetails").First(&user, userIDUint).Error; err != nil {
        log.Printf("User not found: %v", err)
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

	var categories []models.Category
	if err := config.DB.Where("list = ?", true).Find(&categories).Error; err != nil {
		fmt.Println("Failed to fetch categories:", err)
	}

	var wishlist models.Wishlist
	if err := config.DB.Where("user_id = ?", userIDUint).First(&wishlist).Error; err != nil {
		c.HTML(http.StatusOK, "User_Profile_Wishlist.html", gin.H{
			"WishlistItems": []models.WishlistItem{},
			"isLoggedIn":    true,
			"categories":    categories,
		})
		return
	}

	var wishlistItems []models.WishlistItem
	if err := config.DB.
		Preload("Product").
		Preload("Product.Images").
		Preload("ProductVariant").
		Where("wishlist_id = ?", wishlist.ID).
		Find(&wishlistItems).Error; err != nil {
		fmt.Println("Failed to fetch wishlist items:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch wishlist items"})
		return
	}

	type WishlistWithDetails struct {
		models.WishlistItem
		SellingPrice float64
		Discount     float64
	}

	var wishlistWithDetails []WishlistWithDetails
	for _, item := range wishlistItems {
		sellingPrice := 0.0
		discount := 0.0
		if item.ProductVariantID != nil && *item.ProductVariantID != 0 {
			variant := item.ProductVariant
			sellingPrice = variant.SellingPrice
			for _, offer := range item.Product.ProductOffers {
				if offer.Status == "Active" && time.Now().After(offer.StartDate) && time.Now().Before(offer.EndDate) {
					discount = offer.OfferPercentage
					sellingPrice -= (variant.SellingPrice * discount) / 100
					break
				}
			}
		}
		wishlistWithDetails = append(wishlistWithDetails, WishlistWithDetails{
			WishlistItem: item,
			SellingPrice: sellingPrice,
			Discount:     discount,
		})
	}

data:=gin.H{
	"username" : user.FirstName+user.LastName,
	"userImage":    user.UserDetails.Image,
	"WishlistItems": wishlistWithDetails,
	"isLoggedIn":    true,
	"categories":    categories,
}

	c.HTML(http.StatusOK, "User_Profile_Wishlist.html", data)
}

func AddToCart(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		fmt.Println("UserID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		fmt.Println("Invalid userID type:", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}
	fmt.Println("AddToCart - UserID:", userIDUint)

	var input struct {
		ProductID uint `json:"product_id"`
		VariantID uint `json:"variant_id"`
		Quantity  int  `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		fmt.Println("Invalid input:", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid input"})
		return
	}

	if input.ProductID == 0 || input.VariantID == 0 || input.Quantity <= 0 || input.Quantity > 4 {
		fmt.Println("Invalid input data:", input)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid input data"})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			fmt.Println("Panic recovered:", r)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Unexpected error occurred"})
		}
	}()

	var product models.Product
	if err := tx.Where("id = ? AND is_active = ?", input.ProductID, true).First(&product).Error; err != nil {
		tx.Rollback()
		fmt.Println("Product not found:", err)
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Product not found"})
		return
	}

	var variant models.ProductVariant
	if err := tx.Where("id = ? AND product_id = ? AND is_active = ?", input.VariantID, input.ProductID, true).First(&variant).Error; err != nil {
		tx.Rollback()
		fmt.Println("Variant not found:", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Variant not found"})
		return
	}

	if variant.StockCount < input.Quantity {
		tx.Rollback()
		fmt.Println("Insufficient stock - Requested:", input.Quantity, "Available:", variant.StockCount)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Insufficient stock"})
		return
	}

	var cart models.Cart
	if err := tx.Where("user_id = ?", userIDUint).First(&cart).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			cart = models.Cart{UserID: userIDUint}
			if err := tx.Create(&cart).Error; err != nil {
				tx.Rollback()
				fmt.Println("Failed to create cart:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create cart"})
				return
			}
		} else {
			tx.Rollback()
			fmt.Println("Database error fetching cart:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Database error"})
			return
		}
	}

	var cartItem models.CartItem
	if err := tx.Where("cart_id = ? AND product_id = ? AND product_variant_id = ?", cart.ID, input.ProductID, input.VariantID).
		First(&cartItem).Error; err == nil {
		newQuantity := cartItem.Quantity + input.Quantity
		if newQuantity > 4 || newQuantity > variant.StockCount {
			tx.Rollback()
			fmt.Println("Quantity exceeds limit - New:", newQuantity, "Stock:", variant.StockCount)
			c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Quantity exceeds limit or stock"})
			return
		}
		cartItem.Quantity = newQuantity
		if err := tx.Save(&cartItem).Error; err != nil {
			tx.Rollback()
			fmt.Println("Failed to update cart item:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update cart item"})
			return
		}
	} else {
		cartItem = models.CartItem{
			CartID:           cart.ID,
			ProductID:        input.ProductID,
			ProductVariantID: input.VariantID,
			Quantity:         input.Quantity,
		}
		if err := tx.Create(&cartItem).Error; err != nil {
			tx.Rollback()
			fmt.Println("Failed to add to cart:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to add to cart"})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		fmt.Println("Failed to commit transaction:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to commit transaction"})
		return
	}

	fmt.Println("Item added to cart successfully for user:", userIDUint)
	c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Item added to cart"})
}

type CartItemWithTotal struct {
    CartItem  models.CartItem
    ItemTotal float64
    Size      string
    Discount  float64
}

func ViewCart(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
        return
    }

    var user models.User
    if err := config.DB.Preload("UserDetails").First(&user, userIDUint).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch user data"})
        return
    }

    var cart models.Cart
    if err := config.DB.
        Preload("Items.ProductVariant").
        Preload("Items.Product.Images").
        Preload("Items.Product").
        Where("user_id = ?", userIDUint).First(&cart).Error; err != nil || len(cart.Items) == 0 {
        data := CommonData(&user)
        data["cartItems"] = nil
        data["itemCount"] = 0
        data["subtotal"] = 0.0
        data["discount"] = 0.0
        data["tax"] = 0.0
        data["delivery"] = 0.0
        data["total"] = 0.0
        data["isLoggedIn"] = true
        c.HTML(http.StatusOK, "User_Cart.html", data)
        return
    }

    var subtotal, discount float64
    const taxRate = 0.03
    const freeShippingThreshold = 1000.0
    const deliveryCharge = 99.0

    cartItemsWithTotal := make([]CartItemWithTotal, 0, len(cart.Items))

    for _, item := range cart.Items {
        if item.ProductVariant.ID == 0 || item.Product.ID == 0 || !item.Product.IsActive || !item.ProductVariant.IsActive {
            continue
        }

        sellingPrice, itemDiscount := CalculateSellingPrice(item.ProductVariant, config.DB)
        itemTotal := float64(item.Quantity) * sellingPrice
        discount += float64(item.Quantity) * (item.ProductVariant.ActualPrice - sellingPrice)
        cartItemsWithTotal = append(cartItemsWithTotal, CartItemWithTotal{
            CartItem:  item,
            ItemTotal: itemTotal,
            Size:      item.ProductVariant.Size,
            Discount:  itemDiscount,
        })
        subtotal += float64(item.Quantity) * item.ProductVariant.ActualPrice
    }

    tax := (subtotal - discount) * taxRate
    delivery := 0.0
    if subtotal-discount < freeShippingThreshold && len(cartItemsWithTotal) > 0 {
        delivery = deliveryCharge
    }
    total := subtotal - discount + tax + delivery

    data := CommonData(&user)
    data["cartItems"] = cartItemsWithTotal
    data["itemCount"] = len(cartItemsWithTotal)
    data["subtotal"] = subtotal
    data["discount"] = discount
    data["tax"] = tax
    data["delivery"] = delivery
    data["total"] = total
    data["isLoggedIn"] = true
    c.HTML(http.StatusOK, "User_Cart.html", data)
}

func UpdateCart(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	var input struct {
		CartItemID uint `json:"cart_item_id"`
		Quantity   int  `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid input: " + err.Error()})
		return
	}

	var cartItem models.CartItem
	if err := config.DB.Preload("ProductVariant").
		Where("id = ? AND cart_id IN (SELECT id FROM carts WHERE user_id = ?)", input.CartItemID, userIDUint).
		First(&cartItem).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Cart item not found"})
		return
	}

	const maxQuantity = 4
	stock := cartItem.ProductVariant.StockCount
	if input.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Quantity must be greater than 0"})
		return
	}
	if input.Quantity > maxQuantity {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": fmt.Sprintf("Quantity cannot exceed %d", maxQuantity)})
		return
	}
	if input.Quantity > stock {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": fmt.Sprintf("Quantity exceeds available stock (%d)", stock)})
		return
	}

	cartItem.Quantity = input.Quantity
	if err := config.DB.Save(&cartItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update cart: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Cart updated"})
}

func RemoveFromCart(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	var input struct {
		CartItemID uint `json:"cart_item_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid input"})
		return
	}

	var cartItem models.CartItem
	if err := config.DB.
		Where("id = ? AND cart_id IN (SELECT id FROM carts WHERE user_id = ?)", input.CartItemID, userIDUint).
		First(&cartItem).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Cart item not found"})
		return
	}

	if err := config.DB.Delete(&cartItem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to remove item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Success", "message": "Item removed from cart"})
}

type CheckoutAddressData struct {
	Addresses      []models.Address
	DefaultAddress *models.Address
}

func CheckoutAddress(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		log.Println("User not authenticated, redirecting to login")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		log.Println("UserID is not uint type, redirecting to login")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	if c.Request.Method == "POST" {
		addressID := c.PostForm("address_id")
		log.Printf("Received POST with address_id: %s", addressID)
		if addressID == "" {
			log.Println("No address selected in POST request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Please select an address"})
			return
		}

		var address models.Address
		if err := config.DB.Where("id = ? AND user_id = ?", addressID, userIDUint).First(&address).Error; err != nil {
			log.Printf("Invalid address_id %s for user %d: %v", addressID, userIDUint, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address selected"})
			return
		}
		redirectURL := "/checkout/payment?address_id=" + addressID
		log.Printf("Redirecting to: %s", redirectURL)
		c.Redirect(http.StatusSeeOther, redirectURL)
		return
	}

	var data CheckoutAddressData
	if err := config.DB.Where("user_id = ?", userIDUint).Find(&data.Addresses).Error; err != nil {
		log.Printf("Error fetching addresses for user %d: %v", userIDUint, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch addresses"})
		return
	}

	var defaultAddr models.Address
	if err := config.DB.Where("user_id = ? AND default_address = ?", userIDUint, true).First(&defaultAddr).Error; err == nil {
		data.DefaultAddress = &defaultAddr
	}

	c.HTML(http.StatusOK, "User_Checkout_Address.html", data)
}

func CheckoutPayment(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
        return
    }

    addressID := c.Query("address_id")
    if addressID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Address ID not provided"})
        return
    }

    var address models.Address
    if err := config.DB.Where("id = ? AND user_id = ?", addressID, userIDUint).First(&address).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Address not found"})
        return
    }

    var cart models.Cart
    if err := config.DB.
        Preload("Items.Product").
        Preload("Items.ProductVariant").
        Where("user_id = ?", userIDUint).
        First(&cart).Error; err != nil {
        if err == gorm.ErrRecordNotFound {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    if len(cart.Items) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
        return
    }

    var wallet models.Wallet
    if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
        wallet = models.Wallet{UserID: userIDUint, Balance: 0.00}
        if err := config.DB.Create(&wallet).Error; err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet"})
            return
        }
    }

    var subtotal, discount float64
    const taxRate = 0.03
    const freeShippingThreshold = 1000.0
    const deliveryCharge = 99.0
    var expectedDelivery time.Time

    for i, item := range cart.Items {
        if !item.Product.IsActive || !item.ProductVariant.IsActive {
            continue
        }
        sellingPrice, _ := CalculateSellingPrice(item.ProductVariant, config.DB)
        subtotal += float64(item.Quantity) * item.ProductVariant.ActualPrice
        discount += float64(item.Quantity) * (item.ProductVariant.ActualPrice - sellingPrice)
        if i == 0 {
            expectedDelivery = time.Now().AddDate(0, 0, 5)
        }
    }

    var coupons []models.Coupon
    if err := config.DB.
        Where("is_active = ? AND expiration_date > ? AND used_count < usage_limit", true, time.Now()).
        Find(&coupons).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch coupons"})
        return
    }

    var applicableCoupons []models.Coupon
    for _, coupon := range coupons {
        if subtotal >= coupon.MinAmount {
            applicableCoupons = append(applicableCoupons, coupon)
        }
        
    }

    subtotalMinusDiscount := subtotal - discount
    tax := subtotalMinusDiscount * taxRate
    delivery := 0.0
    if subtotalMinusDiscount < freeShippingThreshold {
        delivery = deliveryCharge
    }
    total := subtotalMinusDiscount + tax + delivery

    allowCOD := total <= 1000.0

    paymentData := utils.PaymentData{
        Address:          address,
        ExpectedDelivery: expectedDelivery.Format("2006-01-02"),
        Subtotal:         subtotal,
        Discount:         discount,
        Tax:              tax,
        Delivery:         delivery,
        Total:            total,
        ItemCount:        len(cart.Items),
    }

    signedToken, err := utils.SignPaymentData(paymentData, os.Getenv("PAYMENT_DATA_SECRET"))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign payment data"})
        return
    }

    c.HTML(http.StatusOK, "User_Checkout_Payment.html", gin.H{
        "Address":               address,
        "ExpectedDelivery":      expectedDelivery.Format("02 Jan 2006"),
        "Subtotal":              subtotal,
        "Discount":              discount,
        "SubtotalMinusDiscount": subtotalMinusDiscount,
        "Tax":                   tax,
        "Delivery":              delivery,
        "Total":                 total,
        "ItemCount":             len(cart.Items),
        "SignedToken":           signedToken,
        "RazorpayKeyID":         os.Getenv("RAZORPAY_KEY_ID"),
        "Coupons":               applicableCoupons,
        "CSRFToken":             c.GetString("csrf_token"),
        "WalletBalance":         wallet.Balance,
        "AllowCOD":              allowCOD,
    })
}

func CreateRazorpayOrder(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		log.Println("User not authenticated in CreateRazorpayOrder")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		log.Println("Invalid user ID type in CreateRazorpayOrder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	var cart models.Cart
	if err := config.DB.
		Preload("Items").
		Preload("Items.Product").
		Preload("Items.ProductVariant").
		Where("user_id = ?", userIDUint).
		First(&cart).Error; err != nil {
		log.Printf("Failed to fetch cart for user %d: %v", userIDUint, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	if len(cart.Items) == 0 {
		log.Printf("Cart is empty for user %d", userIDUint)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
		return
	}

	var subtotal, initialDiscount float64
	const taxRate = 0.03
	const freeShippingThreshold = 1000.0
	const deliveryCharge = 99.0
	var validItems []models.CartItem

	for _, item := range cart.Items {
		if item.Product.ID == 0 || item.ProductVariant.ID == 0 || !item.Product.IsActive || !item.ProductVariant.IsActive {
			continue
		}
		sellingPrice, _ := CalculateSellingPrice(item.ProductVariant, config.DB)
		validItems = append(validItems, item)
		subtotal += float64(item.Quantity) * item.ProductVariant.ActualPrice
		initialDiscount += float64(item.Quantity) * (item.ProductVariant.ActualPrice - sellingPrice)
	}

	if len(validItems) == 0 {
		log.Printf("No valid items in cart for user %d", userIDUint)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid items in cart"})
		return
	}

	taxableAmount := subtotal - initialDiscount
	tax := taxableAmount * taxRate
	delivery := 0.0
	if taxableAmount < freeShippingThreshold {
		delivery = deliveryCharge
	}
	total := taxableAmount + tax + delivery
	if total < 0 {
		total = 0
	}

	amountInPaise := int(total * 100)

	razorpayKeyID := os.Getenv("RAZORPAY_KEY_ID")
	razorpayKeySecret := os.Getenv("RAZORPAY_KEY_SECRET")
	if razorpayKeyID == "" || razorpayKeySecret == "" {
		log.Println("Razorpay environment variables (RAZORPAY_KEY_ID or RAZORPAY_KEY_SECRET) are not set")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment gateway configuration error"})
		return
	}

	client := razorpay.NewClient(razorpayKeyID, razorpayKeySecret)
	data := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": "INR",
		"receipt":  "receipt_" + strconv.FormatUint(uint64(userIDUint), 10),
	}

	order, err := client.Order.Create(data, nil)
	if err != nil {
		log.Printf("Failed to create Razorpay order for user %d: %v", userIDUint, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Razorpay order: " + err.Error()})
		return
	}

	orderID, ok := order["id"].(string)
	if !ok {
		log.Println("Invalid Razorpay order ID format")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Razorpay order ID"})
		return
	}

	log.Printf("Created Razorpay order for user %d: orderID=%s", userIDUint, orderID)
	c.JSON(http.StatusOK, gin.H{
		"order_id": orderID,
		"amount":   amountInPaise,
		"currency": "INR",
	})
}

func CreatePreOrder(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
        return
    }

    addressID := c.PostForm("address_id")
    paymentMethod := c.PostForm("payment_method")
    paymentDataToken := c.PostForm("payment_data_token")
    couponIDStr := c.PostForm("coupon_id")

    if addressID == "" || paymentMethod == "" || paymentDataToken == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
        return
    }

    paymentData, err := utils.VerifyAndDecodePaymentData(paymentDataToken, os.Getenv("PAYMENT_DATA_SECRET"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment data"})
        return
    } 

    var cart models.Cart
    if err := config.DB.
        Preload("Items.Product").
        Preload("Items.ProductVariant").
        Where("user_id = ?", userIDUint).
        First(&cart).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
        return
    }

    if len(cart.Items) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
        return
    }

    var subtotal, initialDiscount float64
    const taxRate = 0.03
    const freeShippingThreshold = 1000.0
    const deliveryCharge = 99.0

    for _, item := range cart.Items {
        if !item.Product.IsActive || !item.ProductVariant.IsActive {
            continue
        }
        sellingPrice, _ := CalculateSellingPrice(item.ProductVariant, config.DB)
        subtotal += float64(item.Quantity) * item.ProductVariant.ActualPrice
        initialDiscount += float64(item.Quantity) * (item.ProductVariant.ActualPrice - sellingPrice)
    }

    var couponDiscount float64
    var couponID *uint
    if couponIDStr != "" {
        couponIDUint, err := strconv.ParseUint(couponIDStr, 10, 32)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon ID"})
            return
        }
        var coupon models.Coupon
        if err := config.DB.Where("id = ?", couponIDUint).First(&coupon).Error; err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon not found"})
            return
        }
        if !coupon.IsActive || coupon.ExpirationDate.Before(time.Now()) || coupon.UsedCount >= coupon.UsageLimit || subtotal < coupon.MinAmount {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon not applicable"})
            return
        }
        couponDiscount = (coupon.Discount / 100) * subtotal
        if coupon.MaxAmount > 0 && couponDiscount > coupon.MaxAmount {
            couponDiscount = coupon.MaxAmount
        }
        if couponDiscount > (subtotal - initialDiscount) {
            couponDiscount = subtotal - initialDiscount
        }
        couponID = new(uint)
        *couponID = uint(couponIDUint)
    }

    taxableAmount := subtotal - initialDiscount - couponDiscount
    tax := taxableAmount * taxRate
    delivery := 0.0
    if taxableAmount < freeShippingThreshold {
        delivery = deliveryCharge
    }
    total := taxableAmount + tax + delivery
    if total < 0 {
        total = 0
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
        return
    }

    order := models.Order{
        OrderUID:       "ORD-" + uuid.New().String(),
        UserID:         userIDUint,
        CouponID:       couponID,
        SubTotal:       subtotal,
        TotalDiscount:  initialDiscount,
        CouponDiscount: couponDiscount,
        ShippingCharge: delivery,
        Tax:            tax,
        TotalAmount:    total,
        OrderDate:      time.Now(),
    }

    if paymentMethod == "cod" && order.TotalAmount > 1000.0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Cash on Delivery is not available for orders above ₹1000"})
        return
    }

    var validItems []models.CartItem
for _, item := range cart.Items {
    if !item.Product.IsActive || !item.ProductVariant.IsActive || item.ProductVariant.StockCount < item.Quantity {
        log.Printf("Skipping invalid item: ProductID=%d, VariantID=%d, ProductName=%s, IsActive=%v, VariantIsActive=%v, Stock=%d, Quantity=%d",
            item.ProductID, item.ProductVariantID, item.Product.ProductName, item.Product.IsActive, item.ProductVariant.IsActive, item.ProductVariant.StockCount, item.Quantity)
        continue
    }
    validItems = append(validItems, item)
}
if len(validItems) == 0 {
    tx.Rollback()
    c.JSON(http.StatusBadRequest, gin.H{"error": "No valid items in cart"})
    return
}

    orderItems := make([]models.OrderItem, 0, len(validItems))
for _, item := range validItems {
    sellingPrice, _ := CalculateSellingPrice(item.ProductVariant, config.DB)
    itemSubtotal := float64(item.Quantity) * item.ProductVariant.ActualPrice
    itemInitialDiscount := float64(item.Quantity) * (item.ProductVariant.ActualPrice - sellingPrice)
    itemCouponDiscount := (itemSubtotal / subtotal) * couponDiscount
    itemTaxable := itemSubtotal - itemInitialDiscount - itemCouponDiscount
    itemTax := itemTaxable * taxRate
    orderItem := models.OrderItem{
        ProductVariantID:     item.ProductVariantID,
        ProductID:            item.ProductID,
        Quantity:             item.Quantity,
        ProductName:          item.Product.ProductName,
        ProductCategory:      item.Product.Category.CategoryName,
        ProductActualPrice:   item.ProductVariant.ActualPrice,
        ProductSellPrice:     sellingPrice,
        Tax:                  itemTax,
        Total:                itemTaxable + itemTax,
        ExpectedDeliveryDate: time.Now().AddDate(0, 0, 5),
        OrderStatus:          "Pending",
        Size:                 item.ProductVariant.Size,
    }
    orderItems = append(orderItems, orderItem)
}

    order.OrderItem = orderItems
    if err := tx.Create(&order).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
        return
    }

    shippingAddress := models.ShippingAddress{
        OrderID:     order.ID,
        UserID:      userIDUint,
        FirstName:   paymentData.Address.FirstName,
        LastName:    paymentData.Address.LastName,
        Email:       paymentData.Address.Email,
        PhoneNumber: paymentData.Address.PhoneNumber,
        Country:     paymentData.Address.Country,
        Postcode:    paymentData.Address.Postcode,
        State:       paymentData.Address.State,
        City:        paymentData.Address.City,
        AddressLine: paymentData.Address.AddressLine,
        Landmark:    paymentData.Address.Landmark,
    }
    if err := tx.Create(&shippingAddress).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create shipping address"})
        return
    }

    paymentDetails := models.PaymentDetails{
        OrderID:       order.ID,
        UserID:        userIDUint,
        PaymentAmount: total,
        PaymentMethod: paymentMethod,
        PaymentStatus: "Pending",
        PaymentDate:   time.Now(),
    }
    if err := tx.Create(&paymentDetails).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment details"})
        return
    }

    var razorpayOrderID string
    if paymentMethod == "razorpay" {
        client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
        orderData := map[string]interface{}{
            "amount":   int(total * 100),
            "currency": "INR",
            "receipt":  order.OrderUID,
        }
        razorpayOrder, err := client.Order.Create(orderData, nil)
        if err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Razorpay order: " + err.Error()})
            return
        }
        razorpayOrderID = razorpayOrder["id"].(string)
        paymentDetails.TransactionID = razorpayOrderID
        if err := tx.Save(&paymentDetails).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment details"})
            return
        }
    }

    if couponID != nil {
        var coupon models.Coupon
        if err := tx.Where("id = ?", *couponID).First(&coupon).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch coupon"})
            return
        }
        coupon.UsedCount++
        if err := tx.Save(&coupon).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update coupon usage"})
            return
        }
    }

    if err := tx.Commit().Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
        return
    }

    response := gin.H{
        "status":   "Success",
        "order_id": order.OrderUID,
    }
    if paymentMethod == "razorpay" {
        response["razorpay_order_id"] = razorpayOrderID
    }
    c.JSON(http.StatusOK, response)
}

func CreateOrder(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
        return
    }

    var cart models.Cart
    if err := config.DB.
        Preload("Items").
        Preload("Items.Product").
        Preload("Items.ProductVariant").
        Preload("Items.Product.Category").
        Where("user_id = ?", userIDUint).
        First(&cart).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch cart"})
        return
    }

    if len(cart.Items) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Cart is empty"})
        return
    }

    var address models.Address
    if err := config.DB.Where("user_id = ? AND default_address = ?", userIDUint, true).First(&address).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Default address not found"})
        return
    }

    var subtotal, initialDiscount float64
    const taxRate = 0.03
    const freeShippingThreshold = 1000.0
    const deliveryCharge = 99.0
    var expectedDelivery time.Time
    var validItems []models.CartItem

    for i, item := range cart.Items {
        if item.Product.ID == 0 || item.ProductVariant.ID == 0 || !item.Product.IsActive || !item.ProductVariant.IsActive {
            continue
        }
        sellingPrice, _ := CalculateSellingPrice(item.ProductVariant, config.DB)
        validItems = append(validItems, item)
        subtotal += float64(item.Quantity) * item.ProductVariant.ActualPrice
        initialDiscount += float64(item.Quantity) * (item.ProductVariant.ActualPrice - sellingPrice)
        itemDeliveryDate := time.Now().AddDate(0, 0, 5)
        if i == 0 {
            expectedDelivery = itemDeliveryDate
        } else if itemDeliveryDate.After(expectedDelivery) {
            expectedDelivery = itemDeliveryDate
        }
    }

    if len(validItems) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "No valid items in cart"})
        return
    }

    taxableAmount := subtotal - initialDiscount
    tax := taxableAmount * taxRate
    delivery := 0.0
    if taxableAmount < freeShippingThreshold {
        delivery = deliveryCharge
    }
    total := taxableAmount + tax + delivery
    if total < 0 {
        total = 0
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to start transaction"})
        return
    }

    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Unexpected error occurred"})
        }
    }()

    order := models.Order{
        OrderUID:       "ORD-" + uuid.New().String(),
        UserID:         userIDUint,
        SubTotal:       subtotal,
        TotalDiscount:  initialDiscount,
        CouponDiscount: 0.0, 
        ShippingCharge: delivery,
        Tax:            tax,
        TotalAmount:    total,
        OrderDate:      time.Now(),
    }

    var orderItems []models.OrderItem
    for _, cartItem := range cart.Items {
        if cartItem.ProductVariant.ID == 0 || cartItem.Product.ID == 0 || !cartItem.Product.IsActive || !cartItem.ProductVariant.IsActive {
            continue
        }
        if cartItem.ProductVariant.StockCount < cartItem.Quantity {
            tx.Rollback()
            c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Insufficient stock for product: " + cartItem.Product.ProductName})
            return
        }
        sellingPrice, _ := CalculateSellingPrice(cartItem.ProductVariant, config.DB)
        itemSubtotal := float64(cartItem.Quantity) * cartItem.ProductVariant.ActualPrice
        itemDiscount := float64(cartItem.Quantity) * (cartItem.ProductVariant.ActualPrice - sellingPrice)
        itemTaxable := itemSubtotal - itemDiscount
        itemTax := itemTaxable * taxRate
        orderItem := models.OrderItem{
            ProductVariantID:     cartItem.ProductVariantID,
            ProductID:            cartItem.ProductID,
            Quantity:             cartItem.Quantity,
            ProductName:          cartItem.Product.ProductName,
            ProductCategory:      cartItem.Product.Category.CategoryName,
            ProductActualPrice:   cartItem.ProductVariant.ActualPrice,
            ProductSellPrice:     sellingPrice,
            Tax:                  itemTax,
            Total:                itemTaxable + itemTax,
            ExpectedDeliveryDate: time.Now().AddDate(0, 0, 5),
            OrderStatus:          "Processing",
            Size:                 cartItem.ProductVariant.Size,
        }
        orderItems = append(orderItems, orderItem)

        cartItem.ProductVariant.StockCount -= cartItem.Quantity
        if err := tx.Save(&cartItem.ProductVariant).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update stock count"})
            return
        }
    }

    order.OrderItem = orderItems
    if err := tx.Create(&order).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create order"})
        return
    }

    shippingAddress := models.ShippingAddress{
        OrderID:     order.ID,
        UserID:      userIDUint,
        FirstName:   address.FirstName,
        LastName:    address.LastName,
        Email:       address.Email,
        PhoneNumber: address.PhoneNumber,
        Country:     address.Country,
        Postcode:    address.Postcode,
        State:       address.State,
        City:        address.City,
        AddressLine: address.AddressLine,
        Landmark:    address.Landmark,
    }
    if err := tx.Create(&shippingAddress).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create shipping address"})
        return
    }

    paymentDetails := models.PaymentDetails{
        OrderID:       order.ID,
        UserID:        userIDUint,
        PaymentAmount: total,
        PaymentMethod: "cod",
        PaymentStatus: "Pending",
        PaymentDate:   time.Now(),
    }
    if err := tx.Create(&paymentDetails).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create payment details"})
        return
    }

    if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to clear cart"})
        return
    }

    if err := tx.Commit().Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to commit transaction"})
        return
    }

    c.Redirect(http.StatusFound, "/order/success?order_id="+order.OrderUID)
}

func ConfirmOrder(c *gin.Context) {
    log.Printf("Received request to /confirm-order")
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
        return
    }

    orderID := c.PostForm("order_id")
    paymentMethod := c.PostForm("payment_method")
    clientError := c.PostForm("error")
    couponIDStr := c.PostForm("coupon_id")

    log.Printf("ConfirmOrder: order_id=%s, payment_method=%s, client_error=%s, coupon_id=%s", orderID, paymentMethod, clientError, couponIDStr)

    if orderID == "" || paymentMethod == "" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Missing required fields"})
        return
    }

    var order models.Order
    if err := config.DB.
        Preload("OrderItem").
        Preload("PaymentDetails").
        Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
        First(&order).Error; err != nil {
        log.Printf("Order %s not found for user %d: %v", orderID, userIDUint, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
        return
    }

    var cart models.Cart
    if err := config.DB.Where("user_id = ?", userIDUint).First(&cart).Error; err != nil && err != gorm.ErrRecordNotFound {
        log.Printf("Failed to fetch cart for user %d: %v", userIDUint, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch cart"})
        return
    }

    var cartItems []models.CartItem
    if err := config.DB.
        Preload("ProductVariant").
        Where("cart_id = ?", cart.ID).
        Find(&cartItems).Error; err != nil {
        log.Printf("Failed to fetch cart items for cart %d: %v", cart.ID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch cart items"})
        return
    }
    cart.Items = cartItems

    paymentStatus := "Pending"
    orderStatus := "Pending"
    redirectURL := "/order/failure?order_id=" + order.OrderUID
    var errorMessage string

    tx := config.DB.Begin()
    if tx.Error != nil {
        log.Printf("Failed to start transaction: %v", tx.Error)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to start transaction"})
        return
    }

    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            log.Printf("Panic recovered in ConfirmOrder: %v", r)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Unexpected error occurred"})
        }
    }()

    if paymentMethod == "cod" && clientError == "" {
        paymentStatus = "Pending"
        orderStatus = "Processing"
        redirectURL = "/order/success?order_id=" + order.OrderUID
    } else if paymentMethod == "razorpay" {
        razorpayPaymentID := c.PostForm("razorpay_payment_id")
        razorpayOrderID := c.PostForm("razorpay_order_id")
        razorpaySignature := c.PostForm("razorpay_signature")

        log.Printf("Razorpay: payment_id=%s, order_id=%s, signature=%s", razorpayPaymentID, razorpayOrderID, razorpaySignature)

        if razorpayPaymentID == "" || razorpayOrderID == "" || razorpaySignature == "" {
            errorMessage = clientError
            if errorMessage == "" {
                errorMessage = "Incomplete Razorpay payment details"
            }
            paymentStatus = "Failed"
            orderStatus = "Failed"
        } else if razorpayOrderID != order.PaymentDetails.TransactionID {
            errorMessage = "Razorpay order ID mismatch"
            paymentStatus = "Failed"
            orderStatus = "Failed"
        } else {
            client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
            payload := razorpayOrderID + "|" + razorpayPaymentID
            if utils.HmacSha256(payload, os.Getenv("RAZORPAY_KEY_SECRET")) != razorpaySignature {
                errorMessage = "Invalid Razorpay signature"
                paymentStatus = "Failed"
                orderStatus = "Failed"
            } else {
                payment, err := client.Payment.Fetch(razorpayPaymentID, nil, nil)
                if err != nil {
                    errorMessage = "Failed to verify payment: " + err.Error()
                    paymentStatus = "Failed"
                    orderStatus = "Failed"
                } else if payment["status"] != "captured" {
                    errorMessage = "Payment not captured"
                    paymentStatus = "Failed"
                    orderStatus = "Failed"
                } else {
                    paymentStatus = "Completed"
                    orderStatus = "Processing"
                    order.PaymentDetails.TransactionID = razorpayPaymentID
                    redirectURL = "/order/success?order_id=" + order.OrderUID
                }
            }
        }
    } else if paymentMethod == "wallet" && clientError == "" {
        var wallet models.Wallet
        if err := tx.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
            tx.Rollback()
            log.Printf("Wallet not found for user %d: %v", userIDUint, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Wallet not found"})
            return
        }

        if wallet.Balance < order.TotalAmount {
            errorMessage = "Insufficient wallet balance"
            paymentStatus = "Failed"
            orderStatus = "Failed"
        } else {
            wallet.Balance -= order.TotalAmount
            if err := tx.Save(&wallet).Error; err != nil {
                tx.Rollback()
                log.Printf("Failed to update wallet balance for user %d: %v", userIDUint, err)
                c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update wallet balance"})
                return
            }

            transaction := models.WalletTransaction{
                WalletID:          wallet.ID,
                TransactionUID:    "TXN-" + uuid.New().String(),
                TransactionAmount: order.TotalAmount,
                TransactionType:   "debit",
                TransactionStatus: "Completed",
                TransactionDate:   time.Now(),
                Description:       fmt.Sprintf("Payment for order %s", order.OrderUID),
            }
            if err := tx.Create(&transaction).Error; err != nil {
                tx.Rollback()
                log.Printf("Failed to create wallet transaction for user %d: %v", userIDUint, err)
                c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create wallet transaction"})
                return
            }

            paymentStatus = "Completed"
            orderStatus = "Processing"
            order.PaymentDetails.TransactionID = transaction.TransactionUID
            redirectURL = "/order/success?order_id=" + order.OrderUID
        }
    }

    if paymentStatus == "Failed" && order.CouponID != nil {
        var coupon models.Coupon
        if err := tx.Where("id = ?", *order.CouponID).First(&coupon).Error; err != nil {
            tx.Rollback()
            log.Printf("Failed to fetch coupon for order %s: %v", orderID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch coupon"})
            return
        }
        if coupon.UsedCount > 0 {
            coupon.UsedCount--
            if err := tx.Save(&coupon).Error; err != nil {
                tx.Rollback()
                log.Printf("Failed to update coupon usage for order %s: %v", orderID, err)
                c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update coupon usage"})
                return
            }
        }
    }

    for i := range order.OrderItem {
        order.OrderItem[i].OrderStatus = orderStatus
        if err := tx.Save(&order.OrderItem[i]).Error; err != nil {
            tx.Rollback()
            log.Printf("Failed to update order items for order %s: %v", orderID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update order items"})
            return
        }
    }

    order.PaymentDetails.PaymentStatus = paymentStatus
    order.PaymentDetails.PaymentDate = time.Now()
    if err := tx.Save(&order.PaymentDetails).Error; err != nil {
        tx.Rollback()
        log.Printf("Failed to update payment details for order %s: %v", orderID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update payment details"})
        return
    }

    if paymentStatus == "Completed" || paymentMethod == "cod" {
        for _, item := range cart.Items {
            if item.ProductVariantID == 0 || item.ProductVariant.ID == 0 {
                tx.Rollback()
                log.Printf("Invalid variant for cart item %d in order %s", item.ID, orderID)
                c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid cart item variant"})
                return
            }
            if item.ProductVariant.StockCount < item.Quantity {
                tx.Rollback()
                log.Printf("Insufficient stock for variant %d in order %s", item.ProductVariantID, orderID)
                c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": fmt.Sprintf("Insufficient stock for %s", item.Product.ProductName)})
                return
            }
            item.ProductVariant.StockCount -= item.Quantity
            if err := tx.Save(&item.ProductVariant).Error; err != nil {
                tx.Rollback()
                log.Printf("Failed to update stock for variant %d in order %s: %v", item.ProductVariantID, orderID, err)
                c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update stock"})
                return
            }
        }
        if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
            tx.Rollback()
            log.Printf("Failed to clear cart for order %s: %v", orderID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to clear cart"})
            return
        }
    }

    if err := tx.Commit().Error; err != nil {
        log.Printf("Failed to commit transaction for order %s: %v", orderID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to commit transaction"})
        return
    }

    if errorMessage != "" {
        redirectURL += "&error=" + url.QueryEscape(errorMessage)
    }

    log.Printf("ConfirmOrder response: redirectURL=%s", redirectURL)
    c.JSON(http.StatusOK, gin.H{"redirectURL": redirectURL})
}

func OrderSuccess(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	orderID := c.Query("order_id")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Order ID not provided"})
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("ShippingAddress").
		Preload("PaymentDetails").
		Preload("OrderItem").
		Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
		return
	}

	expectedDelivery := time.Now().AddDate(0, 0, 5)
	if len(order.OrderItem) > 0 {
		expectedDelivery = order.OrderItem[0].ExpectedDeliveryDate
		for _, item := range order.OrderItem {
			if item.ExpectedDeliveryDate.After(expectedDelivery) {
				expectedDelivery = item.ExpectedDeliveryDate
			}
		}
	}

	c.HTML(http.StatusOK, "Order_Success.html", gin.H{
		"OrderUID":         order.OrderUID,
		"ShippingAddress":  order.ShippingAddress,
		"PaymentMethod":    order.PaymentDetails.PaymentMethod,
		"TotalAmount":      order.TotalAmount,
		"ExpectedDelivery": expectedDelivery.Format("02 Jan 2006"),
	})
}

func OrderFailure(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	orderID := c.Query("order_id")
	errorMessage := c.Query("error")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Order ID not provided"})
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("ShippingAddress").
		Preload("PaymentDetails").
		Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
		First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
		return
	}
	var address models.Address
	addressExists := false
	if order.ShippingAddress.ID != 0 {
		if err := config.DB.Where("id = ? AND user_id = ?", order.ShippingAddress.ID, userIDUint).First(&address).Error; err == nil {
			addressExists = true
		}
	}

	data := gin.H{
		"OrderUID":     order.OrderUID,
		"ErrorMessage": errorMessage,
	}
	if addressExists {
		data["AddressID"] = fmt.Sprintf("%d", order.ShippingAddress.ID)
	}

	c.HTML(http.StatusOK, "Order_Failure.html", data)
}

func RetryPayment(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		log.Println("User not authenticated in RetryPayment")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		log.Println("Invalid user ID type in RetryPayment")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	orderID := c.Query("order_id")
	if orderID == "" {
		log.Println("Order ID not provided in RetryPayment")
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Order ID not provided"})
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("PaymentDetails").
		Preload("ShippingAddress").
		Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
		First(&order).Error; err != nil {
		log.Printf("Order %s not found for user %d: %v", orderID, userIDUint, err)
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
		return
	}

	if order.PaymentDetails.PaymentStatus != "Failed" {
		log.Printf("Retry payment not allowed for order %s with status %s", orderID, order.PaymentDetails.PaymentStatus)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Retry payment is only available for failed payments"})
		return
	}

	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
	data := map[string]interface{}{
		"amount":   int(order.TotalAmount * 100),
		"currency": "INR",
		"receipt":  order.OrderUID,
	}
	razorpayOrder, err := client.Order.Create(data, nil)
	if err != nil {
		log.Printf("Failed to create Razorpay order for retry: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create Razorpay order: " + err.Error()})
		return
	}
	razorpayOrderID, ok := razorpayOrder["id"].(string)
	if !ok {
		log.Println("Invalid Razorpay order ID format")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid Razorpay order ID"})
		return
	}

	order.PaymentDetails.TransactionID = razorpayOrderID
	if err := config.DB.Save(&order.PaymentDetails).Error; err != nil {
		log.Printf("Failed to update payment details for order %s: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update payment details"})
		return
	}

	c.HTML(http.StatusOK, "Retry_Payment.html", gin.H{
		"Order":           order,
		"RazorpayOrderID": razorpayOrderID,
		"RazorpayKeyID":   os.Getenv("RAZORPAY_KEY_ID"),
		"TotalAmount":     order.TotalAmount,
		"CSRFToken":       c.GetString("csrf_token"),
		"AddressID":       order.ShippingAddress.ID,
	})
}

func ConfirmRetryPayment(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		log.Println("User not authenticated in ConfirmRetryPayment")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		log.Println("Invalid user ID type in ConfirmRetryPayment")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	orderID := c.PostForm("order_id")
	razorpayPaymentID := c.PostForm("razorpay_payment_id")
	razorpayOrderID := c.PostForm("razorpay_order_id")
	razorpaySignature := c.PostForm("razorpay_signature")
	clientError := c.PostForm("error")

	log.Printf("ConfirmRetryPayment: order_id=%s, payment_id=%s, order_id=%s, signature=%s, error=%s", orderID, razorpayPaymentID, razorpayOrderID, razorpaySignature, clientError)

	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Missing order ID"})
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("OrderItem").
		Preload("PaymentDetails").
		Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
		First(&order).Error; err != nil {
		log.Printf("Order %s not found for user %d: %v", orderID, userIDUint, err)
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found"})
		return
	}

	var cart models.Cart
	if err := config.DB.
		Preload("Items.ProductVariant").
		Where("user_id = ?", userIDUint).
		First(&cart).Error; err != nil && err != gorm.ErrRecordNotFound {
		log.Printf("Failed to fetch cart for user %d: %v", userIDUint, err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch cart"})
		return
	}

	paymentStatus := "Failed"
	orderStatus := "Failed"
	redirectURL := "/order/failure?order_id=" + order.OrderUID
	var errorMessage string

	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to start transaction"})
		return
	}

	if clientError != "" {
		errorMessage = clientError
	} else if razorpayPaymentID == "" || razorpayOrderID == "" || razorpaySignature == "" {
		errorMessage = "Incomplete Razorpay payment details"
	} else if razorpayOrderID != order.PaymentDetails.TransactionID {
		errorMessage = "Razorpay order ID mismatch"
	} else {
		client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
		payload := razorpayOrderID + "|" + razorpayPaymentID
		if utils.HmacSha256(payload, os.Getenv("RAZORPAY_KEY_SECRET")) != razorpaySignature {
			errorMessage = "Invalid Razorpay signature"
		} else {
			payment, err := client.Payment.Fetch(razorpayPaymentID, nil, nil)
			if err != nil {
				errorMessage = "Failed to verify payment: " + err.Error()
			} else if payment["status"] != "captured" {
				errorMessage = "Payment not captured"
			} else {
				paymentStatus = "Completed"
				orderStatus = "Processing"
				order.PaymentDetails.TransactionID = razorpayPaymentID
				redirectURL = "/order/success?order_id=" + order.OrderUID
			}
		}
	}

	if paymentStatus == "Failed" && order.CouponID != nil {
		var coupon models.Coupon
		if err := tx.Where("id = ?", *order.CouponID).First(&coupon).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to fetch coupon for order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch coupon"})
			return
		}
		if coupon.UsedCount > 0 {
			coupon.UsedCount--
			if err := tx.Save(&coupon).Error; err != nil {
				tx.Rollback()
				log.Printf("Failed to update coupon usage for order %s: %v", orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update coupon usage"})
				return
			}
		}
	}

	for i := range order.OrderItem {
		order.OrderItem[i].OrderStatus = orderStatus
		if err := tx.Save(&order.OrderItem[i]).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to update order items for order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update order items"})
			return
		}
	}

	order.PaymentDetails.PaymentStatus = paymentStatus
	order.PaymentDetails.PaymentDate = time.Now()
	if err := tx.Save(&order.PaymentDetails).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to update payment details for order %s: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update payment details"})
		return
	}

	if paymentStatus == "Completed" {
		for _, item := range order.OrderItem {
			var variant models.ProductVariant
			if err := tx.Where("id = ?", item.ProductVariantID).First(&variant).Error; err != nil {
				tx.Rollback()
				log.Printf("Failed to fetch variant %d for order %s: %v", item.ProductVariantID, orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to fetch product variant"})
				return
			}
			if variant.StockCount < item.Quantity {
				tx.Rollback()
				log.Printf("Insufficient stock for variant %d in order %s", item.ProductVariantID, orderID)
				c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Insufficient stock"})
				return
			}
			variant.StockCount -= item.Quantity
			if err := tx.Save(&variant).Error; err != nil {
				tx.Rollback()
				log.Printf("Failed to update stock for variant %d in order %s: %v", item.ProductVariantID, orderID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update stock"})
				return
			}
		}
		if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to clear cart for order %s: %v", orderID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to clear cart"})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction for order %s: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to commit transaction"})
		return
	}

	if errorMessage != "" {
		redirectURL += "&error=" + url.QueryEscape(errorMessage)
	}

	log.Printf("ConfirmRetryPayment response: redirectURL=%s", redirectURL)
	c.JSON(http.StatusOK, gin.H{"redirectURL": redirectURL})
}

type ProductDisplay struct {
	ProductName  string
	ProductImage string
	Size         string
	Quantity     int
	Status       string
	StatusClass  string
}

type OrderDisplay struct {
	OrderUID         string
	OrderDate        string
	Products         []ProductDisplay
	ExpectedDelivery string
	TotalAmount      float64
}

func Orders(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	var user models.User
	if err := config.DB.Preload("UserDetails").Where("id = ?", userIDUint).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user details"})
		return
	}

	var categories []models.Category
	if err := config.DB.Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	var orders []models.Order
	if err := config.DB.
		Preload("OrderItem").
		Preload("OrderItem.Product").
		Preload("OrderItem.Product.Images").
		Where("user_id = ?", userIDUint).
		Order("order_date DESC").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	var orderDisplays []OrderDisplay
	for _, order := range orders {
		if len(order.OrderItem) == 0 {
			continue
		}

		var products []ProductDisplay
		for _, item := range order.OrderItem {
			productImage := ""
			if item.ProductID != 0 && item.Product.ID != 0 && len(item.Product.Images) > 0 {
				productImage = item.Product.Images[0].ImageURL
			}
			status := item.OrderStatus
			statusClass := ""
			switch item.OrderStatus {
			case "Processing":
				statusClass = "processing"
			case "Shipped", "OutForDelivery":
				statusClass = "shipped"
			case "Delivered":
				statusClass = "delivered"
			case "Cancelled":
				statusClass = "cancelled"
			case "Failed":
				statusClass = "failed"
			default:
				statusClass = "failed"
			}

			products = append(products, ProductDisplay{
				ProductName:  item.ProductName,
				ProductImage: productImage,
				Size:         item.Size,
				Quantity:     item.Quantity,
				Status:       status,
				StatusClass:  statusClass,
			})
		}

		var expectedDelivery time.Time
		for _, item := range order.OrderItem {
			if expectedDelivery.IsZero() || item.ExpectedDeliveryDate.After(expectedDelivery) {
				expectedDelivery = item.ExpectedDeliveryDate
			}
		}

		orderDisplay := OrderDisplay{
			OrderUID:         order.OrderUID,
			OrderDate:        order.OrderDate.Format("02 Jan 2006"),
			Products:         products,
			ExpectedDelivery: expectedDelivery.Format("02 Jan 2006"),
			TotalAmount:      order.TotalAmount,
		}
		orderDisplays = append(orderDisplays, orderDisplay)
	}

	c.HTML(http.StatusOK, "User_Profile_MyOrders.html", gin.H{
		"username":   user.FirstName + " " + user.LastName,
		"userImage":  user.UserDetails.Image,
		"categories": categories,
		"orders":     orderDisplays,
		"isLoggedIn": true,
	})
}

func OrderDetails(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        log.Println("User not authenticated in OrderDetails")
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        log.Println("Invalid user ID type in OrderDetails")
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
        return
    }

    orderID := html.EscapeString(c.Query("order_id"))
    if orderID == "" {
        log.Println("Order ID not provided in OrderDetails")
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Order ID not provided"})
        return
    }

    var order models.Order
    if err := config.DB.
        Preload("ShippingAddress").
        Preload("PaymentDetails").
        Preload("OrderItem.Product.Images").
        Preload("Coupon").
        Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
        First(&order).Error; err != nil {
        log.Printf("Order %s not found for user %d: %v", orderID, userIDUint, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found or does not belong to user"})
        return
    }

    if order.OrderDate.IsZero() {
        order.OrderDate = time.Now()
        if err := config.DB.Save(&order).Error; err != nil {
            log.Printf("Failed to save OrderDate for order %s: %v", order.OrderUID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to process order data"})
            return
        }
    }

    updateItems := false
    for i := range order.OrderItem {
        item := &order.OrderItem[i]
        if item.ExpectedDeliveryDate.IsZero() {
            item.ExpectedDeliveryDate = order.OrderDate.AddDate(0, 0, 5)
            updateItems = true
        }
        switch item.OrderStatus {
        case "Shipped":
            if item.ShippedDate.IsZero() {
                item.ShippedDate = order.OrderDate.AddDate(0, 0, 1)
                updateItems = true
            }
        case "OutForDelivery":
            if item.OutForDeliveryDate.IsZero() {
                item.OutForDeliveryDate = order.OrderDate.AddDate(0, 0, 3)
                updateItems = true
            }
        case "Delivered":
            if item.DeliveryDate.IsZero() {
                item.DeliveryDate = order.OrderDate.AddDate(0, 0, 5)
                updateItems = true
            }
        }
        log.Printf("Order %s, Item %d: Status=%s, DeliveryDate=%s", order.OrderUID, item.ID, item.OrderStatus, item.DeliveryDate.Format(time.RFC3339))
    }
    if updateItems {
        if err := config.DB.Save(&order.OrderItem).Error; err != nil {
            log.Printf("Failed to save OrderItem dates for order %s: %v", order.OrderUID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to process order items"})
            return
        }
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
    var shippedDate, outForDeliveryDate, deliveryDate time.Time
    activeItems := 0
    hasProcessing := false
    hasShipped := false
    hasOutForDelivery := false
    allDelivered := true
    allCancelled := true

    if len(order.OrderItem) > 0 {
        for _, item := range order.OrderItem {
            if item.OrderStatus == "Cancelled" || item.OrderStatus == "Failed" {
                allDelivered = false
                continue
            }
            activeItems++
            allCancelled = false

            switch item.OrderStatus {
            case "Processing":
                hasProcessing = true
                allDelivered = false
            case "Shipped":
                hasShipped = true
                allDelivered = false
                if !item.ShippedDate.IsZero() && (shippedDate.IsZero() || item.ShippedDate.After(shippedDate)) {
                    shippedDate = item.ShippedDate
                }
            case "OutForDelivery":
                hasOutForDelivery = true
                allDelivered = false
                if !item.OutForDeliveryDate.IsZero() && (outForDeliveryDate.IsZero() || item.OutForDeliveryDate.After(outForDeliveryDate)) {
                    outForDeliveryDate = item.OutForDeliveryDate
                }
            case "Delivered":
                if !item.DeliveryDate.IsZero() && (deliveryDate.IsZero() || item.DeliveryDate.After(deliveryDate)) {
                    deliveryDate = item.DeliveryDate
                }
            case "Refunded":
                allDelivered = false
            default:
                log.Printf("Order %s: Invalid OrderItem status %s for item %d", order.OrderUID, item.OrderStatus, item.ID)
                allDelivered = false
            }
        }

        log.Printf("Order %s: hasProcessing=%v, hasShipped=%v, hasOutForDelivery=%v, allDelivered=%v, activeItems=%d", order.OrderUID, hasProcessing, hasShipped, hasOutForDelivery, allDelivered, activeItems)

        if activeItems == 0 {
            if allCancelled {
                overallStatus = "Cancelled"
            } else {
                overallStatus = "Failed"
            }
        } else {
            switch order.PaymentDetails.PaymentStatus {
            case "Failed":
                overallStatus = "Failed"
            case "Pending":
                if order.PaymentDetails.PaymentMethod == "razorpay" {
                    allPendingOrFailed := true
                    for _, item := range order.OrderItem {
                        if item.OrderStatus != "Pending" && item.OrderStatus != "Failed" && item.OrderStatus != "Cancelled" {
                            allPendingOrFailed = false
                            break
                        }
                    }
                    if allPendingOrFailed {
                        overallStatus = "Failed"
                    } else {
                        overallStatus = "Processing"
                    }
                } else if order.PaymentDetails.PaymentMethod == "cod" {
                    if allCancelled {
                        overallStatus = "Cancelled"
                    } else if allDelivered && activeItems > 0 {
                        overallStatus = "Delivered"
                    } else if hasOutForDelivery {
                        overallStatus = "OutForDelivery"
                    } else if hasShipped {
                        overallStatus = "Shipped"
                    } else if hasProcessing {
                        overallStatus = "Processing"
                    } else {
                        overallStatus = "Pending"
                    }
                } else {
                    overallStatus = "Pending"
                }
            case "Completed":
                if allCancelled {
                    overallStatus = "Cancelled"
                } else if allDelivered && activeItems > 0 {
                    overallStatus = "Delivered"
                } else if hasOutForDelivery {
                    overallStatus = "OutForDelivery"
                } else if hasShipped {
                    overallStatus = "Shipped"
                } else if hasProcessing {
                    overallStatus = "Processing"
                } else {
                    overallStatus = "Pending"
                }
            default:
                log.Printf("Order %s: Invalid PaymentStatus %s", order.OrderUID, order.PaymentDetails.PaymentStatus)
                overallStatus = "Pending"
            }
        }
        log.Printf("Order %s: OverallStatus=%s, ActiveItems=%d", order.OrderUID, overallStatus, activeItems)
    } else {
        overallStatus = "Pending"
        log.Printf("Order %s has no items, setting status to Pending", order.OrderUID)
    }

    if order.PaymentDetails.ID == 0 {
        order.PaymentDetails = models.PaymentDetails{
            PaymentMethod: "Unknown",
            PaymentStatus: "Pending",
            PaymentDate:   order.OrderDate,
        }
        if err := config.DB.Save(&order.PaymentDetails).Error; err != nil {
            log.Printf("Failed to save PaymentDetails for order %s: %v", order.OrderUID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to process payment details"})
            return
        }
    }

    if (order.PaymentDetails.PaymentStatus == "Completed" || order.PaymentDetails.PaymentStatus == "Failed") && order.PaymentDetails.PaymentDate.IsZero() {
        order.PaymentDetails.PaymentDate = order.OrderDate
        if err := config.DB.Save(&order.PaymentDetails).Error; err != nil {
            log.Printf("Failed to save PaymentDate for order %s: %v", order.OrderUID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to process payment details"})
            return
        }
    }

    if order.PaymentDetails.TransactionID == "" {
        order.PaymentDetails.TransactionID = "N/A"
    }

    returnRequests := make(map[uint]models.ReturnRequest)
    var requests []models.ReturnRequest
    orderItemIDs := getOrderItemIDs(order.OrderItem)
    if len(orderItemIDs) > 0 {
        if err := config.DB.Where("order_item_id IN ?", orderItemIDs).Find(&requests).Error; err != nil {
            log.Printf("Failed to load return requests for order %s: %v", order.OrderUID, err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to load return requests"})
            return
        }
    }
    for _, req := range requests {
        returnRequests[req.OrderItemID] = req
        log.Printf("Order %s: ReturnRequest for OrderItemID=%d, Status=%s", order.OrderUID, req.OrderItemID, req.Status)
    }

    type OrderItemWithDetails struct {
        OrderItem        models.OrderItem
        IsReturnEligible bool
    }
    orderItemsWithDetails := make([]OrderItemWithDetails, 0, len(order.OrderItem))
    for _, item := range order.OrderItem {
        isReturnEligible := false
        if item.OrderStatus == "Delivered" && !item.DeliveryDate.IsZero() {
            daysSinceDelivery := int(time.Now().Sub(item.DeliveryDate).Hours() / 24)
            hasReturnRequest := returnRequests[item.ID].ID != 0 && returnRequests[item.ID].Status != "cancelled"
            isReturnEligible = daysSinceDelivery <= 7 && !hasReturnRequest
            log.Printf("Order %s, Item %d: Delivered, DaysSinceDelivery=%d, HasReturnRequest=%v, IsReturnEligible=%v",
                order.OrderUID, item.ID, daysSinceDelivery, hasReturnRequest, isReturnEligible)
        }
        orderItemsWithDetails = append(orderItemsWithDetails, OrderItemWithDetails{
            OrderItem:        item,
            IsReturnEligible: isReturnEligible,
        })
    }

    data := gin.H{
        "Order":              order,
        "OrderUID":          order.OrderUID,
        "ShippingAddress":   order.ShippingAddress,
        "PaymentDetails":    order.PaymentDetails,
        "PaymentMethod":     order.PaymentDetails.PaymentMethod,
        "TotalAmount":       order.TotalAmount,
        "OverallStatus":     overallStatus,
        "PaymentStatus":     order.PaymentDetails.PaymentStatus,
        "ShippedDate":       shippedDate,
        "OutForDeliveryDate": outForDeliveryDate,
        "DeliveryDate":      deliveryDate,
        "ExpectedDelivery":  expectedDelivery.Format("2006-01-02"),
        "Coupon":            order.Coupon,
        "OrderItems":        orderItemsWithDetails,
        "ReturnRequests":    returnRequests,
        "InitialDiscount":   order.TotalDiscount,
        "CouponDiscount":    order.CouponDiscount,
        "CSRFToken":         c.GetString("csrf_token"),
        "Now":               time.Now(),
    }

    c.HTML(http.StatusOK, "User_Order_Details.html", data)
}


func DownloadInvoice(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		log.Println("User not authenticated in DownloadInvoice")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}
	userIDUint, ok := userID.(uint)
	if !ok {
		log.Println("Invalid user ID type in DownloadInvoice")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
		return
	}

	orderID := html.EscapeString(c.Query("order_id"))
	if orderID == "" {
		log.Println("Order ID not provided in DownloadInvoice")
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Order ID not provided"})
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("ShippingAddress").
		Preload("PaymentDetails").
		Preload("OrderItem").
		Preload("OrderItem.Product").
		Preload("OrderItem.Product.Images").
		Preload("Coupon").
		Preload("User").
		Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
		First(&order).Error; err != nil {
		log.Printf("Order %s not found for user %d: %v", orderID, userIDUint, err)
		c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Order not found or does not belong to user"})
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)

	pdf.Cell(40, 10, "VogueLuxe Invoice")
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Order #%s", order.OrderUID))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Date: %s", order.OrderDate.Format("02 Jan 2006")))
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Customer Details")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("%s %s", order.User.FirstName, order.User.LastName))
	pdf.Ln(6)
	if order.ShippingAddress.Email != nil {
		pdf.Cell(40, 10, *order.ShippingAddress.Email)
		pdf.Ln(6)
	}
	pdf.Cell(40, 10, order.ShippingAddress.PhoneNumber)
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Shipping Address")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("%s %s", order.ShippingAddress.FirstName, order.ShippingAddress.LastName))
	pdf.Ln(6)
	pdf.Cell(40, 10, order.ShippingAddress.AddressLine)
	pdf.Ln(6)
	pdf.Cell(40, 10, fmt.Sprintf("%s, %s - %s", order.ShippingAddress.City, order.ShippingAddress.State, order.ShippingAddress.Postcode))
	pdf.Ln(6)
	pdf.Cell(40, 10, order.ShippingAddress.Country)
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Order Items")
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(80, 10, "Product", "1", 0, "", false, 0, "")
	pdf.CellFormat(20, 10, "Qty", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Unit Price ", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Tax ", "1", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, "Total ", "1", 1, "", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, item := range order.OrderItem {
		pdf.CellFormat(80, 10, fmt.Sprintf("%s (%s)", item.ProductName, item.Size), "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 10, fmt.Sprintf("%d", item.Quantity), "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", item.ProductSellPrice), "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", item.Tax), "1", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", item.Total), "1", 1, "", false, 0, "")
	}

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Order Summary")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(160, 10, "Subtotal", "", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.SubTotal), "", 1, "", false, 0, "")
	pdf.CellFormat(160, 10, "Discount", "", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.TotalDiscount), "", 1, "", false, 0, "")
	if order.Coupon.ID != 0 {
		pdf.CellFormat(160, 10, fmt.Sprintf("Coupon (%s)", order.Coupon.CouponCode), "", 0, "", false, 0, "")
		pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.CouponDiscount), "", 1, "", false, 0, "")
	}
	pdf.CellFormat(160, 10, "Shipping", "", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.ShippingCharge), "", 1, "", false, 0, "")
	pdf.CellFormat(160, 10, "Tax", "", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.Tax), "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(160, 10, "Total", "", 0, "", false, 0, "")
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.TotalAmount), "", 1, "", false, 0, "")

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, "Payment Details")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Method: %s", order.PaymentDetails.PaymentMethod))
	pdf.Ln(6)
	pdf.Cell(40, 10, fmt.Sprintf("Transaction ID: %s", order.PaymentDetails.TransactionID))
	pdf.Ln(6)
	pdf.Cell(40, 10, fmt.Sprintf("Status: %s", order.PaymentDetails.PaymentStatus))
	pdf.Ln(6)
	pdf.Cell(40, 10, fmt.Sprintf("Date: %s", order.PaymentDetails.PaymentDate.Format("02 Jan 2006")))

	filename := fmt.Sprintf("invoice_%s.pdf", order.OrderUID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/pdf")
	if err := pdf.Output(c.Writer); err != nil {
		log.Printf("Failed to generate invoice PDF for order %s: %v", order.OrderUID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate invoice"})
		return
	}
}


func getOrderItemIDs(items []models.OrderItem) []uint {
    ids := make([]uint, 0, len(items))
    for _, item := range items {
        if item.ID != 0 {
            ids = append(ids, item.ID)
        }
    }
    log.Printf("OrderItem IDs: %v", ids)
    return ids
}

func RequestReturn(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        log.Println("User not authenticated in RequestReturn")
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        log.Println("Invalid user ID type in RequestReturn")
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID type"})
        return
    }

    orderID := c.PostForm("order_id")
    itemIDStr := c.PostForm("item_id")
    reason := c.PostForm("reason")
    refundMethod := c.PostForm("refund_method")
    amountStr := c.PostForm("amount")

    if orderID == "" || itemIDStr == "" || reason == "" || refundMethod == "" || amountStr == "" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Missing required fields"})
        return
    }

    itemID, err := strconv.ParseUint(itemIDStr, 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid item ID format"})
        return
    }

    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil || amount <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid amount"})
        return
    }

    var orderItem models.OrderItem
    if err := config.DB.
        Preload("Product").
        Where("id = ? AND order_id IN (SELECT id FROM orders WHERE order_uid = ? AND user_id = ?)", itemID, orderID, userIDUint).
        First(&orderItem).Error; err != nil {
        log.Printf("Order item %d not found for order %s and user %d: %v", itemID, orderID, userIDUint, err)
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order item not found"})
        return
    }

    if orderItem.OrderStatus != "Delivered" {
        c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Only delivered items can be returned"})
        return
    }

    if amount != orderItem.Total {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Requested amount does not match the item total"})
        return
    }

    var existingRequest models.ReturnRequest
    if err := config.DB.Where("order_item_id = ? AND status NOT IN ('cancelled')", orderItem.ID).First(&existingRequest).Error; err == nil {
        c.JSON(http.StatusConflict, gin.H{"status": "error", "message": "A return request is already pending or approved for this item"})
        return
    }

    returnRequest := models.ReturnRequest{
        RequestUID:       generateUniqueID(),
        OrderItemID:      orderItem.ID,
        ProductID:        orderItem.ProductID,
        ProductVariantID: orderItem.ProductVariantID,
        UserID:           userIDUint,
        Reason:           reason,
        Status:           "pending",
    }

    if err := config.DB.Create(&returnRequest).Error; err != nil {
        log.Printf("Failed to create return request for order item %d: %v", orderItem.ID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create return request"})
        return
    }

    log.Printf("Return request %s created successfully for order item %d", returnRequest.RequestUID, orderItem.ID)
    c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Return request submitted"})
}

func generateUniqueID() string {
    rand.Seed(time.Now().UnixNano()) 
    return "RET" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", rand.Intn(1000000))
}

func CancelOrder(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Invalid user ID type"})
        return
    }

    orderID := c.PostForm("order_id")
    reason := c.PostForm("reason")
    if orderID == "" || reason == "" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Missing order ID or reason"})
        return
    }

    var order models.Order
    if err := config.DB.
        Preload("OrderItem").
        Preload("OrderItem.Product").
        Preload("OrderItem.Product.Variants").
        Preload("PaymentDetails").
        Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
        First(&order).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order not found"})
        return
    }

    overallStatus := determineOverallStatus(order.OrderItem)
    if overallStatus != "Processing" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Order cannot be cancelled at this stage"})
        return
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction"})
        return
    }

    now := time.Now()
    refundAmount := 0.0
    for i := range order.OrderItem {
        if order.OrderItem[i].OrderStatus == "Processing" {
            order.OrderItem[i].OrderStatus = "Cancelled"
            order.OrderItem[i].Reason = reason
            order.OrderItem[i].CancelDate = now
            refundAmount += order.OrderItem[i].Total 

            for _, variant := range order.OrderItem[i].Product.Variants {
                if variant.ID == order.OrderItem[i].ProductVariantID {
                    variant.StockCount += order.OrderItem[i].Quantity
                    if err := tx.Save(&variant).Error; err != nil {
                        tx.Rollback()
                        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update stock"})
                        return
                    }
                    break
                }
            }

            if err := tx.Save(&order.OrderItem[i]).Error; err != nil {
                tx.Rollback()
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update order item"})
                return
            }
        }
    }

    if order.CouponID != nil {
        var coupon models.Coupon
        if err := tx.Where("id = ?", *order.CouponID).First(&coupon).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to fetch coupon"})
            return
        }
        if coupon.UsedCount > 0 {
            coupon.UsedCount--
            if err := tx.Save(&coupon).Error; err != nil {
                tx.Rollback()
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update coupon usage"})
                return
            }
        }
    }

    if order.PaymentDetails.PaymentStatus == "Completed" && refundAmount > 0 {
        var wallet models.Wallet
        if err := tx.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
            wallet = models.Wallet{
                UserID:  userIDUint,
                Balance: 0.0,
            }
            if err := tx.Create(&wallet).Error; err != nil {
                tx.Rollback()
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create wallet"})
                return
            }
        }

        wallet.Balance += refundAmount
        if err := tx.Save(&wallet).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update wallet balance"})
            return
        }

        transaction := models.WalletTransaction{
            WalletID:          wallet.ID,
            OrderID:           &order.ID,
            TransactionUID:    "TXN" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", rand.Intn(1000000)),
            TransactionAmount: refundAmount,
            TransactionType:   "Credit",
            TransactionStatus: "Completed",
            TransactionDate:   now,
            Description:       fmt.Sprintf("Refund for order cancellation #%s", order.OrderUID),
        }
        if err := tx.Create(&transaction).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create wallet transaction"})
            return
        }
    }

    if err := tx.Commit().Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Order cancelled successfully"})
}

func determineOverallStatus(items []models.OrderItem) string {
	if len(items) == 0 {
		return "Pending"
	}
	allCancelled := true
	allDelivered := true
	anyOutForDelivery := false
	anyShipped := false
	for _, item := range items {
		switch item.OrderStatus {
		case "Cancelled":
			allDelivered = false
		case "Delivered":
			allCancelled = false
		case "OutForDelivery":
			allCancelled = false
			allDelivered = false
			anyOutForDelivery = true
		case "Shipped":
			allCancelled = false
			allDelivered = false
			anyShipped = true
		case "Processing":
			allCancelled = false
			allDelivered = false
		}
	}
	if allCancelled {
		return "Cancelled"
	}
	if allDelivered {
		return "Delivered"
	}
	if anyOutForDelivery {
		return "OutForDelivery"
	}
	if anyShipped {
		return "Shipped"
	}
	return "Processing"
}

func CancelOrderItem(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "User not authenticated"})
        return
    }
    userIDUint, ok := userID.(uint)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Invalid user ID type"})
        return
    }

    orderID := c.PostForm("order_id")
    itemIDStr := c.PostForm("item_id")
    reason := c.PostForm("reason")
    if orderID == "" || itemIDStr == "" || reason == "" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Missing order ID, item ID, or reason"})
        return
    }

    itemID, err := strconv.ParseUint(itemIDStr, 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid item ID"})
        return
    }

    var order models.Order
    if err := config.DB.
        Preload("OrderItem").
        Preload("OrderItem.Product").
        Preload("OrderItem.Product.Variants").
        Preload("PaymentDetails"). 
        Where("order_uid = ? AND user_id = ?", orderID, userIDUint).
        First(&order).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Order not found"})
        return
    }

    var targetItem *models.OrderItem
    for i := range order.OrderItem {
        if order.OrderItem[i].ID == uint(itemID) {
            targetItem = &order.OrderItem[i]
            break
        }
    }
    if targetItem == nil {
        c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Item not found in order"})
        return
    }

    if targetItem.OrderStatus != "Processing" {
        c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Item cannot be cancelled at this stage"})
        return
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to start transaction"})
        return
    }

    now := time.Now()
    targetItem.OrderStatus = "Cancelled"
    targetItem.Reason = reason
    targetItem.CancelDate = now

    for _, variant := range targetItem.Product.Variants {
        if variant.ID == targetItem.ProductVariantID {
            variant.StockCount += targetItem.Quantity
            if err := tx.Save(&variant).Error; err != nil {
                tx.Rollback()
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update stock"})
                return
            }
            break
        }
    }

    if err := tx.Save(targetItem).Error; err != nil {
        tx.Rollback()
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update order item"})
        return
    }

    if order.PaymentDetails.PaymentStatus == "Completed" {
        var wallet models.Wallet
        if err := tx.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
            wallet = models.Wallet{
                UserID:  userIDUint,
                Balance: 0.0,
            }
            if err := tx.Create(&wallet).Error; err != nil {
                tx.Rollback()
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create wallet"})
                return
            }
        }

        refundAmount := targetItem.Total
        wallet.Balance += refundAmount
        if err := tx.Save(&wallet).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update wallet balance"})
            return
        }

        transaction := models.WalletTransaction{
            WalletID:          wallet.ID,
            OrderID:           &order.ID,
            OrderItemID:       &targetItem.ID,
            TransactionUID:    "TXN" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", rand.Intn(1000000)),
            TransactionAmount: refundAmount,
            TransactionType:   "Credit",
            TransactionStatus: "Completed",
            TransactionDate:   now,
            Description:       fmt.Sprintf("Refund for item cancellation #%s (Item ID: %d)", order.OrderUID, targetItem.ID),
        }
        if err := tx.Create(&transaction).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to create wallet transaction"})
            return
        }
    }

    allCancelled := true
    for _, item := range order.OrderItem {
        if item.OrderStatus != "Cancelled" {
            allCancelled = false
            break
        }
    }

    if allCancelled && order.CouponID != nil {
        var coupon models.Coupon
        if err := tx.Where("id = ?", *order.CouponID).First(&coupon).Error; err != nil {
            tx.Rollback()
            c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to fetch coupon"})
            return
        }
        if coupon.UsedCount > 0 {
            coupon.UsedCount--
            if err := tx.Save(&coupon).Error; err != nil {
                tx.Rollback()
                c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to update coupon usage"})
                return
            }
        }
    }

    if err := tx.Commit().Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to commit transaction"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Item cancelled successfully"})
}
