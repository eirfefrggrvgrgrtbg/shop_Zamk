ALTER TABLE sellers DROP CONSTRAINT valid_status;
ALTER TABLE sellers ADD CONSTRAINT valid_status CHECK (status IN ('pending_setup', 'pending', 'active', 'blocked', 'archived'));
