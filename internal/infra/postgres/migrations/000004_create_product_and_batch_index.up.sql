CREATE INDEX IF NOT EXISTS idx_products_company_id
ON products(company_id);

CREATE INDEX IF NOT EXISTS idx_products_category_id
ON products(category_id);

CREATE INDEX IF NOT EXISTS idx_products_is_active
ON products(is_active);

CREATE INDEX IF NOT EXISTS idx_batches_product_id
ON product_batches(product_id);

CREATE INDEX IF NOT EXISTS idx_batches_expiration_date
ON product_batches(expiration_date);