package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

// TestSeeder is a simple seeder for testing
type TestSeeder struct {
	BaseSeeder
	executed bool
}

func NewTestSeeder() *TestSeeder {
	return &TestSeeder{
		BaseSeeder: BaseSeeder{TableName: "test_table"},
		executed:   false,
	}
}

func (ts *TestSeeder) Run(db *sql.DB) error {
	ts.executed = true
	return nil
}

func TestSeederRegistry(t *testing.T) {
	registry := NewSeederRegistry()
	
	t.Run("register and list seeders", func(t *testing.T) {
		seeder1 := NewTestSeeder()
		seeder2 := NewTestSeeder()
		
		registry.Register("test1", seeder1)
		registry.Register("test2", seeder2)
		
		seeders := registry.ListSeeders()
		assert.Len(t, seeders, 2)
		assert.Contains(t, seeders, "test1")
		assert.Contains(t, seeders, "test2")
	})
	
	t.Run("run specific seeder", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		
		registry := NewSeederRegistry()
		seeder := NewTestSeeder()
		registry.Register("test", seeder)
		
		err := registry.Run(db, "test")
		require.NoError(t, err)
		assert.True(t, seeder.executed)
	})
	
	t.Run("run non-existent seeder", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		
		registry := NewSeederRegistry()
		
		err := registry.Run(db, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "seeder nonexistent not found")
	})
	
	t.Run("run all seeders", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		
		registry := NewSeederRegistry()
		seeder1 := NewTestSeeder()
		seeder2 := NewTestSeeder()
		
		registry.Register("test1", seeder1)
		registry.Register("test2", seeder2)
		
		err := registry.RunAll(db)
		require.NoError(t, err)
		assert.True(t, seeder1.executed)
		assert.True(t, seeder2.executed)
	})
}

func TestBaseSeeder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	// Create test table
	createTableSQL := `
		CREATE TABLE test_users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER
		)
	`
	_, err := db.Exec(createTableSQL)
	require.NoError(t, err)
	
	seeder := &BaseSeeder{TableName: "test_users"}
	
	t.Run("bulk insert", func(t *testing.T) {
		data := []map[string]interface{}{
			{"name": "John Doe", "email": "john@example.com", "age": 30},
			{"name": "Jane Smith", "email": "jane@example.com", "age": 25},
			{"name": "Bob Johnson", "email": "bob@example.com", "age": 35},
		}
		
		err := seeder.BulkInsert(db, data)
		require.NoError(t, err)
		
		// Verify data was inserted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM test_users").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
		
		// Verify specific data
		var name, email string
		var age int
		err = db.QueryRow("SELECT name, email, age FROM test_users WHERE id = 1").Scan(&name, &email, &age)
		require.NoError(t, err)
		assert.Equal(t, "John Doe", name)
		assert.Equal(t, "john@example.com", email)
		assert.Equal(t, 30, age)
	})
	
	t.Run("truncate table", func(t *testing.T) {
		// First verify we have data
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM test_users").Scan(&count)
		require.NoError(t, err)
		assert.Greater(t, count, 0)
		
		// Truncate
		err = seeder.TruncateTable(db)
		require.NoError(t, err)
		
		// Verify table is empty
		err = db.QueryRow("SELECT COUNT(*) FROM test_users").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
	
	t.Run("drop and recreate table", func(t *testing.T) {
		createSQL := `
			CREATE TABLE test_users (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				email TEXT UNIQUE NOT NULL,
				age INTEGER,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`
		
		err := seeder.DropAndRecreateTable(db, createSQL)
		require.NoError(t, err)
		
		// Verify table structure by inserting data
		_, err = db.Exec("INSERT INTO test_users (name, email, age) VALUES (?, ?, ?)", 
			"Test User", "test@example.com", 25)
		require.NoError(t, err)
		
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM test_users").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestFactory(t *testing.T) {
	t.Run("create records", func(t *testing.T) {
		factory := NewFactory()
		
		template := map[string]interface{}{
			"name":  "User",
			"email": "user@example.com",
			"age":   25,
		}
		
		factory.Create(3, template)
		data := factory.GetData()
		
		assert.Len(t, data, 3)
		for _, record := range data {
			assert.Equal(t, "User", record["name"])
			assert.Equal(t, "user@example.com", record["email"])
			assert.Equal(t, 25, record["age"])
		}
	})
	
	t.Run("create one record", func(t *testing.T) {
		factory := NewFactory()
		
		record := map[string]interface{}{
			"name":  "Single User",
			"email": "single@example.com",
			"age":   30,
		}
		
		factory.CreateOne(record)
		data := factory.GetData()
		
		assert.Len(t, data, 1)
		assert.Equal(t, "Single User", data[0]["name"])
		assert.Equal(t, "single@example.com", data[0]["email"])
		assert.Equal(t, 30, data[0]["age"])
	})
	
	t.Run("create with functions", func(t *testing.T) {
		factory := NewFactory()
		faker := NewFaker()
		
		template := map[string]interface{}{
			"name":  faker.Name(),
			"email": faker.Email(),
			"age":   faker.RandomInt(18, 80),
		}
		
		factory.Create(5, template)
		data := factory.GetData()
		
		assert.Len(t, data, 5)
		
		// Check that different values are generated
		names := make(map[string]bool)
		emails := make(map[string]bool)
		
		for _, record := range data {
			name := record["name"].(string)
			email := record["email"].(string)
			age := record["age"].(int)
			
			names[name] = true
			emails[email] = true
			
			assert.NotEmpty(t, name)
			assert.NotEmpty(t, email)
			assert.GreaterOrEqual(t, age, 18)
			assert.LessOrEqual(t, age, 80)
		}
		
		// Should have some variety in names and emails
		assert.Greater(t, len(names), 1)
		assert.Greater(t, len(emails), 1)
	})
}

func TestFaker(t *testing.T) {
	faker := NewFaker()
	
	t.Run("name generator", func(t *testing.T) {
		nameFunc := faker.Name()
		
		name1 := nameFunc(0).(string)
		name2 := nameFunc(1).(string)
		
		assert.NotEmpty(t, name1)
		assert.NotEmpty(t, name2)
		assert.IsType(t, "", name1)
		assert.IsType(t, "", name2)
	})
	
	t.Run("email generator", func(t *testing.T) {
		emailFunc := faker.Email()
		
		email1 := emailFunc(0).(string)
		email2 := emailFunc(1).(string)
		
		assert.Contains(t, email1, "@")
		assert.Contains(t, email2, "@")
		assert.NotEqual(t, email1, email2)
	})
	
	t.Run("random int generator", func(t *testing.T) {
		intFunc := faker.RandomInt(10, 20)
		
		for i := 0; i < 10; i++ {
			val := intFunc(i).(int)
			assert.GreaterOrEqual(t, val, 10)
			assert.LessOrEqual(t, val, 20)
		}
	})
	
	t.Run("sequence generator", func(t *testing.T) {
		seqFunc := faker.Sequence("item-%d")
		
		assert.Equal(t, "item-1", seqFunc(0).(string))
		assert.Equal(t, "item-2", seqFunc(1).(string))
		assert.Equal(t, "item-3", seqFunc(2).(string))
	})
	
	t.Run("boolean generator", func(t *testing.T) {
		boolFunc := faker.Boolean()
		
		val1 := boolFunc(0).(bool)
		val2 := boolFunc(1).(bool)
		val3 := boolFunc(2).(bool)
		
		assert.True(t, val1)   // index 0 (even)
		assert.False(t, val2)  // index 1 (odd)
		assert.True(t, val3)   // index 2 (even)
	})
}

func TestUserSeeder(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	// Create users table
	createTableSQL := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER,
			is_active BOOLEAN,
			created_at TEXT,
			updated_at TEXT
		)
	`
	_, err := db.Exec(createTableSQL)
	require.NoError(t, err)
	
	t.Run("run user seeder", func(t *testing.T) {
		seeder := NewUserSeeder()
		
		err := seeder.Run(db)
		require.NoError(t, err)
		
		// Verify data was inserted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 101, count) // 100 generated + 1 admin
		
		// Verify admin user exists
		var name, email string
		err = db.QueryRow("SELECT name, email FROM users WHERE email = 'admin@example.com'").Scan(&name, &email)
		require.NoError(t, err)
		assert.Equal(t, "Admin User", name)
		assert.Equal(t, "admin@example.com", email)
	})
}

func TestCreateSeederRegistry(t *testing.T) {
	registry := CreateSeederRegistry()
	
	seeders := registry.ListSeeders()
	assert.Contains(t, seeders, "users")
}

// Helper function to setup test database
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	
	// Enable foreign keys for SQLite
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	
	return db
}