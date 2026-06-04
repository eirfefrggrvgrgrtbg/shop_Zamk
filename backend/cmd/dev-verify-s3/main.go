package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/config"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/storage"
	"github.com/google/uuid"
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

	fmt.Println("Testing S3 Provider...")
	provider, err := storage.NewS3Client(&cfg.S3)
	if err != nil {
		log.Fatalf("Failed to initialize S3 provider: %v", err)
	}
	fmt.Println("Credentials accepted, provider initialized.")

	// Test uploading a tiny image
	testContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // Minimal PNG header
	fileReader := bytes.NewReader(testContent)
	
	fileName := "test-image.png"
	fileSize := int64(len(testContent))
	mimeType := "image/png"
	ownerID := uuid.New()
	
	fmt.Println("Uploading test object...")
	ctx := context.Background()
	objKey := fmt.Sprintf("test/%s/%s", ownerID.String(), fileName)
	
	storedObj, err := provider.UploadImage(ctx, fileReader, fileSize, objKey, mimeType)
	if err != nil {
		log.Fatalf("Failed to upload test object: %v", err)
	}
	
	fmt.Printf("Upload successful.\nObject Key: %s\nPublic URL: %s\n", storedObj.ObjectKey, storedObj.ObjectURL)
	
	// Test public URL
	fmt.Println("Testing public URL access...")
	resp, err := http.Get(storedObj.ObjectURL)
	if err != nil {
		fmt.Printf("HTTP GET failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("HTTP GET status: %s\n", resp.Status)
		if resp.StatusCode == 403 {
			fmt.Println("Note: 403 Forbidden means the bucket policy does not allow public reads.")
		} else if resp.StatusCode == 404 {
			fmt.Println("Note: 404 Not Found means the object does not exist or S3_PUBLIC_BASE_URL is incorrect.")
		}
	}
	
	// Clean up test object
	fmt.Println("Cleaning up test object...")
	err = provider.DeleteObject(ctx, storedObj.ObjectKey)
	if err != nil {
		fmt.Printf("Failed to delete test object: %v\n", err)
	} else {
		fmt.Println("Cleanup successful.")
	}
	
	fmt.Println("Verification complete.")
}
