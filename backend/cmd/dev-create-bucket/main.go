package main

import (
	"context"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	endpoint := "localhost:9000"
	accessKey := "zamk_minio"
	secretKey := "zamk_minio_password"

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Init error: %v", err)
	}

	bucketName := "zamk-products"
	ctx := context.Background()

	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatalf("BucketExists error: %v", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatalf("MakeBucket error: %v", err)
		}
		log.Printf("Successfully created bucket %s", bucketName)
	} else {
		log.Printf("Bucket %s already exists", bucketName)
	}
	
	policy := `{"Version": "2012-10-17","Statement": [{"Action": ["s3:GetObject"],"Effect": "Allow","Principal": {"AWS": ["*"]},"Resource": ["arn:aws:s3:::` + bucketName + `/*"],"Sid": ""}]}`
	err = client.SetBucketPolicy(ctx, bucketName, policy)
	if err != nil {
		log.Fatalf("SetBucketPolicy error: %v", err)
	}
	log.Println("Set public policy successfully")
}
