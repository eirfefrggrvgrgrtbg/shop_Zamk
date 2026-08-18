package testlab

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateIsolatedIdentity transactionally creates a coherent user, seller, and linkage for Test Lab.
func (r *Repository) CreateIsolatedIdentity(ctx context.Context, owner *users.User, seller *sellers.Seller, link *sellers.SellerUser) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// insert user
	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, name, email, phone, password_hash, role, status, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		owner.ID, owner.Name, owner.Email, owner.Phone, owner.PasswordHash, owner.Role, owner.Status, owner.CreatedAt, owner.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	// insert seller
	_, err = tx.Exec(ctx, `
		INSERT INTO sellers (id, brand_name, slug, description, contact_email, contact_phone, status, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		seller.ID, seller.BrandName, seller.Slug, seller.Description, seller.ContactEmail, seller.ContactPhone, seller.Status, seller.CreatedAt, seller.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert seller: %w", err)
	}

	// insert link
	_, err = tx.Exec(ctx, `
		INSERT INTO seller_users (id, seller_id, user_id, role, created_at) 
		VALUES ($1, $2, $3, $4, $5)`,
		link.ID, link.SellerID, link.UserID, link.Role, link.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert seller_users: %w", err)
	}

	return tx.Commit(ctx)
}

// CleanupRun removes all data associated with a specific run ID.
// It uses the runID to find the isolated test seller.
func (r *Repository) CleanupRun(ctx context.Context, runID string) error {
	brandPrefix := fmt.Sprintf("TESTLAB %s", runID)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Find the seller ID
	var sellerID uuid.UUID
	var brandName string
	err = tx.QueryRow(ctx, "SELECT id, brand_name FROM sellers WHERE brand_name LIKE $1 LIMIT 1", brandPrefix+"%").Scan(&sellerID, &brandName)
	if err != nil {
		// If not found, nothing to clean up
		if err.Error() == "no rows in result set" {
			return nil
		}
		return err
	}

	// Safety check
	if !strings.HasPrefix(brandName, "TESTLAB ") {
		return fmt.Errorf("safety violation: seller %s (%s) does not belong to test lab", sellerID, brandName)
	}

	// Delete ledger entries and payouts
	_, err = tx.Exec(ctx, "DELETE FROM seller_ledger_entries WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM payout_batches WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM seller_commission_rules WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}



	// Delete orders & cart
	_, err = tx.Exec(ctx, "DELETE FROM order_reservations WHERE order_id IN (SELECT order_id FROM order_items WHERE seller_id = $1)", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM order_items WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM order_fulfillments WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM orders WHERE id IN (SELECT order_id FROM order_items WHERE seller_id = $1)", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM cart_items WHERE product_variant_id IN (SELECT id FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1))", sellerID)
	if err != nil {
		return err
	}

	// Delete inventory
	_, err = tx.Exec(ctx, "DELETE FROM reservations WHERE inventory_item_id IN (SELECT id FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1))", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM stock_movements WHERE inventory_item_id IN (SELECT id FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1))", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM inventory_items WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
	if err != nil {
		return err
	}

	// Delete products
	_, err = tx.Exec(ctx, "DELETE FROM product_images WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM product_variants WHERE product_id IN (SELECT id FROM products WHERE seller_id = $1)", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM products WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}

	// Delete seller user links
	var userIDs []uuid.UUID
	rows, err := tx.Query(ctx, "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var uid uuid.UUID
		if err := rows.Scan(&uid); err == nil {
			userIDs = append(userIDs, uid)
		}
	}
	rows.Close()

	_, err = tx.Exec(ctx, "DELETE FROM seller_users WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}

	// Delete seller
	_, err = tx.Exec(ctx, "DELETE FROM sellers WHERE id = $1", sellerID)
	if err != nil {
		return err
	}

	// Delete associated users (since they are test lab isolated users)
	for _, uid := range userIDs {
		// Only delete if the user email has TESTLAB in it
		_, err = tx.Exec(ctx, "DELETE FROM users WHERE id = $1 AND email LIKE '%testlab%'", uid)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// SetOrderCreatedAt allows test lab to backdate orders for timeseries metrics.
func (r *Repository) SetOrderCreatedAt(ctx context.Context, orderID uuid.UUID, createdAt time.Time) error {
	_, err := r.db.Exec(ctx, "UPDATE orders SET created_at = $1 WHERE id = $2", createdAt, orderID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "UPDATE order_items SET created_at = $1 WHERE order_id = $2", createdAt, orderID)
	return err
}

// SetLedgerEntryCreatedAt backdates ledger entries to make them appear in historical periods.
func (r *Repository) SetLedgerEntryCreatedAt(ctx context.Context, orderID uuid.UUID, createdAt time.Time) error {
	// A sale creates ledger entries linked to order_items.
	// We'll update the ledger entries belonging to the order items of this order.
	_, err := r.db.Exec(ctx, `
		UPDATE seller_ledger_entries 
		SET created_at = $1 
		WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id = $2)
	`, createdAt, orderID)
	return err
}

// GetAnyDeliveryMethod retrieves a valid delivery method ID to mock orders
func (r *Repository) GetAnyDeliveryMethod(ctx context.Context) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "SELECT id FROM delivery_methods LIMIT 1").Scan(&id)
	return id, err
}
