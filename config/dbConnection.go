package config

import (
	"ecommerce/models"
	"ecommerce/pkg/logger"
	"os"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func DBconnect() {
    logger.Log.Info("Attempting to connect to database")

	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")

	if dbHost == "" || dbUser == "" || dbPassword == "" || dbName == "" || dbPort == "" {
		logger.Log.Fatal("One or more database environment variables are missing",
			zap.String("DB_HOST", dbHost),
			zap.String("DB_USER", dbUser),
			zap.String("DB_NAME", dbName),
			zap.String("DB_PORT", dbPort),
		)
	}

	dsn := "host=" + dbHost +
		" user=" + dbUser +
		" password=" + dbPassword +
		" dbname=" + dbName +
		" port=" + dbPort +
		" sslmode=require"

	logger.Log.Debug("Database DSN created successfully")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Log.Fatal("Failed to establish database connection",
			zap.Error(err),
			zap.String("connectionMethod", "gorm.Open"),
		)
	}

    logger.Log.Info("Database connection established successfully")
    DB = db

    logger.Log.Info("Starting database migration",
        zap.Int("modelCount", 19))

    modelsToMigrate := []interface{}{
        &models.User{},
        &models.Otp{},
        &models.Admin{},
        &models.Category{},
        &models.CategoryOffer{},
        &models.Product{},
        &models.UserDetails{},
        &models.ProductImage{},
        &models.ProductVariant{},
        &models.ProductOffer{},
        &models.Address{},
        &models.Cart{},
        &models.CartItem{},
        &models.Wishlist{},
        &models.WishlistItem{},
        &models.Wallet{},
        &models.WalletTransaction{},
        &models.Order{},
        &models.OrderItem{},
        &models.ReturnRequest{},
        &models.PaymentDetails{},
        &models.ShippingAddress{},
        &models.Coupon{},
    }

    err = DB.AutoMigrate(modelsToMigrate...)
    if err != nil {
        logger.Log.Fatal("Database migration failed",
            zap.Error(err),
            zap.Any("models", modelsToMigrate))
    }

    logger.Log.Info("Database migration completed successfully",
        zap.Int("migratedModels", len(modelsToMigrate)))
}
