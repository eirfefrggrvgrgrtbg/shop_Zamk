export interface UserDTO {
  id: string;
  name: string;
  email: string;
  role: string;
  status: string;
  mustChangePassword?: boolean;
}

// ---------------------------------------------------------
// PUBLIC DTOs
// ---------------------------------------------------------

export interface ProductSummary {
  id: string;
  title: string;
  slug: string;
  shortDescription?: string;
  priceCents: number;
  oldPriceCents?: number;
  mainImageUrl?: string;
  categoryId?: string;
  brandId?: string;
  sellerId?: string;
  createdAt: string;
  status: string;
  inStock?: boolean;
  // Included from rating summary usually
  rating?: RatingSummary;
}

export interface ProductDetail extends ProductSummary {
  description: string;
  images: string[];
  variants?: ProductVariant[];
  material?: string;
  color?: string;
}

export interface ProductVariant {
  id: string;
  productId: string;
  sku?: string;
  size?: string;
  color?: string;
  barcode?: string;
  priceCents?: number;
  isActive: boolean;
  inStock?: boolean;
}

export interface Category {
  id: string;
  name: string;
  slug: string;
  parentId?: string | null;
}

export interface Brand {
  id: string;
  name: string;
  slug: string;
  logoUrl?: string;
}

export interface PublicReview {
  id: string;
  productId: string;
  rating: number;
  title?: string;
  content: string;
  customerName: string; // usually masked
  createdAt: string;
}

export interface RatingSummary {
  averageRating: number;
  reviewCount: number;
}

// ---------------------------------------------------------
// CUSTOMER DTOs
// ---------------------------------------------------------

export interface CartItem {
  id: string;
  productId: string;
  productVariantId: string;
  quantity: number;
  product?: ProductSummary;
  title?: string;
  priceCents?: number;
  inStock?: boolean;
}

export interface Cart {
  id: string;
  items: CartItem[];
  totalPriceCents: number;
}

export interface OrderItem {
  id: string;
  orderId: string;
  productId: string;
  productVariantId: string;
  sellerId: string;
  title: string;
  productSlug: string;
  variantSize?: string;
  variantColor?: string;
  sku?: string;
  imageUrl?: string;
  priceCents: number;
  quantity: number;
  subtotalPriceCents: number;
  createdAt: string;
}

export interface Order {
  id: string;
  userId: string;
  status: string;
  totalPriceCents: number;
  currency: string;
  customerName: string;
  customerPhone: string;
  customerEmail: string;
  deliveryAddress: string;
  createdAt: string;
  updatedAt: string;
  cancelledAt?: string;
  items: OrderItem[];
}

export interface ReturnRequest {
  orderId: string;
  reason: string;
}

export interface ReturnResponse {
  id: string;
  status: string;
  createdAt: string;
}

export interface ReviewCreateRequest {
  productId: string;
  rating: number;
  title: string;
  content: string;
}

// ---------------------------------------------------------
// SELLER DTOs
// ---------------------------------------------------------

export interface SellerMe {
  user: UserDTO;
  seller: {
    id: string;
    name: string;
    description: string;
    logoUrl?: string;
    status: string;
  };
}

export interface SellerProduct extends ProductDetail {
  // Any extra fields specific to SellerProduct can go here.
}

export interface InventoryItem {
  productId: string;
  quantityAvailable: number;
  quantityReserved: number;
}

export interface SellerOrder {
  id: string;
  status: string;
  totalPriceCents: number;
  createdAt: string;
  items?: OrderItem[];
}

export interface SellerReturn {
  id: string;
  status: string;
  reason?: string;
  condition?: string;
  items?: any[];
}

export interface SellerReview {
  id: string;
  rating: number;
  content: string;
  title?: string;
  status: string;
  createdAt?: string;
  productId?: string;
}

export interface SellerBalance {
  availableBalanceCents: number;
  pendingBalanceCents: number;
  requestedPayoutsCents?: number;
  paidPayoutsCents?: number;
  currency?: string;
}

export interface Payout {
  id: string;
  amountCents: number;
  status: string;
  requestedAt: string;
  approvedAt?: string;
  rejectedAt?: string;
  paidAt?: string;
  comment?: string;
}

// ---------------------------------------------------------
// ADMIN DTOs
// ---------------------------------------------------------

export interface AdminSeller {
  id: string;
  name: string;
  status: string;
}

export interface AdminProductVariant {
  id: string;
  productId: string;
  sku?: string;
  size?: string;
  color?: string;
  barcode?: string;
  priceCents?: number;
  isActive: boolean;
  inStock?: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AdminProductImage {
  id: string;
  productId: string;
  imageUrl: string;
  objectKey?: string;
  altText?: string;
  sortOrder: number;
  createdAt: string;
}

export interface AdminProduct {
  id: string;
  sellerId: string;
  categoryId?: string;
  brandId?: string;
  title: string;
  slug: string;
  description?: string;
  status: string;
  gender?: string;
  color?: string;
  material?: string;
  careInstructions?: string;
  priceCents: number;
  oldPriceCents?: number;
  currency: string;
  mainImageUrl?: string;
  createdAt: string;
  updatedAt: string;
  submittedAt?: string;
  approvedAt?: string;
  publishedAt?: string;
  rejectedAt?: string;
  moderationComment?: string;
  inStock?: boolean;
  variants?: AdminProductVariant[];
  images?: AdminProductImage[];
}

export interface ModerationProduct {
  id: string;
  sellerId: string;
  categoryId?: string;
  brandId?: string;
  title: string;
  slug: string;
  description?: string;
  status: string;
  priceCents: number;
  oldPriceCents?: number;
  currency: string;
  mainImageUrl?: string;
  createdAt: string;
  updatedAt: string;
  submittedAt?: string;
  moderationComment?: string;
  variants?: AdminProductVariant[];
  images?: AdminProductImage[];
}

export interface AdminOrder {
  id: string;
  userId?: string;
  status: string;
  totalPriceCents: number;
  currency: string;
  customerName?: string;
  customerPhone?: string;
  customerEmail?: string;
  deliveryAddress?: string;
  createdAt: string;
  updatedAt?: string;
  cancelledAt?: string;
  items?: OrderItem[];
}

export interface AdminPayment {
  id: string;
  orderId: string;
  provider: string;
  providerPaymentId?: string;
  amountCents: number;
  currency: string;
  status: string;
  createdAt: string;
  updatedAt?: string;
  paidAt?: string;
  failedAt?: string;
  cancelledAt?: string;
}

export interface AdminShipment {
  id: string;
  orderId: string;
  status: string;
  carrier?: string;
  trackingNumber?: string;
  trackingUrl?: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AdminInventoryItem {
  id: string;
  productId: string;
  productVariantId: string;
  sellerId?: string;
  totalStock: number;
  reservedStock: number;
  availableStock: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface AdminInventoryMovement {
  id: string;
  inventoryItemId: string;
  productId: string;
  productVariantId: string;
  sellerId?: string;
  type: string;
  quantity: number;
  reason?: string;
  actorUserId?: string;
  referenceType?: string;
  referenceId?: string;
  createdAt: string;
}

export interface AdminReturn {
  id: string;
  orderId: string;
  userId?: string;
  status: string;
  reason?: string;
  comment?: string;
  adminComment?: string;
  createdAt?: string;
  updatedAt?: string;
  approvedAt?: string;
  rejectedAt?: string;
  completedAt?: string;
  items?: AdminReturnItem[];
}

export interface AdminReturnItem {
  id: string;
  returnId?: string;
  orderItemId: string;
  productTitle?: string;
  quantity: number;
  reason?: string;
  condition?: string;
  restock?: boolean;
  createdAt?: string;
}

export interface AdminRefund {
  id: string;
  returnId?: string;
  paymentId?: string;
  orderId: string;
  amountCents: number;
  currency: string;
  status: string;
  provider?: string;
  providerRefundId?: string;
  reason?: string;
  createdAt?: string;
  updatedAt?: string;
  processedAt?: string;
  failedAt?: string;
}

export interface AdminPayout {
  id: string;
  sellerId: string;
  sellerName?: string;
  amountCents: number;
  currency: string;
  status: string;
  requestedAt?: string;
  approvedAt?: string;
  rejectedAt?: string;
  paidAt?: string;
  adminUserId?: string;
  comment?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AdminReview {
  id: string;
  productId: string;
  productTitle?: string;
  sellerId?: string;
  sellerName?: string;
  rating: number;
  title?: string;
  comment?: string;
  status: string;
  createdAt?: string;
  publishedAt?: string;
  rejectedAt?: string;
  moderationComment?: string;
}
