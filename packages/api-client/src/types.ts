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
  moderationComment?: string;
  rejectedAt?: string;
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
  average: number;
  count: number;
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

export interface CustomerFulfillmentItem {
  orderItemId: string;
  productId: string;
  productTitle: string;
  variantId?: string | null;
  sku?: string | null;
  quantity: number;
  unitPriceCents: number;
  lineTotalCents: number;
  imageUrl?: string | null;
}

export interface CustomerFulfillment {
  id: string;
  orderId: string;
  sellerId: string;
  sellerName?: string | null;
  status: string;
  createdAt: string;
  updatedAt: string;
  shipmentId?: string | null;
  shipmentStatus?: string | null;
  items: CustomerFulfillmentItem[];
}

export interface ReturnItemRequest {
  orderItemId: string;
  quantity: number;
  reason?: string;
  condition?: string;
}

export interface ReturnRequest {
  reason: string;
  comment?: string;
  items: ReturnItemRequest[];
}

export interface ReturnResponse {
  id: string;
  status: string;
  createdAt: string;
}

export interface ReviewCreateRequest {
  rating: number;
  title?: string;
  comment?: string;
}

// ---------------------------------------------------------
// SELLER DTOs
// ---------------------------------------------------------

export interface SellerMe {
  user: UserDTO;
  sellerUser: {
    id: string;
    sellerId: string;
    userId: string;
    role: string;
  };
  seller: {
    id: string;
    brandName: string;
    slug: string;
    description?: string;
    contactEmail: string;
    contactPhone?: string;
    logoUrl?: string;
    status: string;
    createdAt: string;
    updatedAt: string;
  };
}

export interface UpdateSellerProfileRequest {
  brandName?: string;
  description?: string;
  contactEmail?: string;
  contactPhone?: string;
  slug?: string;
}

export interface ModerationLogEntry {
  id: string;
  productId: string;
  fromStatus?: string;
  toStatus: string;
  comment?: string;
  createdAt: string;
}

export interface ModerationHistoryResponse {
  items: ModerationLogEntry[];
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

export interface SellerFulfillmentItem {
  orderItemId: string;
  productId: string;
  productTitle: string;
  variantId?: string | null;
  sku?: string | null;
  quantity: number;
  unitPriceCents: number;
  lineTotalCents: number;
  imageUrl?: string | null;
}

export interface SellerFulfillment {
  id: string;
  orderId: string;
  sellerId: string;
  sellerName?: string | null;
  status: string;
  subtotalCents: number;
  commissionBps: number;
  sellerAmountCents: number;
  createdAt: string;
  updatedAt: string;
  shipmentStatus?: string | null;
  shipmentId?: string | null;
  deliveryAddress?: string | null;
  customerName?: string | null;
  customerPhone?: string | null;
  items: SellerFulfillmentItem[];
}

export interface SellerReview {
  id: string;
  rating: number;
  comment?: string;
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
  brandName: string;
  slug: string;
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
  fulfillmentId?: string | null;
  status: string;
  carrier?: string;
  trackingNumber?: string;
  trackingUrl?: string;
  shippedAt?: string;
  deliveredAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AdminFulfillmentItem {
  orderItemId: string;
  productId: string;
  productTitle: string;
  variantId?: string | null;
  sku?: string | null;
  quantity: number;
  unitPriceCents: number;
  lineTotalCents: number;
  imageUrl?: string | null;
}

export interface AdminFulfillment {
  id: string;
  orderId: string;
  sellerId: string;
  sellerName?: string | null;
  status: string;
  subtotalCents: number;
  commissionBps: number;
  sellerAmountCents: number;
  createdAt: string;
  updatedAt: string;
  shipmentId?: string | null;
  shipmentStatus?: string | null;
  customerName?: string | null;
  customerPhone?: string | null;
  deliveryAddress?: string | null;
  items: AdminFulfillmentItem[];
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

// ---------------------------------------------------------
// STAFF MANAGEMENT DTOs (Phase C)
// ---------------------------------------------------------

export interface StaffMemberView {
  userId: string;
  name: string;
  email: string;
  userStatus: string;
  mustChangePassword: boolean;
  staffStatus: string;
  roleCode: string;
  roleName: string;
  roleId: string;
  permissions: string[];
  createdAt: string;
  updatedAt: string;
}

export interface StaffRoleWithPermissions {
  id: string;
  code: string;
  name: string;
  description?: string;
  isSystem: boolean;
  permissions: string[];
  createdAt: string;
  updatedAt: string;
}

export interface AdminMeResponse {
  user: {
    id: string;
    name: string;
    email: string;
    role: string;
    status: string;
  };
  staff: {
    roleCode: string;
    roleName: string;
    status: string;
    permissions: string[];
  } | null;
}

export interface CreateStaffMemberRequest {
  name: string;
  email: string;
  phone?: string;
  roleCode: string;
  temporaryPassword: string;
}

export interface CreateStaffMemberResponse {
  userId: string;
  email: string;
  roleCode: string;
  temporaryPasswordReturned: boolean;
}

export interface UpdateStaffRoleRequest {
  roleCode: string;
}

export interface UpdateStaffStatusRequest {
  status: 'active' | 'blocked' | 'archived';
}

export interface ResetStaffPasswordRequest {
  temporaryPassword: string;
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

// ---------------------------------------------------------
// PHASE E: SELLER MANAGEMENT DTOs
// ---------------------------------------------------------

export interface SellerStatusHistoryItem {
  id: string;
  oldStatus?: string;
  newStatus: string;
  reason?: string;
  actorUserId?: string;
  createdAt: string;
}

export interface SellerWarning {
  id: string;
  sellerId: string;
  type: string;
  title: string;
  message: string;
  severity: 'low' | 'medium' | 'high';
  status: 'active' | 'resolved' | 'cancelled';
  actorUserId?: string;
  createdAt: string;
  resolvedAt?: string;
  resolutionNote?: string;
}

export interface SellerViolation {
  id: string;
  sellerId: string;
  type: string;
  title: string;
  description: string;
  severity: 'low' | 'medium' | 'high';
  status: 'active' | 'resolved' | 'cancelled';
  countsForPenalty: boolean;
  actorUserId?: string;
  createdAt: string;
  resolvedAt?: string;
  resolutionNote?: string;
}

export interface SellerDetail {
  id: string;
  brandName: string;
  slug?: string;
  description?: string;
  logoUrl?: string;
  contactEmail?: string;
  contactPhone?: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  owner: {
    id: string;
    name: string;
    email: string;
    status: string;
  };
  counts: {
    warningsActive: number;
    violationsActive: number;
    activePenaltyViolations: number;
  };
  commissionPolicy: {
    baseCommissionBps: number;
    penaltyCommissionBps: number;
    penaltyRule: string;
    currentAppliedCommissionBps: number;
    automaticPenaltyEnabled: boolean;
  };
}

export interface CreateWarningRequest {
  type: string;
  title: string;
  message: string;
  severity: 'low' | 'medium' | 'high';
}

export interface CreateViolationRequest {
  type: string;
  title: string;
  description: string;
  severity: 'low' | 'medium' | 'high';
  countsForPenalty: boolean;
}

// --- Auctions ---
export type AuctionStatus = 'draft' | 'scheduled' | 'live' | 'ended' | 'cancelled' | 'paused';
export type LotStatus = 'draft' | 'active' | 'ended_no_bids' | 'won_pending_payment' | 'paid' | 'unpaid_manual_review' | 'moved_to_direct_sale' | 'cancelled';
export type NoBidsPolicy = 'manual_review' | 'auto_direct_sale';
export type UnpaidWinnerPolicy = 'manual_review' | 'offer_second_bidder';

export interface AuctionEvent {
  id: string;
  title: string;
  description?: string | null;
  status: AuctionStatus;
  startsAt: string;
  endsAt: string;
  bidStepCents: number;
  paymentDeadlineHours: number;
  antiSnipingEnabled: boolean;
  antiSnipingTriggerSeconds: number;
  antiSnipingExtensionSeconds: number;
  maxBidsPerUserPerLotPerMinute: number;
  maxRejectedBidsPerUserPerMinute: number;
  noBidsPolicy: NoBidsPolicy;
  unpaidWinnerPolicy: UnpaidWinnerPolicy;
  isPublic: boolean;
  showOnHomepage: boolean;
  highlightInNav: boolean;
  biddingEnabled: boolean;
  createdBy?: string | null;
  createdAt: string;
  updatedAt: string;
  lots?: AuctionLot[];
}

export interface AuctionLotImage {
  id: string;
  lotId: string;
  imageUrl: string;
  sortOrder: number;
  isPrimary: boolean;
  createdAt: string;
}

export interface AuctionLotAttribute {
  id: string;
  lotId: string;
  name: string;
  value: string;
  sortOrder: number;
}

export interface AuctionLot {
  id: string;
  auctionId: string;
  title: string;
  description?: string | null;
  imageUrl?: string | null;
  startPriceCents: number;
  currentBidCents?: number | null;
  bidStepCents: number;
  currentWinnerUserId?: string | null;
  status: LotStatus;
  orderId?: string | null;
  paymentDeadlineAt?: string | null;
  canRelaunch: boolean;
  canMoveToDirectSale: boolean;
  directSalePriceCents?: number | null;
  directSaleProductId?: string | null;
  adminNote?: string | null;
  createdAt: string;
  updatedAt: string;
  images?: AuctionLotImage[];
  attributes?: AuctionLotAttribute[];
}

export interface AuctionBid {
  id: string;
  auctionId: string;
  lotId: string;
  userId: string;
  amountCents: number;
  idempotencyKey?: string | null;
  createdAt: string;
}

export interface BidRequest {
  amountCents?: number;
  idempotencyKey?: string;
  clientKnownBidCents?: number;
}

export interface BidResponse {
  success: boolean;
  newCurrentBid: number;
  isLeading: boolean;
  lotStatus: LotStatus;
  endsAt: string;
  extensionApplied: boolean;
}

export interface AdminCreateAuctionRequest {
  title: string;
  description?: string | null;
  startsAt: string;
  endsAt: string;
  bidStepCents: number;
  paymentDeadlineHours: number;
  antiSnipingEnabled: boolean;
  antiSnipingTriggerSeconds: number;
  antiSnipingExtensionSeconds: number;
  maxBidsPerUserPerLotPerMinute: number;
  maxRejectedBidsPerUserPerMinute: number;
  noBidsPolicy: NoBidsPolicy;
  unpaidWinnerPolicy: UnpaidWinnerPolicy;
  isPublic: boolean;
  showOnHomepage: boolean;
  highlightInNav: boolean;
  biddingEnabled: boolean;
}

export interface AdminUpdateAuctionRequest {
  title?: string;
  description?: string | null;
  startsAt?: string;
  endsAt?: string;
  bidStepCents?: number;
  paymentDeadlineHours?: number;
  antiSnipingEnabled?: boolean;
  antiSnipingTriggerSeconds?: number;
  antiSnipingExtensionSeconds?: number;
  maxBidsPerUserPerLotPerMinute?: number;
  maxRejectedBidsPerUserPerMinute?: number;
  noBidsPolicy?: NoBidsPolicy;
  unpaidWinnerPolicy?: UnpaidWinnerPolicy;
  isPublic?: boolean;
  showOnHomepage?: boolean;
  highlightInNav?: boolean;
  biddingEnabled?: boolean;
}

export interface AdminCreateLotRequest {
  title: string;
  description?: string | null;
  startPriceCents: number;
  bidStepCents: number;
  canRelaunch: boolean;
  canMoveToDirectSale: boolean;
  directSalePriceCents?: number | null;
  adminNote?: string | null;
  images: { imageUrl: string; sortOrder: number; isPrimary: boolean }[];
  attributes: { name: string; value: string; sortOrder: number }[];
}

export interface AdminUpdateLotRequest {
  title?: string;
  description?: string | null;
  startPriceCents?: number;
  bidStepCents?: number;
  canRelaunch?: boolean;
  canMoveToDirectSale?: boolean;
  directSalePriceCents?: number | null;
}

export interface AdminAuction {
  id: string;
  title: string;
  description?: string;
  status: string;
  startsAt: string;
  endsAt: string;
  bidStepCents: number;
  paymentDeadlineHours: number;
  antiSnipingEnabled: boolean;
  antiSnipingTriggerSeconds: number;
  antiSnipingExtensionSeconds: number;
  maxBidsPerUserPerLotPerMinute: number;
  maxRejectedBidsPerUserPerMinute: number;
  noBidsPolicy: string;
  unpaidWinnerPolicy: string;
  isPublic: boolean;
  showOnHomepage: boolean;
  highlightInNav: boolean;
  biddingEnabled: boolean;
  createdAt?: string;
  updatedAt?: string;
  lotsCount?: number;
}

export interface AdminAuctionLot {
  id: string;
  auctionId: string;
  title: string;
  description?: string;
  mainImageUrl?: string;
  startPriceCents: number;
  currentBidCents?: number;
  bidStepCents: number;
  allowRestart: boolean;
  allowDirectSale: boolean;
  directSalePriceCents?: number;
  adminNote?: string;
  status: string;
  currentWinnerUserId?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AdminAuctionBid {
  id: string;
  auctionId: string;
  lotId: string;
  userId: string;
  amountCents: number;
  createdAt: string;
  isLeader?: boolean;
}

