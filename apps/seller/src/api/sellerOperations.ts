import { InventoryItem, SellerOrder, SellerReturn, SellerReview } from '@zamk/api-client/src/types';

export function adaptInventory(items: InventoryItem[]) {
  return items.map(item => ({
    productId: item.productId,
    availableStock: item.quantityAvailable,
    reservedStock: item.quantityReserved,
    totalStock: item.quantityAvailable + item.quantityReserved,
  }));
}

export function adaptOrders(orders: SellerOrder[]) {
  return orders.map(order => ({
    id: order.id,
    status: order.status,
    totalPriceCents: order.totalPriceCents,
    createdAt: order.createdAt,
  }));
}

export function adaptReturns(returns: SellerReturn[]) {
  return returns.map(ret => ({
    id: ret.id,
    status: ret.status,
  }));
}

export function adaptReviews(reviews: SellerReview[]) {
  return reviews.map(rev => ({
    id: rev.id,
    rating: rev.rating,
    content: rev.content,
    status: rev.status,
  }));
}
