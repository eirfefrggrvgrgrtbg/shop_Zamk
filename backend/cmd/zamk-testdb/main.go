// Command zamk-testdb creates and drops the throw-away PostgreSQL database of one backend test run
// (used only by backend/scripts/safe_go_test.sh).
//
//	zamk-testdb create            prints KEY=VALUE lines: NAME, DSN, TOKEN, HOST, PORT, USER, PASSWORD
//	zamk-testdb drop <name> <token>   drops that database, only if it carries this ownership token
//
// It never touches zamk, zamk_test or zamk_demo and never drops by pattern.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/eirfefrggrvgrgrtbg/shop-zamk/backend/internal/testutil"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	switch {
	case len(os.Args) == 2 && os.Args[1] == "create":
		db, err := testutil.CreateRunnerDatabase(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "REFUSED: cannot create an isolated test database:", err)
			os.Exit(1)
		}
		fmt.Printf("NAME=%s\nDSN=%s\nTOKEN=%s\nHOST=%s\nPORT=%s\nUSER=%s\nPASSWORD=%s\n", db.Name, db.DSN, db.Token, db.Host, db.Port, db.User, db.Password)
	case len(os.Args) == 4 && os.Args[1] == "drop":
		if err := testutil.DropRunnerDatabase(ctx, os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: zamk-testdb create | drop <name> <token>")
		os.Exit(2)
	}
}
