package controllers

import (
	"context"
	"ecommerce/config"
	"ecommerce/middleware"
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"ecommerce/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/razorpay/razorpay-go"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)


var tempUserStore = make(map[string]models.User)

func UserSignupPage(c *gin.Context) {
	if middleware.ValidateUserToken(c) {
		logger.Log.Info("User already authenticated, redirecting to home")
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	logger.Log.Info("Rendering User_SignUp.html")
	c.HTML(http.StatusOK, "User_SignUp.html", gin.H{})
}

func UserSignup(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	firstName := c.PostForm("first_name")
	lastName := c.PostForm("last_name")
	confirmPassword := c.PostForm("confirm_password")

	logger.Log.Info("User signup attempt", zap.String("email", email))

	if email == "" || password == "" || firstName == "" || lastName == "" {
		logger.Log.Warn("Missing required fields in signup")
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{
			"error": "All fields are required",
		})
		return
	}

	if password != confirmPassword {
		logger.Log.Warn("Passwords do not match in signup", zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{"error": "Passwords do not match"})
		return
	}

	if len(password) < 8 {
		logger.Log.Warn("Password too short in signup", zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{"error": "Password must be at least 8 characters"})
		return
	}

	var existingUser models.User
	if err := config.DB.Where("email = ?", email).First(&existingUser).Error; err == nil {
		logger.Log.Warn("Email already exists in signup", zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{
			"error": "Email already exists",
		})
		return
	} else if err != gorm.ErrRecordNotFound {
		logger.Log.Error("Failed to check existing user in signup",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to check if email exists. Please try again later.",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("Failed to hash password in signup",
			zap.String("email", email),
			zap.Error(err))
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
	logger.Log.Info("Stored user in temp store", zap.String("email", email))

	otp, err := utils.GenerateOTP(4)
	if err != nil {
		logger.Log.Error("Failed to generate OTP in signup",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to generate OTP. Please try again later.",
		})
		return
	}

	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		logger.Log.Error("Failed to store OTP in signup",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to store OTP. Please try again later.",
		})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		logger.Log.Error("Failed to send OTP email in signup",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_SignUp.html", gin.H{
			"error": "Failed to send OTP. Please try again later.",
		})
		return
	}

	logger.Log.Info("OTP sent successfully, redirecting to verify OTP",
		zap.String("email", email),
		zap.String("otp", otp))
	c.Redirect(http.StatusSeeOther, "/verify-otp?email="+email)
}

func GoogleLogin(c *gin.Context) {
	url := config.GoogleOauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	logger.Log.Info("Initiating Google OAuth login", zap.String("redirect_url", url))
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		logger.Log.Warn("Authorization code not provided in Google callback")
		c.HTML(http.StatusBadRequest, "User_Login.html", gin.H{"error": "Authorization code not provided"})
		return
	}

	logger.Log.Info("Processing Google OAuth callback", zap.String("code", code))

	token, err := config.GoogleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Log.Error("Failed to exchange token in Google callback",
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to authenticate with Google"})
		return
	}

	client := config.GoogleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		logger.Log.Error("Failed to get user info from Google",
			zap.Error(err))
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
		logger.Log.Error("Failed to decode user info from Google",
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to decode user information"})
		return
	}

	logger.Log.Info("Retrieved Google user info",
		zap.String("email", userInfo.Email),
		zap.String("google_id", userInfo.ID))

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
			logger.Log.Error("Failed to create user in Google callback",
				zap.String("email", userInfo.Email),
				zap.Error(err))
			c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Failed to create user account"})
			return
		}
		logger.Log.Info("Created new user from Google login",
			zap.String("email", userInfo.Email),
			zap.Uint("user_id", user.ID))
	} else if err != nil {
		logger.Log.Error("Failed to check user in Google callback",
			zap.String("email", userInfo.Email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Database error"})
		return
	}

	jwtToken, err := middleware.GenerateToken(user.ID, user.Email, "User")
	if err != nil {
		logger.Log.Error("Failed to generate token in Google callback",
			zap.String("email", userInfo.Email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Could not generate token"})
		return
	}

	logger.Log.Info("Generated JWT token for Google login",
		zap.String("email", userInfo.Email),
		zap.String("token", jwtToken))
	c.SetCookie("jwtTokensUser", jwtToken, 3600, "/", "", false, true)
	c.Redirect(http.StatusSeeOther, "/")
}

func VerifyOTPPage(c *gin.Context) {
	if middleware.ValidateUserToken(c) {
		logger.Log.Info("User already authenticated, redirecting to home")
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	email := c.Query("email")
	if email == "" {
		logger.Log.Warn("Email parameter missing in VerifyOTPPage")
		c.HTML(http.StatusBadRequest, "User_SignUp.html", gin.H{"error": "Email parameter missing"})
		return
	}
	logger.Log.Info("Rendering OTP verification page",
		zap.String("email", email))
	c.HTML(http.StatusOK, "User_Otp_Verify.html", gin.H{"email": email})
}

func VerifyOTP(c *gin.Context) {
	email := c.PostForm("email")
	otp := c.PostForm("otp")

	logger.Log.Info("OTP verification attempt",
		zap.String("email", email),
		zap.String("otp", otp))

	if email == "" || otp == "" {
		logger.Log.Warn("Missing email or OTP in verification",
			zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_Otp_Verify.html", gin.H{"error": "Email and OTP are required", "email": email})
		return
	}

	if len(otp) != 4 || !utils.IsNumeric(otp) {
		logger.Log.Warn("Invalid OTP format",
			zap.String("email", email),
			zap.String("otp", otp))
		c.HTML(http.StatusBadRequest, "User_Otp_Verify.html", gin.H{"error": "Please enter a valid 4-digit OTP", "email": email})
		return
	}

	var otpRecord models.Otp
	if err := config.DB.Order("created_at DESC").Where("email = ? AND expire_time > ?", email, time.Now()).First(&otpRecord).Error; err != nil {
		logger.Log.Error("Invalid or expired OTP",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusUnauthorized, "User_Otp_Verify.html", gin.H{"error": "Invalid or expired OTP", "email": email})
		return
	}

	user, exists := tempUserStore[email]
	if !exists {
		logger.Log.Warn("User data not found in temp store",
			zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_Otp_Verify.html", gin.H{"error": "User data not found. Please sign up again.", "email": email})
		return
	}
	if otp != otpRecord.Otp {
		logger.Log.Warn("Invalid OTP provided",
			zap.String("email", email),
			zap.String("otp", otp))
		c.HTML(http.StatusUnauthorized, "User_Otp_Verify.html", gin.H{"error": "Invalid or expired OTP", "email": email})
		return
	}

	if err := config.DB.Create(&user).Error; err != nil {
		logger.Log.Error("Failed to create user in VerifyOTP",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Otp_Verify.html", gin.H{
			"error": "Failed to create user. Please try again later.",
			"email": email,
		})
		return
	}

	if err := config.DB.Delete(&otpRecord).Error; err != nil {
		logger.Log.Error("Failed to delete OTP record",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Otp_Verify.html", gin.H{
			"error": "Failed to delete OTP. Please try again later.",
			"email": email,
		})
		return
	}

	delete(tempUserStore, email)
	logger.Log.Info("User created and OTP verified successfully",
		zap.String("email", email),
		zap.Uint("user_id", user.ID))

	c.Redirect(http.StatusSeeOther, "/login")
}

func ResendOTP(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		logger.Log.Warn("Email parameter missing in ResendOTP")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email parameter missing"})
		return
	}

	if _, exists := tempUserStore[email]; !exists {
		logger.Log.Warn("User data not found in temp store for OTP resend",
			zap.String("email", email))
		c.JSON(http.StatusBadRequest, gin.H{"error": "User data not found. Please sign up again."})
		return
	}

	otp, err := utils.GenerateOTP(4)
	if err != nil {
		logger.Log.Error("Failed to generate OTP in ResendOTP",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		logger.Log.Error("Failed to store OTP in ResendOTP",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store OTP"})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		logger.Log.Error("Failed to send OTP email in ResendOTP",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP"})
		return
	}

	logger.Log.Info("OTP resent successfully",
		zap.String("email", email),
		zap.String("otp", otp))
	c.JSON(http.StatusOK, gin.H{"message": "OTP resent successfully"})
}

func ForgotPassword(c *gin.Context) {
	logger.Log.Info("Rendering Forgot Password page")
	c.HTML(http.StatusOK, "User_Forgot_Password.html", gin.H{})
}

func SubmitForgotPassword(c *gin.Context) {
	email := c.PostForm("email")
	if email == "" {
		logger.Log.Warn("Email is required in SubmitForgotPassword")
		c.HTML(http.StatusBadRequest, "User_Forgot_Password.html", gin.H{"error": "Email is required", "email": email})
		return
	}

	logger.Log.Info("Forgot password request", zap.String("email", email))

	var user models.User
	if err := config.DB.Preload("UserDetails", "is_active = ?", true).Where("email = ?", email).First(&user).Error; err != nil {
		logger.Log.Error("No active account found for forgot password",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusOK, "User_Forgot_Password.html", gin.H{"error": "No active account found with this email", "email": email})
		return
	}

	if user.UserDetails.ID == 0 || !user.UserDetails.IsActive {
		logger.Log.Warn("No active account found for forgot password",
			zap.String("email", email))
		c.HTML(http.StatusOK, "User_Forgot_Password.html", gin.H{"error": "No active account found with this email", "email": email})
		return
	}

	otp, err := utils.GenerateOTP(4)
	if err != nil {
		logger.Log.Error("Failed to generate OTP for forgot password",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Forgot_Password.html", gin.H{"error": "Failed to process your request", "email": email})
		return
	}

	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		logger.Log.Error("Failed to store OTP for forgot password",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Forgot_Password.html", gin.H{"error": "Failed to process your request", "email": email})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		logger.Log.Error("Failed to send OTP email for forgot password",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Forgot_Password.html", gin.H{"error": "Failed to send OTP. Please try again", "email": email})
		return
	}

	logger.Log.Info("OTP sent for forgot password",
		zap.String("email", email),
		zap.String("otp", otp))
	c.Redirect(http.StatusSeeOther, "/forgot/verify-otp?email="+email)
}

func ForgotVerifyOtpPage(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		logger.Log.Warn("Email is required in ForgotVerifyOtpPage")
		c.HTML(http.StatusBadRequest, "User_Forgot_Otp_Verify.html", gin.H{"error": "Email is required"})
		return
	}

	logger.Log.Info("Rendering Forgot OTP verification page",
		zap.String("email", email))
	c.HTML(http.StatusOK, "User_Forgot_Otp_Verify.html", gin.H{"email": email})
}

func ForgotVerifyOtp(c *gin.Context) {
	email := c.PostForm("email")
	otp := c.PostForm("otp")
	if email == "" || otp == "" {
		logger.Log.Warn("Invalid OTP or email in ForgotVerifyOtp",
			zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_Forgot_Otp_Verify.html", gin.H{"email": email, "error": "Invalid OTP or email"})
		return
	}

	logger.Log.Info("Forgot OTP verification attempt",
		zap.String("email", email),
		zap.String("otp", otp))

	var otpRecord models.Otp
	if err := config.DB.Order("created_at DESC").Where("email = ? AND otp = ? AND expire_time > ?", email, otp, time.Now()).First(&otpRecord).Error; err != nil {
		logger.Log.Error("Invalid or expired OTP in ForgotVerifyOtp",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusOK, "User_Forgot_Otp_Verify.html", gin.H{"email": email, "error": "Invalid or expired OTP"})
		return
	}

	if err := config.DB.Delete(&otpRecord).Error; err != nil {
		logger.Log.Error("Failed to delete OTP record in ForgotVerifyOtp",
			zap.String("email", email),
			zap.Error(err))
	}

	logger.Log.Info("OTP verified successfully for password reset",
		zap.String("email", email))
	c.Redirect(http.StatusSeeOther, "/reset-password?email="+email)
}

func ForgotResendOtp(c *gin.Context) {
	email := c.PostForm("email")
	if email == "" {
		email = c.Query("email")
	}
	if email == "" {
		logger.Log.Warn("Email parameter missing in ForgotResendOtp")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email parameter is required"})
		return
	}

	logger.Log.Info("Forgot OTP resend attempt",
		zap.String("email", email))

	var user models.User
	if err := config.DB.Preload("UserDetails", "is_active = ?", true).Where("email = ?", email).First(&user).Error; err != nil {
		logger.Log.Error("No active account found for OTP resend",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active account found with this email"})
		return
	}

	if user.UserDetails.ID == 0 || !user.UserDetails.IsActive {
		logger.Log.Warn("No active account found for OTP resend",
			zap.String("email", email))
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active account found with this email"})
		return
	}

	otp, err := utils.GenerateOTP(4)
	if err != nil {
		logger.Log.Error("Failed to generate OTP in ForgotResendOtp",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OTP"})
		return
	}

	if err := utils.StoreOTP(email, otp, 5*time.Minute); err != nil {
		logger.Log.Error("Failed to store OTP in ForgotResendOtp",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store OTP"})
		return
	}

	if err := utils.SendOTPEmail(email, otp); err != nil {
		logger.Log.Error("Failed to send OTP email in ForgotResendOtp",
			zap.String("email", email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP email"})
		return
	}

	logger.Log.Info("OTP resent successfully for forgot password",
		zap.String("email", email),
		zap.String("otp", otp))
	c.JSON(http.StatusOK, gin.H{"message": "OTP resent successfully. Check your email."})
}

func ResetPassword(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		logger.Log.Warn("Email is required in ResetPassword")
		c.HTML(http.StatusBadRequest, "User_Reset_Password.html", gin.H{"error": "Email is required"})
		return
	}

	logger.Log.Info("Rendering Reset Password page",
		zap.String("email", email))
	c.HTML(http.StatusOK, "User_Reset_Password.html", gin.H{"email": email})
}

func SubmitResetPassword(c *gin.Context) {
	var user models.User
	email := c.PostForm("email")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	logger.Log.Info("Password reset attempt",
		zap.String("email", email))

	if email == "" || newPassword == "" || confirmPassword == "" {
		logger.Log.Warn("Missing required fields in password reset",
			zap.String("email", email))
		c.HTML(http.StatusBadRequest, "User_Reset_Password.html", gin.H{"email": email, "error": "All fields are required"})
		return
	}

	if newPassword != confirmPassword {
		logger.Log.Warn("Passwords do not match in password reset",
			zap.String("email", email))
		c.HTML(http.StatusOK, "User_Reset_Password.html", gin.H{"email": email, "error": "Passwords do not match"})
		return
	}

	if err := config.DB.Preload("UserDetails", "is_active = ?", true).Where("email = ?", email).First(&user).Error; err != nil {
		logger.Log.Error("No active account found for password reset",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusOK, "reset-password.html", gin.H{"email": email, "error": "No active account found"})
		return
	}

	if user.UserDetails.ID == 0 || !user.UserDetails.IsActive {
		logger.Log.Warn("No active account found for password reset",
			zap.String("email", email))
		c.HTML(http.StatusOK, "reset-password.html", gin.H{"email": email, "error": "No active account found"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("Failed to hash password in reset",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Reset_Password.html", gin.H{"email": email, "error": "Failed to reset password"})
		return
	}

	if err := config.DB.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		logger.Log.Error("Failed to update password in reset",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Reset_Password.html", gin.H{"email": email, "error": "Failed to reset password"})
		return
	}

	logger.Log.Info("Password reset successfully",
		zap.String("email", email),
		zap.Uint("user_id", user.ID))
	c.Redirect(http.StatusSeeOther, "/login")
}

func UserLoginPage(c *gin.Context) {
	if middleware.ValidateUserToken(c) {
		logger.Log.Info("User already authenticated, redirecting to home")
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	logger.Log.Info("Rendering User_Login.html")
	c.HTML(http.StatusOK, "User_Login.html", gin.H{})
}

func UserLogin(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	logger.Log.Info("User login attempt", zap.String("email", email))

	if email == "" || password == "" {
		logger.Log.Warn("Missing email or password in login")
		c.HTML(http.StatusBadRequest, "User_Login.html", gin.H{"error": "Email and password are required"})
		return
	}

	var user models.User
	if err := config.DB.Preload("UserDetails").Where("email = ?", email).First(&user).Error; err != nil {
		logger.Log.Error("User not found in login",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusUnauthorized, "User_Login.html", gin.H{"error": "User not found"})
		return
	}

	if !user.UserDetails.IsActive {
		logger.Log.Warn("Login attempt by blocked user",
			zap.String("email", email))
		c.HTML(http.StatusUnauthorized, "User_Login.html", gin.H{"error": "Your account is blocked. Please contact support."})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		logger.Log.Warn("Invalid password in login",
			zap.String("email", email))
		c.HTML(http.StatusUnauthorized, "User_Login.html", gin.H{"error": "Invalid password"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Email, "User")
	if err != nil {
		logger.Log.Error("Failed to generate token in login",
			zap.String("email", email),
			zap.Error(err))
		c.HTML(http.StatusInternalServerError, "User_Login.html", gin.H{"error": "Could not generate token. Please try again later."})
		return
	}

	logger.Log.Info("Generated JWT token for login",
		zap.String("email", email),
		zap.String("token", token),
		zap.Uint("user_id", user.ID))
	c.SetCookie("jwtTokensUser", token, 24*3600, "/", "", false, false)
	logger.Log.Info("Cookie set for user", zap.String("email", email))
	c.Redirect(http.StatusSeeOther, "/")
}

func UserHomePage(c *gin.Context) {
	tokenString, err := c.Cookie("jwtTokensUser")
	isLoggedIn := false
	var userID uint

	logger.Log.Info("Rendering User Home page")

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
				logger.Log.Error("User not found in UserHomePage",
					zap.Uint("user_id", userID),
					zap.Error(err))
				c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
				isLoggedIn = false
			} else if user.UserDetails.IsActive {
				logger.Log.Info("User authenticated for home page",
					zap.String("email", user.Email),
					zap.Uint("user_id", userID))
			} else {
				logger.Log.Warn("Blocked user attempted to access home page",
					zap.String("email", user.Email),
					zap.Uint("user_id", userID))
				c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
				isLoggedIn = false
			}
		} else {
			logger.Log.Warn("Invalid token in UserHomePage",
				zap.Error(err))
		}
	}

	var categories []models.Category
	if err := config.DB.Model(&models.Category{}).
		Select("id, category_name").
		Where("list = ?", true).
		Find(&categories).Error; err != nil {
		logger.Log.Error("Failed to fetch categories in UserHomePage",
			zap.Error(err))
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
		logger.Log.Error("Failed to fetch products in UserHomePage",
			zap.Error(err))
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
			logger.Log.Info("Calculated product pricing",
				zap.Uint("product_id", product.ID),
				zap.String("product_name", product.ProductName),
				zap.Float64("selling_price", sellingPrice),
				zap.Float64("discount", discount))
			productsWithOffer = append(productsWithOffer, ProductWithOffer{
				Product:      product,
				SellingPrice: sellingPrice,
				Discount:     discount,
			})
		} else {
			logger.Log.Warn("Product has no variants",
				zap.Uint("product_id", product.ID),
				zap.String("product_name", product.ProductName))
			productsWithOffer = append(productsWithOffer, ProductWithOffer{
				Product:      product,
				SellingPrice: 0,
				Discount:     0,
			})
		}
	}

	logger.Log.Info("Rendering User_Home.html",
		zap.Int("product_count", len(productsWithOffer)),
		zap.Bool("is_logged_in", isLoggedIn))
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

	logger.Log.Info("Rendering Product Listing page")

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
				logger.Log.Error("User not found in ProductListing",
					zap.Uint("user_id", userID),
					zap.Error(err))
				c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
				isLoggedIn = false
			} else if user.UserDetails.IsActive {
				logger.Log.Info("User authenticated for product listing",
					zap.String("email", user.Email),
					zap.Uint("user_id", userID))
			} else {
				logger.Log.Warn("Blocked user attempted to access product listing",
					zap.String("email", user.Email),
					zap.Uint("user_id", userID))
				c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
				isLoggedIn = false
			}
		} else {
			logger.Log.Warn("Invalid token in ProductListing",
				zap.Error(err))
		}
	}

	var categories []models.Category
	if err := config.DB.Model(&models.Category{}).
		Select("id, category_name").
		Where("list = ?", true).
		Find(&categories).Error; err != nil {
		logger.Log.Error("Failed to fetch categories in ProductListing",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	var brands []string
	if err := config.DB.Model(&models.Product{}).
		Distinct("brand").
		Where("is_active = ?", true).
		Pluck("brand", &brands).Error; err != nil {
		logger.Log.Error("Failed to fetch brands in ProductListing",
			zap.Error(err))
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

	logger.Log.Info("Product listing query",
		zap.String("search", search),
		zap.Strings("category_ids", categoryIDs),
		zap.Float64("price_min", priceMin),
		zap.Float64("price_max", priceMax),
		zap.Strings("brands", brandNames))

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
		logger.Log.Error("Failed to count products in ProductListing",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
		return
	}

	if err := query.Find(&products).Error; err != nil {
		logger.Log.Error("Failed to fetch products in ProductListing",
			zap.Error(err))
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

	logger.Log.Info("Rendering User_Product.html",
		zap.Int("product_count", len(productsWithOffers)),
		zap.Int64("total_products", totalProducts),
		zap.Bool("is_logged_in", isLoggedIn))
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

	logger.Log.Info("Rendering Product Detail page",
		zap.String("product_id", c.Param("id")),
		zap.String("variant_id", c.Query("variant")))

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
				logger.Log.Error("User not found in ProductDetail",
					zap.Uint("user_id", userID),
					zap.Error(err))
				c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
				isLoggedIn = false
			} else if user.UserDetails.IsActive {
				logger.Log.Info("User authenticated for product detail",
					zap.String("email", user.Email),
					zap.Uint("user_id", userID))
			} else {
				logger.Log.Warn("Blocked user attempted to access product detail",
					zap.String("email", user.Email),
					zap.Uint("user_id", userID))
				c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
				isLoggedIn = false
			}
		} else {
			logger.Log.Warn("Invalid token in ProductDetail",
				zap.Error(err))
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
		logger.Log.Error("Product not found in ProductDetail",
			zap.String("product_id", productID),
			zap.Error(err))
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
		logger.Log.Error("Failed to fetch categories in ProductDetail",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch categories: " + err.Error(),
		})
		return
	}

	logger.Log.Info("Rendering User_Product_Detail.html",
		zap.Uint("product_id", product.ID),
		zap.String("selected_variant_id", selectedVariantID),
		zap.Bool("is_logged_in", isLoggedIn))
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

	logger.Log.Info("Fetched common data for user",
		zap.String("email", user.Email),
		zap.Uint("user_id", user.ID))
	return gin.H{
		"username":   user.FirstName + " " + user.LastName,
		"userImage":  user.UserDetails.Image,
		"categories": categories,
	}
}

func UserProfilePage(c *gin.Context) {
	userID, _ := c.Get("userid")
	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
		logger.Log.Error("User not found in UserProfilePage",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}

	data := CommonData(&user)
	data["firstName"] = user.FirstName
	data["lastName"] = user.LastName
	data["email"] = user.Email
	data["phoneNumber"] = user.UserDetails.PhoneNumber

	logger.Log.Info("Rendering User_Profile_Personal_Information.html",
		zap.String("email", user.Email),
		zap.Uint("user_id", user.ID))
	c.HTML(http.StatusOK, "User_Profile_Personal_Information.html", data)
}

func UpdateUserProfile(c *gin.Context) {
	userID, _ := c.Get("userid")
	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user, userID).Error; err != nil {
		logger.Log.Error("User not found in UpdateUserProfile",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	logger.Log.Info("User profile update attempt",
		zap.String("email", user.Email),
		zap.Uint("user_id", user.ID))

	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		logger.Log.Warn("Unable to parse form in UpdateUserProfile",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to parse form"})
		return
	}

	firstName := c.PostForm("firstName")
	lastName := c.PostForm("lastName")
	phoneNumber := c.PostForm("phoneNumber")

	if firstName == "" || lastName == "" {
		logger.Log.Warn("Missing first name or last name in profile update",
			zap.String("email", user.Email))
		c.JSON(http.StatusBadRequest, gin.H{"error": "First name and last name are required"})
		return
	}

	user.FirstName = firstName
	user.LastName = lastName
	if phoneNumber != "" {
		user.UserDetails.PhoneNumber = phoneNumber
	}

	cloudService, err := utils.NewCloudinaryService()
	if err != nil {
		logger.Log.Error("Failed to initialize Cloudinary in UpdateUserProfile",
			zap.String("email", user.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Cloudinary"})
		return
	}

	file, _, err := c.Request.FormFile("profile-upload")
	if err == nil {
		defer file.Close()
		newImageURL, err := cloudService.UploadImage(file, "user_profiles")
		if err != nil {
			logger.Log.Error("Failed to upload image to Cloudinary",
				zap.String("email", user.Email),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
			return
		}
		if user.UserDetails.Image != "" {
			publicID := utils.ExtractPublicIDFromURL(user.UserDetails.Image)
			if publicID != "" {
				if err := cloudService.DeleteImage(publicID); err != nil {
					logger.Log.Warn("Failed to delete old image from Cloudinary",
						zap.String("email", user.Email),
						zap.String("public_id", publicID),
						zap.Error(err))
				}
			}
		}
		user.UserDetails.Image = newImageURL
		logger.Log.Info("Uploaded new profile image",
			zap.String("email", user.Email),
			zap.String("image_url", newImageURL))
	} else if err != http.ErrMissingFile {
		logger.Log.Error("Error processing image upload in UpdateUserProfile",
			zap.String("email", user.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing image upload"})
		return
	}

	if err := config.DB.Save(&user).Error; err != nil {
		logger.Log.Error("Failed to save user changes in UpdateUserProfile",
			zap.String("email", user.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save changes"})
		return
	}
	if err := config.DB.Save(&user.UserDetails).Error; err != nil {
		logger.Log.Error("Failed to save user details in UpdateUserProfile",
			zap.String("email", user.Email),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user details"})
		return
	}

	logger.Log.Info("User profile updated successfully",
		zap.String("email", user.Email),
		zap.Uint("user_id", user.ID))
	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func UserAddressPage(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("UserID not found in context in UserAddressPage")
		c.Redirect(http.StatusFound, "/login")
		return
	}

	var user models.User
	if err := config.DB.Preload("Addresses").Preload("UserDetails").First(&user, userID).Error; err != nil {
		logger.Log.Error("User not found in UserAddressPage",
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	data := CommonData(&user)
	data["addresses"] = user.Addresses

	logger.Log.Info("Rendering User_Profile_ManageAddress.html",
		zap.String("email", user.Email),
		zap.Uint("user_id", user.ID),
		zap.Int("address_count", len(user.Addresses)))
	c.HTML(http.StatusOK, "User_Profile_ManageAddress.html", data)
}

type AddressInput struct {
	FirstName      string `form:"first_name" binding:"required"`
	LastName       string `form:"last_name" binding:"required"`
	Email          string `form:"email"`
	PhoneNumber    string `form:"phone_number" binding:"required"`
	Country        string `form:"country" binding:"required"`
	Postcode       string `form:"postcode" binding:"required"`
	State          string `form:"state" binding:"required"`
	City           string `form:"city" binding:"required"`
	AddressLine    string `form:"address" binding:"required"`
	Landmark       string `form:"landmark"`
	AlternatePhone string `form:"alternate_phone"`
	DefaultAddress bool   `form:"default_address"`
}

func UserAddAddress(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("Unauthorized access in UserAddAddress")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		logger.Log.Error("Invalid user ID type in UserAddAddress",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	logger.Log.Info("User address add attempt",
		zap.Uint("user_id", userIDUint),
		zap.Any("form_data", c.Request.PostForm))

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
			logger.Log.Warn("Invalid address data in UserAddAddress",
				zap.Uint("user_id", userIDUint),
				zap.Any("form_data", formData),
				zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address data"})
			return
		}
	}

	logger.Log.Info("Received address input",
		zap.Uint("user_id", userIDUint),
		zap.Any("input", input))

	address := models.Address{
		FirstName:      input.FirstName,
		LastName:       input.LastName,
		Country:        input.Country,
		State:          input.State,
		City:           input.City,
		AddressLine:    input.AddressLine,
		UserID:         userIDUint,
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
			logger.Log.Error("Failed to update default addresses in UserAddAddress",
				zap.Uint("user_id", userIDUint),
				zap.Error(err))
		}
	}

	if err := config.DB.Create(&address).Error; err != nil {
		logger.Log.Error("Failed to add address in UserAddAddress",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add address"})
		return
	}

	logger.Log.Info("Address added successfully",
		zap.Uint("user_id", userIDUint),
		zap.Uint("address_id", address.ID))
	c.JSON(http.StatusOK, gin.H{"message": "Address added successfully", "address": address})
}

func UserUpdateAddress(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("Unauthorized access in UserUpdateAddress")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		logger.Log.Error("Invalid user ID type in UserUpdateAddress",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	id, err := strconv.Atoi(c.PostForm("id"))
	if err != nil {
		logger.Log.Warn("Invalid address ID in UserUpdateAddress",
			zap.String("id", c.PostForm("id")))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
		return
	}

	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		logger.Log.Error("Address not found or not authorized in UserUpdateAddress",
			zap.Uint("address_id", uint(id)),
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found or not authorized"})
		return
	}

	logger.Log.Info("User address update attempt",
		zap.Uint("user_id", userIDUint),
		zap.Uint("address_id", uint(id)),
		zap.Any("form_data", c.Request.PostForm))

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
			logger.Log.Warn("Invalid address data in UserUpdateAddress",
				zap.Uint("user_id", userIDUint),
				zap.Any("form_data", formData),
				zap.Error(err))
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
		logger.Log.Error("Failed to update address in UserUpdateAddress",
			zap.Uint("user_id", userIDUint),
			zap.Uint("address_id", uint(id)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
		return
	}

	logger.Log.Info("Address updated successfully",
		zap.Uint("user_id", userIDUint),
		zap.Uint("address_id", uint(id)))
	c.JSON(http.StatusOK, gin.H{"message": "Address updated successfully", "address": address})
}

func UserDeleteAddress(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("Unauthorized access in UserDeleteAddress")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		logger.Log.Error("Invalid user ID type in UserDeleteAddress",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	id, err := strconv.Atoi(c.PostForm("id"))
	if err != nil {
		logger.Log.Warn("Invalid address ID in UserDeleteAddress",
			zap.String("id", c.PostForm("id")))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
		return
	}

	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		logger.Log.Error("Address not found or not authorized in UserDeleteAddress",
			zap.Uint("address_id", uint(id)),
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found or not authorized"})
		return
	}

	if err := config.DB.Delete(&address).Error; err != nil {
		logger.Log.Error("Failed to delete address in UserDeleteAddress",
			zap.Uint("user_id", userIDUint),
			zap.Uint("address_id", uint(id)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete address"})
		return
	}

	logger.Log.Info("Address deleted successfully",
		zap.Uint("user_id", userIDUint),
		zap.Uint("address_id", uint(id)))
	c.JSON(http.StatusOK, gin.H{"message": "Address deleted successfully"})
}

func UserSetDefaultAddress(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("Unauthorized access in UserSetDefaultAddress")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		logger.Log.Error("Invalid user ID type in UserSetDefaultAddress",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	id, err := strconv.Atoi(c.PostForm("id"))
	if err != nil {
		logger.Log.Warn("Invalid address ID in UserSetDefaultAddress",
			zap.String("id", c.PostForm("id")))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address ID"})
		return
	}

	config.DB.Model(&models.Address{}).Where("user_id = ? AND default_address = ?", userID, true).Update("default_address", false)
	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&address).Error; err != nil {
		logger.Log.Error("Address not found or not authorized in UserSetDefaultAddress",
			zap.Uint("address_id", uint(id)),
			zap.Any("user_id", userID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found or not authorized"})
		return
	}

	address.DefaultAddress = true
	if err := config.DB.Save(&address).Error; err != nil {
		logger.Log.Error("Failed to set default address in UserSetDefaultAddress",
			zap.Uint("user_id", userIDUint),
			zap.Uint("address_id", uint(id)),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default address"})
		return
	}

	logger.Log.Info("Default address set successfully",
		zap.Uint("user_id", userIDUint),
		zap.Uint("address_id", uint(id)))
	c.JSON(http.StatusOK, gin.H{"message": "Default address set successfully"})
}

func UserWalletPage(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("UserID not found in context in UserWalletPage")
		c.Redirect(http.StatusFound, "/login")
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		logger.Log.Error("Invalid user ID type in UserWalletPage",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := config.DB.Preload("UserDetails").First(&user, userIDUint).Error; err != nil {
		logger.Log.Error("User not found in UserWalletPage",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var wallet models.Wallet
	if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
		wallet = models.Wallet{UserID: userIDUint, Balance: 0.00}
		if err := config.DB.Create(&wallet).Error; err != nil {
			logger.Log.Error("Failed to create wallet in UserWalletPage",
				zap.Uint("user_id", userIDUint),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet"})
			return
		}
		logger.Log.Info("Created new wallet for user",
			zap.Uint("user_id", userIDUint),
			zap.Uint("wallet_id", wallet.ID))
	}

	var transactions []models.WalletTransaction
	if err := config.DB.Where("wallet_id = ?", wallet.ID).Order("transaction_date DESC").Find(&transactions).Error; err != nil {
		logger.Log.Error("Failed to fetch transactions in UserWalletPage",
			zap.Uint("user_id", userIDUint),
			zap.Uint("wallet_id", wallet.ID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}

	var categories []models.Category
	if err := config.DB.Where("list = ?", true).Find(&categories).Error; err != nil {
		logger.Log.Error("Failed to fetch categories in UserWalletPage",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
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

	logger.Log.Info("Rendering User_Profile_Wallet.html",
		zap.String("email", user.Email),
		zap.Uint("user_id", user.ID),
		zap.Float64("wallet_balance", wallet.Balance),
		zap.Int("transaction_count", len(transactions)))
	c.HTML(http.StatusOK, "User_Profile_Wallet.html", data)
}

func AddWalletAmount(c *gin.Context) {
	userID, exists := c.Get("userid")
	if !exists {
		logger.Log.Warn("User not authenticated in AddWalletAmount")
		c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
		return
	}

	userIDUint, ok := userID.(uint)
	if !ok {
		logger.Log.Error("Invalid user ID type in AddWalletAmount",
			zap.Any("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
		return
	}

	amountStr := c.PostForm("amount")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		logger.Log.Warn("Invalid amount provided in AddWalletAmount",
			zap.Uint("user_id", userIDUint),
			zap.String("amount", amountStr))
		c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Invalid amount"})
		return
	}

	logger.Log.Info("Wallet amount add attempt",
		zap.Uint("user_id", userIDUint),
		zap.Float64("amount", amount))

	var wallet models.Wallet
	if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
		wallet = models.Wallet{UserID: userIDUint, Balance: 0.00}
		if err := config.DB.Create(&wallet).Error; err != nil {
			logger.Log.Error("Failed to create wallet in AddWalletAmount",
				zap.Uint("user_id", userIDUint),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create wallet"})
			return
		}
		logger.Log.Info("Created new wallet for user",
			zap.Uint("user_id", userIDUint),
			zap.Uint("wallet_id", wallet.ID))
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
		logger.Log.Error("Failed to create Razorpay order in AddWalletAmount",
			zap.Uint("user_id", userIDUint),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create Razorpay order"})
		return
	}

	razorpayOrderID, ok := body["id"].(string)
	if !ok {
		logger.Log.Error("Invalid Razorpay order ID in AddWalletAmount",
			zap.Uint("user_id", userIDUint))
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
		logger.Log.Error("Failed to create transaction in AddWalletAmount",
			zap.Uint("user_id", userIDUint),
			zap.Uint("wallet_id", wallet.ID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to create transaction"})
		return
	}

	logger.Log.Info("Created Razorpay order and transaction",
		zap.Uint("user_id", userIDUint),
		zap.String("razorpay_order_id", razorpayOrderID),
		zap.String("transaction_uid", transaction.TransactionUID),
		zap.Float64("amount", amount))
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
        logger.Log.Warn("User not authenticated - redirecting to login")
        c.JSON(http.StatusUnauthorized, gin.H{"status": "Fail", "message": "User not authenticated"})
        return
    }

    userIDUint, ok := userID.(uint)
    if !ok {
        logger.Log.Error("Invalid user ID type",
            zap.Any("userID", userID),
            zap.String("type", fmt.Sprintf("%T", userID)))
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Invalid user ID"})
        return
    }

    transactionID := c.PostForm("transaction_id")
    razorpayPaymentID := c.PostForm("razorpay_payment_id")
    razorpayOrderID := c.PostForm("razorpay_order_id")
    razorpaySignature := c.PostForm("razorpay_signature")
    clientError := c.PostForm("error")

    if transactionID == "" {
        logger.Log.Warn("Missing transaction_id in request")
        c.JSON(http.StatusBadRequest, gin.H{"status": "Fail", "message": "Missing transaction ID"})
        return
    }

    logger.Log.Info("Processing wallet payment confirmation",
        zap.Uint("userID", userIDUint),
        zap.String("transactionID", transactionID))

    var wallet models.Wallet
    if err := config.DB.Where("user_id = ?", userIDUint).First(&wallet).Error; err != nil {
        logger.Log.Error("Wallet not found",
            zap.Uint("userID", userIDUint),
            zap.Error(err))
        c.JSON(http.StatusNotFound, gin.H{"status": "Fail", "message": "Wallet not found"})
        return
    }

    var transaction models.WalletTransaction
    if err := config.DB.Where("transaction_uid = ? AND wallet_id = ?", transactionID, wallet.ID).First(&transaction).Error; err != nil {
        logger.Log.Error("Transaction not found",
            zap.String("transactionID", transactionID),
            zap.Uint("walletID", wallet.ID),
            zap.Error(err))
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
            logger.Log.Warn("Signature verification failed",
                zap.String("razorpayOrderID", razorpayOrderID),
                zap.String("razorpayPaymentID", razorpayPaymentID))
        } else {
            payment, err := client.Payment.Fetch(razorpayPaymentID, nil, nil)
            if err != nil {
                errorMessage = "Failed to verify payment: " + err.Error()
                logger.Log.Error("Failed to fetch payment from Razorpay",
                    zap.String("paymentID", razorpayPaymentID),
                    zap.Error(err))
            } else if payment["status"] != "captured" {
                errorMessage = "Payment not captured"
                logger.Log.Warn("Payment not captured",
                    zap.String("paymentID", razorpayPaymentID),
                    zap.Any("paymentStatus", payment["status"]))
            } else {
                paymentStatus = "Completed"
                redirectURL = "/wallet?status=success"
                errorMessage = ""
                logger.Log.Info("Payment successfully captured",
                    zap.String("paymentID", razorpayPaymentID),
                    zap.Float64("amount", transaction.TransactionAmount))
            }
        }
    }

    tx := config.DB.Begin()
    if tx.Error != nil {
        logger.Log.Error("Failed to start database transaction",
            zap.Error(tx.Error))
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to start transaction"})
        return
    }

    transaction.TransactionStatus = paymentStatus
    if paymentStatus == "Completed" {
        if err := tx.Model(&wallet).Update("Balance", gorm.Expr("balance + ?", transaction.TransactionAmount)).Error; err != nil {
            tx.Rollback()
            logger.Log.Error("Failed to update wallet balance",
                zap.Uint("walletID", wallet.ID),
                zap.Error(err))
            c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update wallet balance"})
            return
        }
    }
    if err := tx.Save(&transaction).Error; err != nil {
        tx.Rollback()
        logger.Log.Error("Failed to update transaction",
            zap.String("transactionID", transactionID),
            zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to update transaction"})
        return
    }

    if err := tx.Commit().Error; err != nil {
        logger.Log.Error("Failed to commit transaction",
            zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Fail", "message": "Failed to commit transaction"})
        return
    }

    if errorMessage != "" {
        redirectURL += "&error=" + url.QueryEscape(errorMessage)
    }

    logger.Log.Info("Payment confirmation completed",
        zap.String("status", paymentStatus),
        zap.String("redirectURL", redirectURL))
    c.JSON(http.StatusOK, gin.H{"redirectURL": redirectURL})
}

func UserSettingPage(c *gin.Context) {
    userID, exists := c.Get("userid")
    if !exists {
        logger.Log.Warn("UserID not found in context - redirecting to login")
        c.Redirect(http.StatusFound, "/login")
        return
    }

    userIDUint, ok := userID.(uint)
    if !ok {
        logger.Log.Error("Invalid user ID type",
            zap.Any("userID", userID),
            zap.String("type", fmt.Sprintf("%T", userID)))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
        return
    }

    logger.Log.Info("Loading user settings page",
        zap.Uint("userID", userIDUint))

    var user models.User
    if err := config.DB.Preload("UserDetails").First(&user, userIDUint).Error; err != nil {
        logger.Log.Error("User not found",
            zap.Uint("userID", userIDUint),
            zap.Error(err))
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    var categories []models.Category
    if err := config.DB.Where("list = ?", true).Find(&categories).Error; err != nil {
        logger.Log.Error("Failed to fetch categories",
            zap.Error(err))
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

    logger.Log.Info("Successfully loaded user settings",
        zap.Uint("userID", userIDUint),
        zap.Int("categoriesCount", len(categories)))
    c.HTML(http.StatusOK, "User_Profile_Setting.html", data)
}

func UserLogout(c *gin.Context) {
    userID, exists := c.Get("userid")
    if exists {
        logger.Log.Info("User logging out",
            zap.Any("userID", userID))
    } else {
        logger.Log.Warn("Logout requested without userID in context")
    }
    
    c.SetCookie("jwtTokensUser", "", -1, "/", "", false, true)
    c.Redirect(http.StatusSeeOther, "/login")
}