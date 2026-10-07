ALTER TABLE campaign_tracking_links DROP CONSTRAINT IF EXISTS chk_ctl_target_exact_one;
ALTER TABLE campaign_tracking_links RENAME COLUMN landing_path TO target_url;
ALTER TABLE campaign_tracking_links ADD CONSTRAINT chk_ctl_target_product CHECK (target_type != 'product' OR target_product_id IS NOT NULL);
ALTER TABLE campaign_tracking_links ADD CONSTRAINT chk_ctl_target_seller CHECK (target_type != 'seller' OR target_seller_id IS NOT NULL);
ALTER TABLE campaign_tracking_links ADD CONSTRAINT chk_ctl_target_landing CHECK (target_type != 'landing' OR target_url IS NOT NULL);
ALTER TABLE marketing_campaigns DROP COLUMN IF EXISTS planned_budget_cents;
ALTER TABLE promo_codes ALTER COLUMN seller_id DROP NOT NULL;
