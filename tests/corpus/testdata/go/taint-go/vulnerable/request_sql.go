package web

import (
	"database/sql"
	"fmt"
	"net/http"
)

func userName(r *http.Request) string {
	return r.FormValue("name")
}

func handler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	name := userName(r)
	query := fmt.Sprintf("SELECT * FROM users WHERE name = '%s'", name)
	rows, err := db.Query(query)
	_ = rows
	_ = err
}
