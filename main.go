package main

import (
	"ecommerce/config"
	"ecommerce/pkg/logger"
	"ecommerce/routers"

	"github.com/gin-gonic/gin"
)

func init() {
	logger.InitLogger()
	config.Envload()
	config.DBconnect()
}
func main() {
	r := gin.Default()
	r.LoadHTMLGlob("views/**/*.html")
	routers.AdminRoutes(r)
	routers.UserRoutes(r)
	r.Run(":8080")
}