package marketing_test

// MARKETING.M1.2 — how campaign status and period interact with tracking links (zamk_test).
// These tests document the CURRENT behaviour; product decisions on it are listed in the M1.2 report.

import (
	"context"
	"testing"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/marketing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCampaignStatus_TrackingLinkInteraction(t *testing.T) {
	_, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	newCampaign := func() *marketing.MarketingCampaign {
		c := &marketing.MarketingCampaign{
			Title: "M12 tracking " + uuid.NewString()[:8], FundingMode: marketing.FundingModeZamk,
			Status: marketing.CampaignStatusApproved, DiscountType: marketing.DiscountTypePercent,
		}
		require.NoError(t, svc.CreateAdminCampaign(ctx, c))
		return c
	}
	newLink := func(c *marketing.MarketingCampaign) *marketing.CampaignTrackingLink {
		path := "/catalog"
		l := &marketing.CampaignTrackingLink{CampaignID: c.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path}
		require.NoError(t, svc.CreateTrackingLink(ctx, l))
		return l
	}
	setStatus := func(c *marketing.MarketingCampaign, to marketing.CampaignStatus) {
		_, err := svc.UpdateAdminCampaign(ctx, c.ID, marketing.AdminUpdateCampaignRequest{Status: &to})
		require.NoError(t, err)
	}
	resolves := func(l *marketing.CampaignTrackingLink) error {
		_, err := svc.ResolveTrackingLink(ctx, l.Token, "https://shop.test")
		return err
	}
	attributes := func(l *marketing.CampaignTrackingLink) error {
		_, err := svc.ResolveCampaignToken(ctx, l.Token)
		return err
	}

	t.Run("approved campaign already redirects and attributes like an active one", func(t *testing.T) {
		c := newCampaign()
		l := newLink(c)
		assert.NoError(t, resolves(l))
		assert.NoError(t, attributes(l))
		setStatus(c, marketing.CampaignStatusActive)
		assert.NoError(t, resolves(l))
		assert.NoError(t, attributes(l))
	})

	t.Run("ended and cancelled campaigns stop redirects and attribution", func(t *testing.T) {
		for _, to := range []marketing.CampaignStatus{marketing.CampaignStatusEnded, marketing.CampaignStatusCancelled} {
			c := newCampaign()
			l := newLink(c)
			setStatus(c, to)
			assert.ErrorIs(t, resolves(l), marketing.ErrCampaignNotActive, string(to))
			assert.ErrorIs(t, attributes(l), marketing.ErrCampaignNotActive, string(to))
		}
	})

	t.Run("links can still be created and disabled after the campaign is finished (current behaviour)", func(t *testing.T) {
		c := newCampaign()
		existing := newLink(c)
		setStatus(c, marketing.CampaignStatusEnded)

		late := newLink(c) // creation does not check the campaign status
		assert.ErrorIs(t, resolves(late), marketing.ErrCampaignNotActive, "but it never resolves")

		disabled, err := svc.DisableTrackingLink(ctx, c.ID, existing.ID)
		require.NoError(t, err)
		assert.False(t, disabled.IsActive)
		links, err := svc.ListCampaignTrackingLinks(ctx, c.ID)
		require.NoError(t, err)
		assert.Len(t, links, 2)
	})

	t.Run("a disabled link stays disabled while the campaign is active", func(t *testing.T) {
		c := newCampaign()
		l := newLink(c)
		_, err := svc.DisableTrackingLink(ctx, c.ID, l.ID)
		require.NoError(t, err)
		assert.ErrorIs(t, resolves(l), marketing.ErrTrackingLinkDisabled)
		assert.ErrorIs(t, attributes(l), marketing.ErrTrackingLinkDisabled)
	})
}

// The period is checked as: not started when now < starts_at; expired when now > ends_at (strictly).
// A stored end of 23:59:59.000 UTC therefore expires 1 s - 1 µs before the next day starts.
func TestCampaignPeriod_BoundaryBehaviour(t *testing.T) {
	client, _, svc := setupTestMarketingDB(t)
	ctx := context.Background()

	c := &marketing.MarketingCampaign{
		Title: "M12 period " + uuid.NewString()[:8], FundingMode: marketing.FundingModeZamk,
		Status: marketing.CampaignStatusApproved, DiscountType: marketing.DiscountTypePercent,
	}
	require.NoError(t, svc.CreateAdminCampaign(ctx, c))
	path := "/catalog"
	l := &marketing.CampaignTrackingLink{CampaignID: c.ID, TargetType: marketing.CampaignTargetLanding, LandingPath: &path}
	require.NoError(t, svc.CreateTrackingLink(ctx, l))

	setPeriod := func(starts, ends *time.Time) {
		_, err := client.Pool.Exec(ctx, `UPDATE marketing_campaigns SET starts_at = $1, ends_at = $2 WHERE id = $3`, starts, ends, c.ID)
		require.NoError(t, err)
	}
	resolve := func() error {
		_, err := svc.ResolveTrackingLink(ctx, l.Token, "https://shop.test")
		return err
	}
	ts := func(d time.Duration) *time.Time { v := time.Now().UTC().Add(d); return &v }

	setPeriod(nil, nil)
	assert.NoError(t, resolve(), "no period = always within the period")

	setPeriod(ts(-time.Hour), ts(time.Hour))
	assert.NoError(t, resolve())

	setPeriod(ts(time.Hour), nil)
	assert.ErrorIs(t, resolve(), marketing.ErrCampaignNotStarted)

	// The end moment is exclusive of anything after it, down to fractional seconds.
	setPeriod(nil, ts(-time.Millisecond))
	assert.ErrorIs(t, resolve(), marketing.ErrCampaignExpired, "1 ms past the end is already expired")

	// A yesterday 23:59:59.999999 end is expired; the inclusive-day contract of the Admin UI is
	// 23:59:59.000 UTC, i.e. the last 999 ms of that day are NOT covered.
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	endOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999_999_000, time.UTC)
	setPeriod(nil, &endOfYesterday)
	assert.ErrorIs(t, resolve(), marketing.ErrCampaignExpired)

	// Fractional seconds survive a round trip (PostgreSQL timestamptz keeps microseconds).
	frac := time.Date(2030, 5, 5, 23, 59, 59, 999_999_000, time.UTC)
	setPeriod(nil, &frac)
	var stored time.Time
	require.NoError(t, client.Pool.QueryRow(ctx, `SELECT ends_at FROM marketing_campaigns WHERE id = $1`, c.ID).Scan(&stored))
	assert.True(t, stored.Equal(frac))
	assert.NoError(t, resolve(), "a far-future end is still valid")
}
