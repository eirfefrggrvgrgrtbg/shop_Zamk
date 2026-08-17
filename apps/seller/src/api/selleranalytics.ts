import { request } from '@zamk/api-client/src/client';

export interface PeriodDTO {
  from: string;
  to: string;
  timezone: string;
}

export interface MetricCentsDTO {
  currentCents: number;
  previousCents: number;
  changePercent: number | null;
  comparisonState: 'positive' | 'negative' | 'unchanged' | 'new';
}

export interface MetricCountDTO {
  current: number;
  previous: number;
  changePercent: number | null;
  comparisonState: 'positive' | 'negative' | 'unchanged' | 'new';
}

export interface MetricCentsSimpleDTO {
  currentCents: number;
  previousCents: number;
}

export interface MetricCountSimpleDTO {
  current: number;
  previous: number;
}

export interface MetricPercentDTO {
  currentPercent: number;
  previousPercent: number;
}

export interface TimeseriesBucketDTO {
  date: string;
  grossSalesCents: number;
  ordersCount: number;
  unitsSold: number;
  commissionCents: number;
  sellerEarningCents: number;
  returnDeductionsCents: number;
  netCommercialEarningCents: number;
  returnedUnits: number;
}

export interface InsightEvidence {
  available?: number;
  salesVelocity?: number;
  daysOfStock?: number;
  grossSalesCents?: number;
  previousGrossSalesCents?: number;
  changePercent?: number;
  unitsSold?: number;
  returnedUnits?: number;
  returnRatePercent?: number;
}

export interface InsightDTO {
  type: string;
  severity: 'high' | 'medium' | 'low';
  productId: string;
  variantId?: string;
  messageCode: string;
  evidence: InsightEvidence;
}

export interface OverviewResponse {
  period: PeriodDTO;
  hasHistoricalSales: boolean;
  grossSales: MetricCentsDTO;
  orders: MetricCountDTO;
  unitsSold: MetricCountDTO;
  averageOrderValue: MetricCentsDTO;
  commission: MetricCentsSimpleDTO;
  sellerEarningBeforeReturns: MetricCentsSimpleDTO;
  returnDeductions: MetricCentsSimpleDTO;
  otherAdjustments: MetricCentsSimpleDTO;
  netCommercialEarning: MetricCentsDTO;
  returnedUnits: MetricCountSimpleDTO;
  returnRate: MetricPercentDTO;
  timeseries: TimeseriesBucketDTO[];
  insights?: InsightDTO[];
}

export interface ProductRow {
  productId: string;
  title: string;
  grossSalesCents: number;
  ordersCount: number;
  unitsSold: number;
  returnedUnits: number;
  returnRatePercent: number;
  availableStock: number;
  previousGrossSalesCents: number;
  grossSalesChangePercent: number | null;
  comparisonState: string;
}



export interface VariantRow {
  variantId: string;
  sku: string;
  displayName: string;
  unitsSold: number;
  grossSalesCents: number;
  returnedUnits: number;
  returnRatePercent: number;
  availableStock: number;
  salesVelocity: number;
  daysOfStock: number | null;
  stockCoverageState: string;
}

export interface ProductDetailResponse {
  productId: string;
  title: string;
  grossSales: MetricCentsDTO;
  unitsSold: MetricCountDTO;
  orders: MetricCountDTO;
  returnedUnits: MetricCountSimpleDTO;
  returnRate: MetricPercentDTO;
  currentAvailableStock: number;
  timeseries: TimeseriesBucketDTO[];
  variants: VariantRow[];
  insights: InsightDTO[];
}

export interface InventoryRow {
  productId: string;
  variantId: string;
  sku: string;
  available: number;
  onHand: number;
  reserved: number;
  inbound: number;
  unitsSold: number;
  salesVelocity: number;
  daysOfStock: number | null;
  stockCoverageState: string;
}

export interface InventoryResponse {
  items: InventoryRow[];
}

export async function getAnalyticsOverview(from: string, to: string): Promise<OverviewResponse> {
  const query = new URLSearchParams({ from, to });
  return request<OverviewResponse>('GET', `/seller/analytics/overview?${query.toString()}`);
}

export interface ProductsResponse {
  items: ProductRow[];
  totalCount: number;
}

export async function getAnalyticsProducts(
  from: string, 
  to: string, 
  sort: string = 'gross_sales', 
  order: 'asc' | 'desc' = 'desc', 
  limit: number = 50, 
  offset: number = 0
): Promise<ProductsResponse> {
  const query = new URLSearchParams({ 
    from, to, sort, order, limit: limit.toString(), offset: offset.toString() 
  });
  return request<ProductsResponse>('GET', `/seller/analytics/products?${query.toString()}`);
}

export async function getAnalyticsProductDetail(productId: string, from: string, to: string): Promise<ProductDetailResponse> {
  const query = new URLSearchParams({ from, to });
  return request<ProductDetailResponse>('GET', `/seller/analytics/products/${productId}?${query.toString()}`);
}

export async function getAnalyticsInventory(from: string, to: string): Promise<InventoryResponse> {
  const query = new URLSearchParams({ from, to });
  return request<InventoryResponse>('GET', `/seller/analytics/inventory?${query.toString()}`);
}
