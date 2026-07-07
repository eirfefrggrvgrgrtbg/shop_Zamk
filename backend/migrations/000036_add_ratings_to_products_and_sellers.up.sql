ALTER TABLE products ADD COLUMN average_rating NUMERIC(3,1) NOT NULL DEFAULT 0.0;
ALTER TABLE products ADD COLUMN reviews_count INT NOT NULL DEFAULT 0;
CREATE INDEX products_average_rating_idx ON products(average_rating);

ALTER TABLE sellers ADD COLUMN average_rating NUMERIC(3,1) NOT NULL DEFAULT 0.0;
ALTER TABLE sellers ADD COLUMN reviews_count INT NOT NULL DEFAULT 0;
CREATE INDEX sellers_average_rating_idx ON sellers(average_rating);
