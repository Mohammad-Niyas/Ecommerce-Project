package controllers

import (
	"context"
	"ecommerce/config"
	"ecommerce/middleware"
	"ecommerce/models"
	"ecommerce/utils"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/razorpay/razorpay-go"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)


var tempUserStore = make(map[string]models.User)

func UserSignupPage(c *gin.Context) {
	if middleware.ValidateUserToken(c) {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	c.HTML(http.StatusOK, "User_SignUp.html", gin.H{})
}

func UserSignup(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	firstName := c.PostForm("first_name")
	lastName := c.PostForm("last_name")
	confirmPassword := c.PostForm("confirm_password")

	
	if email == "" || password == "" || firstName == "" || lastName == "" {
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{
			"error": "All fields are required",
		})
		return
	}

	if password != confirmPassword {
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{"error": "Passwords do not match"})
		return
	}

	if len(password) < 8 {
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{"error": "Password must be at least 8 characters"})
		return
	}

	
	var existingUser models.User
	if err := config.DB.Where("email = ?", email).First(&existingUser).Error; err == nil {
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{
			"error": "Email already exists",
		})
		return
	} else if err != gorm.ErrRecordNotFound {
		fmt.Println("Failed to check existing user:", err)
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to check if email exists. Please try again later.",
		})
		return
	}

	
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("Failed to hash password:", err)
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to hash password. Please try again later.",
		})
		return
	}

	
	user := models.User{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Password:  string(hashedPassword),
		UserDetails: models.UserDetails{
			IsActive: true,
		},
	}

	
	tempUserStore[email] = user

	
	otp, err := utils.GenerateOTP(4)
	if err != nil {
		fmt.Println("Failed to generate OTP:", err)
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to generate OTP. Please try again later.",
		})
		return
	}

	
	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		fmt.Println("Failed to store OTP:", err)
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to store OTP. Please try again later.",
		})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		fmt.Println("Failed to send OTP email:", err)
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to send OTP. Please try again later.",
		})
		return
	}

	
	c.Redirect(http.StatusSeeOther, "/verify-otp?email="+email)
}

func GoogleLogin(c *gin.Context) {
	url := config.GoogleOauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.HTML(http.StatusBadRequest, "User_Login.html", gin.H{"error": "Authorization code not provided"})
		return
	}

	token, err := config.GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		fmt.Println("Failed to exchange token:", err)
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to authenticate with Google"})
		return
	}

	client := config.GoogleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		fmt.Println("Failed to get user info:", err)
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to retrieve user information"})
		return
	}
	defer resp.Body.Close()

	var userInfo struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"given_name"`
		LastName  string `json:"family_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		fmt.Println("Failed to decode user info:", err)
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to decode user information"})
		return
	}

	
	var user models.User
	if err := config.DB.Where("google_id = ?", userInfo.ID).Or("email = ?", userInfo.Email).First(&user).Error; err == gorm.ErrRecordNotFound {
		
		user = models.User{
			FirstName:   userInfo.FirstName,
			LastName:    userInfo.LastName,
			Email:       userInfo.Email,
			GoogleID:    userInfo.ID,
			UserDetails: models.UserDetails{IsActive: true},
		}
		if err := config.DB.Create(&user).Error; err != nil {
			fmt.Println("Failed to create user:", err)
			c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to create user account"})
			return
		}
	} else if err != nil {
		fmt.Println("Failed to check user:", err)
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Database error"})
		return
	}

	jwtToken, err := middleware.GenerateToken(user.ID, user.Email, "User")
	if err != nil {
		fmt.Println("Failed to generate token:", err)
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Could not generate token"})
		return
	}

	
	c.SetCookie("jwtTokensUser", jwtToken, 3600, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/")
}

func VerifyOTPPage(c *gin.Context) {
	if middleware.ValidateUserToken(c) {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	email := c.Query("email")
	if email == "" {
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{"error": "Email parameter missing"})
		return
	}
	c.HTML(http.StatusOK, "User_Otp_Verify.html", gin.H{"email": email})
}

func VerifyOTP(c *gin.Context) {
	email := c.PostForm("email")
	otp := c.PostForm("otp")

	
	if email == "" || otp == "" {
		c.HTML(http.StatusBadRequest, "User_Otp_Verify.html", gin.H{"error": "Email and OTP are required", "email": email})
		return
	}

	if len(otp) != 4 || !utils.IsNumeric(otp) {
		c.HTML(http.StatusBadRequest, "User_Otp_Verify.html", gin.H{"error": "Please enter a valid 4-digit OTP", "email": email})
		return
	}

	
	var otpRecord models.Otp
	if err := config.DB.Order("created_at DESC").Where("email = ? AND expire_time > ?", email, time.Now()).First(&otpRecord).Error; err != nil {
		c.HTML(http.StatusUnauthorized, "User_Otp_Verify.html", gin.H{"error": "Invalid or expired OTP", "email": email})
		return
	}

	
	user, exists := tempUserStore[email]
	if !exists {
		c.HTML(http.StatusBadRequest, "User_Otp_Verify.html", gin.H{"error": "User data not found. Please sign up again.", "email": email})
		return
	}
	if otp != otpRecord.Otp {
		c.HTML(http.StatusUnauthorized, "User_Otp_Verify.html", gin.H{"error": "Invalid or expired OTP", "email": email})
		return
	}

	
	if err := config.DB.Create(&user).Error; err != nil {
		fmt.Println("Failed to create user:", err)
		c.HTML(http.StatusInternalServerError, "User_Otp_Verify.html", gin.H{
			"error": "Failed to create user. Please try again later.",
			"email": email,
		})
		return
	}

	
	if err := config.DB.Delete(&otpRecord).Error; err != nil {
		fmt.Println("Failed to delete OTP:", err)
		c.HTML(http.StatusInternalServerError, "User_Otp_Verify.html", gin.H{"error": "Failed to delete OTP. Please try again later.", "email": email})
		return
	}

	
	delete(tempUserStore, email)

	
	c.Redirect(http.StatusSeeOther, "/login")
}

func ResendOTP(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email parameter missing"})
		return
	}

	
	if _, exists := tempUserStore[email]; !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User data not found. Please sign up again."})
		return
	}

	
	otp, err := utils.GenerateOTP(4)
	if err != nil {
		fmt.Println("Failed to generate OTP:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	
	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		fmt.Println("Failed to store OTP:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store OTP"})
		return
	}

	
	if err := utils.SendOTPEmail(email, otp); err != nil {
		fmt.Println("Failed to send OTP email:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP resent successfully"})
}

func ForgotPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "User_Forgot_Password.html", gin.H{})
}

func SubmitForgotPassword(c *gin.Context) {
	email := c.PostForm("email")
	if email == "" {
		c.HTML(http.StatusBadRequest, "User_Forgot_Password.html", gin.H{"error": "Email is required", "email": email})
		return
	}

	var user models.User
	if err := config.DB.Preload("UserDetails", "is_active = ?", true).Where("email = ?", email).First(&user).Error; err != nil {
		log.Printf("Error checking user: %v", err)
		c.HTML(http.StatusOK, "User_Forgot_Password.html", gin.H{"error": "No active account found with this email", "email": email})
		return
	}

	if user.UserDetails.ID == 0 || !user.UserDetails.IsActive {
		c.HTML(http.StatusOK, "User_Forgot_Password.html", gin.H{"error": "No active account found with this email", "email": email})
		return
	}

	otp, err := utils.GenerateOTP(4)
	if err != nil {
		log.Printf("Error generating OTP: %v", err)
		c.HTML(http.StatusInternalServerError, "User_Forgot_Password.html", gin.H{"error": "Failed to process your request", "email": email})
		return
	}

	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		log.Printf("Error storing OTP: %v", err)
		c.HTML(http.StatusInternalServerError, "User_Forgot_Password.html", gin.H{"error": "Failed to process your request", "email": email})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		log.Printf("Error sending OTP email: %v", err)
		c.HTML(http.StatusInternalServerError, "User_Forgot_Password.html", gin.H{"error": "Failed to send OTP. Please try again", "email": email})
		return
	}

	c.Redirect(http.StatusSeeOther, "/forgot/verify-otp?email="+email)
}

func ForgotVerifyOtpPage(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.HTML(http.StatusBadRequest, "User_Forgot_Otp_Verify.html", gin.H{"error": "Email is required"})
		return
	}

	c.HTML(http.StatusOK, "User_Forgot_Otp_Verify.html", gin.H{"email": email})
}

func ForgotVerifyOtp(c *gin.Context) {
	email := c.PostForm("email")
	otp := c.PostForm("otp")
	if email == "" || otp == "" {
		c.HTML(http.StatusBadRequest, "User_Forgot_Otp_Verify.html", gin.H{"email": email, "error": "Invalid OTP or email"})
		return
	}

	var otpRecord models.Otp
	if err := config.DB.Order("created_at DESC").Where("email = ? AND otp = ? AND expire_time > ?", email, otp, time.Now()).First(&otpRecord).Error; err != nil {
		log.Printf("Error verifying OTP: %v", err)
		c.HTML(http.StatusOK, "User_Forgot_Otp_Verify.html", gin.H{"email": email, "error": "Invalid or expired OTP"})
		return
	}

	if err := config.DB.Delete(&otpRecord).Error; err != nil {
		log.Printf("Error deleting OTP record: %v", err)
	}

	c.Redirect(http.StatusSeeOther, "/reset-password?email="+email)
}

func ForgotResendOtp(c *gin.Context) {
	email := c.PostForm("email") 
	if email == "" {
		email = c.Query("email") 
	}
	if email == "" {
		log.Printf("ResendOTP: Email parameter missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email parameter is required"})
		return
	}

	var user models.User
	if err := config.DB.Preload("UserDetails", "is_active = ?", true).Where("email = ?", email).First(&user).Error; err != nil {
		log.Printf("Error checking user for resend OTP: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active account found with this email"})
		return
	}

	if user.UserDetails.ID == 0 || !user.UserDetails.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active account found with this email"})
		return
	}

	otp, err := utils.GenerateOTP(4)
	if err != nil {
		log.Printf("Error generating OTP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		log.Printf("Error storing OTP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store OTP"})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		log.Printf("Error sending OTP email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP resent successfully. Check your email."})
}

func ResetPassword(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.HTML(http.StatusBadRequest, "User_Reset_Password.html", gin.H{"error": "Email is required"})
		return
	}

	c.HTML(http.StatusOK, "User_Reset_Password.html", gin.H{"email": email})
}

func SubmitResetPassword(c *gin.Context) {
	var user models.User
	email := c.PostForm("email")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if email == "" || newPassword == "" || confirmPassword == "" {
		c.HTML(http.StatusBadRequest, "User_Reset_Password.html", gin.H{"email": email, "error": "All fields are required"})
		return
	}

	if newPassword != confirmPassword {
		c.HTML(http.StatusOK, "User_Reset_Password.html", gin.H{"email": email, "error": "Passwords do not match"})
		return
	}

	if err := config.DB.Preload("UserDetails", "is_active = ?", true).Where("email = ?", email).First(&user).Error; err != nil {
		log.Printf("Error checking user: %v", err)
		c.HTML(http.StatusOK, "reset-password.html", gin.H{"email": email, "error": "No active account found"})
		return
	}

	if user.UserDetails.ID == 0 || !user.UserDetails.IsActive {
		c.HTML(http.StatusOK, "reset-password.html", gin.H{"email": email, "error": "No active account found"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		c.HTML(http.StatusInternalServerError, "User_Reset_Password.html", gin.H{"email": email, "error": "Failed to reset password"})
		return
	}

	if err := config.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		log.Printf("Error updating password: %v", err)
		c.HTML(http.StatusInternalServerError, "User_Reset_Password.html", gin.H{"email": email, "error": "Failed to reset password"})
		return
	}

	c.Redirect(http.StatusSeeOther, "/login")
}

func UserLoginPage(c *gin.Context) {
	if middleware.ValidateUserToken(c) {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	fmt.Println("Rendering login page")
	c.HTML(http.StatusOK, "User_Login.html", gin.H{})
}

func UserLogin(c *gin.Context) {
    email := c.PostForm("email")
    password := c.PostForm("password")

    if email == "" || password == "" {
        c.HTML(http.StatusBadRequest, "User_Login.html", gin.H{"error": "Email and password are required"})
        return
    }

    var user models.User
    if err := config.DB.Preload("UserDetails").Where("email = ?", email).First(&user).Error; err != nil {
        fmt.Println("User not found:", err)
        c.HTML(http.StatusUnauthorized, "User_Login.html", gin.H{"error": "User not found"})
        return
    }

    if !user.UserDetails.IsActive {
        fmt.Println("Login attempt by blocked user:", email)
        c.HTML(http.StatusUnauthorized, "User_Login.html", gin.H{"error": "Your account is blocked. Please contact support."})
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
        fmt.Println("Invalid password for user:", email)
        c.HTML(http.StatusUnauthorized, "User_Login.html", gin.H{"error": "Invalid password"})
        return
    }

    token, err := middleware.GenerateToken(user.ID, user.Email, "User")
    if err != nil {
        fmt.Println("Failed to generate token:", err)
        c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Could not generate token. Please try again later."})
        return
    }

    fmt.Println("Generated Token:", token)
    c.SetCookie("jwtTokensUser", token, 24*3600, "/", "localhost", false, false) 
    fmt.Println("Cookie set for user:", email)
    c.Redirect(http.StatusSeeOther, "/")
}

func UserHomePage(c *gin.Context) {
    tokenString, err := c.Cookie("jwtTokensUser")
    isLoggedIn := false
    var userID uint

    if err == nil && tokenString != "" {
        claims := &middleware.Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return middleware.GetJwtKey(), nil
        })
        if err == nil && token.Valid && claims.Role == "User" {
            isLoggedIn = true
            userID = claims.UserId
            var user models.User
            if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
                c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
                isLoggedIn = false
            } else if !user.UserDetails.IsActive {
                c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
                isLoggedIn = false
            }
        }
    }

    var categories []models.Category
    if err := config.DB.Model(&models.Category{}).
        Select("id, category_name").
        Where("list = ?", true).
        Find(&categories).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
        return
    }

    var products []models.Product
    if err := config.DB.Model(&models.Product{}).
        Preload("Images").
        Preload("Variants").
        Joins("JOIN categories ON categories.id = products.category_id").
        Where("products.is_active = ?", true).
        Where("categories.list = ?", true).
        Limit(4).
        Find(&products).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
        return
    }

    type ProductWithOffer struct {
        Product      models.Product
        SellingPrice float64
        Discount     float64
    }
    var productsWithOffer []ProductWithOffer
    for _, product := range products {
        if len(product.Variants) > 0 {
            sellingPrice, discount := CalculateSellingPrice(product.Variants[0], config.DB)
            log.Printf("Product ID: %d, Name: %s, SellingPrice: %.2f, Discount: %.2f", product.ID, product.ProductName, sellingPrice, discount)
            productsWithOffer = append(productsWithOffer, ProductWithOffer{
                Product:      product,
                SellingPrice: sellingPrice,
                Discount:     discount,
            })
        } else {
            log.Printf("Product ID: %d, Name: %s has no variants", product.ID, product.ProductName)
            productsWithOffer = append(productsWithOffer, ProductWithOffer{
                Product:      product,
                SellingPrice: 0,
                Discount:     0,
            })
        }
    }

    log.Printf("Rendering User_Home.html with %d products", len(productsWithOffer))
    c.HTML(http.StatusOK, "User_Home.html", gin.H{
        "products":   productsWithOffer,
        "categories": categories,
        "isLoggedIn": isLoggedIn,
    })
}

type CategoryWithSelection struct {
	ID           uint
	CategoryName string
	Selected     bool
}

type BrandWithSelection struct {
	Name     string
	Selected bool
}

func ProductListing(c *gin.Context) {
    tokenString, err := c.Cookie("jwtTokensUser")
    isLoggedIn := false
    var userID uint

    if err == nil && tokenString != "" {
        claims := &middleware.Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return middleware.GetJwtKey(), nil
        })
        if err == nil && token.Valid && claims.Role == "User" {
            isLoggedIn = true
            userID = claims.UserId
            var user models.User
            if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
                c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
                isLoggedIn = false
            } else if !user.UserDetails.IsActive {
                c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
                isLoggedIn = false
            }
        }
    }

    var categories []models.Category
    if err := config.DB.Model(&models.Category{}).
        Select("id, category_name").
        Where("list = ?", true).
        Find(&categories).Error; err != nil {
        log.Printf("Failed to fetch categories: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
        return
    }

    var brands []string
    if err := config.DB.Model(&models.Product{}).
        Distinct("brand").
        Where("is_active = ?", true).
        Pluck("brand", &brands).Error; err != nil {
        log.Printf("Failed to fetch brands: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch brands"})
        return
    }

    search := c.Query("search")
    categoryIDs := c.QueryArray("category")
    priceMinStr := c.DefaultQuery("price_min", "0")
    priceMaxStr := c.DefaultQuery("price_max", "10000")
    brandNames := c.QueryArray("brand")

    priceMin, _ := strconv.ParseFloat(priceMinStr, 64)
    priceMax, _ := strconv.ParseFloat(priceMaxStr, 64)

    var products []models.Product
    query := config.DB.Model(&models.Product{}).
        Preload("Images").
        Preload("Variants").
		Preload("Category.CategoryOffers"). 
        Preload("ProductOffers").           
        Joins("JOIN categories ON categories.id = products.category_id").
        Where("products.is_active = ?", true).
        Where("categories.list = ?", true)

    if search != "" {
        query = query.Where("products.product_name LIKE ?", "%"+search+"%")
    }
    if len(categoryIDs) > 0 {
        query = query.Where("products.category_id IN ?", categoryIDs)
    }
    if len(brandNames) > 0 {
        query = query.Where("products.brand IN ?", brandNames)
    }

    var totalProducts int64
    if err := query.Count(&totalProducts).Error; err != nil {
        log.Printf("Failed to count products: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
        return
    }

    if err := query.Find(&products).Error; err != nil {
        log.Printf("Failed to fetch products: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
        return
    }

    type ProductWithOffer struct {
        Product      models.Product
        SellingPrice float64
        Discount     float64
    }
    var productsWithOffers []ProductWithOffer
    for _, product := range products {
        if len(product.Variants) > 0 {
            sellingPrice, discount := CalculateSellingPrice(product.Variants[0], config.DB)
            if (priceMin == 0 && priceMax == 10000) || (sellingPrice >= priceMin && sellingPrice <= priceMax) {
                productsWithOffers = append(productsWithOffers, ProductWithOffer{
                    Product:      product,
                    SellingPrice: sellingPrice,
                    Discount:     discount,
                })
            }
        }
    }

    var categoriesWithSelection []CategoryWithSelection
    for _, cat := range categories {
        selected := false
        for _, selectedID := range categoryIDs {
            if strconv.Itoa(int(cat.ID)) == selectedID {
                selected = true
                break
            }
        }
        categoriesWithSelection = append(categoriesWithSelection, CategoryWithSelection{
            ID:           cat.ID,
            CategoryName: cat.CategoryName,
            Selected:     selected,
        })
    }

    var brandsWithSelection []BrandWithSelection
    for _, brand := range brands {
        selected := false
        for _, selectedBrand := range brandNames {
            if brand == selectedBrand {
                selected = true
                break
            }
        }
        brandsWithSelection = append(brandsWithSelection, BrandWithSelection{
            Name:     brand,
            Selected: selected,
        })
    }

    log.Printf("Rendering User_Product.html with %d products, search: %s, categories: %v, priceMin: %f, priceMax: %f, brands: %v",
        len(productsWithOffers), search, categoryIDs, priceMin, priceMax, brandNames)

    c.HTML(http.StatusOK, "User_Product.html", gin.H{
        "products":      productsWithOffers,
        "totalProducts": totalProducts,
        "categories":    categoriesWithSelection,
        "brands":        brandsWithSelection,
        "isLoggedIn":    isLoggedIn,
        "priceMin":      priceMinStr,
        "priceMax":      priceMaxStr,
        "search":        search,
    })
}

func ProductDetail(c *gin.Context) {
    tokenString, err := c.Cookie("jwtTokensUser")
    isLoggedIn := false
    var userID uint

    if err == nil && tokenString != "" {
        claims := &middleware.Claims{}
        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return middleware.GetJwtKey(), nil
        })
        if err == nil && token.Valid && claims.Role == "User" {
            isLoggedIn = true
            userID = claims.UserId
            var user models.User
            if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
                c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
                isLoggedIn = false
            } else if !user.UserDetails.IsActive {
                c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
                isLoggedIn = false
            }
        }
    }

    productID := c.Param("id")
    variantID := c.Query("variant")

    var product models.Product
    if err := config.DB.Model(&models.Product{}).
        Preload("Images").
        Preload("Variants", func(db *gorm.DB) *gorm.DB {
            return db.Where("is_active = ?", true)
        }).
        Preload("Category.CategoryOffers").
        Preload("ProductOffers").
        Joins("JOIN categories ON categories.id = products.category_id").
        Where("products.id = ? AND products.is_active = ? AND categories.list = ?", productID, true, true).
        First(&product).Error; err != nil {
        c.HTML(http.StatusNotFound, "error.html", gin.H{
            "message": "Product not found",
        })
        return
    }

    selectedVariantID := ""
    if variantID != "" {
        for _, variant := range product.Variants {
            if fmt.Sprintf("%d", variant.ID) == variantID && variant.IsActive {
                selectedVariantID = variantID
                break
            }
        }
    }
    if selectedVariantID == "" && len(product.Variants) > 0 {
        selectedVariantID = fmt.Sprintf("%d", product.Variants[0].ID)
    }

    type VariantWithOffer struct {
        Variant      models.ProductVariant
        SellingPrice float64
        Discount     float64
    }
    var variantsWithOffers []VariantWithOffer
    for _, variant := range product.Variants {
        sellingPrice, discount := CalculateSellingPrice(variant, config.DB)
        variantsWithOffers = append(variantsWithOffers, VariantWithOffer{
            Variant:      variant,
            SellingPrice: sellingPrice,
            Discount:     discount,
        })
    }

    var categories []models.Category
    if err := config.DB.Model(&models.Category{}).
        Select("id, category_name").
        Where("list = ?", true).
        Find(&categories).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to fetch categories: " + err.Error(),
        })
        return
    }

    c.HTML(http.StatusOK, "User_Product_Detail.html", gin.H{
		"product":           product,
		"variants":          variantsWithOffers,
		"categories":        categories,
		"isLoggedIn":        isLoggedIn,
		"selectedVariantId": selectedVariantID,
		"jwtToken":          tokenString,
	})
}

func CommonData(user *models.User) gin.H {
	var categories []models.Category
	config.DB.Model(&models.Category{}).
		Select("id, category_name").
		Where("list = ?", true).
		Find(&categories)

	return gin.H{
		"username":   user.FirstName + " " + user.LastName,
		"userImage":  user.UserDetails.Image,
		"categories": categories,
	}
}

func UserProfilePage(c *gin.Context){
	userID, _ := c.Get("userid")
	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	data := CommonData(&user)
	data["firstName"] = user.FirstName
	data["lastName"] = user.LastName
	data["email"] = user.Email
	data["phoneNumber"] = user.UserDetails.PhoneNumber

	c.HTML(http.StatusOK,"User_Profile_Personal_Information.html",data)
}

func UpdateUserProfile(c *gin.Context){
	userID,_:=c.Get("userid")
	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user,userID).Error;err!=nil{
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to parse form"})
		return
	}

	firstName := c.PostForm("firstName")
	lastName := c.PostForm("lastName")
	phoneNumber := c.PostForm("phoneNumber")

	if firstName == "" || lastName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "First name and last name are required"})
		return
	}

	user.FirstName = firstName
	user.LastName = lastName
	if phoneNumber != "" {
		user.UserDetails.PhoneNumber = phoneNumber
	}

	cloudService, err := utils.NewCloudinaryService()
if err != nil{
	log.Printf("Failed to initialize Cloudinary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Cloudinary"})
		return
}

	file, _, err := c.Request.FormFile("profile-upload")
	if err == nil {
		defer file.Close()
		newImageURL, err := cloudService.UploadImage(file, "user_profiles")
		if err != nil {
			fmt.Println("Failed to upload image to Cloudinary:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
			return
		}
		if user.UserDetails.Image != "" {
			publicID := utils.ExtractPublicIDFromURL(user.UserDetails.Image)
			if publicID != "" {
				if err := cloudService.DeleteImage(publicID); err != nil {
					fmt.Println("Failed to delete old image from Cloudinary:", err)
				}
			}
		}
		user.UserDetails.Image = newImageURL
	} else if err != http.ErrMissingFile {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing image upload"})
		return
	}

	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save changes"})
		return
	}
	if err := config.DB.Save(&user.UserDetails).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func UserAddressPage(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        log.Println("UserID not found in context")
        c.Redirect(http.StatusFound, "/login")
        return
    }

    var user models.User
    if err := config.DB.Preload("Addresses").Preload("UserDetails").First(&user, userID).Error; err != nil {
        log.Printf("User not found: %v", err)
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    data := CommonData(&user)
    data["addresses"] = user.Addresses

    c.HTML(http.StatusOK, "User_Profile_ManageAddress.html", data)
}

type AddressInput struct {
    FirstName      string  `form:"first_name" binding:"required"`
    LastName       string  `form:"last_name" binding:"required"`
    Email          string  `form:"email"`
    PhoneNumber    string  `form:"phone_number" binding:"required"`
    Country        string  `form:"country" binding:"required"`
    Postcode       string  `form:"postcode" binding:"required"`
    State          string  `form:"state" binding:"required"`
    City           string  `form:"city" binding:"required"`
    AddressLine    string  `form:"address" binding:"required"`
    Landmark       string  `form:"landmark"`
    AlternatePhone string  `form:"alternate_phone"`
    DefaultAddress bool    `form:"default_address"`
}

func UserAddAddress(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }

    log.Printf("Raw form data: %+v", c.Request.PostForm)

    var input AddressInput
    if err := c.ShouldBind(&input); err != nil {
        formData := c.Request.PostForm
        input.FirstName = formData.Get("first_name")
        input.LastName = formData.Get("last_name")
        input.Email = formData.Get("email")
        input.PhoneNumber = formData.Get("phone_number")
        input.Country = formData.Get("country")
        input.Postcode = formData.Get("postcode")
        input.State = formData.Get("state")
        input.City = formData.Get("city")
        input.AddressLine = formData.Get("address")
        input.Landmark = formData.Get("landmark")
        input.AlternatePhone = formData.Get("alternate_phone")
        defaultAddr := formData.Get("default_address")
        input.DefaultAddress = defaultAddr == "on"

        if input.FirstName == "" || input.LastName == "" || input.PhoneNumber == "" || input.Country == "" ||
            input.Postcode == "" || input.State == "" || input.City == "" || input.AddressLine == "" {
            log.Printf("Invalid address data: %v, Form data: %+v", err, formData)
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address data"})
            return
        }
    }

    log.Printf("Received input data: %+v", input)

    address := models.Address{
        FirstName:   input.FirstName,
        LastName:    input.LastName,
        Country:     input.Country,
        State:       input.State,
        City:        input.City,
        AddressLine: input.AddressLine,
        UserID:      userID.(uint),
        DefaultAddress: input.DefaultAddress,
    }

    if input.Email != "" {
        email := input.Email
        address.Email = &email
    }

    address.PhoneNumber = input.PhoneNumber

    address.Postcode = input.Postcode

    if input.Landmark != "" {
        address.Landmark = input.Landmark
    }

    if input.AlternatePhone != "" {
        alternatePhone := input.AlternatePhone
        address.AlternatePhone = &alternatePhone
    }

    if address.DefaultAddress {
        if err := config.DB.Model(&models.Address{}).Where("user_id = ? AND default_address = ?", address.UserID, true).Update("default_address", false).Error; err != nil {
            log.Printf("Failed to update default addresses: %v", err)
        }
    }

    if err := config.DB.Create(&address).Error; err != nil {
        log.Printf("Failed to add address: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add address"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Address added successfully", "address": address})
}

func UserUpdateAddress(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    id, err := strconv.Atoi(c.PostForm("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
        return
    }
    var address models.Address
    if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Address not found or not authorized"})
        return
    }

    log.Printf("Raw form data: %+v", c.Request.PostForm)

    var input AddressInput
    if err := c.ShouldBind(&input); err != nil {
        formData := c.Request.PostForm
        input.FirstName = formData.Get("first_name")
        input.LastName = formData.Get("last_name")
        input.Email = formData.Get("email")
        input.PhoneNumber = formData.Get("phone_number")
        input.Country = formData.Get("country")
        input.Postcode = formData.Get("postcode")
        input.State = formData.Get("state")
        input.City = formData.Get("city")
        input.AddressLine = formData.Get("address")
        input.Landmark = formData.Get("landmark")
        input.AlternatePhone = formData.Get("alternate_phone")
        defaultAddr := formData.Get("default_address")
        input.DefaultAddress = defaultAddr == "on"

        if input.FirstName == "" || input.LastName == "" || input.PhoneNumber == "" || input.Country == "" ||
            input.Postcode == "" || input.State == "" || input.City == "" || input.AddressLine == "" {
            log.Printf("Invalid address data: %v, Form data: %+v", err, formData)
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address data"})
            return
        }
    }

    address.FirstName = input.FirstName
    address.LastName = input.LastName
    if input.Email != "" {
        email := input.Email
        address.Email = &email
    }
    address.PhoneNumber = input.PhoneNumber
    address.Country = input.Country
    address.Postcode = input.Postcode
    address.State = input.State
    address.City = input.City
    address.AddressLine = input.AddressLine
    if input.Landmark != "" {
        address.Landmark = input.Landmark
    }
    if input.AlternatePhone != "" {
        alternatePhone := input.AlternatePhone
        address.AlternatePhone = &alternatePhone
    }
    address.DefaultAddress = input.DefaultAddress

    if address.DefaultAddress {
        config.DB.Model(&models.Address{}).Where("user_id = ? AND default_address = ? AND id != ?", userID, true, id).Update("default_address", false)
    }

    if err := config.DB.Save(&address).Error; err != nil {
        log.Printf("Failed to update address: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Address updated successfully", "address": address})
}

func UserDeleteAddress(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, err := strconv.Atoi(c.PostForm("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
		return
	}
	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found or not authorized"})
		return
	}
	if err := config.DB.Delete(&address).Error; err != nil {
		log.Printf("Failed to delete address: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete address"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Address deleted successfully"})
}

func UserSetDefaultAddress(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    id, err := strconv.Atoi(c.PostForm("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
        return
    }
    config.DB.Model(&models.Address{}).Where("user_id = ? AND default_address = ?", userID, true).Update("default_address", false)
    var address models.Address
    if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Address not found or not authorized"})
        return
    }
    address.DefaultAddress = true
    if err := config.DB.Save(&address).Error; err != nil {
        log.Printf("Failed to set default address: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default address"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Default address set successfully"})
}

func UserWalletPage(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        log.Println("UserID not found in context - redirecting to login")
        c.Redirect(http.StatusFound, "/login")
        return
    }

    userIDUint, ok := userID.(uint)
    if !ok {
        log.Println("Invalid user ID type")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
        return
    }

    var user models.User
    if err := config.DB.Preload("UserDetails").First(&user, userIDUint).Error; err != nil {
        log.Printf("User not found: %v", err)
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    var wallet models.Wallet
    if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
        wallet = models.Wallet{UserID: userIDUint, Balance: 0.00}
        if err := config.DB.Create(&wallet).Error; err != nil {
            log.Printf("Failed to create wallet: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet"})
            return
        }
    }

    var transactions []models.WalletTransaction
    if err := config.DB.Where("wallet_id = ?", wallet.ID).Order("transaction_date DESC").Find(&transactions).Error; err != nil {
        log.Printf("Failed to fetch transactions: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
        return
    }

    var categories []models.Category
    if err := config.DB.Where("list = ?", true).Find(&categories).Error; err != nil {
        log.Printf("Failed to fetch categories: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
        return
    }

    data := gin.H{
        "username":     user.FirstName + " " + user.LastName,
        "userImage":    user.UserDetails.Image,
        "userEmail":    user.Email,
        "wallet":       wallet,
        "transactions": transactions,
        "categories":   categories,
        "isLoggedIn":   true,
    }

    c.HTML(http.StatusOK, "User_Profile_Wallet.html", data)
}

func AddWalletAmount(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		log.Println("User not authenticated - redirecting to login")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		log.Println("Invalid user ID type")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
		return
	}

	amountStr := c.PostForm("amount")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		log.Printf("Invalid amount provided: %s", amountStr)
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid amount"})
		return
	}

	var wallet models.Wallet
	if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
		wallet = models.Wallet{UserID: userIDUint, Balance: 0.00}
		if err := config.DB.Create(&wallet).Error; err != nil {
			log.Printf("Failed to create wallet: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create wallet"})
			return
		}
	}

	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))

	shortUUID := uuid.New().String()[:8]
	receipt := "wallet_" + shortUUID

	data := map[string]interface{}{
		"amount":   int(amount * 100),
		"currency": "INR",
		"receipt":  receipt,
	}
	body, err := client.Order.Create(data, nil)
	if err != nil {
		log.Printf("Failed to create Razorpay order: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create Razorpay order"})
		return
	}

	razorpayOrderID, ok := body["id"].(string)
	if !ok {
		log.Println("Invalid Razorpay order ID")
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid Razorpay order response"})
		return
	}

	transaction := models.WalletTransaction{
		WalletID:          wallet.ID,
		TransactionUID:    "TXN-" + uuid.New().String(),
		TransactionAmount: amount,
		TransactionType:   "credit",
		TransactionStatus: "Pending",
		TransactionDate:   time.Now(),
	}
	if err := config.DB.Create(&transaction).Error; err != nil {
		log.Printf("Failed to create transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "Success",
		"razorpay_order_id": razorpayOrderID,
		"amount":            int(amount * 100), 
		"currency":          "INR",
		"key":               os.Getenv("RAZORPAY_KEY_ID"),
		"transaction_id":    transaction.TransactionUID,
		"name":              c.GetString("username"),
		"email":             "user@example.com", 
	})
}

func ConfirmWalletPayment(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        log.Println("User not authenticated - redirecting to login")
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }

    userIDUint, ok := userID.(uint)
    if !ok {
        log.Println("Invalid user ID type")
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
        return
    }

    transactionID := c.PostForm("transaction_id")
    razorpayPaymentID := c.PostForm("razorpay_payment_id")
    razorpayOrderID := c.PostForm("razorpay_order_id")
    razorpaySignature := c.PostForm("razorpay_signature")
    clientError := c.PostForm("error")

    if transactionID == "" {
        log.Println("Missing transaction_id")
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Missing transaction ID"})
        return
    }

    var wallet models.Wallet
    if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
        log.Printf("Wallet not found: %v", err)
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Wallet not found"})
        return
    }

    var transaction models.WalletTransaction
    if err := config.DB.Where("transaction_uid = ? AND wallet_id = ?", transactionID, wallet.ID).First(&transaction).Error; err != nil {
        log.Printf("Transaction not found: %v", err)
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Transaction not found"})
        return
    }

    redirectURL := "/wallet?status=failed"
    paymentStatus := "Failed"
    errorMessage := clientError

    if clientError == "" { 
        client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET"))
        payload := razorpayOrderID + "|" + razorpayPaymentID
        if utils.HmacSha256(payload, os.Getenv("RAZORPAY_KEY_SECRET")) != razorpaySignature {
            errorMessage = "Invalid Razorpay signature"
            log.Printf("Signature verification failed: %s", errorMessage)
        } else {
            payment, err := client.Payment.Fetch(razorpayPaymentID, nil, nil)
            if err != nil {
                errorMessage = "Failed to verify payment: " + err.Error()
                log.Printf("Payment fetch error: %v", err)
            } else if payment["status"] != "captured" {
                errorMessage = "Payment not captured"
                log.Printf("Payment not captured: %v", payment)
            } else {
                paymentStatus = "Completed"
                redirectURL = "/wallet?status=success"
                errorMessage = ""
            }
        }
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        log.Printf("Failed to start transaction: %v", tx.Error)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to start transaction"})
        return
    }

    transaction.TransactionStatus = paymentStatus
    if paymentStatus == "Completed" {
        if err := tx.Model(&wallet).Update("Balance", gorm.Expr("balance + ?", transaction.TransactionAmount)).Error; err != nil {
            tx.Rollback()
            log.Printf("Failed to update wallet balance: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update wallet balance"})
            return
        }
    }
    if err := tx.Save(&transaction).Error; err != nil {
        tx.Rollback()
        log.Printf("Failed to update transaction: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update transaction"})
        return
    }

    if err := tx.Commit().Error; err != nil {
        log.Printf("Failed to commit transaction: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to commit transaction"})
        return
    }

    if errorMessage != "" {
        redirectURL += "&error=" + url.QueryEscape(errorMessage)
    }

    log.Printf("Payment confirmation completed: status=%s, redirectURL=%s", paymentStatus, redirectURL)
    c.JSON(http.StatusOK, gin.H{"redirectURL": redirectURL})
}

func UserSettingPage(c *gin.Context){
	userID, exists := c.Get("userid")
    if !exists {
        log.Println("UserID not found in context - redirecting to login")
        c.Redirect(http.StatusFound, "/login")
        return
    }

    userIDUint, ok := userID.(uint)
    if !ok {
        log.Println("Invalid user ID type")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
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
        log.Printf("Failed to fetch categories: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
        return
    }

    data := gin.H{
        "username":     user.FirstName + " " + user.LastName,
        "userImage":    user.UserDetails.Image,
        "userEmail":    user.Email,
        "categories":   categories,
        "isLoggedIn":   true,
    }

	c.HTML(http.StatusOK,"User_Profile_Setting.html",data)
}

func UserLogout(c *gin.Context) {
	c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/login")
}