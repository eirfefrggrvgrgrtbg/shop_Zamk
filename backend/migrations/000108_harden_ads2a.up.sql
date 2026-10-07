ALTER TABLE promo_codes ALTER COLUMN seller_id SET NOT NULL;
ALTER TABLE marketing_campaigns ADD COLUMN planned_budget_cents BIGINT CHECK (planned_budget_cents >= 0);
ALTER TABLE campaign_tracking_links RENAME COLUMN target_url TO landing_path;

ALTER TABLE campaign_tracking_links DROP CONSTRAINT IF EXISTS chk_ctl_target_product;
ALTER TABLE campaign_tracking_links DROP CONSTRAINT IF EXISTS chk_ctl_target_seller;
ALTER TABLE campaign_tracking_links DROP CONSTRAINT IF EXISTS chk_ctl_target_landing;

ALTER TABLE campaign_tracking_links ADD CONSTRAINT chk_ctl_target_exact_one CHECK (
    (target_type = 'product' AND target_product_id IS NOT NULL AND target_seller_id IS NULL AND landing_path IS NULL) OR
    (target_type = 'seller'  AND target_seller_id IS NOT NULL AND target_product_id IS NULL AND landing_path IS NULL) OR
    (target_type = 'landing' AND landing_path IS NOT NULL AND target_product_id IS NULL AND target_seller_id IS NULL)
);
