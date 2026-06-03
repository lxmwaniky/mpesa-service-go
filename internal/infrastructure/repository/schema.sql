CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    external_reference VARCHAR(255) NOT NULL,
    merchant_request_id VARCHAR(255) UNIQUE,
    checkout_request_id VARCHAR(255) UNIQUE,
    phone_number VARCHAR(15) NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    mpesa_receipt_number VARCHAR(50) UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    result_code INT DEFAULT 0,
    result_desc TEXT,
    transaction_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_transactions_checkout_id ON transactions(checkout_request_id);
CREATE INDEX IF NOT EXISTS idx_transactions_external_ref ON transactions(external_reference);
CREATE INDEX IF NOT EXISTS idx_transactions_receipt ON transactions(mpesa_receipt_number);