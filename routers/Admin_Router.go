package routers

import (
	"ecommerce/controllers"
	"ecommerce/middleware"

	"github.com/gin-gonic/gin"
)

var RoleAdmin = "Admin"

func AdminRoutes(r *gin.Engine){
	r.GET("/admin/login", controllers.AdminLoginPage)
	r.POST("/admin/login",controllers.AdminLogin)
	r.GET("/admin/dashboard",middleware.AuthMiddleware(RoleAdmin),controllers.AdminDashboard)
	r.GET("/admin/dashboard/data",middleware.AuthMiddleware(RoleAdmin), controllers.GetDashboardData)
	r.GET("/admin/download/pdf",middleware.AuthMiddleware(RoleAdmin), controllers.DownloadPDF)
	r.GET("/admin/download/excel",middleware.AuthMiddleware(RoleAdmin), controllers.DownloadExcel)
	r.GET("/admin/products",middleware.AuthMiddleware(RoleAdmin),controllers.AdminProducts)
	r.POST("/admin/products/toggle-status/:id",middleware.AuthMiddleware(RoleAdmin), controllers.ToggleProductStatus)
	r.GET("/admin/products/add",middleware.AuthMiddleware(RoleAdmin), controllers.ShowAddProductPage)
	r.POST("/admin/products/add",middleware.AuthMiddleware(RoleAdmin),controllers.AddProduct)
	r.GET("/admin/products/detail/:id",middleware.AuthMiddleware(RoleAdmin),controllers.ProductDetailPage)
	r.POST("/admin/variants/toggle-status/:id",middleware.AuthMiddleware(RoleAdmin), controllers.ToggleVariantStatus)
	r.GET("/admin/products/edit/:id", middleware.AuthMiddleware(RoleAdmin), controllers.AdminEditProductPage)
	r.POST("/admin/products/edit/:id", middleware.AuthMiddleware(RoleAdmin), controllers.AdminEditProduct)

	r.GET("/admin/offers/:id", middleware.AuthMiddleware(RoleAdmin), controllers.GetOffer)
	r.POST("/admin/offers/add", middleware.AuthMiddleware(RoleAdmin), controllers.AddOffer)
	r.PUT("/admin/offers/edit/:id", middleware.AuthMiddleware(RoleAdmin), controllers.EditOffer)

	r.GET("/admin/users-management",middleware.AuthMiddleware(RoleAdmin), controllers.AdminUserManagement)
    r.GET("/admin/users-management/block/:id",middleware.AuthMiddleware(RoleAdmin), controllers.AdminBlockUser)
    r.GET("/admin/users-management/unblock/:id",middleware.AuthMiddleware(RoleAdmin), controllers.AdminUnblockUser)

	r.GET("/admin/categories",middleware.AuthMiddleware(RoleAdmin),controllers.CategoryManagementPage)
	r.GET("/admin/categories/add",middleware.AuthMiddleware(RoleAdmin),controllers.AddCategoryPage)
	r.POST("/admin/categories/add",middleware.AuthMiddleware(RoleAdmin),controllers.AddCategory)
	r.GET("/admin/categories/edit/:id", middleware.AuthMiddleware(RoleAdmin), controllers.EditCategoryPage)
    r.POST("/admin/categories/edit/:id", middleware.AuthMiddleware(RoleAdmin), controllers.EditCategory)
	r.GET("/admin/categories/toggle/:id",middleware.AuthMiddleware(RoleAdmin), controllers.ToggleCategoryStatus)
	r.GET("/admin/categories/details/:id",middleware.AuthMiddleware(RoleAdmin), controllers.CategoryDetailsPage)
	r.POST("/admin/categories/offers/add",middleware.AuthMiddleware(RoleAdmin), controllers.AddCategoryOffer)
	r.PUT("/admin/categories/offers/edit/:id",middleware.AuthMiddleware(RoleAdmin), controllers.EditCategoryOffer)
	r.POST("/admin/categories/offers/toggle/:id",middleware.AuthMiddleware(RoleAdmin), controllers.ToggleCategoryOfferStatus)

	r.GET("/admin/orders",middleware.AuthMiddleware(RoleAdmin), controllers.AdminOrders)
    r.GET("/admin/order/details",middleware.AuthMiddleware(RoleAdmin), controllers.AdminOrderDetails)
    r.POST("/admin/order/update",middleware.AuthMiddleware(RoleAdmin), controllers.AdminUpdateOrderStatus)
	r.POST("/admin/return/action",middleware.AuthMiddleware(RoleAdmin), controllers.AdminReturnAction)

	r.GET("/admin/coupon/management",middleware.AuthMiddleware(RoleAdmin), controllers.AdminCouponManagement)
    r.POST("/admin/coupon/create",middleware.AuthMiddleware(RoleAdmin), controllers.AdminCreateCoupon)
	r.GET("/admin/coupon/edit/:id",middleware.AuthMiddleware(RoleAdmin), controllers.AdminEditCoupon)
    r.POST("/admin/coupon/update/:id",middleware.AuthMiddleware(RoleAdmin), controllers.AdminUpdateCoupon)
    r.GET("/admin/coupon/toggle/:id",middleware.AuthMiddleware(RoleAdmin), controllers.AdminToggleCouponStatus)
	
	r.GET("/admin/wallet",middleware.AuthMiddleware(RoleAdmin), controllers.WalletManagement)
	r.GET("/admin/transaction/:id",middleware.AuthMiddleware(RoleAdmin), controllers.TransactionDetail)

	r.GET("/admin/logout", controllers.AdminLogout)
}