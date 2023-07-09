CREATE TABLE IF NOT EXISTS products (
    product_id bigserial PRIMARY KEY,
    name text NOT NULL,
    about text NOT NULL,
    stars smallint NOT NULL DEFAULT 0,
    version int NOT NULL DEFAULT 1
);