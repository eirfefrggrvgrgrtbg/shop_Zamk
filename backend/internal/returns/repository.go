package returns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateReturnTx(ctx context.Context, tx pgx.Tx, ret *Return, items []ReturnItem) error {
	query := `
		INSERT INTO returns (id, order_id, fulfillment_id, user_id, status, reason, comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	err := tx.QueryRow(ctx, query, ret.ID, ret.OrderID, ret.FulfillmentID, ret.UserID, ret.Status, ret.Reason, ret.Comment).Scan(&ret.CreatedAt, &ret.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range items {
		itemQuery := `
			INSERT INTO return_items (id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING created_at
		`
		err = tx.QueryRow(ctx, itemQuery, items[i].ID, items[i].ReturnID, items[i].OrderItemID, items[i].Quantity, items[i].Reason, items[i].Condition, items[i].Restock, items[i].AcceptedQuantity, items[i].DamagedQuantity, items[i].RejectedQuantity).Scan(&items[i].CreatedAt)
		if err != nil {
			return err
		}

		allocID := uuid.New()
		allocQuery := `
			INSERT INTO return_responsibility_allocations (id, return_item_id, quantity, status)
			VALUES ($1, $2, $3, 'pending')
		`
		if _, err := tx.Exec(ctx, allocQuery, allocID, items[i].ID, items[i].Quantity); err != nil {
			return err
		}

		histQuery := `
			INSERT INTO return_responsibility_allocation_history (id, allocation_id, return_item_id, quantity, status)
			VALUES ($1, $2, $3, $4, 'pending')
		`
		if _, err := tx.Exec(ctx, histQuery, uuid.New(), allocID, items[i].ID, items[i].Quantity); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) UpdateReturnTx(ctx context.Context, tx pgx.Tx, ret *Return) error {
	query := `
		UPDATE returns
		SET status = $1, admin_comment = $2, updated_at = now(), approved_at = $3, rejected_at = $4, completed_at = $5, receiving_started_at = $7
		WHERE id = $6
		RETURNING updated_at
	`
	return tx.QueryRow(ctx, query, ret.Status, ret.AdminComment, ret.ApprovedAt, ret.RejectedAt, ret.CompletedAt, ret.ID, ret.ReceivingStartedAt).Scan(&ret.UpdatedAt)
}

func (r *Repository) UpdateReturnItemRestockTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, restock bool) error {
	query := `UPDATE return_items SET restock = $1 WHERE id = $2`
	_, err := tx.Exec(ctx, query, restock, itemID)
	return err
}

func (r *Repository) GetReturn(ctx context.Context, id uuid.UUID) (*Return, []ReturnItem, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns WHERE id = $1
	`
	var ret Return
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment,
		&ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt,
	)
	if err != nil {
		fmt.Printf("GetReturn %v err: %v\n", id, err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrReturnNotFound
		}
		return nil, nil, err
	}

	itemsQuery := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items WHERE return_id = $1
	`
	rows, err := r.db.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []ReturnItem
	for rows.Next() {
		var item ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = make([]ReturnItem, 0)
	}

	return &ret, items, nil
}

func (r *Repository) GetReturnTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Return, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns WHERE id = $1 FOR UPDATE
	`
	var ret Return
	err := tx.QueryRow(ctx, query, id).Scan(
		&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason,
		&ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt,
		&ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}
	return &ret, nil
}

func (r *Repository) ListReturnsByCustomer(ctx context.Context, userID uuid.UUID, limit, offset int, buildURL func(key string) string) ([]ReturnResponse, int, error) {
	var totalCount int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM returns WHERE user_id = $1", userID).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			r.id, r.order_id, o.order_number, r.fulfillment_id, r.user_id,
			r.status, r.reason, r.comment, r.admin_comment,
			r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at, r.receiving_started_at
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		WHERE r.user_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []ReturnResponse
	var returnIDs []uuid.UUID
	for rows.Next() {
		var ret Return
		var orderNum *string
		if err := rows.Scan(
			&ret.ID, &ret.OrderID, &orderNum, &ret.FulfillmentID, &ret.UserID,
			&ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment,
			&ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, ReturnResponse{
			Return:      ret,
			OrderNumber: orderNum,
			Items:       make([]CustomerReturnItemDetail, 0),
		})
		returnIDs = append(returnIDs, ret.ID)
	}
	rows.Close()

	if len(returnIDs) > 0 {
		itemsQuery := `
			SELECT
				ri.id, ri.return_id, ri.order_item_id,
				oi.title, oi.image_url, oi.variant_size, oi.variant_color, oi.sku,
				ri.quantity, oi.price_cents, (oi.price_cents * ri.quantity),
				ri.reason, ri.condition
			FROM return_items ri
			JOIN order_items oi ON oi.id = ri.order_item_id
			WHERE ri.return_id = ANY($1)
			ORDER BY ri.created_at ASC
		`
		iRows, err := r.db.Query(ctx, itemsQuery, returnIDs)
		if err != nil {
			return nil, 0, err
		}
		defer iRows.Close()

		itemMap := make(map[uuid.UUID][]CustomerReturnItemDetail)
		for iRows.Next() {
			var it CustomerReturnItemDetail
			if err := iRows.Scan(
				&it.ID, &it.ReturnID, &it.OrderItemID,
				&it.ProductTitle, &it.ProductImageURL, &it.VariantSize, &it.VariantColor, &it.SKU,
				&it.Quantity, &it.PriceCents, &it.SubtotalPriceCents,
				&it.Reason, &it.Condition,
			); err != nil {
				return nil, 0, err
			}
			itemMap[it.ReturnID] = append(itemMap[it.ReturnID], it)
		}
		iRows.Close()

		// Load evidence if present
		if len(itemMap) > 0 {
			var allItemIDs []uuid.UUID
			for _, itemList := range itemMap {
				for _, it := range itemList {
					allItemIDs = append(allItemIDs, it.ID)
				}
			}
			if len(allItemIDs) > 0 {
				evQuery := `
					SELECT id, return_item_id, storage_key, content_type, sort_order, created_at
					FROM return_item_evidences
					WHERE return_item_id = ANY($1)
					ORDER BY sort_order ASC, created_at ASC
				`
				evRows, err := r.db.Query(ctx, evQuery, allItemIDs)
				if err == nil {
					defer evRows.Close()
					evMap := make(map[uuid.UUID][]CustomerReturnEvidence)
					for evRows.Next() {
						var evID, retItemID uuid.UUID
						var storageKey, contentType string
						var sortOrder int
						var createdAt time.Time
						if err := evRows.Scan(&evID, &retItemID, &storageKey, &contentType, &sortOrder, &createdAt); err == nil {
							url := "/media/" + storageKey
							if buildURL != nil {
								url = buildURL(storageKey)
							}
							evMap[retItemID] = append(evMap[retItemID], CustomerReturnEvidence{
								ID:          evID,
								URL:         url,
								ContentType: contentType,
								SortOrder:   sortOrder,
								CreatedAt:   createdAt,
							})
						}
					}
					evRows.Close()

					for retID, itemList := range itemMap {
						for idx := range itemList {
							if evList, ok := evMap[itemList[idx].ID]; ok {
								itemList[idx].Evidence = evList
							} else {
								itemList[idx].Evidence = make([]CustomerReturnEvidence, 0)
							}
						}
						itemMap[retID] = itemList
					}
				}
			}
		}

		for i := range list {
			if items, ok := itemMap[list[i].ID]; ok {
				list[i].Items = items
			}
		}

		// Also load shipments if present
		shipmentsQuery := `
			SELECT DISTINCT ON (return_id)
				id, return_id, provider, method, tracking_number, provider_shipment_id, status, selected_cdek_office_code
			FROM return_shipments
			WHERE return_id = ANY($1)
			ORDER BY
				return_id,
				CASE WHEN status != 'cancelled' THEN 0 ELSE 1 END ASC,
				created_at DESC,
				id DESC
		`
		sRows, err := r.db.Query(ctx, shipmentsQuery, returnIDs)
		if err == nil {
			defer sRows.Close()
			shipmentMap := make(map[uuid.UUID]*ReturnShipmentResponse)
			for sRows.Next() {
				var s ReturnShipmentResponse
				var retID uuid.UUID
				if err := sRows.Scan(
					&s.ID, &retID, &s.Provider, &s.Method,
					&s.TrackingNumber, &s.ProviderShipmentID, &s.Status, &s.SelectedCDEKOfficeCode,
				); err == nil {
					shipmentMap[retID] = &s
				}
			}
			sRows.Close()

			for i := range list {
				if sh, ok := shipmentMap[list[i].ID]; ok {
					list[i].Shipment = sh
				}
			}
		}
	}

	if list == nil {
		list = make([]ReturnResponse, 0)
	}
	return list, totalCount, nil
}

func (r *Repository) GetCustomerReturn(ctx context.Context, userID, returnID uuid.UUID, buildURL func(key string) string) (*ReturnResponse, error) {
	query := `
		SELECT
			r.id, r.order_id, o.order_number, r.fulfillment_id, r.user_id,
			r.status, r.reason, r.comment, r.admin_comment,
			r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at, r.receiving_started_at
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		WHERE r.id = $1 AND r.user_id = $2
	`
	var ret Return
	var orderNum *string
	err := r.db.QueryRow(ctx, query, returnID, userID).Scan(
		&ret.ID, &ret.OrderID, &orderNum, &ret.FulfillmentID, &ret.UserID,
		&ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment,
		&ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}

	itemsQuery := `
		SELECT
			ri.id, ri.return_id, ri.order_item_id,
			oi.title, oi.image_url, oi.variant_size, oi.variant_color, oi.sku,
			ri.quantity, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.reason, ri.condition
		FROM return_items ri
		JOIN order_items oi ON oi.id = ri.order_item_id
		WHERE ri.return_id = $1
		ORDER BY ri.created_at ASC
	`
	rows, err := r.db.Query(ctx, itemsQuery, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CustomerReturnItemDetail
	for rows.Next() {
		var it CustomerReturnItemDetail
		if err := rows.Scan(
			&it.ID, &it.ReturnID, &it.OrderItemID,
			&it.ProductTitle, &it.ProductImageURL, &it.VariantSize, &it.VariantColor, &it.SKU,
			&it.Quantity, &it.PriceCents, &it.SubtotalPriceCents,
			&it.Reason, &it.Condition,
		); err != nil {
			return nil, err
		}
		it.Evidence = make([]CustomerReturnEvidence, 0)
		items = append(items, it)
	}
	rows.Close()

	if len(items) > 0 {
		itemIDs := make([]uuid.UUID, len(items))
		for i, it := range items {
			itemIDs[i] = it.ID
		}
		evQuery := `
			SELECT id, return_item_id, storage_key, content_type, sort_order, created_at
			FROM return_item_evidences
			WHERE return_item_id = ANY($1)
			ORDER BY sort_order ASC, created_at ASC
		`
		evRows, err := r.db.Query(ctx, evQuery, itemIDs)
		if err == nil {
			defer evRows.Close()
			evMap := make(map[uuid.UUID][]CustomerReturnEvidence)
			for evRows.Next() {
				var evID, retItemID uuid.UUID
				var storageKey, contentType string
				var sortOrder int
				var createdAt time.Time
				if err := evRows.Scan(&evID, &retItemID, &storageKey, &contentType, &sortOrder, &createdAt); err == nil {
					url := "/media/" + storageKey
					if buildURL != nil {
						url = buildURL(storageKey)
					}
					evMap[retItemID] = append(evMap[retItemID], CustomerReturnEvidence{
						ID:          evID,
						URL:         url,
						ContentType: contentType,
						SortOrder:   sortOrder,
						CreatedAt:   createdAt,
					})
				}
			}
			evRows.Close()

			for i := range items {
				if evList, ok := evMap[items[i].ID]; ok {
					items[i].Evidence = evList
				} else {
					items[i].Evidence = make([]CustomerReturnEvidence, 0)
				}
			}
		}
	}

	if items == nil {
		items = make([]CustomerReturnItemDetail, 0)
	}

	res := &ReturnResponse{
		Return:      ret,
		OrderNumber: orderNum,
		Items:       items,
	}

	shipment, err := r.GetReturnShipmentByReturnID(ctx, returnID)
	if err == nil && shipment != nil {
		res.Shipment = &ReturnShipmentResponse{
			ID:                     shipment.ID,
			Provider:               shipment.Provider,
			Method:                 shipment.Method,
			TrackingNumber:         shipment.TrackingNumber,
			ProviderShipmentID:     shipment.ProviderShipmentID,
			Status:                 shipment.Status,
			SelectedCDEKOfficeCode: shipment.SelectedCDEKOfficeCode,
		}
	}

	return res, nil
}

func (r *Repository) ListAllReturns(ctx context.Context, limit, offset int) ([]Return, error) {
	query := `
		SELECT id, order_id, fulfillment_id, user_id, status, reason, comment, admin_comment, created_at, updated_at, approved_at, rejected_at, completed_at, receiving_started_at
		FROM returns ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Return
	for rows.Next() {
		var ret Return
		if err := rows.Scan(&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt); err != nil {
			return nil, err
		}
		list = append(list, ret)
	}
	if list == nil {
		list = make([]Return, 0)
	}
	return list, nil
}

func (r *Repository) GetAdminReturn(ctx context.Context, returnID uuid.UUID, buildURL func(key string) string) (*AdminReturnResponse, error) {
	query := `
		SELECT
			r.id, r.order_id, o.order_number, r.fulfillment_id, r.user_id,
			o.customer_name, o.customer_email, o.customer_phone,
			of.seller_id, s.brand_name,
			r.status, r.reason, r.comment, r.admin_comment,
			r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at,
			sh.delivered_at,
			(SELECT COUNT(*) FROM return_item_evidences rie JOIN return_items ri ON ri.id = rie.return_item_id WHERE ri.return_id = r.id) as evidence_count
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		LEFT JOIN order_fulfillments of ON of.id = r.fulfillment_id
		LEFT JOIN sellers s ON s.id = of.seller_id
		LEFT JOIN shipments sh ON sh.fulfillment_id = r.fulfillment_id
		WHERE r.id = $1
	`
	var res AdminReturnResponse
	err := r.db.QueryRow(ctx, query, returnID).Scan(
		&res.ID, &res.OrderID, &res.OrderNumber, &res.FulfillmentID, &res.UserID,
		&res.CustomerName, &res.CustomerEmail, &res.CustomerPhone,
		&res.SellerID, &res.SellerName,
		&res.Status, &res.Reason, &res.Comment, &res.AdminComment,
		&res.CreatedAt, &res.UpdatedAt, &res.ApprovedAt, &res.RejectedAt, &res.CompletedAt,
		&res.DeliveredAt,
		&res.EvidenceCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}

	itemsQuery := `
		SELECT
			ri.id, ri.return_id, ri.order_item_id,
			oi.title, oi.image_url, oi.variant_size, oi.variant_color, oi.sku,
			ri.quantity, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.reason, ri.condition, ri.restock
		FROM return_items ri
		JOIN order_items oi ON oi.id = ri.order_item_id
		WHERE ri.return_id = $1
		ORDER BY ri.created_at ASC
	`
	rows, err := r.db.Query(ctx, itemsQuery, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var itemIDs []uuid.UUID
	var items []AdminReturnItemDetail
	for rows.Next() {
		var it AdminReturnItemDetail
		if err := rows.Scan(
			&it.ID, &it.ReturnID, &it.OrderItemID,
			&it.ProductTitle, &it.ProductImageURL, &it.VariantSize, &it.VariantColor, &it.SKU,
			&it.Quantity, &it.PriceCents, &it.SubtotalPriceCents,
			&it.Reason, &it.Condition, &it.Restock,
		); err != nil {
			return nil, err
		}
		it.Evidence = make([]AdminReturnEvidence, 0)
		items = append(items, it)
		itemIDs = append(itemIDs, it.ID)
	}
	rows.Close()

	if len(itemIDs) > 0 {
		evQuery := `
			SELECT id, return_item_id, storage_key, content_type, sort_order, created_at
			FROM return_item_evidences
			WHERE return_item_id = ANY($1)
			ORDER BY sort_order ASC, created_at ASC
		`
		evRows, err := r.db.Query(ctx, evQuery, itemIDs)
		if err != nil {
			return nil, err
		}
		defer evRows.Close()

		evMap := make(map[uuid.UUID][]AdminReturnEvidence)
		for evRows.Next() {
			var evID, retItemID uuid.UUID
			var storageKey, contentType string
			var sortOrder int
			var createdAt time.Time
			if err := evRows.Scan(&evID, &retItemID, &storageKey, &contentType, &sortOrder, &createdAt); err != nil {
				return nil, err
			}
			url := "/media/" + storageKey
			if buildURL != nil {
				url = buildURL(storageKey)
			}
			evMap[retItemID] = append(evMap[retItemID], AdminReturnEvidence{
				ID:          evID,
				URL:         url,
				ContentType: contentType,
				SortOrder:   sortOrder,
				CreatedAt:   createdAt,
			})
		}
		evRows.Close()

		for i := range items {
			if evList, ok := evMap[items[i].ID]; ok {
				items[i].Evidence = evList
			}
		}
	}

	if items == nil {
		items = make([]AdminReturnItemDetail, 0)
	}
	res.Items = items
	shipment, err := r.GetReturnShipmentByReturnID(ctx, returnID)
	if err != nil {
		return nil, err
	}
	if shipment != nil {
		var pickupDTO *PickupAddressDTO
		if len(shipment.PickupAddress) > 0 {
			var p PickupAddressDTO
			if err := json.Unmarshal(shipment.PickupAddress, &p); err == nil {
				pickupDTO = &p
			}
		}
		res.Shipment = &ReturnShipmentResponse{
			ID:                     shipment.ID,
			Provider:               shipment.Provider,
			Method:                 shipment.Method,
			TrackingNumber:         shipment.TrackingNumber,
			ProviderShipmentID:     shipment.ProviderShipmentID,
			Status:                 shipment.Status,
			SelectedCDEKOfficeCode: shipment.SelectedCDEKOfficeCode,
			CustomerName:           shipment.CustomerName,
			CustomerPhone:          shipment.CustomerPhone,
			PickupAddress:          pickupDTO,
			CDEKOfficeAddress:      shipment.CDEKOfficeAddress,
		}
		res.ShipmentStatus = &shipment.Status
		res.ShipmentMethod = &shipment.Method
	}
	return &res, nil
}

func (r *Repository) ListAdminReturns(ctx context.Context, limit, offset int, warehouseOnly bool, buildURL func(key string) string) ([]AdminReturnResponse, int, error) {
	var totalCount int

	countQuery := "SELECT COUNT(*) FROM returns r"
	listQuery := `
		SELECT
			r.id, r.order_id, o.order_number, r.fulfillment_id, r.user_id,
			o.customer_name, o.customer_email, o.customer_phone,
			of.seller_id, s.brand_name,
			r.status, r.reason, r.comment, r.admin_comment,
			r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at,
			sh.delivered_at,
			(SELECT COUNT(*) FROM return_item_evidences rie JOIN return_items ri ON ri.id = rie.return_item_id WHERE ri.return_id = r.id) as evidence_count
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		LEFT JOIN order_fulfillments of ON of.id = r.fulfillment_id
		LEFT JOIN sellers s ON s.id = of.seller_id
		LEFT JOIN shipments sh ON sh.fulfillment_id = r.fulfillment_id
	`

	if warehouseOnly {
		lateralJoin := `
		LEFT JOIN LATERAL (
			SELECT status
			FROM return_shipments rs
			WHERE rs.return_id = r.id
			ORDER BY
				CASE WHEN rs.status != 'cancelled' THEN 0 ELSE 1 END ASC,
				rs.created_at DESC,
				rs.id DESC
			LIMIT 1
		) latest_rs ON true
		`
		whereClause := `
		WHERE (r.status = 'approved' AND latest_rs.status = 'arrived_at_zamk')
		   OR r.status IN ('receiving', 'item_received')
		`
		countQuery += lateralJoin + whereClause
		listQuery += lateralJoin + whereClause
	}

	if err := r.db.QueryRow(ctx, countQuery).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	listQuery += " ORDER BY r.created_at DESC LIMIT $1 OFFSET $2"
	rows, err := r.db.Query(ctx, listQuery, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []AdminReturnResponse
	var returnIDs []uuid.UUID
	for rows.Next() {
		var res AdminReturnResponse
		if err := rows.Scan(
			&res.ID, &res.OrderID, &res.OrderNumber, &res.FulfillmentID, &res.UserID,
			&res.CustomerName, &res.CustomerEmail, &res.CustomerPhone,
			&res.SellerID, &res.SellerName,
			&res.Status, &res.Reason, &res.Comment, &res.AdminComment,
			&res.CreatedAt, &res.UpdatedAt, &res.ApprovedAt, &res.RejectedAt, &res.CompletedAt,
			&res.DeliveredAt,
			&res.EvidenceCount,
		); err != nil {
			return nil, 0, err
		}
		res.Items = make([]AdminReturnItemDetail, 0)
		list = append(list, res)
		returnIDs = append(returnIDs, res.ID)
	}
	rows.Close()

	if len(returnIDs) > 0 {
		itemsQuery := `
			SELECT
				ri.id, ri.return_id, ri.order_item_id,
				oi.title, oi.image_url, oi.variant_size, oi.variant_color, oi.sku,
				ri.quantity, oi.price_cents, (oi.price_cents * ri.quantity),
				ri.reason, ri.condition, ri.restock
			FROM return_items ri
			JOIN order_items oi ON oi.id = ri.order_item_id
			WHERE ri.return_id = ANY($1)
			ORDER BY ri.created_at ASC
		`
		itemRows, err := r.db.Query(ctx, itemsQuery, returnIDs)
		if err != nil {
			return nil, 0, err
		}
		defer itemRows.Close()

		itemsMap := make(map[uuid.UUID][]AdminReturnItemDetail)
		for itemRows.Next() {
			var it AdminReturnItemDetail
			if err := itemRows.Scan(
				&it.ID, &it.ReturnID, &it.OrderItemID,
				&it.ProductTitle, &it.ProductImageURL, &it.VariantSize, &it.VariantColor, &it.SKU,
				&it.Quantity, &it.PriceCents, &it.SubtotalPriceCents,
				&it.Reason, &it.Condition, &it.Restock,
			); err != nil {
				return nil, 0, err
			}
			it.Evidence = make([]AdminReturnEvidence, 0)
			itemsMap[it.ReturnID] = append(itemsMap[it.ReturnID], it)
		}
		itemRows.Close()

		for i := range list {
			if itList, ok := itemsMap[list[i].ID]; ok {
				list[i].Items = itList
			}
		}

		// Load shipments for all return IDs using deterministic canonical selection:
		// Active (status != 'cancelled') first, then newest created_at, then tie-break by id.
		shipmentsQuery := `
			SELECT DISTINCT ON (return_id)
				id, return_id, provider, method, tracking_number, provider_shipment_id, status, selected_cdek_office_code
			FROM return_shipments
			WHERE return_id = ANY($1)
			ORDER BY
				return_id,
				CASE WHEN status != 'cancelled' THEN 0 ELSE 1 END ASC,
				created_at DESC,
				id DESC
		`
		sRows, err := r.db.Query(ctx, shipmentsQuery, returnIDs)
		if err == nil {
			defer sRows.Close()
			shipmentMap := make(map[uuid.UUID]*ReturnShipmentResponse)
			for sRows.Next() {
				var s ReturnShipmentResponse
				var retID uuid.UUID
				if err := sRows.Scan(
					&s.ID, &retID, &s.Provider, &s.Method,
					&s.TrackingNumber, &s.ProviderShipmentID, &s.Status, &s.SelectedCDEKOfficeCode,
				); err == nil {
					shipmentMap[retID] = &s
				}
			}
			sRows.Close()

			for i := range list {
				if sh, ok := shipmentMap[list[i].ID]; ok {
					list[i].Shipment = sh
					list[i].ShipmentStatus = &sh.Status
					list[i].ShipmentMethod = &sh.Method
				}
			}
		}
	}

	if list == nil {
		list = make([]AdminReturnResponse, 0)
	}
	return list, totalCount, nil
}

func (r *Repository) fetchSellerReturnItems(ctx context.Context, whereClause string, args ...interface{}) ([]SellerReturnItem, error) {
	query := fmt.Sprintf(`
		SELECT
			ri.id, ri.return_id, r.order_id, o.order_number, ri.order_item_id,
			r.status, ri.quantity, ri.reason, ri.condition,
			oi.title, oi.variant_size, oi.variant_color, oi.sku, oi.image_url, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.restock,
			adj.amount_cents,
			CASE
				WHEN adj.amount_cents IS NULL THEN NULL
				WHEN adj.reason = 'return_post_payout' THEN 'post_payout'
				WHEN COALESCE(adj.adjustment_available_at, se.available_at) IS NOT NULL
				     AND adj.adjusted_at < COALESCE(adj.adjustment_available_at, se.available_at) THEN 'hold'
				ELSE 'available'
			END,
			adj.adjusted_at,
			r.created_at, r.updated_at,
			r.receiving_started_at, r.completed_at,
			rs.status, rs.tracking_number, rs.method,
			ri.accepted_quantity, ri.damaged_quantity, ri.rejected_quantity,
			EXISTS (SELECT 1 FROM order_item_allocations oia WHERE oia.order_item_id = ri.order_item_id)
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
		JOIN orders o ON o.id = r.order_id
		LEFT JOIN LATERAL (
			SELECT available_at, payout_batch_id
			FROM seller_ledger_entries
			WHERE order_item_id = oi.id AND type = 'seller_earning'
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) se ON true
		LEFT JOIN LATERAL (
			SELECT
				sle.amount_cents,
				sle.created_at AS adjusted_at,
				sle.metadata->>'reason' AS reason,
				sle.available_at AS adjustment_available_at
			FROM seller_ledger_entries sle
			WHERE sle.order_item_id = oi.id
			  AND sle.type = 'adjustment'
			  AND sle.metadata->>'return_id' = r.id::text
			  AND sle.metadata->>'reason' IN ('return_deduction', 'return_post_payout')
			  AND sle.amount_cents < 0
			ORDER BY sle.created_at DESC, sle.id DESC
			LIMIT 1
		) adj ON true
		LEFT JOIN LATERAL (
			SELECT status, tracking_number, method
			FROM return_shipments
			WHERE return_id = r.id
			ORDER BY
				CASE WHEN status != 'cancelled' THEN 0 ELSE 1 END ASC,
				created_at DESC,
				id DESC
			LIMIT 1
		) rs ON true
		WHERE %s
	`, whereClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawItem struct {
		item               SellerReturnItem
		rawRestock         bool
		adjAmountCents     *int64
		adjContext         *string
		adjAdjustedAt      *time.Time
		receivingStartedAt *time.Time
		completedAt        *time.Time
		shipmentStatus     *string
		trackingNumber     *string
		shipmentMethod     *string
		legacyAccepted     int
		legacyDamaged      int
		legacyRejected     int
		isSerialized       bool
	}

	var rawItems []rawItem
	var returnItemIDs []uuid.UUID

	for rows.Next() {
		var ri rawItem
		if err := rows.Scan(
			&ri.item.ReturnItemID, &ri.item.ReturnID, &ri.item.OrderID, &ri.item.OrderNumber, &ri.item.OrderItemID,
			&ri.item.Status, &ri.item.Quantity, &ri.item.Reason, &ri.item.Condition,
			&ri.item.ProductTitle, &ri.item.VariantSize, &ri.item.VariantColor, &ri.item.SKU, &ri.item.ImageURL, &ri.item.PriceCents, &ri.item.SubtotalPriceCents,
			&ri.rawRestock,
			&ri.adjAmountCents, &ri.adjContext, &ri.adjAdjustedAt,
			&ri.item.CreatedAt, &ri.item.UpdatedAt,
			&ri.receivingStartedAt, &ri.completedAt,
			&ri.shipmentStatus, &ri.trackingNumber, &ri.shipmentMethod,
			&ri.legacyAccepted, &ri.legacyDamaged, &ri.legacyRejected,
			&ri.isSerialized,
		); err != nil {
			return nil, err
		}
		if ri.adjAmountCents != nil && ri.adjContext != nil && ri.adjAdjustedAt != nil {
			deduction := -(*ri.adjAmountCents)
			if deduction < 0 {
				deduction = -deduction
			}
			ri.item.FinancialAdjustment = &SellerReturnFinancialAdjustment{
				DeductionCents: deduction,
				Context:        *ri.adjContext,
				AdjustedAt:     *ri.adjAdjustedAt,
			}
		}
		rawItems = append(rawItems, ri)
		returnItemIDs = append(returnItemIDs, ri.item.ReturnItemID)
	}
	if len(rawItems) == 0 {
		return make([]SellerReturnItem, 0), nil
	}

	// Batch query return_item_units
	unitsByItem := make(map[uuid.UUID][]SellerReturnUnitDetail)
	unitQuery := `
		SELECT riu.return_item_id, iu.unit_code, riu.disposition, riu.scanned_at
		FROM return_item_units riu
		JOIN order_item_allocations oia ON oia.id = riu.order_item_allocation_id
		JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
		WHERE riu.return_item_id = ANY($1)
		ORDER BY riu.created_at ASC
	`
	uRows, err := r.db.Query(ctx, unitQuery, returnItemIDs)
	if err != nil {
		return nil, err
	}
	defer uRows.Close()
	for uRows.Next() {
		var rItemID uuid.UUID
		var u SellerReturnUnitDetail
		if err := uRows.Scan(&rItemID, &u.UnitCode, &u.Disposition, &u.ScannedAt); err != nil {
			return nil, err
		}
		unitsByItem[rItemID] = append(unitsByItem[rItemID], u)
	}
	if err := uRows.Err(); err != nil {
		return nil, err
	}

	result := make([]SellerReturnItem, 0, len(rawItems))
	for _, raw := range rawItems {
		item := raw.item
		item.ReceivingStartedAt = raw.receivingStartedAt
		item.CompletedAt = raw.completedAt
		item.LogisticsStatus = raw.shipmentStatus
		item.TrackingNumber = raw.trackingNumber
		item.ShipmentMethod = raw.shipmentMethod

		units := unitsByItem[item.ReturnItemID]
		if units == nil {
			units = make([]SellerReturnUnitDetail, 0)
		}
		item.Units = units

		if raw.isSerialized {
			var restocked, damaged, rejected int
			for _, u := range units {
				if u.Disposition != nil {
					switch *u.Disposition {
					case "restock":
						restocked++
					case "damaged":
						damaged++
					case "reject":
						rejected++
					}
				}
			}
			item.RestockedQuantity = restocked
			item.DamagedQuantity = damaged
			item.RejectedQuantity = rejected
			notRec := item.Quantity - len(units)
			if notRec < 0 {
				notRec = 0
			}
			item.NotReceivedQuantity = notRec
		} else {
			item.RestockedQuantity = raw.legacyAccepted
			item.DamagedQuantity = raw.legacyDamaged
			item.RejectedQuantity = raw.legacyRejected
			notRec := item.Quantity - (raw.legacyAccepted + raw.legacyDamaged + raw.legacyRejected)
			if notRec < 0 {
				notRec = 0
			}
			item.NotReceivedQuantity = notRec
		}

		// Arrival determination
		arrived := false
		if raw.shipmentStatus != nil && *raw.shipmentStatus == "arrived_at_zamk" {
			arrived = true
		}
		if item.Status == "receiving" || item.Status == "item_received" || item.Status == "completed" || item.Status == "refunded" || item.ReceivingStartedAt != nil {
			arrived = true
		}
		item.ArrivedAtZamk = arrived

		// Inspection completion determination
		item.InspectionCompleted = (item.Status == "item_received" || item.Status == "completed" || item.Status == "refunded")

		// Processing status & Physical outcome determination
		switch item.Status {
		case "cancelled":
			item.ProcessingStatus = "cancelled"
			item.PhysicalOutcome = "cancelled"
		case "rejected":
			item.ProcessingStatus = "rejected"
			item.PhysicalOutcome = "rejected_by_support"
		case "needs_info":
			item.ProcessingStatus = "needs_info"
			item.PhysicalOutcome = "needs_info"
		case "requested":
			item.ProcessingStatus = "requested"
			item.PhysicalOutcome = "requested"
		case "receiving":
			item.ProcessingStatus = "receiving"
			item.PhysicalOutcome = "in_inspection"
		case "item_received", "completed", "refunded":
			item.ProcessingStatus = "completed"
			if item.RestockedQuantity == item.Quantity {
				item.PhysicalOutcome = "restocked"
			} else if item.DamagedQuantity == item.Quantity {
				item.PhysicalOutcome = "damaged"
			} else if item.RejectedQuantity == item.Quantity {
				item.PhysicalOutcome = "rejected"
			} else if item.NotReceivedQuantity == item.Quantity {
				item.PhysicalOutcome = "not_received"
			} else if item.RestockedQuantity > 0 {
				item.PhysicalOutcome = "partial_restock"
			} else if item.DamagedQuantity > 0 {
				item.PhysicalOutcome = "damaged"
			} else if item.RejectedQuantity > 0 {
				item.PhysicalOutcome = "rejected"
			} else {
				item.PhysicalOutcome = "completed"
			}
		default: // "approved" and active pre-receiving states
			if item.ArrivedAtZamk {
				item.ProcessingStatus = "arrived_at_zamk"
				item.PhysicalOutcome = "arrived_at_zamk"
			} else if raw.shipmentStatus != nil {
				switch *raw.shipmentStatus {
				case "in_transit", "handed_over":
					item.ProcessingStatus = "in_transit"
					item.PhysicalOutcome = "in_transit"
				case "awaiting_handover":
					item.ProcessingStatus = "awaiting_handover"
					item.PhysicalOutcome = "awaiting_handover"
				default:
					item.ProcessingStatus = "awaiting_shipment"
					item.PhysicalOutcome = "awaiting_shipment"
				}
			} else {
				item.ProcessingStatus = "awaiting_shipment"
				item.PhysicalOutcome = "awaiting_shipment"
			}
		}

		// Restock flag for backward compatibility
		item.Restock = item.RestockedQuantity > 0

		result = append(result, item)
	}

	return result, nil
}

func (r *Repository) GetSellerReturnItems(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerReturnItem, error) {
	where := "oi.seller_id = $1 ORDER BY r.created_at DESC LIMIT $2 OFFSET $3"
	return r.fetchSellerReturnItems(ctx, where, sellerID, limit, offset)
}

func (r *Repository) GetSellerReturnItemsForReturn(ctx context.Context, sellerID, returnID uuid.UUID) ([]SellerReturnItem, error) {
	where := "oi.seller_id = $1 AND r.id = $2 ORDER BY ri.created_at ASC"
	return r.fetchSellerReturnItems(ctx, where, sellerID, returnID)
}

func (r *Repository) GetTotalRefundedAmountForOrder(ctx context.Context, orderID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM refunds
		WHERE order_id = $1 AND status IN ('pending', 'processing', 'succeeded')
	`
	var total int64
	err := r.db.QueryRow(ctx, query, orderID).Scan(&total)
	return total, err
}

func (r *Repository) GetRefundSumsForOrder(ctx context.Context, orderID uuid.UUID) (succeededCents int64, pendingCents int64, err error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN status IN ('succeeded', 'completed') THEN amount_cents ELSE 0 END), 0) AS succeeded_cents,
			COALESCE(SUM(CASE WHEN status IN ('pending', 'processing') THEN amount_cents ELSE 0 END), 0) AS pending_cents
		FROM refunds
		WHERE order_id = $1
	`
	err = r.db.QueryRow(ctx, query, orderID).Scan(&succeededCents, &pendingCents)
	return succeededCents, pendingCents, err
}

func (r *Repository) GetRefund(ctx context.Context, id uuid.UUID) (*Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds WHERE id = $1
	`
	var ref Refund
	err := r.db.QueryRow(ctx, query, id).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency,
		&ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &ref, nil
}

func (r *Repository) GetRefundTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds WHERE id = $1
	`
	var ref Refund
	err := tx.QueryRow(ctx, query, id).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency,
		&ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &ref, nil
}

func (r *Repository) GetRefundByReturnID(ctx context.Context, returnID uuid.UUID) (*Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds WHERE return_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1
	`
	var ref Refund
	err := r.db.QueryRow(ctx, query, returnID).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency,
		&ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &ref, nil
}

func (r *Repository) GetRefundByReturnIDTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) (*Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds WHERE return_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1
	`
	var ref Refund
	err := tx.QueryRow(ctx, query, returnID).Scan(
		&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency,
		&ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}
	return &ref, nil
}

func (r *Repository) GetPendingRefundsByReturnIDTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) ([]Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds
		WHERE return_id = $1 AND status IN ('pending', 'processing')
		ORDER BY created_at DESC, id DESC
	`
	rows, err := tx.Query(ctx, query, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Refund
	for rows.Next() {
		var ref Refund
		if err := rows.Scan(
			&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency,
			&ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, ref)
	}
	return list, rows.Err()
}

func (r *Repository) ListAllRefunds(ctx context.Context, limit, offset int) ([]Refund, error) {
	query := `
		SELECT id, return_id, payment_id, order_id, status, amount_cents, currency, provider, provider_refund_id, reason, created_at, updated_at, processed_at, failed_at
		FROM refunds ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Refund
	for rows.Next() {
		var ref Refund
		if err := rows.Scan(&ref.ID, &ref.ReturnID, &ref.PaymentID, &ref.OrderID, &ref.Status, &ref.AmountCents, &ref.Currency, &ref.Provider, &ref.ProviderRefundID, &ref.Reason, &ref.CreatedAt, &ref.UpdatedAt, &ref.ProcessedAt, &ref.FailedAt); err != nil {
			return nil, err
		}
		list = append(list, ref)
	}
	if list == nil {
		list = make([]Refund, 0)
	}
	return list, nil
}

func (r *Repository) GetTotalReturnedQuantityForOrderItem(ctx context.Context, orderItemID uuid.UUID) (int, error) {
	query := `
		SELECT COALESCE(SUM(ri.quantity), 0)
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		WHERE ri.order_item_id = $1 AND r.status NOT IN ('rejected', 'cancelled')
	`
	var total int
	err := r.db.QueryRow(ctx, query, orderItemID).Scan(&total)
	return total, err
}

func (r *Repository) GetReturnReceivingState(ctx context.Context, returnID uuid.UUID) (*AdminReturnReceivingState, error) {
	return r.getReturnReceivingState(ctx, r.db, returnID)
}

func (r *Repository) GetReturnReceivingStateTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) (*AdminReturnReceivingState, error) {
	return r.getReturnReceivingState(ctx, tx, returnID)
}

func (r *Repository) getReturnReceivingState(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}, returnID uuid.UUID) (*AdminReturnReceivingState, error) {
	queryReturn := `
		SELECT r.id, r.order_id, r.fulfillment_id, r.user_id, r.status, r.reason, r.comment, r.admin_comment, r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at, r.receiving_started_at, o.order_number
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		WHERE r.id = $1
	`
	var ret Return
	var orderNumber *string
	err := db.QueryRow(ctx, queryReturn, returnID).Scan(&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt, &orderNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}

	queryItems := `
		SELECT
			ri.id, ri.return_id, ri.order_item_id, ri.quantity, ri.reason, ri.condition, ri.restock, ri.accepted_quantity, ri.damaged_quantity, ri.rejected_quantity, ri.created_at,
			oi.title, oi.image_url, oi.variant_size, oi.variant_color, oi.sku, oi.price_cents
		FROM return_items ri
		JOIN order_items oi ON oi.id = ri.order_item_id
		WHERE ri.return_id = $1
		ORDER BY ri.created_at ASC
	`
	rows, err := db.Query(ctx, queryItems, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var state AdminReturnReceivingState
	state.Return = ret
	state.OrderNumber = orderNumber
	state.Items = make([]AdminReturnReceivingItem, 0)

	type itemWithProduct struct {
		item            ReturnItem
		productTitle    *string
		productImageURL *string
		variantSize     *string
		variantColor    *string
		sku             *string
		priceCents      *int64
	}

	var returnItems []itemWithProduct
	for rows.Next() {
		var iwp itemWithProduct
		var title string
		if err := rows.Scan(
			&iwp.item.ID, &iwp.item.ReturnID, &iwp.item.OrderItemID, &iwp.item.Quantity,
			&iwp.item.Reason, &iwp.item.Condition, &iwp.item.Restock,
			&iwp.item.AcceptedQuantity, &iwp.item.DamagedQuantity, &iwp.item.RejectedQuantity,
			&iwp.item.CreatedAt,
			&title, &iwp.productImageURL, &iwp.variantSize, &iwp.variantColor, &iwp.sku, &iwp.priceCents,
		); err != nil {
			return nil, err
		}
		iwp.productTitle = &title
		returnItems = append(returnItems, iwp)
	}

	for _, iwp := range returnItems {
		item := iwp.item
		// 1. Fetch outbound allocations
		queryOutbound := `
			SELECT oia.id, iu.unit_code, oia.picked_at, oia.released_at, iu.status
			FROM order_item_allocations oia
			JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
			WHERE oia.order_item_id = $1
			ORDER BY oia.created_at ASC
		`
		outRows, err := db.Query(ctx, queryOutbound, item.OrderItemID)
		if err != nil {
			return nil, err
		}
		var outboundAllocs []OutboundAllocationDetail
		for outRows.Next() {
			var d OutboundAllocationDetail
			if err := outRows.Scan(&d.AllocationID, &d.UnitCode, &d.PickedAt, &d.ReleasedAt, &d.UnitStatus); err != nil {
				outRows.Close()
				return nil, err
			}
			outboundAllocs = append(outboundAllocs, d)
		}
		outRows.Close()
		if outboundAllocs == nil {
			outboundAllocs = make([]OutboundAllocationDetail, 0)
		}

		// 2. Fetch scanned units
		queryUnits := `
			SELECT riu.id, riu.return_item_id, riu.order_item_allocation_id, iu.unit_code, riu.scanned_at, riu.inspected_condition, riu.disposition, riu.created_at, riu.updated_at
			FROM return_item_units riu
			JOIN order_item_allocations oia ON oia.id = riu.order_item_allocation_id
			JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
			WHERE riu.return_item_id = $1
			ORDER BY riu.created_at ASC
		`
		unitRows, err := db.Query(ctx, queryUnits, item.ID)
		if err != nil {
			return nil, err
		}
		var scannedUnits []ScannedUnitDetail
		for unitRows.Next() {
			var u ScannedUnitDetail
			if err := unitRows.Scan(&u.ID, &u.ReturnItemID, &u.OrderItemAllocationID, &u.UnitCode, &u.ScannedAt, &u.InspectedCondition, &u.Disposition, &u.CreatedAt, &u.UpdatedAt); err != nil {
				unitRows.Close()
				return nil, err
			}
			scannedUnits = append(scannedUnits, u)
		}
		unitRows.Close()
		if scannedUnits == nil {
			scannedUnits = make([]ScannedUnitDetail, 0)
		}

		allocMode := "serialized"
		var notReceivedQty int
		itemCanFinalize := true

		if len(outboundAllocs) == 0 {
			allocMode = "legacy"
			state.LegacyRequested += item.Quantity
			notReceivedQty = item.Quantity - (item.AcceptedQuantity + item.DamagedQuantity + item.RejectedQuantity)
			itemCanFinalize = (item.AcceptedQuantity >= 0 && item.DamagedQuantity >= 0 && item.RejectedQuantity >= 0 && (item.AcceptedQuantity+item.DamagedQuantity+item.RejectedQuantity) <= item.Quantity)
		} else {
			state.SerializedRequested += item.Quantity
			state.SerializedScanned += len(scannedUnits)
			notReceivedQty = item.Quantity - len(scannedUnits)
			for _, u := range scannedUnits {
				if u.Disposition == nil || *u.Disposition == "" {
					itemCanFinalize = false
					break
				}
			}
		}

		scannedQty := len(scannedUnits)
		remainingQty := item.Quantity - scannedQty

		state.TotalRequested += item.Quantity
		state.TotalScanned += scannedQty
		state.TotalRemaining += remainingQty

		state.Items = append(state.Items, AdminReturnReceivingItem{
			ReturnItem:          item,
			ProductTitle:        iwp.productTitle,
			ProductImageURL:     iwp.productImageURL,
			VariantSize:         iwp.variantSize,
			VariantColor:        iwp.variantColor,
			SKU:                 iwp.sku,
			PriceCents:          iwp.priceCents,
			AllocationMode:      allocMode,
			OutboundAllocations: outboundAllocs,
			ScannedUnits:        scannedUnits,
			RequestedQuantity:   item.Quantity,
			ScannedQuantity:     scannedQty,
			RemainingQuantity:   remainingQty,
			NotReceivedQuantity: notReceivedQty,
			AcceptedQuantity:    item.AcceptedQuantity,
			DamagedQuantity:     item.DamagedQuantity,
			RejectedQuantity:    item.RejectedQuantity,
			CanFinalize:         itemCanFinalize,
		})
	}

	state.CanFinalize = (ret.Status == "receiving")
	if ret.Status == "receiving" {
		for _, it := range state.Items {
			if !it.CanFinalize {
				state.CanFinalize = false
				break
			}
		}
	}

	return &state, nil
}

type AllocationLookupResult struct {
	OrderItemAllocationID uuid.UUID
	OrderItemID           uuid.UUID
	FulfillmentID         uuid.UUID
	OrderID               uuid.UUID
	UnitStatus            string
	PickedAt              *time.Time
	ReleasedAt            *time.Time
}

func (r *Repository) GetAllocationByZMUCode(ctx context.Context, code string) (*AllocationLookupResult, error) {
	query := `
		SELECT oia.id, oia.order_item_id, oi.order_fulfillment_id, oi.order_id, iu.status, oia.picked_at, oia.released_at
		FROM inventory_units iu
		JOIN order_item_allocations oia ON oia.inventory_unit_id = iu.id
		JOIN order_items oi ON oi.id = oia.order_item_id
		WHERE iu.unit_code = $1
		ORDER BY oia.created_at DESC
		LIMIT 1
	`
	var res AllocationLookupResult
	var fulfillmentID *uuid.UUID
	err := r.db.QueryRow(ctx, query, code).Scan(&res.OrderItemAllocationID, &res.OrderItemID, &fulfillmentID, &res.OrderID, &res.UnitStatus, &res.PickedAt, &res.ReleasedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("zmu not found or not allocated")
		}
		return nil, err
	}
	if fulfillmentID == nil {
		return nil, errors.New("zmu allocation has no fulfillment")
	}
	res.FulfillmentID = *fulfillmentID
	return &res, nil
}

func (r *Repository) GetReturnItemUnitByAllocationID(ctx context.Context, allocationID uuid.UUID) (*ReturnItemUnit, error) {
	query := `
		SELECT id, return_item_id, order_item_allocation_id, scanned_at, inspected_condition, disposition, created_at, updated_at
		FROM return_item_units WHERE order_item_allocation_id = $1
	`
	var u ReturnItemUnit
	err := r.db.QueryRow(ctx, query, allocationID).Scan(&u.ID, &u.ReturnItemID, &u.OrderItemAllocationID, &u.ScannedAt, &u.InspectedCondition, &u.Disposition, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CreateReturnItemUnitTx(ctx context.Context, tx pgx.Tx, unit *ReturnItemUnit) error {
	query := `
		INSERT INTO return_item_units (id, return_item_id, order_item_allocation_id, scanned_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if unit.ID == uuid.Nil {
		unit.ID = uuid.New()
	}
	now := time.Now()
	if unit.CreatedAt.IsZero() {
		unit.CreatedAt = now
	}
	unit.UpdatedAt = now

	_, err := tx.Exec(ctx, query, unit.ID, unit.ReturnItemID, unit.OrderItemAllocationID, unit.ScannedAt, unit.CreatedAt, unit.UpdatedAt)
	return err
}

func (r *Repository) GetReturnItemsForUpdateTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) ([]ReturnItem, error) {
	query := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items WHERE return_id = $1 FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReturnItem
	for rows.Next() {
		var item ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]ReturnItem, 0)
	}
	return list, nil
}

func (r *Repository) GetScannedUnitCountForReturnItemTx(ctx context.Context, tx pgx.Tx, returnItemID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM return_item_units WHERE return_item_id = $1`
	var count int
	err := tx.QueryRow(ctx, query, returnItemID).Scan(&count)
	return count, err
}

func (r *Repository) GetReturnItemUnitWithReturnIDTx(ctx context.Context, tx pgx.Tx, unitID uuid.UUID) (*ReturnItemUnit, uuid.UUID, error) {
	query := `
		SELECT riu.id, riu.return_item_id, riu.order_item_allocation_id, riu.scanned_at, riu.inspected_condition, riu.disposition, riu.created_at, riu.updated_at, ri.return_id
		FROM return_item_units riu
		JOIN return_items ri ON ri.id = riu.return_item_id
		WHERE riu.id = $1
		FOR UPDATE
	`
	var u ReturnItemUnit
	var returnID uuid.UUID
	err := tx.QueryRow(ctx, query, unitID).Scan(&u.ID, &u.ReturnItemID, &u.OrderItemAllocationID, &u.ScannedAt, &u.InspectedCondition, &u.Disposition, &u.CreatedAt, &u.UpdatedAt, &returnID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, uuid.Nil, ErrUnitNotInReturn
		}
		return nil, uuid.Nil, err
	}
	return &u, returnID, nil
}

func (r *Repository) UpdateSerializedUnitInspectionTx(ctx context.Context, tx pgx.Tx, unitID uuid.UUID, condition *string, disposition string) error {
	query := `
		UPDATE return_item_units
		SET inspected_condition = $1, disposition = $2, updated_at = now()
		WHERE id = $3
	`
	tag, err := tx.Exec(ctx, query, condition, disposition, unitID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUnitNotInReturn
	}
	return nil
}

func (r *Repository) GetReturnItemByIDTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) (*ReturnItem, error) {
	query := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items
		WHERE id = $1
		FOR UPDATE
	`
	var item ReturnItem
	err := tx.QueryRow(ctx, query, itemID).Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) UpdateLegacyItemInspectionTx(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, accepted, damaged, rejected int) error {
	query := `
		UPDATE return_items
		SET accepted_quantity = $1, damaged_quantity = $2, rejected_quantity = $3
		WHERE id = $4
	`
	tag, err := tx.Exec(ctx, query, accepted, damaged, rejected, itemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrReturnNotFound
	}
	return nil
}

func (r *Repository) GetAllocationsForOrderItemTx(ctx context.Context, tx pgx.Tx, orderItemID uuid.UUID) ([]OutboundAllocationDetail, error) {
	query := `
		SELECT oia.id, iu.unit_code, oia.picked_at, oia.released_at, iu.status
		FROM order_item_allocations oia
		JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
		WHERE oia.order_item_id = $1
		ORDER BY oia.id ASC
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, orderItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []OutboundAllocationDetail
	for rows.Next() {
		var d OutboundAllocationDetail
		if err := rows.Scan(&d.AllocationID, &d.UnitCode, &d.PickedAt, &d.ReleasedAt, &d.UnitStatus); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	if list == nil {
		list = make([]OutboundAllocationDetail, 0)
	}
	return list, nil
}

func (r *Repository) GetScannedUnitsForReturnItemTx(ctx context.Context, tx pgx.Tx, returnItemID uuid.UUID) ([]ScannedUnitDetail, error) {
	query := `
		SELECT riu.id, riu.return_item_id, riu.order_item_allocation_id, iu.unit_code, riu.scanned_at, riu.inspected_condition, riu.disposition, riu.created_at, riu.updated_at
		FROM return_item_units riu
		JOIN order_item_allocations oia ON oia.id = riu.order_item_allocation_id
		JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
		WHERE riu.return_item_id = $1
		ORDER BY riu.id ASC
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, returnItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ScannedUnitDetail
	for rows.Next() {
		var u ScannedUnitDetail
		if err := rows.Scan(&u.ID, &u.ReturnItemID, &u.OrderItemAllocationID, &u.UnitCode, &u.ScannedAt, &u.InspectedCondition, &u.Disposition, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	if list == nil {
		list = make([]ScannedUnitDetail, 0)
	}
	return list, nil
}

func (r *Repository) CreateEvidence(ctx context.Context, evidence *ReturnItemEvidence) error {
	query := `
		INSERT INTO return_item_evidences (id, customer_id, return_item_id, storage_key, content_type, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`
	return r.db.QueryRow(ctx, query, evidence.ID, evidence.CustomerID, evidence.ReturnItemID, evidence.StorageKey, evidence.ContentType, evidence.SortOrder).Scan(&evidence.CreatedAt)
}

func (r *Repository) BindEvidenceTx(ctx context.Context, tx pgx.Tx, customerID uuid.UUID, evidenceIDs []uuid.UUID, returnItemID uuid.UUID) error {
	if len(evidenceIDs) == 0 {
		return nil
	}

	seen := make(map[uuid.UUID]bool)
	for _, id := range evidenceIDs {
		if seen[id] {
			return ErrEvidenceDuplicate
		}
		seen[id] = true
	}

	query := `
		SELECT id, customer_id, return_item_id, content_type
		FROM return_item_evidences
		WHERE id = ANY($1)
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, evidenceIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	found := make(map[uuid.UUID]ReturnItemEvidence)
	for rows.Next() {
		var ev ReturnItemEvidence
		if err := rows.Scan(&ev.ID, &ev.CustomerID, &ev.ReturnItemID, &ev.ContentType); err != nil {
			return err
		}
		found[ev.ID] = ev
	}
	rows.Close()

	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}

	for _, id := range evidenceIDs {
		ev, ok := found[id]
		if !ok || ev.CustomerID != customerID {
			return ErrEvidenceNotFound
		}
		if ev.ReturnItemID != nil {
			return ErrEvidenceAlreadyBound
		}
		if !validTypes[ev.ContentType] {
			return ErrEvidenceInvalidFormat
		}
	}

	queryUpdate := `
		UPDATE return_item_evidences SET return_item_id = $1 WHERE id = ANY($2)
	`
	_, err = tx.Exec(ctx, queryUpdate, returnItemID, evidenceIDs)
	return err
}

func (r *Repository) GetEvidencesByReturnItem(ctx context.Context, returnItemID uuid.UUID) ([]ReturnItemEvidence, error) {
	query := `
		SELECT id, customer_id, return_item_id, storage_key, content_type, sort_order, created_at
		FROM return_item_evidences
		WHERE return_item_id = $1
		ORDER BY sort_order ASC
	`
	rows, err := r.db.Query(ctx, query, returnItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evidences []ReturnItemEvidence
	for rows.Next() {
		var ev ReturnItemEvidence
		if err := rows.Scan(&ev.ID, &ev.CustomerID, &ev.ReturnItemID, &ev.StorageKey, &ev.ContentType, &ev.SortOrder, &ev.CreatedAt); err != nil {
			return nil, err
		}
		evidences = append(evidences, ev)
	}
	if evidences == nil {
		evidences = make([]ReturnItemEvidence, 0)
	}
	return evidences, nil
}

func (r *Repository) GetEvidenceByID(ctx context.Context, evidenceID uuid.UUID) (*ReturnItemEvidence, error) {
	query := `
		SELECT id, customer_id, return_item_id, storage_key, content_type, sort_order, created_at
		FROM return_item_evidences
		WHERE id = $1
	`
	var ev ReturnItemEvidence
	err := r.db.QueryRow(ctx, query, evidenceID).Scan(&ev.ID, &ev.CustomerID, &ev.ReturnItemID, &ev.StorageKey, &ev.ContentType, &ev.SortOrder, &ev.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEvidenceNotFound
		}
		return nil, err
	}
	return &ev, nil
}

func (r *Repository) DeleteEvidence(ctx context.Context, evidenceID uuid.UUID) error {
	query := `DELETE FROM return_item_evidences WHERE id = $1`
	res, err := r.db.Exec(ctx, query, evidenceID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrEvidenceNotFound
	}
	return nil
}

func (r *Repository) CreateReturnShipmentTx(ctx context.Context, tx pgx.Tx, shipment *ReturnShipment) error {
	query := `
		INSERT INTO return_shipments (id, return_id, provider, method, status, selected_cdek_office_code, customer_name, customer_phone, pickup_address, cdek_office_address, destination_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at
	`
	return tx.QueryRow(ctx, query, shipment.ID, shipment.ReturnID, shipment.Provider, shipment.Method, shipment.Status, shipment.SelectedCDEKOfficeCode, shipment.CustomerName, shipment.CustomerPhone, shipment.PickupAddress, shipment.CDEKOfficeAddress, shipment.DestinationAddress).Scan(&shipment.CreatedAt, &shipment.UpdatedAt)
}

func (r *Repository) GetReturnShipmentByReturnID(ctx context.Context, returnID uuid.UUID) (*ReturnShipment, error) {
	query := `
		SELECT id, return_id, provider, method, tracking_number, provider_shipment_id, status, selected_cdek_office_code, customer_name, customer_phone, pickup_address, cdek_office_address, destination_address, snapshots, created_at, updated_at
		FROM return_shipments
		WHERE return_id = $1
		ORDER BY
			CASE WHEN status != 'cancelled' THEN 0 ELSE 1 END ASC,
			created_at DESC,
			id DESC
		LIMIT 1
	`
	var s ReturnShipment
	err := r.db.QueryRow(ctx, query, returnID).Scan(
		&s.ID, &s.ReturnID, &s.Provider, &s.Method, &s.TrackingNumber, &s.ProviderShipmentID, &s.Status, &s.SelectedCDEKOfficeCode, &s.CustomerName, &s.CustomerPhone, &s.PickupAddress, &s.CDEKOfficeAddress, &s.DestinationAddress, &s.Snapshots, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) GetReturnShipmentByReturnIDTx(ctx context.Context, tx pgx.Tx, returnID uuid.UUID) (*ReturnShipment, error) {
	query := `
		SELECT id, return_id, provider, method, tracking_number, provider_shipment_id, status, selected_cdek_office_code, customer_name, customer_phone, pickup_address, cdek_office_address, destination_address, snapshots, created_at, updated_at
		FROM return_shipments
		WHERE return_id = $1
		ORDER BY
			CASE WHEN status != 'cancelled' THEN 0 ELSE 1 END ASC,
			created_at DESC,
			id DESC
		LIMIT 1
	`
	var s ReturnShipment
	err := tx.QueryRow(ctx, query, returnID).Scan(
		&s.ID, &s.ReturnID, &s.Provider, &s.Method, &s.TrackingNumber, &s.ProviderShipmentID, &s.Status, &s.SelectedCDEKOfficeCode, &s.CustomerName, &s.CustomerPhone, &s.PickupAddress, &s.CDEKOfficeAddress, &s.DestinationAddress, &s.Snapshots, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *Repository) UpdateReturnShipmentTx(ctx context.Context, tx pgx.Tx, shipment *ReturnShipment) error {
	query := `
		UPDATE return_shipments
		SET provider = $1, method = $2, tracking_number = $3, provider_shipment_id = $4, status = $5, selected_cdek_office_code = $6,
			customer_name = $7, customer_phone = $8, pickup_address = $9, cdek_office_address = $10, destination_address = $11, snapshots = $12, updated_at = NOW()
		WHERE id = $13
		RETURNING updated_at
	`
	return tx.QueryRow(ctx, query, shipment.Provider, shipment.Method, shipment.TrackingNumber, shipment.ProviderShipmentID, shipment.Status, shipment.SelectedCDEKOfficeCode, shipment.CustomerName, shipment.CustomerPhone, shipment.PickupAddress, shipment.CDEKOfficeAddress, shipment.DestinationAddress, shipment.Snapshots, shipment.ID).Scan(&shipment.UpdatedAt)
}

func (r *Repository) CreateReturnMessageTx(ctx context.Context, tx pgx.Tx, msg *ReturnMessage) error {
	query := `
		INSERT INTO return_messages (id, return_id, sender_user_id, sender_role, message_type, body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.Exec(ctx, query,
		msg.ID, msg.ReturnID, msg.SenderUserID, msg.SenderRole, msg.MessageType, msg.Body, msg.CreatedAt,
	)
	return err
}

func (r *Repository) GetReturnMessages(ctx context.Context, returnID uuid.UUID) ([]ReturnMessageResponse, error) {
	query := `
		SELECT id, return_id, sender_role, message_type, body, created_at
		FROM return_messages
		WHERE return_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := r.db.Query(ctx, query, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ReturnMessageResponse
	var msgIDs []uuid.UUID
	msgMap := make(map[uuid.UUID]*ReturnMessageResponse)

	for rows.Next() {
		var m ReturnMessageResponse
		if err := rows.Scan(
			&m.ID, &m.ReturnID, &m.SenderRole, &m.MessageType, &m.Body, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		m.Attachments = make([]ReturnMessageAttachmentResponse, 0)
		messages = append(messages, m)
		msgIDs = append(msgIDs, m.ID)
	}
	rows.Close()

	if len(msgIDs) > 0 {
		attQuery := `
			SELECT id, message_id, storage_key, content_type, size_bytes, original_filename, sort_order
			FROM return_message_attachments
			WHERE message_id = ANY($1)
			ORDER BY sort_order ASC
		`
		attRows, err := r.db.Query(ctx, attQuery, msgIDs)
		if err != nil {
			return nil, err
		}
		defer attRows.Close()

		for i := range messages {
			msgMap[messages[i].ID] = &messages[i]
		}

		for attRows.Next() {
			var a ReturnMessageAttachmentResponse
			var msgID uuid.UUID
			var storageKey string
			if err := attRows.Scan(&a.ID, &msgID, &storageKey, &a.ContentType, &a.SizeBytes, &a.OriginalFilename, &a.SortOrder); err != nil {
				return nil, err
			}
			a.URL = storageKey // TEMPORARY, WILL BE REPLACED BY SERVICE
			if m, ok := msgMap[msgID]; ok {
				m.Attachments = append(m.Attachments, a)
			}
		}
	}

	if messages == nil {
		messages = make([]ReturnMessageResponse, 0)
	}
	return messages, nil
}

func (r *Repository) CreateStagedMessageAttachment(ctx context.Context, att *ReturnStagedMessageAttachment) error {
	query := `
		INSERT INTO return_staged_message_attachments (id, return_id, uploader_user_id, storage_key, content_type, size_bytes, original_filename, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query, att.ID, att.ReturnID, att.UploaderUserID, att.StorageKey, att.ContentType, att.SizeBytes, att.OriginalFilename, att.CreatedAt)
	return err
}

func (r *Repository) GetStagedMessageAttachmentsTx(ctx context.Context, tx pgx.Tx, returnID, uploaderID uuid.UUID, ids []uuid.UUID) ([]ReturnStagedMessageAttachment, error) {
	if len(ids) == 0 {
		return []ReturnStagedMessageAttachment{}, nil
	}
	query := `
		SELECT id, return_id, uploader_user_id, storage_key, content_type, size_bytes, original_filename, created_at
		FROM return_staged_message_attachments
		WHERE return_id = $1 AND uploader_user_id = $2 AND id = ANY($3)
		FOR UPDATE
	`
	rows, err := tx.Query(ctx, query, returnID, uploaderID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atts []ReturnStagedMessageAttachment
	for rows.Next() {
		var a ReturnStagedMessageAttachment
		if err := rows.Scan(&a.ID, &a.ReturnID, &a.UploaderUserID, &a.StorageKey, &a.ContentType, &a.SizeBytes, &a.OriginalFilename, &a.CreatedAt); err != nil {
			return nil, err
		}
		atts = append(atts, a)
	}
	return atts, nil
}

func (r *Repository) BindMessageAttachmentsTx(ctx context.Context, tx pgx.Tx, messageID uuid.UUID, stagedAtts []ReturnStagedMessageAttachment) error {
	if len(stagedAtts) == 0 {
		return nil
	}

	insertQuery := `
		INSERT INTO return_message_attachments (id, message_id, storage_key, content_type, size_bytes, original_filename, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`

	deleteQuery := `DELETE FROM return_staged_message_attachments WHERE id = $1`

	for i, att := range stagedAtts {
		if _, err := tx.Exec(ctx, insertQuery, att.ID, messageID, att.StorageKey, att.ContentType, att.SizeBytes, att.OriginalFilename, i); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, deleteQuery, att.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetReturnResponsibilityAllocationsByReturnID(ctx context.Context, returnID uuid.UUID) ([]ReturnResponsibilityAllocation, error) {
	query := `
		SELECT a.id, a.return_item_id, a.order_item_allocation_id, a.quantity, a.status, a.responsible_party, a.reason_code, a.decision_source, a.internal_note, a.decided_at, a.actor_id, a.legacy_disposition, a.created_at, a.updated_at
		FROM return_responsibility_allocations a
		JOIN return_items ri ON ri.id = a.return_item_id
		WHERE ri.return_id = $1
		ORDER BY a.created_at ASC
	`
	rows, err := r.db.Query(ctx, query, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocs []ReturnResponsibilityAllocation
	for rows.Next() {
		var a ReturnResponsibilityAllocation
		if err := rows.Scan(
			&a.ID, &a.ReturnItemID, &a.OrderItemAllocationID, &a.Quantity, &a.Status, &a.ResponsibleParty, &a.ReasonCode, &a.DecisionSource, &a.InternalNote, &a.DecidedAt, &a.ActorID, &a.LegacyDisposition, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		allocs = append(allocs, a)
	}
	return allocs, nil
}

func (r *Repository) GetReturnResponsibilityAllocationTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*ReturnResponsibilityAllocation, error) {
	query := `
		SELECT id, return_item_id, order_item_allocation_id, quantity, status, responsible_party, reason_code, decision_source, internal_note, decided_at, actor_id, legacy_disposition, created_at, updated_at
		FROM return_responsibility_allocations
		WHERE id = $1 FOR UPDATE
	`
	var a ReturnResponsibilityAllocation
	err := tx.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.ReturnItemID, &a.OrderItemAllocationID, &a.Quantity, &a.Status, &a.ResponsibleParty, &a.ReasonCode, &a.DecisionSource, &a.InternalNote, &a.DecidedAt, &a.ActorID, &a.LegacyDisposition, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) GetAllocationReturnItemIDTx(ctx context.Context, tx pgx.Tx, allocationID uuid.UUID) (uuid.UUID, error) {
	var returnItemID uuid.UUID
	err := tx.QueryRow(ctx, `SELECT return_item_id FROM return_responsibility_allocations WHERE id = $1`, allocationID).Scan(&returnItemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrReturnNotFound
		}
		return uuid.Nil, err
	}
	return returnItemID, nil
}

func (r *Repository) LockReturnItemTx(ctx context.Context, tx pgx.Tx, returnItemID uuid.UUID) (*ReturnItem, error) {
	var ri ReturnItem
	err := tx.QueryRow(ctx, `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items
		WHERE id = $1
		FOR UPDATE
	`, returnItemID).Scan(
		&ri.ID, &ri.ReturnID, &ri.OrderItemID, &ri.Quantity, &ri.Reason, &ri.Condition, &ri.Restock, &ri.AcceptedQuantity, &ri.DamagedQuantity, &ri.RejectedQuantity, &ri.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}
	return &ri, nil
}

func (r *Repository) GetTotalAllocationsQuantityTx(ctx context.Context, tx pgx.Tx, returnItemID uuid.UUID) (int, error) {
	var total int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM return_responsibility_allocations
		WHERE return_item_id = $1
	`, returnItemID).Scan(&total)
	return total, err
}

func (r *Repository) GetAttributedQuantitiesByReturnItemIDTx(ctx context.Context, tx pgx.Tx, returnItemID uuid.UUID) (map[string]int, error) {
	query := `
		SELECT legacy_disposition, SUM(quantity)
		FROM return_responsibility_allocations
		WHERE return_item_id = $1 AND legacy_disposition IS NOT NULL
		GROUP BY legacy_disposition
	`
	rows, err := tx.Query(ctx, query, returnItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]int)
	for rows.Next() {
		var disp string
		var sum int
		if err := rows.Scan(&disp, &sum); err != nil {
			return nil, err
		}
		res[disp] = sum
	}
	return res, nil
}

func (r *Repository) ValidateOrderItemAllocationForReturnItemTx(ctx context.Context, tx pgx.Tx, returnItemID, orderItemAllocID, currentAllocID uuid.UUID) error {
	var matchesOrderItem bool
	var alreadyBound bool

	query := `
		SELECT
			(oia.order_item_id = ri.order_item_id) AS matches_order_item,
			EXISTS (
				SELECT 1 FROM return_responsibility_allocations rra
				WHERE rra.order_item_allocation_id = $2 AND rra.id != $3
			) AS already_bound
		FROM return_items ri
		JOIN order_item_allocations oia ON oia.id = $2
		WHERE ri.id = $1
	`
	err := tx.QueryRow(ctx, query, returnItemID, orderItemAllocID, currentAllocID).Scan(&matchesOrderItem, &alreadyBound)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrResponsibilityInvariants
		}
		return err
	}

	if !matchesOrderItem {
		return ErrResponsibilityInvariants
	}
	if alreadyBound {
		return ErrAllocationAlreadyBound
	}
	return nil
}

func (r *Repository) UpdateReturnResponsibilityAllocationTx(ctx context.Context, tx pgx.Tx, alloc *ReturnResponsibilityAllocation) error {
	query := `
		UPDATE return_responsibility_allocations
		SET order_item_allocation_id = $2, quantity = $3, status = $4, responsible_party = $5, reason_code = $6, decision_source = $7, internal_note = $8, decided_at = $9, actor_id = $10, legacy_disposition = $11, updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`
	err := tx.QueryRow(ctx, query, alloc.ID, alloc.OrderItemAllocationID, alloc.Quantity, alloc.Status, alloc.ResponsibleParty, alloc.ReasonCode, alloc.DecisionSource, alloc.InternalNote, alloc.DecidedAt, alloc.ActorID, alloc.LegacyDisposition).Scan(&alloc.UpdatedAt)
	if err != nil {
		return err
	}

	return r.insertAllocationHistoryTx(ctx, tx, alloc)
}

func (r *Repository) InsertReturnResponsibilityAllocationTx(ctx context.Context, tx pgx.Tx, alloc *ReturnResponsibilityAllocation) error {
	query := `
		INSERT INTO return_responsibility_allocations (id, return_item_id, order_item_allocation_id, quantity, status, responsible_party, reason_code, decision_source, internal_note, decided_at, actor_id, legacy_disposition, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), now())
		RETURNING created_at, updated_at
	`
	err := tx.QueryRow(ctx, query, alloc.ID, alloc.ReturnItemID, alloc.OrderItemAllocationID, alloc.Quantity, alloc.Status, alloc.ResponsibleParty, alloc.ReasonCode, alloc.DecisionSource, alloc.InternalNote, alloc.DecidedAt, alloc.ActorID, alloc.LegacyDisposition).Scan(&alloc.CreatedAt, &alloc.UpdatedAt)
	if err != nil {
		return err
	}

	return r.insertAllocationHistoryTx(ctx, tx, alloc)
}

func (r *Repository) insertAllocationHistoryTx(ctx context.Context, tx pgx.Tx, alloc *ReturnResponsibilityAllocation) error {
	query := `
		INSERT INTO return_responsibility_allocation_history (id, allocation_id, return_item_id, quantity, order_item_allocation_id, status, responsible_party, reason_code, decision_source, internal_note, actor_id, decided_at, legacy_disposition, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now())
	`
	_, err := tx.Exec(ctx, query, uuid.New(), alloc.ID, alloc.ReturnItemID, alloc.Quantity, alloc.OrderItemAllocationID, alloc.Status, alloc.ResponsibleParty, alloc.ReasonCode, alloc.DecisionSource, alloc.InternalNote, alloc.ActorID, alloc.DecidedAt, alloc.LegacyDisposition)
	return err
}

func (r *Repository) GetReturnResponsibilityAllocationHistory(ctx context.Context, allocationID uuid.UUID) ([]ReturnResponsibilityAllocationHistory, error) {
	query := `
		SELECT id, allocation_id, return_item_id, quantity, order_item_allocation_id, status, responsible_party, reason_code, decision_source, internal_note, actor_id, decided_at, legacy_disposition, created_at
		FROM return_responsibility_allocation_history
		WHERE allocation_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := r.db.Query(ctx, query, allocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []ReturnResponsibilityAllocationHistory
	for rows.Next() {
		var h ReturnResponsibilityAllocationHistory
		if err := rows.Scan(
			&h.ID, &h.AllocationID, &h.ReturnItemID, &h.Quantity, &h.OrderItemAllocationID, &h.Status, &h.ResponsibleParty, &h.ReasonCode, &h.DecisionSource, &h.InternalNote, &h.ActorID, &h.DecidedAt, &h.LegacyDisposition, &h.CreatedAt,
		); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}
