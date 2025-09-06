package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// QueryBuilder provides a fluent interface for building SQL queries
type QueryBuilder struct {
	db          *sql.DB
	table       string
	selectCols  []string
	whereConds  []whereCondition
	orderBy     []string
	groupBy     []string
	having      []whereCondition
	joins       []joinClause
	limitCount  int
	offsetCount int
	unionQuery  *QueryBuilder
	unionAll    bool
}

type whereCondition struct {
	column   string
	operator string
	value    interface{}
	logic    string // AND or OR
}

type joinClause struct {
	joinType string // INNER, LEFT, RIGHT, FULL
	table    string
	on       string
}

// NewQueryBuilder creates a new query builder instance
func NewQueryBuilder(db *sql.DB) *QueryBuilder {
	return &QueryBuilder{
		db:         db,
		selectCols: []string{},
		whereConds: []whereCondition{},
		orderBy:    []string{},
		groupBy:    []string{},
		having:     []whereCondition{},
		joins:      []joinClause{},
	}
}

// Table sets the table name for the query
func (qb *QueryBuilder) Table(table string) *QueryBuilder {
	qb.table = table
	return qb
}

// Select specifies the columns to select
func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.selectCols = columns
	return qb
}

// Where adds a WHERE condition
func (qb *QueryBuilder) Where(column, operator string, value interface{}) *QueryBuilder {
	qb.whereConds = append(qb.whereConds, whereCondition{
		column:   column,
		operator: operator,
		value:    value,
		logic:    "AND",
	})
	return qb
}

// OrWhere adds an OR WHERE condition
func (qb *QueryBuilder) OrWhere(column, operator string, value interface{}) *QueryBuilder {
	qb.whereConds = append(qb.whereConds, whereCondition{
		column:   column,
		operator: operator,
		value:    value,
		logic:    "OR",
	})
	return qb
}

// WhereIn adds a WHERE IN condition
func (qb *QueryBuilder) WhereIn(column string, values []interface{}) *QueryBuilder {
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	inClause := fmt.Sprintf("(%s)", strings.Join(placeholders, ", "))
	
	qb.whereConds = append(qb.whereConds, whereCondition{
		column:   column,
		operator: "IN",
		value:    inClause,
		logic:    "AND",
	})
	return qb
}

// WhereBetween adds a BETWEEN condition
func (qb *QueryBuilder) WhereBetween(column string, start, end interface{}) *QueryBuilder {
	qb.whereConds = append(qb.whereConds, whereCondition{
		column:   column,
		operator: "BETWEEN",
		value:    fmt.Sprintf("%v AND %v", start, end),
		logic:    "AND",
	})
	return qb
}

// WhereNull adds a WHERE column IS NULL condition
func (qb *QueryBuilder) WhereNull(column string) *QueryBuilder {
	qb.whereConds = append(qb.whereConds, whereCondition{
		column:   column,
		operator: "IS NULL",
		value:    "",
		logic:    "AND",
	})
	return qb
}

// WhereNotNull adds a WHERE column IS NOT NULL condition
func (qb *QueryBuilder) WhereNotNull(column string) *QueryBuilder {
	qb.whereConds = append(qb.whereConds, whereCondition{
		column:   column,
		operator: "IS NOT NULL",
		value:    "",
		logic:    "AND",
	})
	return qb
}

// Join adds an INNER JOIN
func (qb *QueryBuilder) Join(table, on string) *QueryBuilder {
	qb.joins = append(qb.joins, joinClause{
		joinType: "INNER",
		table:    table,
		on:       on,
	})
	return qb
}

// LeftJoin adds a LEFT JOIN
func (qb *QueryBuilder) LeftJoin(table, on string) *QueryBuilder {
	qb.joins = append(qb.joins, joinClause{
		joinType: "LEFT",
		table:    table,
		on:       on,
	})
	return qb
}

// RightJoin adds a RIGHT JOIN
func (qb *QueryBuilder) RightJoin(table, on string) *QueryBuilder {
	qb.joins = append(qb.joins, joinClause{
		joinType: "RIGHT",
		table:    table,
		on:       on,
	})
	return qb
}

// OrderBy adds an ORDER BY clause
func (qb *QueryBuilder) OrderBy(column, direction string) *QueryBuilder {
	if direction == "" {
		direction = "ASC"
	}
	qb.orderBy = append(qb.orderBy, fmt.Sprintf("%s %s", column, strings.ToUpper(direction)))
	return qb
}

// GroupBy adds a GROUP BY clause
func (qb *QueryBuilder) GroupBy(columns ...string) *QueryBuilder {
	qb.groupBy = append(qb.groupBy, columns...)
	return qb
}

// Having adds a HAVING condition
func (qb *QueryBuilder) Having(column, operator string, value interface{}) *QueryBuilder {
	qb.having = append(qb.having, whereCondition{
		column:   column,
		operator: operator,
		value:    value,
		logic:    "AND",
	})
	return qb
}

// Limit sets the LIMIT clause
func (qb *QueryBuilder) Limit(count int) *QueryBuilder {
	qb.limitCount = count
	return qb
}

// Offset sets the OFFSET clause
func (qb *QueryBuilder) Offset(count int) *QueryBuilder {
	qb.offsetCount = count
	return qb
}

// Union adds a UNION clause
func (qb *QueryBuilder) Union(query *QueryBuilder) *QueryBuilder {
	qb.unionQuery = query
	qb.unionAll = false
	return qb
}

// UnionAll adds a UNION ALL clause
func (qb *QueryBuilder) UnionAll(query *QueryBuilder) *QueryBuilder {
	qb.unionQuery = query
	qb.unionAll = true
	return qb
}

// ToSQL builds the SQL query string and returns it with parameters
func (qb *QueryBuilder) ToSQL() (string, []interface{}, error) {
	if qb.table == "" {
		return "", nil, fmt.Errorf("table name is required")
	}

	var query strings.Builder
	var params []interface{}

	// SELECT clause
	query.WriteString("SELECT ")
	if len(qb.selectCols) == 0 {
		query.WriteString("*")
	} else {
		query.WriteString(strings.Join(qb.selectCols, ", "))
	}

	// FROM clause
	query.WriteString(fmt.Sprintf(" FROM %s", qb.table))

	// JOIN clauses
	for _, join := range qb.joins {
		query.WriteString(fmt.Sprintf(" %s JOIN %s ON %s", join.joinType, join.table, join.on))
	}

	// WHERE clauses
	if len(qb.whereConds) > 0 {
		query.WriteString(" WHERE ")
		for i, cond := range qb.whereConds {
			if i > 0 {
				query.WriteString(fmt.Sprintf(" %s ", cond.logic))
			}
			
			switch cond.operator {
			case "IS NULL", "IS NOT NULL":
				query.WriteString(fmt.Sprintf("%s %s", cond.column, cond.operator))
			case "IN":
				query.WriteString(fmt.Sprintf("%s %s %s", cond.column, cond.operator, cond.value))
			case "BETWEEN":
				query.WriteString(fmt.Sprintf("%s %s %s", cond.column, cond.operator, cond.value))
			default:
				query.WriteString(fmt.Sprintf("%s %s ?", cond.column, cond.operator))
				params = append(params, cond.value)
			}
		}
	}

	// GROUP BY clause
	if len(qb.groupBy) > 0 {
		query.WriteString(fmt.Sprintf(" GROUP BY %s", strings.Join(qb.groupBy, ", ")))
	}

	// HAVING clause
	if len(qb.having) > 0 {
		query.WriteString(" HAVING ")
		for i, cond := range qb.having {
			if i > 0 {
				query.WriteString(fmt.Sprintf(" %s ", cond.logic))
			}
			query.WriteString(fmt.Sprintf("%s %s ?", cond.column, cond.operator))
			params = append(params, cond.value)
		}
	}

	// ORDER BY clause
	if len(qb.orderBy) > 0 {
		query.WriteString(fmt.Sprintf(" ORDER BY %s", strings.Join(qb.orderBy, ", ")))
	}

	// LIMIT clause
	if qb.limitCount > 0 {
		query.WriteString(fmt.Sprintf(" LIMIT %d", qb.limitCount))
	}

	// OFFSET clause
	if qb.offsetCount > 0 {
		query.WriteString(fmt.Sprintf(" OFFSET %d", qb.offsetCount))
	}

	// UNION clause
	if qb.unionQuery != nil {
		unionSQL, unionParams, err := qb.unionQuery.ToSQL()
		if err != nil {
			return "", nil, err
		}
		
		unionType := "UNION"
		if qb.unionAll {
			unionType = "UNION ALL"
		}
		
		query.WriteString(fmt.Sprintf(" %s %s", unionType, unionSQL))
		params = append(params, unionParams...)
	}

	return query.String(), params, nil
}

// Get executes the query and returns all rows
func (qb *QueryBuilder) Get() (*sql.Rows, error) {
	query, params, err := qb.ToSQL()
	if err != nil {
		return nil, err
	}
	
	return qb.db.Query(query, params...)
}

// First executes the query and returns the first row
func (qb *QueryBuilder) First() *sql.Row {
	qb.Limit(1)
	query, params, _ := qb.ToSQL()
	
	return qb.db.QueryRow(query, params...)
}

// Count returns the count of rows
func (qb *QueryBuilder) Count() (int64, error) {
	originalSelect := qb.selectCols
	qb.selectCols = []string{"COUNT(*)"}
	
	query, params, err := qb.ToSQL()
	if err != nil {
		return 0, err
	}
	
	// Restore original select
	qb.selectCols = originalSelect
	
	var count int64
	err = qb.db.QueryRow(query, params...).Scan(&count)
	return count, err
}

// Exists checks if any rows exist
func (qb *QueryBuilder) Exists() (bool, error) {
	count, err := qb.Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Paginate adds pagination to the query
func (qb *QueryBuilder) Paginate(page, perPage int) *QueryBuilder {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}
	
	offset := (page - 1) * perPage
	return qb.Limit(perPage).Offset(offset)
}

// Insert builds and executes an INSERT query
func (qb *QueryBuilder) Insert(data map[string]interface{}) (sql.Result, error) {
	if qb.table == "" {
		return nil, fmt.Errorf("table name is required")
	}
	
	var columns []string
	var placeholders []string
	var values []interface{}
	
	for col, val := range data {
		columns = append(columns, col)
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		qb.table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))
	
	return qb.db.Exec(query, values...)
}

// Update builds and executes an UPDATE query
func (qb *QueryBuilder) Update(data map[string]interface{}) (sql.Result, error) {
	if qb.table == "" {
		return nil, fmt.Errorf("table name is required")
	}
	
	var setParts []string
	var params []interface{}
	
	for col, val := range data {
		setParts = append(setParts, fmt.Sprintf("%s = ?", col))
		params = append(params, val)
	}
	
	query := fmt.Sprintf("UPDATE %s SET %s", qb.table, strings.Join(setParts, ", "))
	
	// Add WHERE conditions
	if len(qb.whereConds) > 0 {
		query += " WHERE "
		for i, cond := range qb.whereConds {
			if i > 0 {
				query += fmt.Sprintf(" %s ", cond.logic)
			}
			query += fmt.Sprintf("%s %s ?", cond.column, cond.operator)
			params = append(params, cond.value)
		}
	}
	
	return qb.db.Exec(query, params...)
}

// Delete builds and executes a DELETE query
func (qb *QueryBuilder) Delete() (sql.Result, error) {
	if qb.table == "" {
		return nil, fmt.Errorf("table name is required")
	}
	
	query := fmt.Sprintf("DELETE FROM %s", qb.table)
	var params []interface{}
	
	// Add WHERE conditions
	if len(qb.whereConds) > 0 {
		query += " WHERE "
		for i, cond := range qb.whereConds {
			if i > 0 {
				query += fmt.Sprintf(" %s ", cond.logic)
			}
			query += fmt.Sprintf("%s %s ?", cond.column, cond.operator)
			params = append(params, cond.value)
		}
	}
	
	return qb.db.Exec(query, params...)
}

// Raw executes a raw SQL query
func (qb *QueryBuilder) Raw(query string, params ...interface{}) (*sql.Rows, error) {
	return qb.db.Query(query, params...)
}

// RawExec executes a raw SQL statement
func (qb *QueryBuilder) RawExec(query string, params ...interface{}) (sql.Result, error) {
	return qb.db.Exec(query, params...)
}