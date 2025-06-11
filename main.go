package main

import (
	"ecommerce/config"
	"ecommerce/routers"

	"github.com/gin-gonic/gin"
)

func init() {
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