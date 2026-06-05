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
}

export interface Payout {
  id: string;
  amountCents: number;
  status: string;
  requestedAt: string;
}

// ---------------------------------------------------------
// ADMIN DTOs
// ---------------------------------------------------------

export interface AdminSeller {
  id: string;
  name: string;
  status: string;
}

export interface AdminProduct {
  id: string;
  name: string;
  status: string;
}

export interface ModerationProduct {
  id: string;
  name: string;
  status: string;
}

export interface AdminOrder {
  id: string;
  status: string;
}

export interface AdminPayment {
  id: string;
  amountCents: number;
  status: string;
}

export interface AdminShipment {
  id: string;
  status: string;
}

export interface AdminReturn {
  id: string;
  status: string;
}

export interface AdminRefund {
  id: string;
  amountCents: number;
  status: string;
}

export interface AdminPayout {
  id: string;
  amountCents: number;
  status: string;
}

export interface AdminReview {
  id: string;
  rating: number;
  status: string;
}
