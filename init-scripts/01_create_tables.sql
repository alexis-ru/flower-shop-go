CREATE TABLE IF NOT EXISTS users (
    id                 SERIAL       PRIMARY KEY,
    full_name          VARCHAR(200) NOT NULL,
    login              VARCHAR(100) UNIQUE  NOT NULL,
    password_hash      VARCHAR(255) NOT NULL,
    role               VARCHAR(20)  NOT NULL DEFAULT 'seller',
    status             VARCHAR(20)  NOT NULL DEFAULT 'active',
    registration_date  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dismissal_date     TIMESTAMP,
    block_date         TIMESTAMP
);

CREATE TABLE IF NOT EXISTS flowers (
    id            SERIAL       PRIMARY KEY,
    seller_id     INTEGER      REFERENCES users(id) ON DELETE CASCADE,
    name          VARCHAR(200) NOT NULL,
    quantity      INTEGER      NOT NULL DEFAULT 0,
    arrival_date  DATE,
    sale_date     DATE,
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_flowers_seller ON flowers(seller_id);
