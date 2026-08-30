package returns

import (
	"context"
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
			SELECT
				id, return_id, provider, method, tracking_number, provider_shipment_id, status, selected_cdek_office_code
			FROM return_shipments
			WHERE return_id = ANY($1)
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
	return &res, nil
}

func (r *Repository) ListAdminReturns(ctx context.Context, limit, offset int, buildURL func(key string) string) ([]AdminReturnResponse, int, error) {
	var totalCount int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM returns").Scan(&totalCount); err != nil {
		return nil, 0, err
	}

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
		ORDER BY r.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
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
	}

	if list == nil {
		list = make([]AdminReturnResponse, 0)
	}
	return list, totalCount, nil
}

func (r *Repository) GetSellerReturnItems(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]SellerReturnItem, error) {
	query := `
		SELECT
			ri.id, ri.return_id, r.order_id, o.order_number, ri.order_item_id,
			r.status, ri.quantity, ri.reason, ri.condition,
			oi.title, oi.variant_size, oi.variant_color, oi.sku, oi.image_url, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.restock, r.admin_comment,
			(SELECT amount_cents FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			(SELECT CASE
				WHEN sle.metadata->>'reason' = 'return_post_payout' THEN 'debt'
				WHEN sle.available_at IS NULL THEN 'debt'
				WHEN sle.available_at > now() THEN 'frozen'
				ELSE 'available' END
			FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			r.created_at, r.updated_at
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
		JOIN orders o ON o.id = r.order_id
		WHERE oi.seller_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SellerReturnItem
	for rows.Next() {
		var item SellerReturnItem
		if err := rows.Scan(&item.ReturnItemID, &item.ReturnID, &item.OrderID, &item.OrderNumber, &item.OrderItemID, &item.Status, &item.Quantity, &item.Reason, &item.Condition, &item.ProductTitle, &item.VariantSize, &item.VariantColor, &item.SKU, &item.ImageURL, &item.PriceCents, &item.SubtotalPriceCents, &item.Restock, &item.AdminComment, &item.FinancialAdjustmentCents, &item.FinancialImpactType, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]SellerReturnItem, 0)
	}
	return list, nil
}

func (r *Repository) GetSellerReturnItemsForReturn(ctx context.Context, sellerID, returnID uuid.UUID) ([]SellerReturnItem, error) {
	query := `
		SELECT
			ri.id, ri.return_id, r.order_id, o.order_number, ri.order_item_id,
			r.status, ri.quantity, ri.reason, ri.condition,
			oi.title, oi.variant_size, oi.variant_color, oi.sku, oi.image_url, oi.price_cents, (oi.price_cents * ri.quantity),
			ri.restock, r.admin_comment,
			(SELECT amount_cents FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			(SELECT CASE
				WHEN sle.metadata->>'reason' = 'return_post_payout' THEN 'debt'
				WHEN sle.available_at IS NULL THEN 'debt'
				WHEN sle.available_at > now() THEN 'frozen'
				ELSE 'available' END
			FROM seller_ledger_entries sle WHERE sle.order_item_id = oi.id AND sle.type = 'adjustment' AND sle.metadata->>'return_id' = r.id::text LIMIT 1),
			r.created_at, r.updated_at
		FROM return_items ri
		JOIN returns r ON r.id = ri.return_id
		JOIN order_items oi ON oi.id = ri.order_item_id
		JOIN orders o ON o.id = r.order_id
		WHERE oi.seller_id = $1 AND r.id = $2
	`
	rows, err := r.db.Query(ctx, query, sellerID, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SellerReturnItem
	for rows.Next() {
		var item SellerReturnItem
		if err := rows.Scan(&item.ReturnItemID, &item.ReturnID, &item.OrderID, &item.OrderNumber, &item.OrderItemID, &item.Status, &item.Quantity, &item.Reason, &item.Condition, &item.ProductTitle, &item.VariantSize, &item.VariantColor, &item.SKU, &item.ImageURL, &item.PriceCents, &item.SubtotalPriceCents, &item.Restock, &item.AdminComment, &item.FinancialAdjustmentCents, &item.FinancialImpactType, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	if list == nil {
		list = make([]SellerReturnItem, 0)
	}
	return list, nil
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
	queryReturn := `
		SELECT r.id, r.order_id, r.fulfillment_id, r.user_id, r.status, r.reason, r.comment, r.admin_comment, r.created_at, r.updated_at, r.approved_at, r.rejected_at, r.completed_at, r.receiving_started_at, o.order_number
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		WHERE r.id = $1
	`
	var ret Return
	var orderNumber *string
	err := r.db.QueryRow(ctx, queryReturn, returnID).Scan(&ret.ID, &ret.OrderID, &ret.FulfillmentID, &ret.UserID, &ret.Status, &ret.Reason, &ret.Comment, &ret.AdminComment, &ret.CreatedAt, &ret.UpdatedAt, &ret.ApprovedAt, &ret.RejectedAt, &ret.CompletedAt, &ret.ReceivingStartedAt, &orderNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnNotFound
		}
		return nil, err
	}

	queryItems := `
		SELECT id, return_id, order_item_id, quantity, reason, condition, restock, accepted_quantity, damaged_quantity, rejected_quantity, created_at
		FROM return_items WHERE return_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, queryItems, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var state AdminReturnReceivingState
	state.Return = ret
	state.OrderNumber = orderNumber
	state.Items = make([]AdminReturnReceivingItem, 0)

	var returnItems []ReturnItem
	for rows.Next() {
		var item ReturnItem
		if err := rows.Scan(&item.ID, &item.ReturnID, &item.OrderItemID, &item.Quantity, &item.Reason, &item.Condition, &item.Restock, &item.AcceptedQuantity, &item.DamagedQuantity, &item.RejectedQuantity, &item.CreatedAt); err != nil {
			return nil, err
		}
		returnItems = append(returnItems, item)
	}

	for _, item := range returnItems {
		// 1. Fetch outbound allocations
		queryOutbound := `
			SELECT oia.id, iu.unit_code, oia.picked_at, oia.released_at, iu.status
			FROM order_item_allocations oia
			JOIN inventory_units iu ON iu.id = oia.inventory_unit_id
			WHERE oia.order_item_id = $1
			ORDER BY oia.created_at ASC
		`
		outRows, err := r.db.Query(ctx, queryOutbound, item.OrderItemID)
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
		unitRows, err := r.db.Query(ctx, queryUnits, item.ID)
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
		WHERE return_id = $1 AND status != 'cancelled'
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
		WHERE return_id = $1 AND status != 'cancelled'
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
