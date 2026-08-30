BEGIN;

-- Allow empty body for attachment-only messages while keeping NOT NULL
ALTER TABLE return_messages DROP CONSTRAINT IF EXISTS return_messages_body_check;
ALTER TABLE return_messages DROP CONSTRAINT IF EXISTS return_messages_check;

-- Harden return_message_attachments foreign key to prevent accidental deletion of message history
ALTER TABLE return_message_attachments DROP CONSTRAINT IF EXISTS return_message_attachments_message_id_fkey;
ALTER TABLE return_message_attachments ADD CONSTRAINT return_message_attachments_message_id_fkey FOREIGN KEY (message_id) REFERENCES return_messages(id) ON DELETE RESTRICT;

COMMIT;
