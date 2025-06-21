package routers

import (
	"ecommerce/controllers"
	"ecommerce/middleware"

	"github.com/gin-gonic/gin"
)

var RoleUser = "User"

func UserRoutes(r *gin.Engine) {
    public := r.Group("/")
    public.Use(middleware.NoCacheMiddleware(), middleware.RedirectIfAuthenticated())
    {
        public.GET("/signup", controllers.UserSignupPage)
        public.POST("/signup", controllers.UserSignup)
        public.GET("/verify-otp", controllers.VerifyOTPPage)
        public.POST("/verify-otp", controllers.VerifyOTP)
        public.POST("/resend-otp", controllers.ResendOTP)
        public.GET("/login", controllers.UserLoginPage)
        public.POST("/login", controllers.UserLogin)
        public.GET("/auth/google", controllers.GoogleLogin)
        public.GET("/auth/google/callback", controllers.GoogleCallback)
        public.GET("/forgot-password", controllers.ForgotPassword)
        public.POST("/forgot-password", controllers.SubmitForgotPassword)
        public.GET("/forgot/verify-otp", controllers.ForgotVerifyOtpPage)
        public.POST("/forgot/verify-otp", controllers.ForgotVerifyOtp)
        public.POST("/forgot/resend-otp", controllers.ForgotResendOtp)
        public.GET("/reset-password", controllers.ResetPassword)
        public.POST("/reset-password", controllers.SubmitResetPassword)
    }

    // Protected routes group with AuthMiddleware
    protected := r.Group("/")
    protected.Use(middleware.AuthMiddleware(RoleUser))
    {
        protected.GET("/", controllers.UserHomePage)
        protected.GET("/product", controllers.ProductListing)
        protected.GET("/product/:id", controllers.ProductDetail)
        protected.GET("/profile", controllers.UserProfilePage)
        protected.POST("/update-profile", controllers.UpdateUserProfile)
        protected.GET("/address", controllers.UserAddressPage)
        protected.POST("/add-address", controllers.UserAddAddress)
        protected.POST("/update-address", controllers.UserUpdateAddress)
        protected.POST("/delete-address", controllers.UserDeleteAddress)
        protected.POST("/set-default-address", controllers.UserSetDefaultAddress)
        protected.GET("/cart", controllers.ViewCart)
        protected.POST("/cart/add", controllers.AddToCart)
        protected.POST("/cart/update", controllers.UpdateCart)
        protected.POST("/cart/remove", controllers.RemoveFromCart)
        protected.GET("/checkout/address", controllers.CheckoutAddress)
        protected.POST("/checkout/address", controllers.CheckoutAddress)
        protected.GET("/checkout/payment", controllers.CheckoutPayment)
        protected.POST("/checkout/confirm", controllers.ConfirmOrder)
        protected.GET("/order/failure", controllers.OrderFailure)
        protected.GET("/order/success", controllers.OrderSuccess)
        protected.GET("/orders", controllers.Orders)
        protected.GET("/order/retry-payment", controllers.RetryPayment)
        protected.POST("/order/confirm-retry", controllers.ConfirmRetryPayment)
        protected.POST("/create-pre-order", controllers.CreatePreOrder)
        protected.POST("/create-razorpay-order", controllers.CreateRazorpayOrder)
        protected.POST("/create-order", controllers.CreateOrder)
        protected.GET("/order/details", controllers.OrderDetails)
        protected.POST("/order/cancel", controllers.CancelOrder)
        protected.POST("/order/cancel-item", controllers.CancelOrderItem)
        protected.POST("/order/return", controllers.RequestReturn)
        protected.GET("/order/invoice", controllers.DownloadInvoice)
        protected.GET("/wishlist", controllers.WishlistPage)
        protected.POST("/wishlist/add", controllers.AddToWishlist)
        protected.DELETE("/wishlist/remove/:id", controllers.RemoveFromWishlist)
        protected.POST("/wishlist/move-to-cart/:id", controllers.MoveToCartFromWishlist)
        protected.GET("/wallet", controllers.UserWalletPage)
        protected.POST("/wallet/add", controllers.AddWalletAmount)
        protected.POST("/wallet/confirm", controllers.ConfirmWalletPayment)
        protected.GET("/settings", controllers.UserSettingPage)
        protected.GET("/logout", controllers.UserLogout)
    }
}
