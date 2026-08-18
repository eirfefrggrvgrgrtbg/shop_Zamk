package testlab

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/auth"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/cart"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/inventory"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/payouts"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/products"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/selleranalytics"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/sellers"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/users"
)

// The ScenarioEngine requires dependencies to create canonical state.
// We use interfaces to limit coupling where possible, or direct structs if required.
type EngineDeps struct {
	UserRepo   *users.Repository
	SellerSvc  *sellers.Service
	ProductSvc *products.Service
	InvSvc     *inventory.Service
	CartSvc    *cart.Service
	OrderSvc   *orders.Service
	PayoutSvc  *payouts.Service
	TestRepo   *Repository // For historical SQL overrides
	Calc       *Calculator
}

type ScenarioEngine struct {
	deps EngineDeps
}

func NewScenarioEngine(deps EngineDeps) *ScenarioEngine {
	return &ScenarioEngine{deps: deps}
}

// Run executes the requested scenario and returns a ScenarioRun containing the ExpectedResult
func (e *ScenarioEngine) Run(ctx context.Context, adminUserID uuid.UUID, cfg ScenarioConfig) (*ScenarioRun, error) {
	runID := uuid.New().String()[:8] // Short run ID
	
	// Create canonical seller and owner
	sellerID, ownerUserID, err := e.createIsolatedSeller(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to create isolated seller: %w", err)
	}

	now := time.Now().UTC()
	var tz string
	if cfg.Timezone != "" {
		tz = cfg.Timezone
	} else {
		tz = "UTC"
	}
	
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	localNow := now.In(loc)

	// Determine period based on local time boundaries
	// Current period is month to date for example, but seller analytics UI supports variable periods.
	// We'll use a fixed period for the test: Last 30 Days.
	startOfToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	period := selleranalytics.TimePeriod{
		From: startOfToday.AddDate(0, 0, -29),
		To:   startOfToday.AddDate(0, 0, 1).Add(-time.Nanosecond), // end of today
	}

	run := &ScenarioRun{
		RunID:    runID,
		SellerID: sellerID,
		Period:   period,
	}

	switch cfg.Preset {
	case PresetNeverSold:
		run.ExpectedResult = e.deps.Calc.BuildNeverSold(period)

	case PresetBasicSales:
		// Basic sales scenario expects exactly 150000 gross with 8% commission.
		// Use real production finance logic.
		err = e.deps.PayoutSvc.SetCommissionRate(ctx, sellerID, payouts.AdminSellerCommissionRequest{
			RateBPS: 800,
			Reason:  "Test Lab Canonical Run",
		}, adminUserID)
		if err != nil {
			return nil, fmt.Errorf("failed to set canonical commission rate: %w", err)
		}

		// 1. Create a basic product
		product, err := e.createCanonicalProduct(ctx, adminUserID, ownerUserID, runID, 150000, 10)
		if err != nil {
			return nil, err
		}
		variantID := product.Variants[0].ID

		// 2. Receive stock
		_, err = e.deps.InvSvc.ReceiveStock(ctx, adminUserID, inventory.ReceiptRequest{
			ProductVariantID: variantID,
			Quantity:         10,
		})
		if err != nil {
			return nil, err
		}

		// 3. Create a buyer user
		buyerID, err := e.createCanonicalBuyer(ctx, runID)
		if err != nil {
			return nil, err
		}

		// 4. Add to cart
		_, err = e.deps.CartSvc.AddItem(ctx, buyerID, cart.AddItemRequest{
			ProductID:        product.ID,
			ProductVariantID: variantID,
			Quantity:         1,
		})
		if err != nil {
			return nil, err
		}

		// 5. Checkout (creates order, reserves stock)
		dmID, err := e.deps.TestRepo.GetAnyDeliveryMethod(ctx)
		if err != nil {
			return nil, err
		}
		order, err := e.deps.OrderSvc.CreateOrder(ctx, buyerID, orders.CreateOrderRequest{
			CustomerName:     "TestLab Buyer",
			CustomerPhone:    "+70010000000",
			CustomerEmail:    "buyer@zamk.ru",
			DeliveryAddress:  "Test Lab St",
			DeliveryMethodID: dmID,
		}, nil)
		if err != nil {
			return nil, err
		}

		// 6. Manual Payout trigger.
		// Payout ledger entries are normally created asynchronously on fulfillment.
		// For the Test Lab, we simulate immediate fulfillment snapshot.
		err = e.deps.PayoutSvc.CreatePendingSalesForOrder(ctx, order.ID)
		if err != nil {
			return nil, err
		}

		// 7. Backdate the order to yesterday to guarantee it falls well inside the period
		// and to prove we can use direct SQL safely for historical timestamps.
		yesterday := startOfToday.AddDate(0, 0, -1).Add(12 * time.Hour) // Noon yesterday
		err = e.deps.TestRepo.SetOrderCreatedAt(ctx, order.ID, yesterday)
		if err != nil {
			return nil, err
		}
		err = e.deps.TestRepo.SetLedgerEntryCreatedAt(ctx, order.ID, yesterday)
		if err != nil {
			return nil, err
		}

		run.ExpectedResult = e.deps.Calc.BuildBasicSales(
			period,
			yesterday,
			*product.Variants[0].PriceCents,
			1,
			800, // Explicit Test Lab commission BPS
			product.ID.String(),
			variantID.String(),
		)

	case PresetZeroCurrentPeriod:
		// 1. Create a product with 0 stock (we don't need stock for historical orders if we just simulate it, but we can use 2)
		product, err := e.createCanonicalProduct(ctx, adminUserID, ownerUserID, runID, 250000, 2)
		if err != nil {
			return nil, err
		}
		variantID := product.Variants[0].ID

		// 2. Create isolated buyer
		buyerUserID, err := e.createCanonicalBuyer(ctx, runID)
		if err != nil {
			return nil, err
		}

		// 3. We need 2 orders, 1 unit each. Price is 2500 RUB (250,000 cents) each, so total Gross = 500,000 cents
		// The preset expects: BuildZeroCurrentPeriod(period, 500000, 2, 2, 45000)
		// So total Gross = 500,000, Orders = 2, Units = 2.
		// Commission = 45000 cents.
		err = e.deps.PayoutSvc.SetCommissionRate(ctx, sellerID, payouts.AdminSellerCommissionRequest{
			RateBPS: 900,
			Reason:  "Test Lab Historical Run",
		}, adminUserID)
		if err != nil {
			return nil, err
		}

		// Calculate historical time (well before the period)
		historicalTime := period.From.AddDate(0, 0, -2)

		for i := 0; i < 2; i++ {
			// Add to Cart (qty: 1)
			_, err = e.deps.CartSvc.AddItem(ctx, buyerUserID, cart.AddItemRequest{
				ProductID:        product.ID,
				ProductVariantID: variantID,
				Quantity:         1,
			})
			if err != nil {
				return nil, err
			}
			
			// Checkout
			dmID, err := e.deps.TestRepo.GetAnyDeliveryMethod(ctx)
			if err != nil {
				return nil, err
			}
			order, err := e.deps.OrderSvc.CreateOrder(ctx, buyerUserID, orders.CreateOrderRequest{
				CustomerName:     "TestLab Historical Buyer",
				CustomerPhone:    "+70020000000",
				CustomerEmail:    "historical@zamk.ru",
				DeliveryAddress:  "Test Lab St",
				DeliveryMethodID: dmID,
			}, nil)
			if err != nil {
				return nil, err
			}
			
			// Payout
			err = e.deps.PayoutSvc.CreatePendingSalesForOrder(ctx, order.ID)
			if err != nil {
				return nil, err
			}

			// Backdate
			err = e.deps.TestRepo.SetOrderCreatedAt(ctx, order.ID, historicalTime)
			if err != nil {
				return nil, err
			}
			err = e.deps.TestRepo.SetLedgerEntryCreatedAt(ctx, order.ID, historicalTime)
			if err != nil {
				return nil, err
			}
		}

		run.ExpectedResult = e.deps.Calc.BuildZeroCurrentPeriod(period, 500000, 2, 2, 45000)

	case PresetInventoryAndInbound:
		// Required: onHand=20, reserved=4, available=16, inbound=10
		product, err := e.createCanonicalProduct(ctx, adminUserID, ownerUserID, runID, 150000, 20)
		if err != nil {
			return nil, err
		}
		variantID := product.Variants[0].ID

		// Buyer
		buyerUserID, err := e.createCanonicalBuyer(ctx, runID)
		if err != nil {
			return nil, err
		}

		// Reserve 4 units via Cart Checkout (without fulfillment)
		_, err = e.deps.CartSvc.AddItem(ctx, buyerUserID, cart.AddItemRequest{
			ProductID:        product.ID,
			ProductVariantID: variantID,
			Quantity:         4,
		})
		if err != nil {
			return nil, err
		}
		dmID, err := e.deps.TestRepo.GetAnyDeliveryMethod(ctx)
		if err != nil {
			return nil, err
		}
		_, err = e.deps.OrderSvc.CreateOrder(ctx, buyerUserID, orders.CreateOrderRequest{
			CustomerName:     "TestLab Inv Buyer",
			CustomerPhone:    "+70030000000",
			CustomerEmail:    "inv@zamk.ru",
			DeliveryAddress:  "Test Lab St",
			DeliveryMethodID: dmID,
		}, nil)
		if err != nil {
			return nil, err
		}

		// Inbound supply of 10
		err = e.deps.TestRepo.CreateInboundSupply(ctx, sellerID, variantID, 10)
		if err != nil {
			return nil, err
		}

		run.ExpectedResult = e.deps.Calc.BuildInventoryAndInbound(period)

	default:
		return nil, fmt.Errorf("unknown preset: %s", cfg.Preset)
	}

	return run, nil
}

func (e *ScenarioEngine) createIsolatedSeller(ctx context.Context, runID string) (uuid.UUID, uuid.UUID, error) {
	// Create an isolated owner user
	hash, _ := auth.HashPassword("TestLab123!")
	ownerID := uuid.New()
	sellerID := uuid.New()

	ownerUser := &users.User{
		ID:                 ownerID,
		Name:               fmt.Sprintf("TestLab Owner %s", runID),
		Email:              strings.ToLower(fmt.Sprintf("owner-testlab-%s@zamk.ru", runID)),
		Phone:              fmt.Sprintf("+7000%s", runID[:7]), // Mock phone
		PasswordHash:       hash,
		Role:               users.RoleSeller,
		Status:             users.StatusActive,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	brandName := fmt.Sprintf("TESTLAB %s", runID)
	slug := fmt.Sprintf("testlab-%s", runID)
	desc := "Test Lab Isolated Seller"
	
	seller := &sellers.Seller{
		ID:           sellerID,
		BrandName:    &brandName,
		Slug:         &slug,
		Description:  &desc,
		ContactEmail: &ownerUser.Email,
		ContactPhone: &ownerUser.Phone,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	sellerUser := &sellers.SellerUser{
		ID:        uuid.New(),
		SellerID:  sellerID,
		UserID:    ownerID,
		Role:      sellers.RoleOwner,
		CreatedAt: time.Now(),
	}

	err := e.deps.TestRepo.CreateIsolatedIdentity(ctx, ownerUser, seller, sellerUser)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return sellerID, ownerID, nil
}

func (e *ScenarioEngine) createCanonicalBuyer(ctx context.Context, runID string) (uuid.UUID, error) {
	hash, _ := auth.HashPassword("TestLabBuyer123!")
	buyerUser := &users.User{
		ID:                 uuid.New(),
		Name:               fmt.Sprintf("TestLab Buyer %s", runID),
		Email:              strings.ToLower(fmt.Sprintf("buyer-testlab-%s@zamk.ru", runID)),
		Phone:              fmt.Sprintf("+7001%s", runID[:7]),
		PasswordHash:       hash,
		Role:               users.RoleCustomer,
		Status:             users.StatusActive,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	err := e.deps.UserRepo.CreateUser(ctx, buyerUser)
	return buyerUser.ID, err
}

func (e *ScenarioEngine) createCanonicalProduct(ctx context.Context, adminUserID, ownerUserID uuid.UUID, runID string, priceCents int64, initStock int) (products.Product, error) {
	sku := uuid.New().String()[:8]
	size := "M"

	req := products.CreateProductRequest{
		Title:      fmt.Sprintf("TestLab Canonical Product %s", runID),
		PriceCents: priceCents,
		Currency:   "RUB",
		Variants: []products.ProductVariantRequest{
			{
				SKU:          &sku,
				Size:         &size,
				PriceCents:   &priceCents,
				InitialStock: &initStock,
			},
		},
	}

	product, err := e.deps.ProductSvc.CreateProductForSeller(ctx, ownerUserID, req)
	if err != nil {
		return product, err
	}

	// 1. Submit for moderation (draft -> pending_moderation)
	err = e.deps.ProductSvc.SubmitProductToModeration(ctx, ownerUserID, product.ID, products.SubmitProductModerationRequest{})
	if err != nil {
		return product, err
	}

	// 2. Approve the product (pending_moderation -> approved)
	err = e.deps.ProductSvc.ApproveProduct(ctx, adminUserID, product.ID, nil)
	if err != nil {
		return product, err
	}
	
	// 3. Publish the product (approved -> published/active)
	err = e.deps.ProductSvc.PublishProduct(ctx, adminUserID, product.ID, nil)
	return product, err
}
