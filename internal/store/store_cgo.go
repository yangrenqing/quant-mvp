//go:build cgo

package store

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

func Exec(path string, query string, args ...any) error {
	return execScript(path, query)
}

func QueryString(path string, query string, args ...any) (string, error) {
	db, err := openDB(path)
	if err != nil {
		return "", err
	}
	defer closeDB(db)

	stmt, err := prepareStatement(db, query)
	if err != nil {
		return "", err
	}
	defer C.sqlite3_finalize(stmt)

	var lines []string
	for {
		rc := C.sqlite3_step(stmt)
		switch rc {
		case C.SQLITE_ROW:
			columnCount := int(C.sqlite3_column_count(stmt))
			columns := make([]string, 0, columnCount)
			for idx := 0; idx < columnCount; idx++ {
				switch C.sqlite3_column_type(stmt, C.int(idx)) {
				case C.SQLITE_NULL:
					columns = append(columns, "")
				default:
					value := C.sqlite3_column_text(stmt, C.int(idx))
					columns = append(columns, cString(value))
				}
			}
			lines = append(lines, strings.Join(columns, "|"))
		case C.SQLITE_DONE:
			return strings.Join(lines, "\n"), nil
		default:
			return "", sqliteError(db, rc)
		}
	}
}

func ExecTx(path string, statements ...string) error {
	filtered := make([]string, 0, len(statements))
	for _, statement := range statements {
		if strings.TrimSpace(statement) != "" {
			filtered = append(filtered, statement)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	script := "BEGIN IMMEDIATE;\n" + strings.Join(filtered, "\n") + "\nCOMMIT;"
	return execScript(path, script)
}

func execScript(path string, script string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer closeDB(db)

	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	var errMsg *C.char
	rc := C.sqlite3_exec(db, cScript, nil, nil, &errMsg)
	if rc != C.SQLITE_OK {
		defer C.sqlite3_free(unsafe.Pointer(errMsg))
		if errMsg != nil {
			return wrapError(errors.New("sqlite exec failed"), C.GoString(errMsg))
		}
		return sqliteError(db, rc)
	}
	return nil
}

func openDB(path string) (*C.sqlite3, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var db *C.sqlite3
	rc := C.sqlite3_open(cPath, &db)
	if rc != C.SQLITE_OK {
		if db != nil {
			defer C.sqlite3_close(db)
		}
		return nil, sqliteError(db, rc)
	}
	if err := execPragma(db, "PRAGMA busy_timeout = 5000;"); err != nil {
		C.sqlite3_close(db)
		return nil, err
	}
	return db, nil
}

func closeDB(db *C.sqlite3) {
	if db != nil {
		C.sqlite3_close(db)
	}
}

func execPragma(db *C.sqlite3, script string) error {
	cScript := C.CString(script)
	defer C.free(unsafe.Pointer(cScript))

	var errMsg *C.char
	rc := C.sqlite3_exec(db, cScript, nil, nil, &errMsg)
	if rc != C.SQLITE_OK {
		defer C.sqlite3_free(unsafe.Pointer(errMsg))
		if errMsg != nil {
			return wrapError(errors.New("sqlite pragma failed"), C.GoString(errMsg))
		}
		return sqliteError(db, rc)
	}
	return nil
}

func prepareStatement(db *C.sqlite3, query string) (*C.sqlite3_stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var stmt *C.sqlite3_stmt
	rc := C.sqlite3_prepare_v2(db, cQuery, -1, &stmt, nil)
	if rc != C.SQLITE_OK {
		return nil, sqliteError(db, rc)
	}
	return stmt, nil
}

func sqliteError(db *C.sqlite3, code C.int) error {
	message := "unknown sqlite error"
	if db != nil {
		message = C.GoString(C.sqlite3_errmsg(db))
	}
	return fmt.Errorf("sqlite error (%d): %s", int(code), message)
}

func cString(value *C.uchar) string {
	if value == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(value)))
}
