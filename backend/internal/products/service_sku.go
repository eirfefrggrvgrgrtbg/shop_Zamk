package products

import (
	"context"
	"fmt"
	"strings"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

func (s *Service) GenerateSKUs(ctx context.Context, currentUserID uuid.UUID, count int) ([]string, error) {
	seller, err := s.getSellerForUser(ctx, currentUserID)
	if err != nil {
		return nil, err
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var generated []string
	
	// We should check existing SKUs for this seller
	// Instead of a massive IN query, we just loop and check
	
	for i := 0; i < count; i++ {
		for {
			candidate := fmt.Sprintf("SKU-%s-%06d", strings.ToUpper(seller.ID.String()[:4]), r.Intn(1000000))
			exists, err := s.repo.CheckSellerSKUExists(ctx, seller.ID, candidate)
			if err != nil {
				return nil, err
			}
			if !exists {
				generated = append(generated, candidate)
				break
			}
		}
	}

	return generated, nil
}
