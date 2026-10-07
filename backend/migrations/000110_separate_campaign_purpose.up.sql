BEGIN;

-- Freeze the evidence while classifying historical rows. Titles are never evidence.
LOCK TABLE marketing_campaigns IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE campaign_tracking_links IN SHARE MODE;
ALTER TABLE marketing_campaigns ADD COLUMN purpose TEXT;

UPDATE marketing_campaigns c SET purpose = 'advertising'
WHERE c.seller_id IS NULL
   OR c.campaign_channel IS NOT NULL
   OR c.campaign_type IS NOT NULL
   OR c.planned_budget_cents IS NOT NULL
   OR EXISTS (SELECT 1 FROM campaign_tracking_links l WHERE l.campaign_id = c.id);

-- Seller promotion / co-funding pipeline: seller ownership and actual discount
-- or subsidy terms, without ADS evidence. An optional promo relation is not
-- sufficient to turn an advertising campaign into a promotion.
UPDATE marketing_campaigns SET purpose = 'promotion'
WHERE purpose IS NULL AND seller_id IS NOT NULL
  AND (seller_discount_bps > 0 OR seller_discount_fixed_cents > 0
       OR requested_zamk_share_bps > 0 OR requested_zamk_budget_cap_cents > 0);

DO $$
DECLARE ambiguous_ids TEXT;
BEGIN
    SELECT string_agg(id::text, ', ' ORDER BY id) INTO ambiguous_ids
    FROM marketing_campaigns WHERE purpose IS NULL;
    IF ambiguous_ids IS NOT NULL THEN
        RAISE EXCEPTION 'Ambiguous campaign purpose; audit required for IDs: %', ambiguous_ids;
    END IF;
END $$;

ALTER TABLE marketing_campaigns ALTER COLUMN purpose SET NOT NULL;
-- Legacy promotion writers retain their domain. Ads creation supplies it explicitly.
ALTER TABLE marketing_campaigns ALTER COLUMN purpose SET DEFAULT 'promotion';
ALTER TABLE marketing_campaigns ADD CONSTRAINT chk_mc_purpose
    CHECK (purpose IN ('advertising', 'promotion'));
CREATE INDEX idx_marketing_campaigns_purpose_created ON marketing_campaigns (purpose, created_at DESC);

CREATE FUNCTION prevent_campaign_purpose_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.purpose IS DISTINCT FROM OLD.purpose THEN
        RAISE EXCEPTION 'Campaign purpose is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER marketing_campaign_purpose_immutable
    BEFORE UPDATE OF purpose ON marketing_campaigns
    FOR EACH ROW EXECUTE FUNCTION prevent_campaign_purpose_change();
COMMIT;
