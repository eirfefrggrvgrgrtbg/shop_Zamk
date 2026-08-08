import { SellerReturn, SellerReview } from '@zamk/api-client/src/types';

// P0 fix: backend returns { items, totalCount } wrapper; unwrap defensively
function unwrap<T>(data: T[] | { items?: T[] } | null | undefined): T[] {
  if (!data) return [];
  return Array.isArray(data) ? data : (data.items ?? []);
}

export interface SellerOrder {
  id: string;
  orderNumber: string;
  createdAt: string;
  status: string;
  commercialStatus: string;
  deliveryStatus: string;
  totalPriceCents?: number;
  sellerItemCount: number;
  sellerUnits: number;
  sellerGrossAmount: number;
  items: any[]; // OrderItem[] but any is okay for this type issue
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
  return unwrap(data);
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
