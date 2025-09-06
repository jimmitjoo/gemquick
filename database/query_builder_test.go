package database

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryBuilder_Select(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("select all", func(t *testing.T) {
		sql, params, err := qb.Table("users").ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users", sql)
		assert.Empty(t, params)
	})
	
	t.Run("select specific columns", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").Select("id", "name", "email").ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT id, name, email FROM users", sql)
		assert.Empty(t, params)
	})
}

func TestQueryBuilder_Where(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("single where condition", func(t *testing.T) {
		sql, params, err := qb.Table("users").Where("id", "=", 1).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", sql)
		assert.Equal(t, []interface{}{1}, params)
	})
	
	t.Run("multiple where conditions", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			Where("id", ">", 1).
			Where("status", "=", "active").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id > ? AND status = ?", sql)
		assert.Equal(t, []interface{}{1, "active"}, params)
	})
	
	t.Run("or where condition", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			Where("id", "=", 1).
			OrWhere("id", "=", 2).
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = ? OR id = ?", sql)
		assert.Equal(t, []interface{}{1, 2}, params)
	})
	
	t.Run("where in", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			WhereIn("id", []interface{}{1, 2, 3}).
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id IN (?, ?, ?)", sql)
		assert.Equal(t, []interface{}{1, 2, 3}, params) // Parameters now properly handled
	})
	
	t.Run("where between", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			WhereBetween("age", 18, 65).
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE age BETWEEN ? AND ?", sql)
		assert.Equal(t, []interface{}{18, 65}, params) // Parameters now properly handled for security
	})
	
	t.Run("where null", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			WhereNull("deleted_at").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE deleted_at IS NULL", sql)
		assert.Empty(t, params)
	})
	
	t.Run("where not null", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			WhereNotNull("email").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE email IS NOT NULL", sql)
		assert.Empty(t, params)
	})
}

func TestQueryBuilder_Joins(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("inner join", func(t *testing.T) {
		sql, params, err := qb.Table("users").
			Join("profiles", "users.id = profiles.user_id").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users INNER JOIN profiles ON users.id = profiles.user_id", sql)
		assert.Empty(t, params)
	})
	
	t.Run("left join", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			LeftJoin("profiles", "users.id = profiles.user_id").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users LEFT JOIN profiles ON users.id = profiles.user_id", sql)
		assert.Empty(t, params)
	})
	
	t.Run("right join", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			RightJoin("profiles", "users.id = profiles.user_id").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users RIGHT JOIN profiles ON users.id = profiles.user_id", sql)
		assert.Empty(t, params)
	})
	
	t.Run("multiple joins", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			Join("profiles", "users.id = profiles.user_id").
			LeftJoin("orders", "users.id = orders.user_id").
			ToSQL()
		require.NoError(t, err)
		expected := "SELECT * FROM users INNER JOIN profiles ON users.id = profiles.user_id LEFT JOIN orders ON users.id = orders.user_id"
		assert.Equal(t, expected, sql)
		assert.Empty(t, params)
	})
}

func TestQueryBuilder_OrderGroupBy(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("order by ascending", func(t *testing.T) {
		sql, params, err := qb.Table("users").
			OrderBy("name", "ASC").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users ORDER BY name ASC", sql)
		assert.Empty(t, params)
	})
	
	t.Run("order by descending", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			OrderBy("created_at", "DESC").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users ORDER BY created_at DESC", sql)
		assert.Empty(t, params)
	})
	
	t.Run("multiple order by", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").
			OrderBy("status", "ASC").
			OrderBy("created_at", "DESC").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users ORDER BY status ASC, created_at DESC", sql)
		assert.Empty(t, params)
	})
	
	t.Run("group by", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("orders").
			Select("user_id", "COUNT(*)").
			GroupBy("user_id").
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT user_id, COUNT(*) FROM orders GROUP BY user_id", sql)
		assert.Empty(t, params)
	})
	
	t.Run("having", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("orders").
			Select("user_id", "COUNT(*)").
			GroupBy("user_id").
			Having("COUNT(*)", ">", 5).
			ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT user_id, COUNT(*) FROM orders GROUP BY user_id HAVING COUNT(*) > ?", sql)
		assert.Equal(t, []interface{}{5}, params)
	})
}

func TestQueryBuilder_LimitOffset(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("limit", func(t *testing.T) {
		sql, params, err := qb.Table("users").Limit(10).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users LIMIT 10", sql)
		assert.Empty(t, params)
	})
	
	t.Run("limit with offset", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").Limit(10).Offset(20).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users LIMIT 10 OFFSET 20", sql)
		assert.Empty(t, params)
	})
	
	t.Run("paginate", func(t *testing.T) {
		qb = NewQueryBuilder(nil)
		sql, params, err := qb.Table("users").Paginate(2, 15).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users LIMIT 15 OFFSET 15", sql)
		assert.Empty(t, params)
	})
}

func TestQueryBuilder_Complex(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("complex query", func(t *testing.T) {
		sql, params, err := qb.Table("users").
			Select("users.id", "users.name", "profiles.bio").
			Join("profiles", "users.id = profiles.user_id").
			Where("users.status", "=", "active").
			Where("users.age", ">=", 18).
			OrWhere("users.role", "=", "admin").
			OrderBy("users.created_at", "DESC").
			Limit(50).
			ToSQL()
		
		require.NoError(t, err)
		expected := "SELECT users.id, users.name, profiles.bio FROM users INNER JOIN profiles ON users.id = profiles.user_id WHERE users.status = ? AND users.age >= ? OR users.role = ? ORDER BY users.created_at DESC LIMIT 50"
		assert.Equal(t, expected, sql)
		assert.Equal(t, []interface{}{"active", 18, "admin"}, params)
	})
}

func TestQueryBuilder_Errors(t *testing.T) {
	qb := NewQueryBuilder(nil)
	
	t.Run("missing table", func(t *testing.T) {
		_, _, err := qb.Where("id", "=", 1).ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table name is required")
	})
}

func TestQueryBuilder_InsertUpdateDelete(t *testing.T) {
	// Create a test database connection for real operations
	db := setupTestDB(t)
	defer db.Close()
	
	// Create a test table
	_, err := db.Exec(`CREATE TABLE test_operations (
		id INTEGER PRIMARY KEY,
		name TEXT,
		email TEXT,
		age INTEGER
	)`)
	require.NoError(t, err)
	
	qb := NewQueryBuilder(db)
	
	t.Run("insert data structure", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		}
		
		result, err := qb.Table("test_operations").Insert(data)
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		id, err := result.LastInsertId()
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))
	})
	
	t.Run("update data structure", func(t *testing.T) {
		data := map[string]interface{}{
			"name": "Jane Doe",
			"age":  25,
		}
		
		result, err := qb.Table("test_operations").
			Where("id", "=", 1).
			Update(data)
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		rowsAffected, err := result.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)
	})
	
	t.Run("delete structure", func(t *testing.T) {
		result, err := qb.Table("test_operations").
			Where("id", "=", 1).
			Delete()
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		rowsAffected, err := result.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsAffected)
	})
}