import pg from 'pg'
const { Pool } = pg

const pool = new Pool({
  connectionString: process.env.DATABASE_URL || 'postgresql://postgres:Gdx9pyrz.@localhost:5432/star_trail'
})

const createTables = `
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    gold INTEGER NOT NULL DEFAULT 1000
);
CREATE TABLE IF NOT EXISTS player_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id VARCHAR(64) NOT NULL,
    item_id VARCHAR(64) NOT NULL,
    item_data JSONB NOT NULL DEFAULT '{}',
    quantity INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_inventory_player ON player_inventory(player_id);
CREATE TABLE IF NOT EXISTS market_listings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id VARCHAR(64) NOT NULL,
    item_id VARCHAR(64) NOT NULL,
    item_data JSONB NOT NULL DEFAULT '{}',
    price INTEGER NOT NULL CHECK (price > 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    status VARCHAR(10) DEFAULT 'active' CHECK (status IN ('active', 'sold', 'cancelled')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_market_status ON market_listings(status);
CREATE INDEX IF NOT EXISTS idx_market_price ON market_listings(price);
CREATE TABLE IF NOT EXISTS market_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id UUID REFERENCES market_listings(id),
    buyer_id VARCHAR(64) NOT NULL,
    seller_id VARCHAR(64) NOT NULL,
    price INTEGER NOT NULL,
    quantity INTEGER NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
`

async function runMigration() {
  try {
    const client = await pool.connect()
    console.log('Connected to PostgreSQL')
    await client.query(createTables)
    console.log('Tables created successfully')
    client.release()
    pool.end()
  } catch (err) {
    console.error('Migration failed:', err)
    process.exit(1)
  }
}

runMigration()