export interface UserDTO {
  id: string;
  name: string;
  email: string;
  role: string;
  status: string;
  mustChangePassword?: boolean;
}

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  firstName?: string;
  lastName?: string;
  role: string;
  status: string;
  mustChangePassword?: boolean;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface PaginatedAdminUsersResponse {
  items: AdminUser[];
  total: number;
  limit: number;
  offset: number;
}

export interface SellerOverviewData {
  period: string;
  sales: {
    grossSalesCents: number;
    ordersCount: number;
    itemsSold: number;
    averageOrderValueCents: number;
    deliveredOrders: number;
    cancelledOrders: number;
    returnedOrders: number;
    returnRate: number;
  };
  catalog: {
    productsTotal: number;
    productsPublished: number;
    productsModeration: number;
    productsRejected: number;
    productsDraft: number;
    productsOutOfStock: number;
    productsLowStock: number;
  };
  fulfillment: {
    fulfillmentsNew: number;
    fulfillmentsProcessing: number;
    fulfillmentsShipped: number;
    fulfillmentsDelivered: number;
    fulfillmentsProblematic: number;
    fulfillmentsOverdue: number;
  };
  finance: {
    paidByCustomersCents: number;
    refundsCents: number;
    pendingPayoutCents: number;
    paidPayoutCents: number;
    frozenCents: number;
    platformCommissionCents: number;
    commissionConfigured: boolean;
  };
  quality: {
    rating: number;
    reviewsCount: number;
    warningsActive: number;
    violationsActive: number;
    openReturns: number;
    rejectedProducts: number;
  };
  performance: {
    category: 'no_data' | 'low' | 'attention' | 'stable' | 'high';
    reasons: string[];
  };
  activity: {
    lastSaleAt?: string;
    lastActiveAt?: string;
    daysSinceLastSale?: number;
  };
  profile: {
    onboardingStage: string;
    storeCreated: boolean;
    storeStatus: string;
    profileCompleteness: number;
    missingFields: string[];
    ownerAccessStatus: string;
    lastLoginAt?: string;
  };
}

// ---------------------------------------------------------
// PUBLIC DTOs
// ---------------------------------------------------------

export interface PublicDeliveryMethod {
  id: string;
  code: string;
  name: string;
  priceCents: number;
  estimatedDaysMin?: number;
  estimatedDaysMax?: number;
}

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
  sellerSlug?: string;
  sellerName?: string;
  createdAt: string;
  status: string;
  inStock?: boolean;
  moderationComment?: string;
  rejectedAt?: string;
  // Included from rating summary usually
  rating?: RatingSummary;
  averageRating?: number;
  reviewsCount?: number;
}

export interface ProductDetail extends ProductSummary {
  description: string;
  images: any[];
  variants?: ProductVariant[];
  material?: string;
  color?: string;
  materialComposition?: AdminProductMaterialComposition[];
  careInstructions?: string;
  sizeChart?: AdminProductSizeChart;
}

export interface ProductVariant {
  id: string;
  productId: string;
  sku?: string;
  size?: string;
  sizeName?: string;
  color?: string;
  colorName?: string;
  sellerSku?: string;
  colorId?: string;
  sizeValueId?: string;
  shadeName?: string;
  optionValues?: any;
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
  comment?: string;
  customerName: string; // usually masked
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface RatingSummary {
  average: number;
  count: number;
}

export interface PublicSeller {
  id: string;
  name: string;
  slug: string;
  description?: string;
  logoUrl?: string;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface PublicSellerStorefrontResponse {
  seller: PublicSeller;
  items: ProductSummary[];
  totalCount: number;
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
  attachments?: ReturnMessageAttachmentResponse[];
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
  variantSize?: string | null;
  variantColor?: string | null;
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
  evidenceIds?: string[];
}

export interface ReturnRequest {
  reason: string;
  comment?: string;
  items: ReturnItemRequest[];
}

export interface CustomerReturnEvidence {
  id: string;
  url: string;
  contentType: string;
  sortOrder: number;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface CustomerReturnItem {
  id: string;
  returnId: string;
  orderItemId: string;
  productTitle?: string;
  productImageUrl?: string | null;
  variantSize?: string | null;
  variantColor?: string | null;
  sku?: string | null;
  quantity: number;
  priceCents?: number;
  subtotalPriceCents?: number;
  reason?: string;
  condition?: string;
  evidence?: CustomerReturnEvidence[];
  evidenceIds?: string[];
  restock?: boolean;
  acceptedQuantity?: number;
  damagedQuantity?: number;
  rejectedQuantity?: number;
  createdAt?: string;
}

export interface CustomerReturnRecord {
  id: string;
  orderId: string;
  orderNumber?: string | null;
  fulfillmentId: string;
  userId: string;
  status: string;
  reason: string;
  comment?: string | null;
  adminComment?: string | null;
  createdAt: string;
  updatedAt: string;
  approvedAt?: string | null;
  rejectedAt?: string | null;
  completedAt?: string | null;
  receivingStartedAt?: string | null;
  items: CustomerReturnItem[];
  shipment?: ReturnShipment | null;
}

export interface CustomerReturnListResponse {
  items: CustomerReturnRecord[];
  totalCount: number;
}

export interface ReturnResponse {
  id: string;
  status: string;
  createdAt: string;
  orderId?: string;
  orderNumber?: string | null;
  fulfillmentId?: string;
  userId?: string;
  reason?: string;
  comment?: string;
  adminComment?: string;
  updatedAt?: string;
  approvedAt?: string;
  rejectedAt?: string;
  completedAt?: string;
  receivingStartedAt?: string;
  items?: CustomerReturnItem[];
  returns?: CustomerReturnRecord[];
  shipment?: ReturnShipment | null;
}

export interface ReviewCreateRequest {
  rating: number;
  title?: string;
  comment?: string;
}

// ---------------------------------------------------------
// SELLER DTOs
// ---------------------------------------------------------

export interface SellerSupplyBoxItem {
  id: string;
  boxId: string;
  variantId: string;
  sku: string;
  quantity: number;
}

export interface SellerSupplyBox {
  id: string;
  supplyId?: string;
  boxNumber: string;
  barcode?: string;
  qrToken?: string;
  createdAt?: string;
  items?: SellerSupplyBoxItem[];
}

export interface SellerSupplyItem {
  id: string;
  supplyId: string;
  variantId: string;
  sku: string;
  sellerSku?: string;
  barcode?: string;
  productTitle?: string;
  colorName?: string;
  sizeName?: string;
  expectedQuantity: number;
  acceptedQuantity: number;
  damagedQuantity: number;
  missingQuantity: number;
  extraQuantity: number;
  receivingComment?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface SellerSupply {
  id: string;
  sellerId: string;
  sellerName?: string;
  humanId?: string;
  supplyNumber?: string;
  qrToken?: string;
  status: string;
  handoffMethod: string;
  carrierName?: string;
  trackingNumber?: string;
  expectedArrivalDate?: string;
  totalExpectedBoxes: number;
  totalExpectedItems: number;
  totalAcceptedItems: number;
  skuCount?: number;
  receivingComment?: string;
  createdAt: string;
  shippedAt?: string;
  arrivedAt?: string;
  receivingStartedAt?: string;
  completedAt?: string;
  updatedAt: string;
  items?: SellerSupplyItem[];
  boxes?: SellerSupplyBox[];
}

export interface SellerSupplyUnitLabelBox {
  id: string;
  boxNumber: string;
}

export interface SellerSupplyUnitLabel {
  inventoryUnitId: string;
  unitCode: string;
  unitIndex: number;
  supplyItemId: string;
  productVariantId: string;
  productTitle: string;
  colorName?: string;
  sizeName?: string;
  sellerSku?: string;
  variantBarcode?: string;
  boxNumber?: string;
}

export interface SellerSupplyUnitLabelsResponse {
  supplyId: string;
  supplyNumber: string;
  serialized: boolean;
  totalUnits: number;
  box?: SellerSupplyUnitLabelBox;
  units: SellerSupplyUnitLabel[];
}


export interface CreateSupplyItemRequest {
  variantId: string;
  expectedQuantity: number;
}

export interface CreateSupplyBoxItemRequest {
  variantId: string;
  quantity: number;
}

export interface CreateSupplyBoxRequest {
  items: CreateSupplyBoxItemRequest[];
}

export interface CreateSupplyRequest {
  handoffMethod: string;
  carrierName?: string;
  trackingNumber?: string;
  items: CreateSupplyItemRequest[];
}

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
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface Notification {
  id: string;
  recipientCustomerId?: string | null;
  recipientSellerId?: string | null;
  recipientStaffId?: string | null;
  recipientKind: 'customer' | 'seller' | 'staff';
  type: string;
  title: string;
  body: string;
  entityType?: string;
  entityId?: string | null;
  metadata?: Record<string, any> | null;
  readAt?: string | null;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface PaginatedNotifications {
  items: Notification[];
  totalCount: number;
}

export interface UnreadCountResponse {
  count: number;
}

export interface ModerationHistoryResponse {
  items: ModerationLogEntry[];
}

export interface SellerProduct extends ProductDetail {
  // Any extra fields specific to SellerProduct can go here.
}

export interface SellerInventoryItem {
  variantId: string;
  productId: string;
  productTitle: string;
  image?: string;
  optionValues?: Record<string, any>;
  sku: string;
  onHand: number;
  reserved: number;
  available: number;
  inbound: number;
  availabilityStatus: string;
}

export interface SellerInventoryListResponse {
  items: SellerInventoryItem[];
  totalCount: number;
}

export interface SellerOrder {
  id: string;
  orderNumber?: string;
  createdAt: string;
  commercialStatus: string;
  deliveryStatus: string;
  payoutStatus?: string;
  sellerItemCount: number;
  sellerUnits: number;
  sellerGrossAmount: number;
  sellerRefundAmount: number;
  sellerNetAmount: number;
  items?: OrderItem[];
}

export interface SellerReturn {
  returnItemId: string;
  returnId: string;
  orderId: string;
  orderNumber?: string;
  orderItemId: string;
  status: string;
  quantity: number;
  reason?: string;
  condition?: string;
  evidenceIds?: string[];
  productTitle: string;
  variantSize?: string;
  variantColor?: string;
  sku?: string;
  imageUrl?: string;
  priceCents: number;
  subtotalPriceCents: number;
  restock: boolean;
  adminComment?: string;
  financialAdjustmentCents?: number;
  financialImpactType?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SellerReturnDetailResponse {
  items: SellerReturn[];
  totalCount?: number;
}

export interface SellerFulfillmentItem {
  orderItemId: string;
  productId: string;
  productTitle: string;
  variantId?: string | null;
  variantSize?: string | null;
  variantColor?: string | null;
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
  grossSalesCents: number;
  commissionCents: number;
  adjustmentsCents: number;
  frozenCents: number;
  availableCents: number;
  paidCents: number;
  currency: string;
  nextPayoutAt?: string;
}

export interface PayoutBatch {
  id: string;
  sellerId: string;
  amountCents: number;
  status: string;
  scheduledFor: string;
  processedAt?: string;
  failureReason?: string;
  createdAt: string;
  updatedAt: string;
}

export interface SellerLedgerEntry {
  id: string;
  sellerId: string;
  orderId?: string;
  orderItemId?: string;
  payoutBatchId?: string;
  type: string;
  amountCents: number;
  currency: string;
  availableAt?: string;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface LedgerListResponse {
  items: SellerLedgerEntry[];
  totalCount: number;
}

export interface PayoutBatchListResponse {
  items: PayoutBatch[];
  totalCount: number;
}

// ---------------------------------------------------------
// REVIEWS DTOs
// ---------------------------------------------------------
export interface ProductReview {
  id: string;
  productId: string;
  customerId: string;
  rating: number;
  comment?: string;
  status: 'pending' | 'approved' | 'rejected';
  createdAt: string;
  updatedAt: string;
}

export interface CreateProductReviewRequest {
  productId: string;
  rating: number;
  comment?: string;
}

export interface ModerateReviewRequest {
  status: 'approved' | 'rejected' | 'hidden' | 'blocked';
  comment?: string;
}

export interface ProductReviewModerationLog {
  id: string;
  reviewId: string;
  adminUserId: string;
  fromStatus: string;
  toStatus: string;
  comment?: string;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}


// ---------------------------------------------------------
// ADMIN DTOs
// ---------------------------------------------------------

export interface PerformanceComponent {
  code: string;
  label: string;
  rawValue: number;
  unit: string;
  score: number;
  weight: number;
  explanation: string;
}

export interface AdminSeller {
  id: string;
  brandName?: string | null;
  slug?: string | null;
  status: string;
  isPlatform?: boolean;
  ownerName: string;
  ownerEmail: string;
  warningsActive: number;
  performanceScore?: number | null;
  performanceCategory: string;
  performanceReasons: string[];
  grossSales30d: number;
  ordersCount30d: number;
  cancelRate30d: number;
  violations: number;
  averageRating: number;
  reviewsCount: number;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface CreateAdminSellerRequest {
  ownerName: string;
  ownerEmail: string;
  grantExistingUser?: boolean;
}

export interface CreateAdminSellerResponse {
  status: 'created_new' | 'granted_existing' | string;
  seller: AdminSeller;
  ownerUser: any;
  temporaryPassword?: string;
  temporaryPasswordReturned: boolean;
}

export interface AdminProductVariant {
  id: string;
  productId: string;
  sku?: string;
  sellerSku?: string;
  size?: string;
  sizeValue?: string;
  color?: string;
  colorName?: string;
  shadeName?: string;
  barcode?: string;
  priceCents?: number;
  isActive: boolean;
  inStock?: boolean;
  hasInventoryRecord?: boolean;
  totalStock?: number;
  reservedStock?: number;
  availableStock?: number;
  createdAt: string;
  updatedAt: string;
}

export interface AdminProductImage {
  id: string;
  productId: string;
  imageUrl: string;
  objectKey?: string;
  renditionUrl?: string;
  renditionObjectKey?: string;
  altText?: string;
  sortOrder: number;
  colorId?: string;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface AdminProductMaterialComposition {
  productId?: string;
  materialId: string;
  materialName?: string;
  percentage: number;
}

export interface AdminProductSizeChartRow {
  sizeChartId?: string;
  sizeValueId: string;
  sizeValueName?: string;
  measurements: Record<string, any>;
}

export interface AdminProductSizeChart {
  id?: string;
  productId?: string;
  categoryId?: string;
  rows: AdminProductSizeChartRow[];
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
  averageRating?: number;
  reviewsCount?: number;
  variants?: AdminProductVariant[];
  images?: AdminProductImage[];
  materialComposition?: AdminProductMaterialComposition[];
  sizeChart?: AdminProductSizeChart;
  attributes?: any[];
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

export interface PaginatedAdminProductsResponse {
  items: AdminProduct[];
  totalCount: number;
}

export interface AdminOrder {
  id: string;
  userId?: string;
  orderNumber?: string;
  status: string;
  paymentStatus?: string;
  fulfillmentStatus: string;
  sourceType: string;
  totalPriceCents: number;
  currency: string;
  customerName?: string;
  customerEmail?: string;
  createdAt: string;
  updatedAt?: string;
  cancelledAt?: string;
}

export interface OrderTimelineEvent {
  id: string;
  type: string;
  title: string;
  timestamp: string;
  comment?: string;
  context?: string;
}

export interface AdminOrderDetail extends AdminOrder {
  customerPhone?: string;
  deliveryAddress?: string;
  deliveryMethodId?: string;
  deliveryMethodCode?: string;
  deliveryMethodName?: string;
  deliveryPriceCents?: number;
  deliveryEstimatedDaysMin?: number;
  deliveryEstimatedDaysMax?: number;
  items?: OrderItem[];
  fulfillments?: AdminFulfillment[];
  timeline?: OrderTimelineEvent[];
}

export interface PaymentProblem {
  code: string;
  severity: string;
}

export interface CustomerSummaryDTO {
  id: string;
  name: string;
  email: string;
  phone: string;
}

export interface AdminPayment {
  paymentId: string;
  paymentNumber: string;
  providerPaymentId: string | null;

  orderId: string;
  orderNumber: string;

  customer: CustomerSummaryDTO | null;

  amountCents: number;
  currency: string;

  status: string;
  provider: string | null;
  paymentMethod: string | null;
  integrationMode: string | null;

  attemptNumber: number;
  attemptsCount: number;

  refundState: string;
  paidAmountCents: number;
  succeededRefundedAmountCents: number;
  pendingRefundAmountCents: number;
  reservedRefundAmountCents: number;
  netAmountCents: number;
  availableToRefundCents: number;

  problems: PaymentProblem[];

  createdAt: string;
  updatedAt: string;
  paidAt: string | null;
  failedAt: string | null;
  cancelledAt: string | null;
}

export interface OrderSummaryDTO {
  orderId: string;
  orderNumber: string;
  orderStatus: string;
  orderTotalCents: number;
  customer: CustomerSummaryDTO | null;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface AdminPaymentAttempt {
  paymentId: string;
  paymentNumber: string;
  attemptNumber: number;
  status: string;
  provider: string | null;
  paymentMethod: string | null;
  amountCents: number;
  providerPaymentId: string | null;
  createdAt: string;
  terminalAt: string | null;
}

export interface SafePaymentEvent {
  eventId: string;
  eventType: string;
  signatureValid: boolean;
  processedAt: string | null;
  createdAt: string;
  eventKey: string;
  safePayloadSummary: Record<string, any>;
}

export interface AdminPaymentRefund {
  refundId: string;
  status: string;
  amountCents: number;
  providerRefundId: string | null;
  createdAt: string;
  processedAt: string | null;
}

export interface AdminPaymentDetail {
  payment: AdminPayment;
  order: OrderSummaryDTO;
  attempts: AdminPaymentAttempt[];
  providerEvents: SafePaymentEvent[];
  refunds: AdminPaymentRefund[];
  problems: PaymentProblem[];
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

export interface AdminAllocatedUnit {
  inventoryUnitId: string;
  unitCode: string;
  pickedAt?: string | null;
}

export interface AdminFulfillmentItem {
  orderItemId: string;
  productId: string;
  productTitle: string;
  title?: string;
  variantId?: string | null;
  variantSize?: string | null;
  variantColor?: string | null;
  sku?: string | null;
  quantity: number;
  unitPriceCents: number;
  lineTotalCents: number;
  imageUrl?: string | null;
  allocationMode?: 'serialized' | 'legacy';
  allocatedUnits?: AdminAllocatedUnit[];
}

export interface AdminFulfillment {
  id: string;
  orderId: string;
  orderNumber?: string | null;
  sellerId: string;
  sellerName?: string | null;
  status: string;
  subtotalCents: number;
  commissionBps: number;
  sellerAmountCents: number;
  receivingCode?: string | null;
  receivingQrToken?: string | null;
  packedAt?: string | null;
  acceptedAt?: string | null;
  acceptedByStaffId?: string | null;
  receivingResult?: any;
  discrepancyReason?: string | null;
  discrepancyComment?: string | null;
  discrepancyAt?: string | null;
  createdAt: string;
  updatedAt: string;
  shipmentId?: string | null;
  shipmentStatus?: string | null;
  customerName?: string | null;
  customerPhone?: string | null;
  deliveryAddress?: string | null;
  items: AdminFulfillmentItem[];
}

export interface PhysicalUnitContext {
  unitCode: string;
  status: string;
  statusLabel: string;
  productTitle: string;
  variant: string;
  sku: string;
  productId: string;
  variantId: string;
}

export interface AggregateStock {
  total: number;
  reserved: number;
  available: number;
}

export interface PhysicalStock {
  warehouse: number;
  allocated: number;
  picked: number;
  free: number;
  expected: number;
  damaged: number;
  writtenOff: number;
  shipped: number;
  staleAllocated?: number;
}

export interface LegacyStock {
  onHand: number;
  reserved: number;
  available: number;
}

export interface InventoryHealth {
  status: 'healthy' | 'warning' | 'critical';
  issues: string[];
}

export interface InventoryProductInfo {
  id: string;
  title: string;
  slug: string;
  mainImageUrl?: string;
}

export interface InventoryVariantInfo {
  id: string;
  sku: string;
  sellerSku?: string;
  barcode?: string;
  size?: string;
  color?: string;
  label: string;
}

export interface InventorySellerInfo {
  id: string;
  name: string;
}

export interface AdminInventoryListResponse {
  items: AdminInventoryItem[];
  totalCount: number;
  issuesCount?: number;
  unitContext?: PhysicalUnitContext;
}

export interface AdminInventoryItem {
  id: string;
  productId: string;
  productVariantId: string;
  productTitle?: string;
  variant?: string;
  sellerId?: string;
  sellerName?: string;
  source?: string;
  totalStock: number;
  reservedStock: number;
  availableStock: number;
  createdAt?: string;
  updatedAt?: string;

  product?: InventoryProductInfo;
  variantInfo?: InventoryVariantInfo;
  seller?: InventorySellerInfo;
  aggregate?: AggregateStock;
  physical?: PhysicalStock;
  legacy?: LegacyStock;
  accountingMode?: 'serialized' | 'mixed' | 'legacy';
  health?: InventoryHealth;
  physicalUnits?: AdminInventoryPhysicalUnit[];
}

export interface AdminInventoryAllocationInfo {
  id: string;
  orderId: string;
  orderNumber: string;
  orderStatus: string;
  fulfillmentId?: string;
  fulfillmentStatus?: string;
  pickedAt?: string;
}

export interface AdminInventorySupplyLineage {
  supplyId: string;
  supplyNumber: string;
  supplyStatus: string;
  receivedAt?: string;
}

export interface AdminInventoryPhysicalUnit {
  id: string;
  unitCode: string;
  status: string; // 'expected' | 'warehouse' | 'damaged' | 'written_off' | 'shipped'
  createdAt: string;
  availability: string; // 'free' | 'allocated' | 'picked' | 'unavailable_expected' | 'unavailable_damaged' | 'unavailable_written_off' | 'unavailable_shipped'
  isStaleAllocation: boolean;
  liveAllocation?: AdminInventoryAllocationInfo;
  staleAllocation?: AdminInventoryAllocationInfo;
  supplyLineage?: AdminInventorySupplyLineage;
}

export interface AdminInventoryUnitIdentity {
  id: string;
  unitCode: string;
  variantId: string;
  productId: string;
  productTitle: string;
  variantName: string;
  sku: string;
  barcode: string;
  size: string;
  color: string;
  sellerId: string;
  sellerName: string;
  source: string;
}

export interface AdminInventoryUnitCurrentState {
  status: string;
  availability: string;
  location: string;
  isStaleAllocation: boolean;
  healthIssue?: string;
}

export interface AdminInventoryUnitContext {
  liveAllocation?: AdminInventoryAllocationInfo;
  staleAllocation?: AdminInventoryAllocationInfo;
}

export interface AdminInventoryUnitTimelineEvent {
  id: string;
  type: string;
  category: 'physical' | 'commitment' | 'operation' | 'order_lifecycle' | 'diagnostic';
  eventName: string;
  description: string;
  timestamp: string;
  sourceEntity: string;
  referenceNumber?: string;
  referenceId?: string;
  actorRole?: 'system' | 'staff' | 'customer' | 'seller';
  actorName?: string;
  link?: string;
}

export interface AdminInventoryUnitTraceability {
  identity: AdminInventoryUnitIdentity;
  currentState: AdminInventoryUnitCurrentState;
  origin?: AdminInventorySupplyLineage;
  currentContext: AdminInventoryUnitContext;
  timeline: AdminInventoryUnitTimelineEvent[];
  hasPartialHistory: boolean;
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
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface AdminReturnEvidence {
  id: string;
  url: string;
  contentType: string;
  sortOrder: number;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface AdminReturnItem {
  id: string;
  returnId?: string;
  orderItemId: string;
  productTitle?: string;
  productImageUrl?: string;
  variantSize?: string;
  variantColor?: string;
  sku?: string;
  quantity: number;
  priceCents: number;
  subtotalPriceCents: number;
  reason?: string;
  condition?: string;
  evidenceIds?: string[];
  evidence?: AdminReturnEvidence[];
  restock?: boolean;
  acceptedQuantity?: number;
  damagedQuantity?: number;
  rejectedQuantity?: number;
  createdAt?: string;
}

export interface AdminReturn {
  id: string;
  orderId: string;
  orderNumber?: string;
  fulfillmentId?: string;
  userId?: string;
  customerName?: string;
  customerEmail?: string;
  customerPhone?: string;
  sellerId?: string;
  sellerName?: string;
  status: string;
  reason?: string;
  comment?: string;
  adminComment?: string;
  createdAt?: string;
  updatedAt?: string;
  approvedAt?: string;
  rejectedAt?: string;
  completedAt?: string;
  deliveredAt?: string;
  evidenceCount?: number;
  receivingStartedAt?: string;
  items?: AdminReturnItem[];
  shipment?: ReturnShipment;
  shipmentStatus?: string | null;
  shipmentMethod?: string | null;
}

export interface OutboundAllocationDetail {
  id?: string;
  allocationId?: string;
  inventoryUnitId?: string;
  unitCode?: string;
  status?: string;
  unitStatus?: string;
  allocatedAt?: string;
  pickedAt?: string;
  releasedAt?: string;
  shippedAt?: string;
}

export interface ScannedUnitDetail {
  id: string;
  returnId?: string;
  returnItemId: string;
  orderItemAllocationId?: string;
  inventoryUnitId?: string;
  unitCode: string;
  disposition?: 'restock' | 'damaged' | 'reject' | string;
  inspectedCondition?: string;
  scannedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface AdminReturnReceivingItem {
  returnItem: AdminReturnItem;
  productTitle?: string;
  productImageUrl?: string;
  variantSize?: string;
  variantColor?: string;
  sku?: string;
  priceCents?: number;
  allocationMode: 'serialized' | 'legacy';
  outboundAllocations: OutboundAllocationDetail[];
  scannedUnits: ScannedUnitDetail[];
  requestedQuantity: number;
  scannedQuantity: number;
  remainingQuantity: number;
  notReceivedQuantity?: number;
  acceptedQuantity: number;
  damagedQuantity: number;
  rejectedQuantity: number;
  canFinalize: boolean;
}

export interface AdminReturnReceivingState {
  return: AdminReturn & {
    fulfillmentId?: string;
    receivingStartedAt?: string;
  };
  orderNumber?: string;
  items: AdminReturnReceivingItem[];
  totalRequested: number;
  totalScanned: number;
  totalRemaining: number;
  serializedRequested: number;
  serializedScanned: number;
  legacyRequested: number;
  canFinalize: boolean;
}

export interface ScanReturnUnitResponse {
  scannedUnit: ScannedUnitDetail;
  item: AdminReturnReceivingItem;
  canFinalize: boolean;
  isDuplicate?: boolean;
  alreadyScanned?: boolean;
}

export interface UpdateSerializedUnitInspectionInput {
  disposition: 'restock' | 'damaged' | 'reject';
  inspectedCondition?: string;
}

export interface UpdateLegacyItemInspectionInput {
  acceptedQuantity: number;
  damagedQuantity: number;
  rejectedQuantity: number;
  condition?: string;
}

export interface AdminReturnRefundQuoteItem {
  orderItemId: string;
  productTitle: string;
  mode: 'serialized' | 'legacy' | string;
  requestedQuantity: number;
  refundableQuantity: number;
  unitPriceCents: number;
  refundCents: number;
}

export interface AdminReturnRefundQuote {
  returnId: string;
  orderNumber?: string | null;
  currency: string;
  items: AdminReturnRefundQuoteItem[];
  productsRefundCents: number;
  deliveryRefundCents: number;
  totalRefundCents: number;
  alreadyRefundedCents: number;
  succeededRefundedCents?: number;
  pendingRefundCents?: number;
  remainingRefundableCents: number;
  canRefund: boolean;
  blockingReason?: string | null;
  latestRefundStatus?: 'pending' | 'succeeded' | 'failed' | null;
  latestRefundProcessedAt?: string | null;
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
  attachments?: ReturnMessageAttachmentResponse[];
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
  isPlatform?: boolean;
  averageRating?: number;
  reviewsCount?: number;
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
  attachments?: ReturnMessageAttachmentResponse[];
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
  attachments?: ReturnMessageAttachmentResponse[];
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
  canRelaunch: boolean;
  canMoveToDirectSale: boolean;
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

// ---------------------------------------------------------
// DASHBOARD DTOs
// ---------------------------------------------------------

export interface DashboardOverviewMetrics {
  totalOrders: number;
  ordersToday: number;
  revenueTodayCents: number;
  revenue7dCents: number;
  pendingModeration: number;
  activeSellers: number;
  activeProducts: number;
  lowStockCount: number;

  averageDailyOrders20d: number;
  averageDailyRevenue20dCents: number;
  previousRevenue7dCents: number;
  averageOrderValue7dCents: number;
  previousAverageOrderValue7dCents: number;
  returns7d: number;
  previousReturns7d: number;
}

export interface DashboardOrdersMetrics {
  newOrPending: number;
  paid: number;
  inFulfillment: number;
  shippedOrDelivered: number;
  cancelledOrRefunded: number;
}

export interface DashboardSellersMetrics {
  active: number;
  waitingModeration: number;
  blocked: number;
}

export interface DashboardProductsMetrics {
  published: number;
  pendingModeration: number;
  rejectedOrBlocked: number;
  outOfStock: number;
}

export interface DashboardAuctionsMetrics {
  active: number;
  awaitingPayment: number;
  unpaidManualReview: number;
  directSaleItems: number;
}

export interface DashboardInventoryMetrics {
  lowStockVariants: number;
  reservedStock: number;
  outOfStockCount: number;
}

export interface DashboardPaymentsMetrics {
  paidOrdersSumCents: number;
  pendingPayoutsCents: number;
  paidPayoutsCents: number;
  failedPaymentsCount: number;
}

export interface DashboardAttentionItem {
  title: string;
  count: number;
  severity: string;
  link?: string;
}

export interface AdminDashboardSummary {
  overview: DashboardOverviewMetrics;
  orders: DashboardOrdersMetrics;
  sellers: DashboardSellersMetrics;
  products: DashboardProductsMetrics;
  auctions: DashboardAuctionsMetrics;
  inventory: DashboardInventoryMetrics;
  payments: DashboardPaymentsMetrics;
  attention: DashboardAttentionItem[];
}


export interface AdminPayoutSummary {
  totalAvailableCents: number;
  totalPendingCents: number;
  totalPaidCents: number;
  totalRejectedCents: number;
  totalCommissionCents: number;
  currency: string;
}

export interface AdminSellerBalance {
  sellerId: string;
  sellerName: string;
  pendingBalanceCents: number;
  availableBalanceCents: number;
  currency: string;
}



// ---- Phase E: Notes and Plans ----

export interface SellerNote {
  id: string;
  sellerId: string;
  authorId?: string;
  noteType: string;
  content: string;
  deadline?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSellerNoteRequest {
  noteType: string;
  content: string;
  deadline?: string;
}

export interface SellerImprovementPlan {
  id: string;
  sellerId: string;
  assigneeId?: string;
  assigneeName?: string;
  creatorId?: string;
  creatorName?: string;
  status: string;
  reason: string;
  actions: { title: string; isCompleted: boolean }[];
  deadline?: string;
  internalComment?: string;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface CreateSellerImprovementPlanRequest {
  assigneeId?: string;
  reason: string;
  actions: { title: string; isCompleted: boolean }[];
  deadline?: string;
  internalComment?: string;
}

// ---------------------------------------------------------
// WAREHOUSE RECEIVING DTOs
// ---------------------------------------------------------

export interface SupplyReceivingSession {
  id: string;
  supplyId: string;
  staffId: string;
  status: string;
  startedAt: string;
  endedAt?: string;
  receivingMode?: 'serialized' | 'legacy';
  items?: ReceivingItem[];
}

export interface ReceivingItem {
  id: string;
  sessionId: string;
  supplyItemId?: string;
  variantId?: string;
  sku: string;
  barcode?: string;
  productTitle: string;
  expectedQuantity: number;
  scannedQuantity: number;
  damagedQuantity: number;
  unexpectedQuantity: number;
}

export interface RecordReceivingScanRequest {
  variantId: string;
  quantity: number;
  isDamage?: boolean;
}

export interface RecordSerializedScanRequest {
  unitCode: string;
  condition: 'ok' | 'damaged';
}

export interface SerializedScanResponse {
  scanId: string;
  unitCode: string;
  condition: string;
  productVariantId: string;
  productTitle: string;
  colorName?: string;
  sizeName?: string;
  sellerSku?: string;
  variantBarcode?: string;
  expected: number;
  scanned: number;
  ok: number;
  damaged: number;
  remaining: number;
}

export interface SerializedRecentScan {
  scanId: string;
  unitCode: string;
  condition: string;
  scannedAt: string;
  voidedAt?: string;
  productTitle: string;
  colorName?: string;
  sizeName?: string;
  sellerSku?: string;
  variantBarcode?: string;
}

export interface UndoSerializedScanResponse {
  scanId: string;
  voidedAt: string;
  expected: number;
  scanned: number;
  ok: number;
  damaged: number;
  remaining: number;
}

export interface FinalizeReceivingRequest {
  notes?: string;
}

// ---------------------------------------------------------
// RETURN SHIPMENT / LOGISTICS DTOs (M5.3.3A)
// ---------------------------------------------------------

export type ReturnShipmentStatus =
  | 'draft'
  | 'awaiting_handover'
  | 'handed_over'
  | 'in_transit'
  | 'arrived_at_zamk'
  | 'cancelled';

export type ReturnShipmentMethod = 'cdek_courier' | 'cdek_office';

export interface PickupAddress {
  city: string;
  street: string;
  house: string;
  flat?: string;
}

export interface CDEKOffice {
  code: string;
  name: string;
  address: string;
  workingHours?: string;
}

export interface CreateReturnShipmentRequest {
  method: ReturnShipmentMethod;
  cdekOfficeCode?: string;
  customerName?: string;
  customerPhone?: string;
  pickupAddress?: PickupAddress;
}

export interface ReturnShipment {
  id: string;
  provider: 'cdek' | string;
  method: ReturnShipmentMethod;
  trackingNumber?: string;
  providerShipmentId?: string;
  status: ReturnShipmentStatus;
  selectedCdekOfficeCode?: string;
  customerName?: string;
  customerPhone?: string;
  pickupAddress?: PickupAddress;
  cdekOfficeAddress?: string;
}

export const RETURN_SHIPMENT_STATUS_LABELS: Record<ReturnShipmentStatus, string> = {
  draft: 'Оформляем отправление',
  awaiting_handover: 'Ожидает передачи в СДЭК',
  handed_over: 'Передано в СДЭК',
  in_transit: 'В пути',
  arrived_at_zamk: 'Прибыло на склад ZAMK',
  cancelled: 'Отправление отменено',
};

export const formatReturnShipmentStatus = (status: string): string => {
  return RETURN_SHIPMENT_STATUS_LABELS[status as ReturnShipmentStatus] || status;
};

export const RETURN_SHIPMENT_METHOD_LABELS: Record<ReturnShipmentMethod, string> = {
  cdek_courier: 'Заберёт курьер СДЭК',
  cdek_office: 'Отнести в отделение СДЭК',
};

export const formatReturnShipmentMethod = (method: string): string => {
  return RETURN_SHIPMENT_METHOD_LABELS[method as ReturnShipmentMethod] || method;
};

// ---------------------------------------------------------
// CUSTOMER RETURN STATUS & REASON LABELS
// ---------------------------------------------------------

export const CUSTOMER_RETURN_STATUS_LABELS: Record<string, string> = {
  requested: 'Заявка на рассмотрении',
  needs_info: 'Требуется уточнение',
  approved: 'Возврат одобрен',
  rejected: 'Возврат отклонён',
  receiving: 'Принимаем возврат',
  item_received: 'Товар принят',
  refunded: 'Деньги возвращены',
  completed: 'Возврат завершён',
  cancelled: 'Возврат отменён',
};

export const formatCustomerReturnStatus = (status?: string): string => {
  if (!status) return '—';
  return CUSTOMER_RETURN_STATUS_LABELS[status] || status;
};

export const RETURN_REASON_LABELS: Record<string, string> = {
  defective: 'Товар неисправен',
  damaged: 'Товар повреждён',
  wrong_item: 'Получен не тот товар',
  not_as_described: 'Не соответствует описанию',
  incomplete: 'Не хватает части комплекта',
  size_fit: 'Не подошёл размер / посадка',
  changed_mind: 'Передумал',
  other: 'Другое',
};

export const formatReturnReason = (reason?: string): string => {
  if (!reason) return '—';
  return RETURN_REASON_LABELS[reason] || reason;
};


export const ACTIVE_RETURN_STATUSES = [
  'requested',
  'needs_info',
  'approved',
  'receiving',
  'item_received',
] as const;

export const TERMINAL_RETURN_STATUSES = [
  'rejected',
  'refunded',
  'completed',
  'cancelled',
] as const;

export type ActiveReturnStatus = (typeof ACTIVE_RETURN_STATUSES)[number];
export type TerminalReturnStatus = (typeof TERMINAL_RETURN_STATUSES)[number];

export const isReturnConversationWritable = (status?: string): boolean => {
  if (!status) return false;
  return (ACTIVE_RETURN_STATUSES as readonly string[]).includes(status);
};

export const isReturnConversationTerminal = (status?: string): boolean => {
  if (!status) return false;
  return (TERMINAL_RETURN_STATUSES as readonly string[]).includes(status);
};

export type ReturnMessageSenderRole = 'customer' | 'admin';
export type ReturnMessageType = 'message' | 'info_request';

export interface ReturnMessage {
  id: string;
  returnId: string;
  senderRole: ReturnMessageSenderRole;
  messageType: ReturnMessageType;
  body: string;
  createdAt: string;
  attachments?: ReturnMessageAttachmentResponse[];
}

export interface ReturnConversationResponse {
  messages: ReturnMessage[];
}



export interface ReplyToReturnInfoRequest {
  message: string;
}

export interface ReturnMessageAttachmentResponse {
  id: string;
  url: string;
  contentType: string;
  sizeBytes: number;
  originalFilename?: string;
}

export interface UploadReturnMessageAttachmentResponse {
  id: string;
  url: string;
}

export interface AdminSendReturnMessageRequest {
  message: string;
  needsResponse: boolean;
  attachmentIds?: string[];
}

export interface CustomerSendReturnMessageRequest {
  message: string;
  attachmentIds?: string[];
}

export type GlobalSearchResultType = 'order' | 'return' | 'inventory_unit' | 'product_variant' | 'product' | 'customer';

export interface GlobalSearchResult {
  type: GlobalSearchResultType;
  id: string;
  title: string;
  subtitle: string;
  canonicalIdentifier: string;
  navigationTarget: string;
}

export interface GlobalSearchResponse {
  results: GlobalSearchResult[];
}
