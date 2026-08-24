package sqlite3local

/*
#cgo LDFLAGS: -l:libsqlite3.so.0
#include <stdlib.h>
typedef struct sqlite3 sqlite3;
typedef struct sqlite3_stmt sqlite3_stmt;
typedef long long sqlite3_int64;
typedef void (*sqlite3_destructor_type)(void*);
int sqlite3_open_v2(const char*, sqlite3**, int, const char*);
int sqlite3_close_v2(sqlite3*);
const char *sqlite3_errmsg(sqlite3*);
int sqlite3_exec(sqlite3*, const char*, void*, void*, char**);
void sqlite3_free(void*);
int sqlite3_prepare_v2(sqlite3*, const char*, int, sqlite3_stmt**, const char**);
int sqlite3_finalize(sqlite3_stmt*);
int sqlite3_reset(sqlite3_stmt*);
int sqlite3_clear_bindings(sqlite3_stmt*);
int sqlite3_bind_parameter_count(sqlite3_stmt*);
int sqlite3_bind_null(sqlite3_stmt*, int);
int sqlite3_bind_int64(sqlite3_stmt*, int, sqlite3_int64);
int sqlite3_bind_double(sqlite3_stmt*, int, double);
int sqlite3_bind_text(sqlite3_stmt*, int, const char*, int, sqlite3_destructor_type);
int sqlite3_bind_blob(sqlite3_stmt*, int, const void*, int, sqlite3_destructor_type);
int sqlite3_step(sqlite3_stmt*);
int sqlite3_column_count(sqlite3_stmt*);
const char *sqlite3_column_name(sqlite3_stmt*, int);
int sqlite3_column_type(sqlite3_stmt*, int);
sqlite3_int64 sqlite3_column_int64(sqlite3_stmt*, int);
double sqlite3_column_double(sqlite3_stmt*, int);
const unsigned char *sqlite3_column_text(sqlite3_stmt*, int);
const void *sqlite3_column_blob(sqlite3_stmt*, int);
int sqlite3_column_bytes(sqlite3_stmt*, int);
int sqlite3_changes(sqlite3*);
sqlite3_int64 sqlite3_last_insert_rowid(sqlite3*);
int sqlite3_busy_timeout(sqlite3*, int);

static sqlite3_destructor_type transient_destructor() { return (sqlite3_destructor_type)-1; }
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	sqliteOK      = 0
	sqliteRow     = 100
	sqliteDone    = 101
	sqliteInteger = 1
	sqliteFloat   = 2
	sqliteText    = 3
	sqliteBlob    = 4
	sqliteNull    = 5
	openReadWrite = 0x00000002
	openCreate    = 0x00000004
	openFullMutex = 0x00010000
)

func init() { sql.Register("sqlite", &Driver{}) }

type Driver struct{}

func (d *Driver) Open(name string) (driver.Conn, error) {
	filename := strings.TrimPrefix(name, "file:")
	if index := strings.IndexByte(filename, '?'); index >= 0 {
		filename = filename[:index]
	}
	cName := C.CString(filename)
	defer C.free(unsafe.Pointer(cName))
	var handle *C.sqlite3
	code := C.sqlite3_open_v2(cName, &handle, openReadWrite|openCreate|openFullMutex, nil)
	if code != sqliteOK {
		message := "无法打开 SQLite"
		if handle != nil {
			message = C.GoString(C.sqlite3_errmsg(handle))
			C.sqlite3_close_v2(handle)
		}
		return nil, errors.New(message)
	}
	conn := &Conn{handle: handle}
	C.sqlite3_busy_timeout(handle, 5000)
	if err := conn.execRaw(`PRAGMA foreign_keys=ON`); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.execRaw(`PRAGMA journal_mode=WAL`); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

type Conn struct {
	mu     sync.Mutex
	handle *C.sqlite3
	closed bool
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, driver.ErrBadConn
	}
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var statement *C.sqlite3_stmt
	if code := C.sqlite3_prepare_v2(c.handle, cQuery, -1, &statement, nil); code != sqliteOK {
		return nil, c.error(code)
	}
	return &Stmt{conn: c, handle: statement, inputs: int(C.sqlite3_bind_parameter_count(statement))}, nil
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	if code := C.sqlite3_close_v2(c.handle); code != sqliteOK {
		return c.error(code)
	}
	c.closed = true
	c.handle = nil
	return nil
}

func (c *Conn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }

func (c *Conn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	if options.ReadOnly {
		return nil, errors.New("SQLite 适配器不支持只读事务选项")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.execRaw(`BEGIN IMMEDIATE`); err != nil {
		return nil, err
	}
	return &Tx{conn: c}, nil
}

func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	statement, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer statement.Close()
	return statement.(driver.StmtExecContext).ExecContext(ctx, args)
}

func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	statement, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	rows, err := statement.(driver.StmtQueryContext).QueryContext(ctx, args)
	if err != nil {
		statement.Close()
		return nil, err
	}
	rows.(*Rows).closeStatement = true
	return rows, nil
}

func (c *Conn) CheckNamedValue(value *driver.NamedValue) error {
	switch value.Value.(type) {
	case nil, int64, float64, bool, []byte, string, time.Time:
		return nil
	case int:
		value.Value = int64(value.Value.(int))
		return nil
	default:
		return driver.ErrSkip
	}
}

func (c *Conn) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	statement, err := c.PrepareContext(ctx, `SELECT 1`)
	if err != nil {
		return err
	}
	defer statement.Close()
	rows, err := statement.(driver.StmtQueryContext).QueryContext(ctx, nil)
	if err != nil {
		return err
	}
	return rows.Close()
}

func (c *Conn) execRaw(query string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return driver.ErrBadConn
	}
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	var errorMessage *C.char
	code := C.sqlite3_exec(c.handle, cQuery, nil, nil, &errorMessage)
	if code != sqliteOK {
		message := C.GoString(errorMessage)
		if errorMessage != nil {
			C.sqlite3_free(unsafe.Pointer(errorMessage))
		}
		return fmt.Errorf("sqlite code %d: %s", code, message)
	}
	return nil
}

func (c *Conn) error(code C.int) error {
	return fmt.Errorf("sqlite code %d: %s", int(code), C.GoString(C.sqlite3_errmsg(c.handle)))
}

var _ driver.Driver = (*Driver)(nil)
var _ driver.Conn = (*Conn)(nil)
var _ driver.ConnPrepareContext = (*Conn)(nil)
var _ driver.ConnBeginTx = (*Conn)(nil)
var _ driver.ExecerContext = (*Conn)(nil)
var _ driver.QueryerContext = (*Conn)(nil)
var _ driver.NamedValueChecker = (*Conn)(nil)
var _ driver.Pinger = (*Conn)(nil)
