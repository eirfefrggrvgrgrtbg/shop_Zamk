export {
  getAdminMarketingProducts,
  getAdminMarketingProducts as getMarketingProducts,
  getAdminMarketingOverview,
  getAdminMarketingSources,
  getAdminMarketingCampaignMetrics,
  getAdminMarketingTrend,
  getAdminMarketingTrend as getMarketingTrend,
  getAdminMarketingOverview as getMarketingOverview,
  getAdminMarketingSources as getMarketingSources,
  getAdminMarketingCampaignMetrics as getMarketingCampaigns,
  getAdminDesignerAnalytics,
  getAdminMarketingSourceDetail,
  getAdminMarketingSourceDetail as getMarketingSourceDetail,
} from '@zamk/api-client/src/admin';

export type {
  AdminProductAnalyticsResponse as ProductAnalyticsResponse,
  AdminProductAnalyticsRow as ProductAnalyticsRow,
  AdminProductAnalyticsCoverage as ProductAnalyticsCoverage,
  MetricCoverage,
  MetricCoverageStatus,
  AdminAnalyticsMetrics as AnalyticsMetrics,
  AdminMarketingTrendResponse as AnalyticsTrendResponse,
  AdminMarketingOverviewResponse as AnalyticsOverviewResponse,
  AdminSourceMetrics as SourceMetrics,
  SourceKind,
  AdminMarketingSourcesResponse as AnalyticsSourcesResponse,
  AdminMarketingSourceDetailResponse as SourceDetailResponse,
  AdminCampaignMetrics as CampaignMetrics,
  AdminMarketingCampaignsAnalyticsResponse as AnalyticsCampaignsResponse,
} from '@zamk/api-client/src/types';

import { executeAdminMarketingQuery as _executeAdminMarketingQuery } from '@zamk/api-client/src/admin';
import type { QueryRequest } from '@zamk/api-client/src/types';

export const executeAdminMarketingQuery = (req: QueryRequest) => _executeAdminMarketingQuery(req);
