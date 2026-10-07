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
  AdminMarketingSourcesResponse as AnalyticsSourcesResponse,
  AdminCampaignMetrics as CampaignMetrics,
  AdminMarketingCampaignsAnalyticsResponse as AnalyticsCampaignsResponse,
} from '@zamk/api-client/src/types';
