import pg from 'pg'
const { Client } = pg

async function createDatabase() {
  const client = new Client({ 
    connectionString: process.env.DATABASE_URL || 'postgresql://postgres:Gdx9pyrz.@localhost:5432/postgres' 
  })
  await client.connect()
  try {
    await client.query('CREATE DATABASE star_trail')
    console.log('Database star_trail created')
  } catch (err) {
    if (err.code === '42P04') {
      console.log('Database already exists')
    } else {
      throw err
    }
  } finally {
    await client.end()
  }
}

createDatabase()
