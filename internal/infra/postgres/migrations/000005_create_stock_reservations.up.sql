CREATE TABLE IF NOT EXISTS stock_reservations (
    id UUID PRIMARY KEY,

    order_id UUID NOT NULL,

    batch_id UUID NOT NULL,

    quantity NUMERIC(12,2) NOT NULL
        CHECK (quantity > 0),

    created_at TIMESTAMP NOT NULL DEFAULT now(),

    CONSTRAINT fk_reservation_batch
        FOREIGN KEY (batch_id)
        REFERENCES product_batches(id)
        ON DELETE CASCADE
);