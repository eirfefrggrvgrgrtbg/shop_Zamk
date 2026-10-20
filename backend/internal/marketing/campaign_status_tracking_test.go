package marketing_test

// MARKETING.M1.3 — campaign status, period and tracking traffic (zamk_test).

import (
	"context"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/behavior"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/orders"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var m13Start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func m13Campaign(t *testing.T, svc *marketing.Service, status marketing.CampaignStatus) *marketing.MarketingCampaign {
	t.Helper()
	ch, tp := marketing.CampaignChannelTelegram, marketing.CampaignTypeDrop
	c := &marketing.MarketingCampaign{
		Title: "M13 " + uuid.NewString()[:8], FundingMode: marketing.FundingModeZamk, Status: marketing.CampaignStatusDraft,
		DiscountType: marketing.DiscountTypePercent, CampaignChannel: &ch, CampaignType: &tp, StartsAt: &m13Start,
	}
	require.NoError(t, svc.CreateAdminCampaign(context.Background(), c))
	path := map[marketing.CampaignStatus][]marketing.CampaignStatus{
		marketing.CampaignStatusDraft:     nil,
		marketing.CampaignStatusApproved:  {marketing.CampaignStatusApproved},
		marketing.CampaignStatusActive:    {marketing.CampaignStatusApproved, marketing.CampaignStatusActive},
		marketing.CampaignStatusPaused:    {marketing.CampaignStatusApproved, marketing.CampaignStatusActive, marketing.CampaignStatusPaused},
		marketing.CampaignStatusEnded:     {marketing.CampaignStatusApproved, marketing.CampaignStatusActive, marketing.CampaignStatusEnded},
		marketing.CampaignStatusCancelled: {marketing.CampaignStatusCancelled},
	}[status]
	for _, st := range path {
		m13SetStatus(t, svc, c.ID, st)
	}
	return c
}

func m13SetStatus(t *testing.T, svc *marketing.Service, id uuid.UUID, to marketing.CampaignStatus) {
	t.Helper()
	_, err := svc.UpdateAdminCampaign(context.Background(), id, marketing.AdminUpdateCampaignRequest{Status: &to})
	require.NoError(t, err, "-> %s", to)
}

func m13Link(t *testing.T, svc *marketing.Service, campaignID uuid.UUID) *marketing.CampaignTrackingLink {
	t.Helper()
	path := "/catalog"
	l := &marketing.CampaignTrackingLink{CampaignID: campaignID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path}
	require.NoError(t, svc.CreateTrackingLink(context.Background(), l))
	return l
}

func TestCampaignStatus_TrackingLinkInteraction(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	resolves := func(l *marketing.CampaignTrackingLink) error {
		_, err := svc.ResolveTrackingLink(ctx, l.Token, "https://shop.test")
		return err
	}
	attributes := func(l *marketing.CampaignTrackingLink) error {
		_, err := svc.ResolveCampaignToken(ctx, l.Token)
		return err
	}

	t.Run("only active campaigns accept traffic; draft, approved and paused links exist but do not work", func(t *testing.T) {
		for _, st := range []marketing.CampaignStatus{marketing.CampaignStatusDraft, marketing.CampaignStatusApproved, marketing.CampaignStatusPaused} {
			c := m13Campaign(t, svc, st)
			if st == marketing.CampaignStatusPaused {
				// a paused campaign keeps the link it had; new links can still be prepared
				m13Link(t, svc, c.ID)
			}
			l := m13Link(t, svc, c.ID)
			assert.ErrorIs(t, resolves(l), marketing.ErrCampaignNotActive, string(st))
			assert.ErrorIs(t, attributes(l), marketing.ErrCampaignNotActive, string(st))
		}
		c := m13Campaign(t, svc, marketing.CampaignStatusApproved)
		l := m13Link(t, svc, c.ID)
		m13SetStatus(t, svc, c.ID, marketing.CampaignStatusActive)
		assert.NoError(t, resolves(l))
		assert.NoError(t, attributes(l))
	})

	t.Run("E/F/J: pause blocks traffic, resume restores it, links are preserved", func(t *testing.T) {
		c := m13Campaign(t, svc, marketing.CampaignStatusActive)
		l := m13Link(t, svc, c.ID)
		require.NoError(t, resolves(l))

		m13SetStatus(t, svc, c.ID, marketing.CampaignStatusPaused)
		assert.ErrorIs(t, resolves(l), marketing.ErrCampaignNotActive)
		assert.ErrorIs(t, attributes(l), marketing.ErrCampaignNotActive)
		links, err := svc.ListCampaignTrackingLinks(ctx, c.ID)
		require.NoError(t, err)
		require.Len(t, links, 1)
		assert.True(t, links[0].IsActive, "pausing the campaign does not disable or delete its links")

		m13SetStatus(t, svc, c.ID, marketing.CampaignStatusActive)
		assert.NoError(t, resolves(l))
		assert.NoError(t, attributes(l))

		// after resuming, the period still applies
		past := time.Now().UTC().Add(-time.Minute)
		_, err = svc.UpdateAdminCampaign(ctx, c.ID, marketing.AdminUpdateCampaignRequest{EndsAt: &past})
		require.NoError(t, err)
		assert.ErrorIs(t, resolves(l), marketing.ErrCampaignExpired)
	})

	t.Run("G/H: completed and cancelled campaigns stop traffic and refuse new links", func(t *testing.T) {
		for _, st := range []marketing.CampaignStatus{marketing.CampaignStatusEnded, marketing.CampaignStatusCancelled} {
			c := m13Campaign(t, svc, marketing.CampaignStatusActive)
			l := m13Link(t, svc, c.ID)
			if st == marketing.CampaignStatusEnded {
				m13SetStatus(t, svc, c.ID, marketing.CampaignStatusEnded)
			} else {
				m13SetStatus(t, svc, c.ID, marketing.CampaignStatusCancelled)
			}
			assert.ErrorIs(t, resolves(l), marketing.ErrCampaignNotActive)
			path := "/catalog"
			err := svc.CreateTrackingLink(ctx, &marketing.CampaignTrackingLink{CampaignID: c.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path})
			assert.ErrorIs(t, err, marketing.ErrCampaignClosed)
			disabled, err := svc.DisableTrackingLink(ctx, c.ID, l.ID)
			require.NoError(t, err, "existing links can still be disabled")
			assert.False(t, disabled.IsActive)
		}
	})

	t.Run("N: pre-M1.3 approved rows keep tracking; an old-app insert gets the legacy rule", func(t *testing.T) {
		c := m13Campaign(t, svc, marketing.CampaignStatusApproved)
		l := m13Link(t, svc, c.ID)
		require.ErrorIs(t, resolves(l), marketing.ErrCampaignNotActive)

		// a row written by the previous application version: no legacy_approved_tracking in the column list
		oldID := uuid.New()
		_, err := client.Pool.Exec(ctx, `
			INSERT INTO marketing_campaigns (id, title, funding_mode, status, discount_type, requested_zamk_share_bps,
			    requested_zamk_budget_cap_cents, approved_zamk_share_bps, approved_zamk_budget_cap_cents, purpose)
			VALUES ($1, 'old app', 'zamk', 'approved', 'percent', 1000, 100000, 1000, 100000, 'advertising')`, oldID)
		require.NoError(t, err)
		old, err := repo.GetCampaignByID(ctx, oldID)
		require.NoError(t, err)
		assert.True(t, old.LegacyApprovedTracking)
		oldLink := m13Link(t, svc, oldID)
		assert.NoError(t, resolves(oldLink), "previously working approved campaign is not switched off")
		assert.NoError(t, attributes(oldLink))

		// once an admin moves it through the lifecycle, the new rules apply
		ch, tp := marketing.CampaignChannelVK, marketing.CampaignTypeDrop
		_, err = svc.UpdateAdminCampaign(ctx, oldID, marketing.AdminUpdateCampaignRequest{CampaignChannel: &ch, CampaignType: &tp, StartsAt: &m13Start})
		require.NoError(t, err)
		assert.NoError(t, resolves(oldLink), "field edits do not change the legacy rule")
		m13SetStatus(t, svc, oldID, marketing.CampaignStatusActive)
		m13SetStatus(t, svc, oldID, marketing.CampaignStatusPaused)
		assert.ErrorIs(t, resolves(oldLink), marketing.ErrCampaignNotActive)
	})
}

// K: the campaign period is the half-open interval [starts_at, ends_at).
func TestCampaignPeriod_HalfOpenBoundaries(t *testing.T) {
	msk := time.FixedZone("MSK", 3*3600)
	// Admin chooses 10.11.2026 - 30.11.2026 in Europe/Moscow:
	start := time.Date(2026, 11, 10, 0, 0, 0, 0, msk)
	endExclusive := time.Date(2026, 12, 1, 0, 0, 0, 0, msk)
	c := &marketing.MarketingCampaign{Purpose: marketing.CampaignPurposeAdvertising, Status: marketing.CampaignStatusActive, StartsAt: &start, EndsAt: &endExclusive}

	check := func(now time.Time) error { return marketing.CheckCampaignAcceptsTraffic(c, now) }

	assert.ErrorIs(t, check(start.Add(-time.Microsecond)), marketing.ErrCampaignNotStarted)
	assert.NoError(t, check(start), "start instant is inside")
	assert.NoError(t, check(start.UTC()), "same instant in UTC (09.11 21:00Z)")
	lastMicro := endExclusive.Add(-time.Microsecond)
	assert.NoError(t, check(lastMicro), "30.11 23:59:59.999999 MSK is inside")
	assert.NoError(t, check(endExclusive.Add(-time.Millisecond)), "30.11 23:59:59.999 MSK is inside")
	assert.ErrorIs(t, check(endExclusive), marketing.ErrCampaignExpired, "01.12 00:00 MSK is outside")
	assert.ErrorIs(t, check(endExclusive.UTC()), marketing.ErrCampaignExpired, "same instant in UTC (30.11 21:00Z)")
	assert.ErrorIs(t, check(endExclusive.In(time.FixedZone("UTC-5", -5*3600))), marketing.ErrCampaignExpired)
	assert.NoError(t, check(time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC).Add(-3*time.Hour)), "30.11 23:00 MSK")

	// legacy end values are NOT re-interpreted: a stored 23:59:59Z end stays exactly that instant
	legacyEnd := time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC)
	c.EndsAt = &legacyEnd
	assert.NoError(t, check(legacyEnd.Add(-time.Nanosecond)))
	assert.ErrorIs(t, check(legacyEnd), marketing.ErrCampaignExpired)

	// period validation: the end must be strictly after the start
	same := start
	assert.ErrorIs(t, marketing.ValidateAdvertisingPeriod(&marketing.MarketingCampaign{StartsAt: &start, EndsAt: &same}), marketing.ErrInvalidCampaignPeriod)
	assert.NoError(t, marketing.ValidateAdvertisingPeriod(&marketing.MarketingCampaign{StartsAt: &start, EndsAt: &endExclusive}))
}

func TestCampaignPeriod_MicrosecondsRoundTrip(t *testing.T) {
	client, repo, svc := setupTestMarketingDB(t)
	ctx := context.Background()
	c := m13Campaign(t, svc, marketing.CampaignStatusDraft)
	frac := time.Date(2030, 5, 5, 23, 59, 59, 999_999_000, time.UTC)
	_, err := svc.UpdateAdminCampaign(ctx, c.ID, marketing.AdminUpdateCampaignRequest{EndsAt: &frac})
	require.NoError(t, err)
	got, err := repo.GetCampaignByID(ctx, c.ID)
	require.NoError(t, err)
	assert.True(t, got.EndsAt.Equal(frac), "PostgreSQL keeps microseconds")
	var raw string
	require.NoError(t, client.Pool.QueryRow(ctx, `SELECT to_char(ends_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US') FROM marketing_campaigns WHERE id = $1`, c.ID).Scan(&raw))
	assert.Equal(t, "2030-05-05T23:59:59.999999", raw)
}

// E: no new campaign attribution during a pause; history and orders from earlier sessions stay as they are.
func TestCampaignPause_SessionsAndAttribution(t *testing.T) {
	f := setupADS2AAttributionFixture(t, context.Background())
	ctx := context.Background()
	c := m13Campaign(t, f.marketingSvc, marketing.CampaignStatusActive)
	l := m13Link(t, f.marketingSvc, c.ID)

	startSession := func(visitor uuid.UUID) uuid.UUID {
		sessionID := uuid.New()
		_, err := f.behaviorSvc.IngestEvents(ctx, nil, behavior.EventIngestionRequest{Events: []behavior.IngestionEvent{{
			EventID: uuid.New(), EventType: "session_started", VisitorID: visitor, SessionID: &sessionID, OccurredAt: time.Now().UTC(),
			Metadata: map[string]interface{}{"landing_path": "/catalog", "zamk_token": l.Token},
		}}})
		require.NoError(t, err)
		return sessionID
	}
	sessionCampaign := func(id uuid.UUID) *uuid.UUID {
		var cid *uuid.UUID
		require.NoError(t, f.db.QueryRow(ctx, `SELECT campaign_id FROM analytics_sessions WHERE id = $1`, id).Scan(&cid))
		return cid
	}

	visitorBefore := uuid.New()
	before := startSession(visitorBefore)
	require.NotNil(t, sessionCampaign(before))

	m13SetStatus(t, f.marketingSvc, c.ID, marketing.CampaignStatusPaused)
	during := startSession(uuid.New())
	assert.Nil(t, sessionCampaign(during), "no new campaign session while paused")
	assert.Equal(t, c.ID, *sessionCampaign(before), "earlier sessions are kept")

	// An order placed during the pause by a visitor whose session started before it keeps the
	// existing last-non-direct-touch rule (30-day window): it is attributed to the campaign.
	f.populateCart(t, ctx, f.buyerID, 1)
	order, err := f.ordersSvc.CreateOrder(ctx, f.buyerID, orders.CreateOrderRequest{
		CustomerName: "M13", CustomerEmail: "m13@test.local", CustomerPhone: "+79991234567",
		DeliveryAddress: "Moscow", DeliveryMethodID: f.dmID,
		AnalyticsContext: &orders.AnalyticsContextDTO{VisitorID: visitorBefore, SessionID: before},
	}, nil)
	require.NoError(t, err)
	var attributed *uuid.UUID
	require.NoError(t, f.db.QueryRow(ctx, `SELECT campaign_id FROM order_attributions WHERE order_id = $1`, order.ID).Scan(&attributed))
	require.NotNil(t, attributed)
	assert.Equal(t, c.ID, *attributed)

	// analytics stays available while paused
	from, to := time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(time.Hour)
	metrics, err := f.marketingSvc.Analytics.GetCampaigns(ctx, from, to)
	require.NoError(t, err)
	found := false
	for _, m := range metrics.Campaigns {
		if m.CampaignID != nil && *m.CampaignID == c.ID.String() {
			found = true
			assert.Equal(t, 1, m.Visits)
			assert.Equal(t, marketing.CampaignStatusPaused, m.Status)
		}
	}
	assert.True(t, found, "paused campaign is still reported")

	m13SetStatus(t, f.marketingSvc, c.ID, marketing.CampaignStatusActive)
	after := startSession(uuid.New())
	require.NotNil(t, sessionCampaign(after))
	assert.Equal(t, c.ID, *sessionCampaign(after))
}
