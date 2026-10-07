DROP TABLE IF EXISTS campaign_tracking_links;
ALTER TABLE marketing_campaigns DROP COLUMN campaign_type;
ALTER TABLE marketing_campaigns DROP COLUMN campaign_channel;
-- Note: Re-adding NOT NULL may fail if rows without seller_id were added,
-- but for standard down migrations this is the expected reverse syntax.
ALTER TABLE promo_codes ALTER COLUMN seller_id SET NOT NULL;
ALTER TABLE marketing_campaigns ALTER COLUMN seller_id SET NOT NULL;
