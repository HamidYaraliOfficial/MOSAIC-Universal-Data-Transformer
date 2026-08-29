package connector

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mosaic/internal/schema"
)

// DBKind enumerates the database engines the Database Connector Layer
// targets. Each maps to a database/sql driver registered by name; the
// actual driver packages (lib/pq, go-sql-driver/mysql, mattn/go-sqlite3,
// microsoft/go-mssqldb) are intentionally not vendored into this module —
// they're added via `go get` per the install instructions in README so the
// core engine has zero mandatory third-party dependencies. Each driver
// self-registers under the sql.Register name below via its blank import.
type DBKind string

const (
	DBPostgres  DBKind = "postgres"
	DBMySQL     DBKind = "mysql"
	DBSQLite    DBKind = "sqlite"
	DBSQLServer DBKind = "sqlserver"
)

var driverNames = map[DBKind]string{
	DBPostgres:  "postgres",
	DBMySQL:     "mysql",
	DBSQLite:    "sqlite",
	DBSQLServer: "sqlserver",
}

// DatabaseConnection describes how to reach a database. Password/secret is
// resolved from security.Vault by the caller and passed in at connect time.
type DatabaseConnection struct {
	Kind     DBKind `json:"kind"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database"`
	Username string `json:"username,omitempty"`
	Password string `json:"-"`
	SSLMode  string `json:"sslMode,omitempty"`
	FilePath string `json:"filePath,omitempty"` // sqlite only
}

// DSN builds the driver-specific connection string for a DatabaseConnection.
func (c DatabaseConnection) DSN() (string, error) {
	switch c.Kind {
	case DBPostgres:
		ssl := c.SSLMode
		if ssl == "" {
			ssl = "prefer"
		}
		return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			c.Host, c.Port, c.Database, c.Username, c.Password, ssl), nil
	case DBMySQL:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", c.Username, c.Password, c.Host, c.Port, c.Database), nil
	case DBSQLite:
		return c.FilePath, nil
	case DBSQLServer:
		return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s", c.Username, c.Password, c.Host, c.Port, c.Database), nil
	default:
		return "", fmt.Errorf("connector: unknown database kind %q", c.Kind)
	}
}

// DB wraps database/sql with the query surface the SQL Studio and
// Database-as-Input/Output nodes need: listing tables, running a bounded,
// validated query, and streaming results as schema.Row.
type DB struct {
	conn *sql.DB
	kind DBKind
}

// Open connects to a database using the registered driver for c.Kind. The
// driver must have been imported (blank import) somewhere in the binary,
// otherwise this returns a clear "unknown driver" error rather than
// panicking.
func Open(c DatabaseConnection) (*DB, error) {
	driver, ok := driverNames[c.Kind]
	if !ok {
		return nil, fmt.Errorf("connector: unsupported database kind %q", c.Kind)
	}
	dsn, err := c.DSN()
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("connector: opening %s connection: %w (is the '%s' driver registered? see README setup)", c.Kind, err, driver)
	}
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetMaxOpenConns(8)
	return &DB{conn: conn, kind: c.Kind}, nil
}

func (d *DB) Close() error { return d.conn.Close() }

// Ping verifies connectivity, used by the Connection editor's "Test
// Connection" button.
func (d *DB) Ping(ctx context.Context) error { return d.conn.PingContext(ctx) }

// maxRows bounds every ad-hoc query result to keep the SQL Studio and
// preview panels responsive; larger exports should go through a proper
// pipeline Input node with streaming instead.
const maxRows = 200_000

// Query runs a validated, read-only-by-convention SQL statement and
// returns MOSAIC rows. The SQL Studio is responsible for surfacing this
// bound to the user; Query itself enforces it defensively.
func (d *DB) Query(ctx context.Context, query string, args ...any) ([]string, []schema.Row, error) {
	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("connector: query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var out []schema.Row
	for rows.Next() {
		if len(out) >= maxRows {
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return cols, out, err
		}
		row := make(schema.Row, len(cols))
		for i, c := range cols {
			row[c] = normalizeSQLValue(vals[i])
		}
		out = append(out, row)
	}
	return cols, out, rows.Err()
}

func normalizeSQLValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return t
	}
}

// Tables lists table names for the SQL Studio's schema browser. The exact
// introspection query is driver-specific; this covers Postgres/MySQL/SQLite.
func (d *DB) Tables(ctx context.Context) ([]string, error) {
	var q string
	switch d.kind {
	case DBPostgres:
		q = `SELECT table_name FROM information_schema.tables WHERE table_schema='public'`
	case DBMySQL:
		q = `SHOW TABLES`
	case DBSQLite:
		q = `SELECT name FROM sqlite_master WHERE type='table'`
	default:
		return nil, fmt.Errorf("connector: table listing not implemented for %q", d.kind)
	}
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return tables, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}
