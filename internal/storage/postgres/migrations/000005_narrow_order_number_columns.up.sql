ALTER TABLE orders ALTER COLUMN number TYPE varchar(32);
ALTER TABLE withdrawals ALTER COLUMN "order" TYPE varchar(32);