DROP TABLE IF EXISTS product_embeddings;
-- Notice: We do NOT DROP EXTENSION vector here because other tables/features
-- might depend on it in the future, and dropping it would cascade and destroy them.
