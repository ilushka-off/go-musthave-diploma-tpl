ALTER TABLE orders
    ALTER COLUMN status TYPE varchar(255) USING status::text;

ALTER TABLE orders
    ADD CONSTRAINT orders_status_check CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED'));

DROP TYPE order_status;