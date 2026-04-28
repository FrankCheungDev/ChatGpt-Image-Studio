package sqliteutil

import (
	"database/sql"
	"net/url"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

const BusyTimeoutMS = 10000

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", DSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

func DSN(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}

	values := url.Values{}
	values.Add("_pragma", "busy_timeout="+strconv.Itoa(BusyTimeoutMS))
	values.Add("_pragma", "journal_mode(WAL)")
	values.Add("_pragma", "synchronous(NORMAL)")
	values.Add("_pragma", "foreign_keys(ON)")

	separator := "?"
	if strings.Contains(trimmed, "?") {
		separator = "&"
	}
	return trimmed + separator + values.Encode()
}
