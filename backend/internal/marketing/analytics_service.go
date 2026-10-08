package marketing

import (
	"context"
	"sort"
	"strings"
	"time"
)

type AnalyticsService struct {
	repo *AnalyticsRepository
}

func NewAnalyticsService(repo *AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) GetOverview(ctx context.Context, from, to time.Time) (*AnalyticsOverviewResponse, error) {
	current, err := s.repo.GetOverviewMetrics(ctx, from, to)
	if err != nil {
		return nil, err
	}

	duration := to.Sub(from)
	prevFrom := from.Add(-duration)
	prevTo := from

	previous, err := s.repo.GetOverviewMetrics(ctx, prevFrom, prevTo)
	if err != nil {
		return nil, err
	}

	return &AnalyticsOverviewResponse{
		Current:  current,
		Previous: previous,
	}, nil
}

func (s *AnalyticsService) GetSources(ctx context.Context, from, to time.Time) (*AnalyticsSourcesResponse, error) {
	raw, err := s.repo.GetSourceMetrics(ctx, from, to)
	if err != nil {
		return nil, err
	}

	sources := make([]SourcePerformanceRow, 0, len(raw))
	for _, r := range raw {
		bps := 0
		if r.Visits > 0 {
			bps = int(float64(r.PaidOrders) / float64(r.Visits) * 10000)
		}
		var sKey *string
		var sKind SourceKind
		if r.Source == "" || r.Source == "Unattributed" {
			sKind = SourceKindUnattributed
		} else if strings.EqualFold(r.Source, "direct") {
			k := "direct"
			sKey = &k
			sKind = SourceKindDirect
		} else {
			k := r.Source
			sKey = &k
			sKind = SourceKindNamed
		}
		sources = append(sources, SourcePerformanceRow{
			SourceKey:         sKey,
			SourceKind:        sKind,
			Source:            r.Source,
			Visits:            r.Visits,
			PaidOrders:        r.PaidOrders,
			RevenueCents:      r.RevenueCents,
			ConversionRateBps: bps,
		})
	}

	// Sort by Visits desc, then Revenue desc
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Visits == sources[j].Visits {
			return sources[i].RevenueCents > sources[j].RevenueCents
		}
		return sources[i].Visits > sources[j].Visits
	})

	return &AnalyticsSourcesResponse{Sources: sources}, nil
}

func (s *AnalyticsService) GetSourcesAnalytics(ctx context.Context, req SourceAnalyticsRequest) (*SourceAnalyticsResponse, error) {
	return s.repo.GetSourcesAnalytics(ctx, req)
}

func (s *AnalyticsService) GetSourceDetail(ctx context.Context, source string, req SourceDetailRequest) (*SourceDetailResponse, error) {
	return s.repo.GetSourceDetail(ctx, source, req)
}

func (s *AnalyticsService) GetCampaigns(ctx context.Context, from, to time.Time) (*AnalyticsCampaignsResponse, error) {
	raw, err := s.repo.GetCampaignMetrics(ctx, from, to)
	if err != nil {
		return nil, err
	}

	campaigns := make([]CampaignPerformanceRow, 0, len(raw))
	for _, r := range raw {
		bps := 0
		if r.Visits > 0 {
			bps = int(float64(r.PaidOrders) / float64(r.Visits) * 10000)
		}

		var cID *string
		if r.CampaignID != nil {
			idStr := r.CampaignID.String()
			cID = &idStr
		}

		campaigns = append(campaigns, CampaignPerformanceRow{
			CampaignID:         cID,
			Status:             r.Status,
			PlannedBudgetCents: r.PlannedBudgetCents,
			Name:               r.Name,
			Visits:             r.Visits,
			PaidOrders:         r.PaidOrders,
			RevenueCents:       r.RevenueCents,
			ConversionRateBps:  bps,
		})
	}

	// Sort by Visits desc, then Revenue desc
	sort.Slice(campaigns, func(i, j int) bool {
		if campaigns[i].Visits == campaigns[j].Visits {
			return campaigns[i].RevenueCents > campaigns[j].RevenueCents
		}
		return campaigns[i].Visits > campaigns[j].Visits
	})

	return &AnalyticsCampaignsResponse{Campaigns: campaigns}, nil
}

func (s *AnalyticsService) GetTrend(ctx context.Context, from, to time.Time) (*AnalyticsTrendResponse, error) {
	trend, err := s.repo.GetTrendMetrics(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if trend == nil {
		trend = []TrendDataPoint{}
	}
	return &AnalyticsTrendResponse{Trend: trend}, nil
}

func (s *AnalyticsService) GetProductAnalytics(ctx context.Context, req ProductAnalyticsRequest) (*ProductAnalyticsResponse, error) {
	return s.repo.GetProductAnalytics(ctx, req)
}

func (s *AnalyticsService) GetDesignerAnalytics(ctx context.Context, req DesignerAnalyticsRequest) (*DesignerAnalyticsResponse, error) {
	return s.repo.GetDesignerAnalytics(ctx, req)
}

func (s *AnalyticsService) ExecuteQuery(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	// QueryRepository takes db
	repo := NewQueryRepository(s.repo.db)
	return repo.ExecuteQuery(ctx, req)
}
