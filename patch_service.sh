#!/bin/bash
sed -i.bak -e 's/func (s \*Service) ListAdminInventory(ctx context.Context, limit, offset int) (InventoryListResponse, error) {/func (s \*Service) ListAdminInventory(ctx context.Context, q, sellerId, source string, lowStock bool, limit, offset int) (AdminInventoryListResponse, error) {/g' backend/internal/inventory/service.go
sed -i.bak -e 's/items, err := s.repo.ListInventory(ctx, limit, offset)/items, total, err := s.repo.ListAdminInventoryRich(ctx, q, sellerId, source, lowStock, limit, offset)/g' backend/internal/inventory/service.go
sed -i.bak -e 's/return InventoryListResponse{}, err/return AdminInventoryListResponse{}, err/g' backend/internal/inventory/service.go
sed -i.bak -e 's/items = \[\]Item{}/items = \[\]AdminInventoryItem{}/g' backend/internal/inventory/service.go
sed -i.bak -e 's/return InventoryListResponse{Items: items, TotalCount: len(items)}, nil/return AdminInventoryListResponse{Items: items, TotalCount: total}, nil/g' backend/internal/inventory/service.go
