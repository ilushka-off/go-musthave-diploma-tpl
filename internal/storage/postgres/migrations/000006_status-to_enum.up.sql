CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');

ALTER TABLE orders
    ALTER COLUMN status TYPE order_status USING status::order_status;

ALTER TABLE orders DROP CONSTRAINT orders_status_check;