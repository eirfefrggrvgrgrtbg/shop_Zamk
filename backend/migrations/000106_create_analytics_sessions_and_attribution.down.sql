DROP INDEX IF EXISTS idx_behav_session_time;
ALTER TABLE behavioral_events DROP COLUMN IF EXISTS session_id;

DROP TABLE IF EXISTS order_attributions;
DROP TABLE IF EXISTS analytics_sessions;
