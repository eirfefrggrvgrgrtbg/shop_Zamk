package testutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TEST.DB.4 static guard: a backend test file may execute table-wide destructive SQL (TRUNCATE, DROP TABLE/DATABASE/
// SCHEMA, DROP COLUMN, DELETE FROM <table> without WHERE) only if it runs on a disposable database and checks it right before
// (testutil.MustBeDisposableDatabase). A new dangerous test therefore fails here instead of wiping a shared database.
var (
	execLineRE    = regexp.MustCompile(`\.(Exec|Query|QueryRow)\(`)
	truncateRE    = regexp.MustCompile(`["\x60]\s*TRUNCATE\b|\bTRUNCATE\s+(TABLE\s+)?[a-z_]+[ ,;"\x60]`)
	dropRE        = regexp.MustCompile(`["\x60]\s*DROP\s+(TABLE|DATABASE|SCHEMA)\b|\bDROP\s+COLUMN\b`)
	deleteAllRE   = regexp.MustCompile(`DELETE\s+FROM\s+[a-z_."]+\s*(["\x60;]|$)`)
	nextLineEndRE = regexp.MustCompile(`^\s*["\x60]`)
)

// destructiveLines returns the lines of src that execute table-wide destructive SQL.
func destructiveLines(src string) []string {
	lines := strings.Split(src, "\n")
	var out []string
	inExec := 0 // lines left in a multi-line Exec call
	for i, line := range lines {
		if execLineRE.MatchString(line) {
			inExec = 6
		}
		if inExec == 0 {
			continue
		}
		inExec--
		hit := truncateRE.MatchString(line) || dropRE.MatchString(line)
		if m := deleteAllRE.FindString(line); m != "" && !strings.Contains(strings.ToUpper(line), "WHERE") {
			// "DELETE FROM t" closing the literal on this line, or the literal closing on the next line
			if strings.ContainsAny(m[len(m)-1:], "\"`;") || (i+1 < len(lines) && nextLineEndRE.MatchString(lines[i+1])) {
				hit = true
			}
		}
		if hit {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func TestDestructiveSQLOnlyOnDisposableDatabases(t *testing.T) {
	root := findRepoRoot()
	if root == "" {
		t.Fatal("cannot find the backend module root")
	}
	var offenders []string
	err := filepath.Walk(filepath.Join(root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "destructive_sql_guard_test.go" {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(b)
		hits := destructiveLines(src)
		if len(hits) == 0 {
			return nil
		}
		guarded := strings.Contains(src, "MustBeDisposableDatabase(") &&
			(strings.Contains(src, "DisposableDatabaseURL(") || strings.Contains(src, "NewDisposableDatabase(") || strings.Contains(src, "UseDisposableDatabaseForPackage("))
		if !guarded {
			rel, _ := filepath.Rel(root, path)
			offenders = append(offenders, rel+": "+hits[0])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("table-wide destructive SQL outside a disposable database (use testutil.DisposableDatabaseURL / NewDisposableDatabase / UseDisposableDatabaseForPackage and testutil.MustBeDisposableDatabase, or scope the statement with WHERE):\n  %s", strings.Join(offenders, "\n  "))
	}
}

func TestDestructiveLinesDetector(t *testing.T) {
	cases := map[string]bool{
		"db.Exec(ctx, \"TRUNCATE TABLE users CASCADE\")":                                  true,
		"pool.Exec(ctx, `TRUNCATE users, sellers CASCADE`)":                               true,
		"db.Exec(ctx, \"DELETE FROM orders\")":                                            true,
		"db.Exec(ctx, \"DELETE FROM refunds WHERE order_id = $1\", id)":                   false,
		"x.Exec(ctx, \"DROP TABLE users\")":                                               true,
		"\t\tALTER TABLE promo_codes DROP COLUMN audience_type;":                          false, // outside an Exec call
		"now := time.Now().Truncate(time.Second)":                                         false,
		"assert.NotContains(t, rawLog, \"DROP TABLE\")":                                   false,
		"svc.Query(ctx, Request{Dimensions: []string{\"source; DROP TABLE users; --\"}})": false,
	}
	for line, want := range cases {
		if got := len(destructiveLines(line)) > 0; got != want {
			t.Errorf("%q: detected=%v, want %v", line, got, want)
		}
	}
	multi := "pool.Exec(ctx, `\n\t\tDELETE FROM staff_member_permissions\n\t\t`)"
	if len(destructiveLines(multi)) != 1 {
		t.Errorf("multi-line DELETE without WHERE not detected")
	}
	scoped := "pool.Exec(ctx, `\n\t\tDELETE FROM staff_member_permissions\n\t\tWHERE user_id = $1\n\t`, id)"
	if len(destructiveLines(scoped)) != 0 {
		t.Errorf("scoped multi-line DELETE wrongly detected")
	}
}
