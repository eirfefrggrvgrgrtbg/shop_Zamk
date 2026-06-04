package main

import (
	"fmt"
	"log"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("S3_ENDPOINT:", cfg.S3.Endpoint)
	fmt.Println("S3_PORT:", cfg.S3.Port)
	fmt.Println("S3_REGION:", cfg.S3.Region)
	fmt.Println("S3_BUCKET:", cfg.S3.Bucket)
	fmt.Println("S3_USE_SSL:", cfg.S3.UseSSL)
	fmt.Println("S3_PUBLIC_BASE_URL:", cfg.S3.PublicBaseURL)
	fmt.Println("S3_UPLOAD_MAX_SIZE_MB:", cfg.S3.UploadMaxSizeMB)
}
