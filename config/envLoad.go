package config

import (
	"log"

	"github.com/joho/godotenv"
)

func Envload() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, using system environment variables")
	}
}
