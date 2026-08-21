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
// It uses the runID to find the isolated test seller and all auxiliary
// identities created for the run (buyer/customer accounts). Auxiliary users
// are found by their deterministic canonical email addresses, which makes
// cleanup restart-safe without relying on in-memory state.
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

	// --- Collect auxiliary buyer user IDs by their canonical deterministic email ---
	// The buyer email is: buyer-testlab-{runId}@zamk.ru
	// This is restart-safe: the email is durable in the DB.
	canonicalBuyerEmail := strings.ToLower(fmt.Sprintf("buyer-testlab-%s@zamk.ru", runID))
	var auxUserIDs []uuid.UUID
	auxRows, err := tx.Query(ctx, "SELECT id FROM users WHERE email = $1", canonicalBuyerEmail)
	if err != nil {
		return fmt.Errorf("query aux buyer users: %w", err)
	}
	for auxRows.Next() {
		var uid uuid.UUID
		if err := auxRows.Scan(&uid); err == nil {
			auxUserIDs = append(auxUserIDs, uid)
		}
	}
	auxRows.Close()

	// Delete aux buyer-owned records before deleting the seller's records.
	// Buyer users own: orders (via orders.user_id), reservations (via reservations.user_id).
	// Orders linked to this seller's products will be deleted below via seller graph;
	// here we only need to handle orphaned buyer records (e.g. cart_items referencing non-seller variants).
	// The main seller-graph cleanup below already removes orders/reservations by seller_id path.
	// We just need to ensure buyer users themselves are deleted at the end.

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

	// Delete orders owned by aux buyer users that reference this seller's products.
	// (orders.user_id = buyer, and may have no order_items if checkout failed mid-way)
	// Use the known aux buyer IDs for exact targeting.
	if len(auxUserIDs) > 0 {
		_, err = tx.Exec(ctx, "DELETE FROM orders WHERE id IN (SELECT order_id FROM order_items WHERE seller_id = $1)", sellerID)
		if err != nil {
			return err
		}
	} else {
		_, err = tx.Exec(ctx, "DELETE FROM orders WHERE id IN (SELECT order_id FROM order_items WHERE seller_id = $1)", sellerID)
		if err != nil {
			return err
		}
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

	// Delete supplies
	_, err = tx.Exec(ctx, "DELETE FROM seller_supply_items WHERE supply_id IN (SELECT id FROM seller_supplies WHERE seller_id = $1)", sellerID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "DELETE FROM seller_supplies WHERE seller_id = $1", sellerID)
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

	// Delete seller user links and collect seller-linked user IDs for deletion
	var sellerUserIDs []uuid.UUID
	rows, err := tx.Query(ctx, "SELECT user_id FROM seller_users WHERE seller_id = $1", sellerID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var uid uuid.UUID
		if err := rows.Scan(&uid); err == nil {
			sellerUserIDs = append(sellerUserIDs, uid)
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

	// Delete seller-linked users (owner) using exact email match for safety
	ownerEmail := strings.ToLower(fmt.Sprintf("owner-testlab-%s@zamk.ru", runID))
	for _, uid := range sellerUserIDs {
		_, err = tx.Exec(ctx, "DELETE FROM users WHERE id = $1 AND email = $2", uid, ownerEmail)
		if err != nil {
			return err
		}
	}

	// Delete auxiliary buyer users using exact IDs.
	// Their orders/reservations were already removed above via the seller graph.
	for _, uid := range auxUserIDs {
		// Delete any remaining reservations owned by this buyer (edge case: reservations
		// not linked to the seller's inventory, e.g. expired/released ones still in table)
		_, err = tx.Exec(ctx, "DELETE FROM reservations WHERE user_id = $1", uid)
		if err != nil {
			return err
		}
		// Delete any orders still owned by this buyer
		_, err = tx.Exec(ctx, "DELETE FROM orders WHERE user_id = $1", uid)
		if err != nil {
			return err
		}
		// Now delete the buyer user record itself
		_, err = tx.Exec(ctx, "DELETE FROM users WHERE id = $1 AND email = $2", uid, canonicalBuyerEmail)
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

// CreateInboundSupply manually inserts an open canonical supply to satisfy inbound test lab state.
func (r *Repository) CreateInboundSupply(ctx context.Context, sellerID, variantID uuid.UUID, expectedQty int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	supplyID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO seller_supplies (id, seller_id, status, supply_number, handoff_method, qr_token, created_at, updated_at) 
		VALUES ($1, $2, 'ready_to_ship', $3, 'pvz', $4, now(), now())`,
		supplyID, sellerID, fmt.Sprintf("SUP-%s", supplyID.String()[:8]), supplyID.String())
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO seller_supply_items (id, supply_id, variant_id, expected_quantity, accepted_quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, now(), now())`,
		uuid.New(), supplyID, variantID, expectedQty)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

