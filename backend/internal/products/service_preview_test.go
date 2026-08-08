package products

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
)

func TestServicePreviewTokens(t *testing.T) {
	ctx := context.Background()
	svc := &Service{}
	adminID := uuid.New()
	productID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	t.Run("1. token != productID and len == 64 and is valid hex", func(t *testing.T) {
		token1, err := svc.CreateProductPreviewLink(ctx, adminID, productID)
		if err != nil {
			t.Fatalf("Failed to create preview link: %v", err)
		}
		if token1 == productID.String() {
			t.Fatalf("Expected token != productID, got token == productID")
		}
		if len(token1) != 64 {
			t.Fatalf("Expected token length 64, got %d", len(token1))
		}
		if _, err := hex.DecodeString(token1); err != nil {
			t.Fatalf("Expected hex string token, got decode error: %v", err)
		}
	})

	t.Run("2. Two links have distinct tokens", func(t *testing.T) {
		token1, _ := svc.CreateProductPreviewLink(ctx, adminID, productID)
		token2, _ := svc.CreateProductPreviewLink(ctx, adminID, productID)
		if token1 == token2 {
			t.Fatalf("Expected distinct random tokens, got identical tokens")
		}
	})

	t.Run("3. Unknown token format returns error", func(t *testing.T) {
		_, err := svc.GetProductPreviewByToken(ctx, "invalid-short-token")
		if err == nil {
			t.Fatalf("Expected error for invalid token format, got nil")
		}
	})

	t.Run("4. Expired / missing token key returns error", func(t *testing.T) {
		fakeToken := "0000000000000000000000000000000000000000000000000000000000000000"
		_, err := svc.GetProductPreviewByToken(ctx, fakeToken)
		if err == nil {
			t.Fatalf("Expected error for missing key, got nil")
		}
	})

	t.Run("5. Store and delete key in memory store", func(t *testing.T) {
		token, _ := svc.CreateProductPreviewLink(ctx, adminID, productID)
		// Delete token from memory store to simulate Redis key deletion / TTL expiry
		previewMemoryStore.Delete(token)

		_, err := svc.GetProductPreviewByToken(ctx, token)
		if err == nil {
			t.Fatalf("Expected error after key deletion, got nil")
		}
	})
}
