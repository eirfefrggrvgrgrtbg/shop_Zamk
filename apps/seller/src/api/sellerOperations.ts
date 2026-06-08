import { InventoryItem, SellerOrder, SellerReturn, SellerReview } from '@zamk/api-client/src/types';

// P0 fix: backend returns { items, totalCount } wrapper; unwrap defensively
function unwrap<T>(data: T[] | { items?: T[] } | null | undefined): T[] {
  if (!data) return [];
  return Array.isArray(data) ? data : (data.items ?? []);
}

export function adaptInventory(data: InventoryItem[] | { items?: InventoryItem[] }) {
  return unwrap(data).map(item => ({
    productId: item.productId,
    availableStock: item.quantityAvailable,
    reservedStock: item.quantityReserved,
    totalStock: item.quantityAvailable + item.quantityReserved,
  }));
}

export function adaptOrders(data: SellerOrder[] | { items?: SellerOrder[] }) {
  return unwrap(data).map(order => ({
    id: order.id,
    status: order.status,
    totalPriceCents: order.totalPriceCents,
    createdAt: order.createdAt,
  }));
}

export function adaptReturns(data: SellerReturn[] | { items?: SellerReturn[] }) {
  return unwrap(data).map(ret => ({
    id: ret.id,
    status: ret.status,
  }));
}

export function adaptReviews(data: SellerReview[] | { items?: SellerReview[] }) {
  return unwrap(data).map(rev => ({
    id: rev.id,
    rating: rev.rating,
    // P0 fix: backend field is 'comment', not 'content'
    content: rev.comment,
    status: rev.status,
  }));
}
