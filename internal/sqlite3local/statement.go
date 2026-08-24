package sqlite3local

/*
#include <stdlib.h>
typedef struct sqlite3 sqlite3;
typedef struct sqlite3_stmt sqlite3_stmt;
typedef long long sqlite3_int64;
typedef void (*sqlite3_destructor_type)(void*);
int sqlite3_finalize(sqlite3_stmt*);
int sqlite3_reset(sqlite3_stmt*);
int sqlite3_clear_bindings(sqlite3_stmt*);
int sqlite3_bind_null(sqlite3_stmt*, int);
int sqlite3_bind_int64(sqlite3_stmt*, int, sqlite3_int64);
int sqlite3_bind_double(sqlite3_stmt*, int, double);
int sqlite3_bind_text(sqlite3_stmt*, int, const char*, int, sqlite3_destructor_type);
int sqlite3_bind_blob(sqlite3_stmt*, int, const void*, int, sqlite3_destructor_type);
int sqlite3_step(sqlite3_stmt*);
int sqlite3_changes(sqlite3*);
sqlite3_int64 sqlite3_last_insert_rowid(sqlite3*);
static sqlite3_destructor_type stmt_transient_destructor() { return (sqlite3_destructor_type)-1; }
*/
import "C"

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"
	"unsafe"
)

type Stmt struct {
	conn   *Conn
	handle *C.sqlite3_stmt
	inputs int
	closed bool
}

func (s *Stmt) Close() error {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	if s.closed {
		return nil
	}
	if code := C.sqlite3_finalize(s.handle); code != sqliteOK {
		return s.conn.error(code)
	}
	s.closed = true
	s.handle = nil
	return nil
}

func (s *Stmt) NumInput() int { return s.inputs }

func (s *Stmt) Exec(values []driver.Value) (driver.Result, error) {
	args := make([]driver.NamedValue, len(values))
	for index, value := range values {
		args[index] = driver.NamedValue{Ordinal: index + 1, Value: value}
	}
	return s.ExecContext(context.Background(), args)
}

func (s *Stmt) Query(values []driver.Value) (driver.Rows, error) {
	args := make([]driver.NamedValue, len(values))
	for index, value := range values {
		args[index] = driver.NamedValue{Ordinal: index + 1, Value: value}
	}
	return s.QueryContext(context.Background(), args)
}

func (s *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	if err := s.resetAndBind(args); err != nil {
		return nil, err
	}
	code := C.sqlite3_step(s.handle)
	if code != sqliteDone {
		return nil, s.conn.error(code)
	}
	return result{lastID: int64(C.sqlite3_last_insert_rowid(s.conn.handle)), changed: int64(C.sqlite3_changes(s.conn.handle))}, nil
}

func (s *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	if err := s.resetAndBind(args); err != nil {
		return nil, err
	}
	return &Rows{statement: s}, nil
}

func (s *Stmt) resetAndBind(args []driver.NamedValue) error {
	if s.closed {
		return driver.ErrBadConn
	}
	C.sqlite3_reset(s.handle)
	C.sqlite3_clear_bindings(s.handle)
	if len(args) != s.inputs {
		return fmt.Errorf("SQLite 参数数量错误: 需要 %d，收到 %d", s.inputs, len(args))
	}
	for index, arg := range args {
		if err := s.bind(index+1, arg.Value); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stmt) bind(index int, value any) error {
	var code C.int
	switch typed := value.(type) {
	case nil:
		code = C.sqlite3_bind_null(s.handle, C.int(index))
	case int64:
		code = C.sqlite3_bind_int64(s.handle, C.int(index), C.sqlite3_int64(typed))
	case int:
		code = C.sqlite3_bind_int64(s.handle, C.int(index), C.sqlite3_int64(typed))
	case bool:
		if typed {
			code = C.sqlite3_bind_int64(s.handle, C.int(index), 1)
		} else {
			code = C.sqlite3_bind_int64(s.handle, C.int(index), 0)
		}
	case float64:
		code = C.sqlite3_bind_double(s.handle, C.int(index), C.double(typed))
	case string:
		text := C.CString(typed)
		defer C.free(unsafe.Pointer(text))
		code = C.sqlite3_bind_text(s.handle, C.int(index), text, C.int(len(typed)), C.stmt_transient_destructor())
	case []byte:
		if len(typed) == 0 {
			code = C.sqlite3_bind_blob(s.handle, C.int(index), nil, 0, C.stmt_transient_destructor())
		} else {
			code = C.sqlite3_bind_blob(s.handle, C.int(index), unsafe.Pointer(&typed[0]), C.int(len(typed)), C.stmt_transient_destructor())
		}
	case time.Time:
		textValue := typed.Format(time.RFC3339Nano)
		text := C.CString(textValue)
		defer C.free(unsafe.Pointer(text))
		code = C.sqlite3_bind_text(s.handle, C.int(index), text, C.int(len(textValue)), C.stmt_transient_destructor())
	default:
		return fmt.Errorf("不支持的 SQLite 参数类型 %T", value)
	}
	if code != sqliteOK {
		return s.conn.error(code)
	}
	return nil
}

type result struct {
	lastID  int64
	changed int64
}

func (r result) LastInsertId() (int64, error) { return r.lastID, nil }
func (r result) RowsAffected() (int64, error) { return r.changed, nil }

var _ driver.Stmt = (*Stmt)(nil)
var _ driver.StmtExecContext = (*Stmt)(nil)
var _ driver.StmtQueryContext = (*Stmt)(nil)
