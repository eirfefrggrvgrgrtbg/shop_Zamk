ALTER TABLE staff_members ADD COLUMN responsibilities TEXT NULL;
ALTER TABLE staff_members ADD COLUMN work_note TEXT NULL;

ALTER TABLE staff_members ADD CONSTRAINT responsibilities_length CHECK (responsibilities IS NULL OR char_length(responsibilities) <= 4000);
ALTER TABLE staff_members ADD CONSTRAINT work_note_length CHECK (work_note IS NULL OR char_length(work_note) <= 4000);
