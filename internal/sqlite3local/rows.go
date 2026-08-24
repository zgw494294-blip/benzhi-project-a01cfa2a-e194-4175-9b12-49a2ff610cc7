package sqlite3local

/*
#include <stdlib.h>
typedef struct sqlite3_stmt sqlite3_stmt;
int sqlite3_reset(sqlite3_stmt*);
int sqlite3_step(sqlite3_stmt*);
int sqlite3_column_count(sqlite3_stmt*);
const char *sqlite3_column_name(sqlite3_stmt*, int);
int sqlite3_column_type(sqlite3_stmt*, int);
long long sqlite3_column_int64(sqlite3_stmt*, int);
double sqlite3_column_double(sqlite3_stmt*, int);
const unsigned char *sqlite3_column_text(sqlite3_stmt*, int);
const void *sqlite3_column_blob(sqlite3_stmt*, int);
int sqlite3_column_bytes(sqlite3_stmt*, int);
*/
import "C"

import (
	"database/sql/driver"
	"io"
	"unsafe"
)

type Rows struct {
	statement      *Stmt
	done           bool
	closeStatement bool
}

func (r *Rows) Columns() []string {
	r.statement.conn.mu.Lock()
	defer r.statement.conn.mu.Unlock()
	count := int(C.sqlite3_column_count(r.statement.handle))
	columns := make([]string, count)
	for index := 0; index < count; index++ {
		columns[index] = C.GoString(C.sqlite3_column_name(r.statement.handle, C.int(index)))
	}
	return columns
}

func (r *Rows) Close() error {
	if r.done {
		if r.closeStatement {
			return r.statement.Close()
		}
		return nil
	}
	r.statement.conn.mu.Lock()
	C.sqlite3_reset(r.statement.handle)
	r.statement.conn.mu.Unlock()
	r.done = true
	if r.closeStatement {
		return r.statement.Close()
	}
	return nil
}

func (r *Rows) Next(destination []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.statement.conn.mu.Lock()
	defer r.statement.conn.mu.Unlock()
	code := C.sqlite3_step(r.statement.handle)
	if code == sqliteDone {
		r.done = true
		return io.EOF
	}
	if code != sqliteRow {
		return r.statement.conn.error(code)
	}
	for index := range destination {
		switch C.sqlite3_column_type(r.statement.handle, C.int(index)) {
		case sqliteInteger:
			destination[index] = int64(C.sqlite3_column_int64(r.statement.handle, C.int(index)))
		case sqliteFloat:
			destination[index] = float64(C.sqlite3_column_double(r.statement.handle, C.int(index)))
		case sqliteText:
			pointer := C.sqlite3_column_text(r.statement.handle, C.int(index))
			size := C.sqlite3_column_bytes(r.statement.handle, C.int(index))
			destination[index] = C.GoStringN((*C.char)(unsafe.Pointer(pointer)), size)
		case sqliteBlob:
			pointer := C.sqlite3_column_blob(r.statement.handle, C.int(index))
			size := C.sqlite3_column_bytes(r.statement.handle, C.int(index))
			destination[index] = C.GoBytes(pointer, size)
		case sqliteNull:
			destination[index] = nil
		}
	}
	return nil
}

var _ driver.Rows = (*Rows)(nil)
