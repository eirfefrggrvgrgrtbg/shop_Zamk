DROP INDEX IF EXISTS sellers_average_rating_idx;
ALTER TABLE sellers DROP COLUMN IF EXISTS reviews_count;
ALTER TABLE sellers DROP COLUMN IF EXISTS average_rating;

DROP INDEX IF EXISTS products_average_rating_idx;
ALTER TABLE products DROP COLUMN IF EXISTS reviews_count;
ALTER TABLE products DROP COLUMN IF EXISTS average_rating;
