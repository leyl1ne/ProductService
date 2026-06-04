CREATE INDEX IF NOT EXISTS idx_stock_reservations_order_id
ON stock_reservations(order_id);

CREATE INDEX IF NOT EXISTS idx_stock_reservations_batch_id
ON stock_reservations(batch_id);