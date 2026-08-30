BEGIN;

ALTER TABLE return_message_attachments DROP CONSTRAINT IF EXISTS return_message_attachments_message_id_fkey;
ALTER TABLE return_message_attachments ADD CONSTRAINT return_message_attachments_message_id_fkey FOREIGN KEY (message_id) REFERENCES return_messages(id) ON DELETE CASCADE;

ALTER TABLE return_messages ADD CONSTRAINT return_messages_body_check CHECK (length(trim(body)) > 0);

COMMIT;
