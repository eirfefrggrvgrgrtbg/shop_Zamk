ALTER TABLE product_vision_runs ADD COLUMN IF NOT EXISTS taxonomy_context_json JSONB;
UPDATE product_vision_runs SET taxonomy_context_json = '{}'::jsonb WHERE taxonomy_context_json IS NULL;
ALTER TABLE product_vision_runs ALTER COLUMN taxonomy_context_json SET NOT NULL;
