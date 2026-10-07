BEGIN;
DROP TRIGGER marketing_campaign_purpose_immutable ON marketing_campaigns;
DROP FUNCTION prevent_campaign_purpose_change();
DROP INDEX idx_marketing_campaigns_purpose_created;
ALTER TABLE marketing_campaigns DROP COLUMN purpose;
COMMIT;
