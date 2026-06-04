CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY,

    company_id UUID NOT NULL,
    category_id UUID,

    name TEXT NOT NULL,
    description TEXT,

    price NUMERIC(12,2) NOT NULL,
    unit TEXT NOT NULL,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),

    CONSTRAINT fk_product_category 
        FOREIGN KEY (category_id) 
        REFERENCES categories(id)
);