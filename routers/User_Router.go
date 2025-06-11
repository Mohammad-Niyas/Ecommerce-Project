package routers

import (
	"ecommerce/controllers"
	"ecommerce/middleware"

	"github.com/gin-gonic/gin"
)

var RoleUser = "User"

func UserRoutes(r *gin.Engine) {
	r.GET("/signup", controllers.UserSignupPage)
	r.POST("/signup", controllers.UserSignup)
	r.GET("/verify-otp", controllers.VerifyOTPPage)
	r.POST("/verify-otp", controllers.VerifyOTP)
	r.POST("/resend-otp", controllers.ResendOTP)
	r.GET("/login", controllers.UserLoginPage)
	r.POST("/login", controllers.UserLogin)
	r.GET("/auth/google", controllers.GoogleLogin)
	r.GET("/auth/google/callback", controllers.GoogleCallback)
	r.GET("/forgot-password", controllers.ForgotPassword)
	r.POST("/forgot-password", controllers.SubmitForgotPassword)
	r.GET("/forgot/verify-otp", controllers.ForgotVerifyOtpPage)
	r.POST("/forgot/verify-otp", controllers.ForgotVerifyOtp)
	r.POST("/forgot/resend-otp", controllers.ForgotResendOtp)
	r.GET("/reset-password", controllers.ResetPassword)
	r.POST("/reset-password", controllers.SubmitResetPassword)

	r.POST("/wishlist/add", middleware.AuthMiddleware(RoleUser), controllers.AddToWishlist)
	r.DELETE("/wishlist/remove/:id", middleware.AuthMiddleware(RoleUser), controllers.RemoveFromWishlist)
	r.POST("/wishlist/move-to-cart/:id", middleware.AuthMiddleware(RoleUser), controllers.MoveToCartFromWishlist)
	r.GET("/wishlist", middleware.AuthMiddleware(RoleUser), controllers.WishlistPage)

	r.GET("/wallet", middleware.AuthMiddleware(RoleUser), controllers.UserWalletPage)
	r.POST("/wallet/add", middleware.AuthMiddleware(RoleUser), controllers.AddWalletAmount)
	r.POST("/wallet/confirm", middleware.AuthMiddleware(RoleUser), controllers.ConfirmWalletPayment)

	r.GET("/", controllers.UserHomePage)
	r.GET("/product", controllers.ProductListing)
	r.GET("/product/:id", controllers.ProductDetail)
	r.GET("/profile", middleware.AuthMiddleware(RoleUser), controllers.UserProfilePage)
	r.POST("/update-profile", middleware.AuthMiddleware(RoleUser), controllers.UpdateUserProfile)
	r.GET("/address", middleware.AuthMiddleware(RoleUser), controllers.UserAddressPage)
	r.POST("/add-address", middleware.AuthMiddleware(RoleUser), controllers.UserAddAddress)
	r.POST("/update-address", middleware.AuthMiddleware(RoleUser), controllers.UserUpdateAddress)
	r.POST("/delete-address", middleware.AuthMiddleware(RoleUser), controllers.UserDeleteAddress)
	r.POST("/set-default-address", middleware.AuthMiddleware(RoleUser), controllers.UserSetDefaultAddress)
	r.GET("/cart", middleware.AuthMiddleware(RoleUser), controllers.ViewCart)
	r.POST("/cart/add", middleware.AuthMiddleware(RoleUser), controllers.AddToCart)
	r.POST("/cart/update", middleware.AuthMiddleware(RoleUser), controllers.UpdateCart)
	r.POST("/cart/remove", middleware.AuthMiddleware(RoleUser), controllers.RemoveFromCart)
	r.GET("/checkout/address", middleware.AuthMiddleware(RoleUser), controllers.CheckoutAddress)
	r.POST("/checkout/address", middleware.AuthMiddleware(RoleUser), controllers.CheckoutAddress)
	r.GET("/checkout/payment", middleware.AuthMiddleware(RoleUser), controllers.CheckoutPayment)
	r.POST("/checkout/confirm", middleware.AuthMiddleware(RoleUser), controllers.ConfirmOrder)
	r.GET("/order/failure", middleware.AuthMiddleware(RoleUser), controllers.OrderFailure)
	r.GET("/order/success", middleware.AuthMiddleware(RoleUser), controllers.OrderSuccess)
	r.GET("/orders", middleware.AuthMiddleware(RoleUser), controllers.Orders)
	r.GET("/order/retry-payment", middleware.AuthMiddleware(RoleUser), controllers.RetryPayment)
	r.POST("/order/confirm-retry", middleware.AuthMiddleware(RoleUser), controllers.ConfirmRetryPayment)
	r.POST("/create-pre-order", middleware.AuthMiddleware(RoleUser), controllers.CreatePreOrder)
	r.POST("/create-razorpay-order", middleware.AuthMiddleware(RoleUser), controllers.CreateRazorpayOrder)
	r.POST("/create-order", middleware.AuthMiddleware(RoleUser), controllers.CreateOrder)
	r.GET("/order/details", middleware.AuthMiddleware(RoleUser), controllers.OrderDetails)
	r.POST("/order/cancel", middleware.AuthMiddleware(RoleUser), controllers.CancelOrder)
	r.POST("/order/cancel-item", middleware.AuthMiddleware(RoleUser), controllers.CancelOrderItem)
	r.POST("/order/return", middleware.AuthMiddleware(RoleUser), controllers.RequestReturn)
	r.GET("/order/invoice", middleware.AuthMiddleware(RoleUser), controllers.DownloadInvoice)

	r.GET("/settings", middleware.AuthMiddleware(RoleUser), controllers.UserSettingPage)

	r.GET("/logout", middleware.AuthMiddleware(RoleUser), controllers.UserLogout)
}
