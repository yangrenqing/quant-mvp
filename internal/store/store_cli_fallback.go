//go:build !cgo

package store

import (
	"os/exec"
	"strings"
)

func Exec(path string, query string, args ...any) error {
	cmd := exec.Command("sqlite3", "-bail", path, query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return wrapError(err, strings.TrimSpace(string(output)))
	}
	return nil
}

func QueryString(path string, query string, args ...any) (string, error) {
	cmd := exec.Command("sqlite3", "-bail", "-noheader", path, query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", wrapError(err, strings.TrimSpace(string(output)))
	}
	return strings.TrimRight(string(output), "\n"), nil
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
	return Exec(path, script)
}
