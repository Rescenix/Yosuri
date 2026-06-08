import pg from 'pg'
const { Pool } = pg

const pool = new Pool({
  connectionString: process.env.DATABASE_URL || 'postgresql://postgres:Gdx9pyrz.@localhost:5432/star_trail',
  max: 20,
})

export default pool