CREATE TABLE IF NOT EXISTS product_batches (
    id UUID PRIMARY KEY,

    product_id UUID NOT NULL,

    warehouse_company_id UUID,

    quantity_total NUMERIC(12,2) NOT NULL,
    quantity_reserved NUMERIC(12,2) NOT NULL,

    production_date DATE,
    expiration_date DATE,

    batch_number TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT now(),

    CONSTRAINT fk_batch_product
        FOREIGN KEY (product_id)
        REFERENCES products(id)
        ON DELETE CASCADE
);