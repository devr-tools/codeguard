package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func main() {
	db, _ := sql.Open("postgres", "dsn")
	userID := os.Getenv("USER_ID")
	rows, _ := db.Query("SELECT * FROM users WHERE id = $1", userID)
	_ = rows
	count, _ := strconv.Atoi(os.Getenv("COUNT"))
	_ = exec.Command("echo", fmt.Sprintf("%d", count))
	static := "uptime"
	_ = exec.Command("sh", "-c", static)
}
