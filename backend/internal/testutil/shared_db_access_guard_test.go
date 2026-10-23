package testutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TEST.DB.5 static guard: no Go file outside internal/testutil may find the shared zamk_test by itself. A test gets its
// database only from testutil (GetTestDatabaseURL / RunnerDatabaseURL / DisposableDatabaseURL / NewDisposableDatabase),
// which hand out disposable databases and nothing else. A new test that hard-codes zamk_test or reads TEST_DATABASE_URL
// directly fails here instead of writing to a shared database.
var (
	sharedDSNLiteralRE = regexp.MustCompile("postgres(?:ql)?://[^\"`\\s]*zamk_test")
	canonicalDSNRE     = regexp.MustCompile(`\bCanonicalTestDatabaseDSN\b`)
	rawTestEnvRE       = regexp.MustCompile(`(?:Getenv|LookupEnv)\(\s*"TEST_DATABASE_URL"\s*\)`)
)

// sharedAccessViolations returns what in src reaches the shared test database without going through testutil.
func sharedAccessViolations(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") {
			continue
		}
		switch {
		case sharedDSNLiteralRE.MatchString(line):
			out = append(out, "hard-coded zamk_test DSN: "+t)
		case canonicalDSNRE.MatchString(line):
			out = append(out, "CanonicalTestDatabaseDSN: "+t)
		case rawTestEnvRE.MatchString(line):
			out = append(out, "raw TEST_DATABASE_URL read (use testutil.RunnerDatabaseURL / GetTestDatabaseURL): "+t)
		}
	}
	return out
}

func TestNoSharedTestDatabaseAccessOutsideTestutil(t *testing.T) {
	root := findRepoRoot()
	if root == "" {
		t.Fatal("cannot find the backend module root")
	}
	var offenders []string
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, top), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if strings.HasPrefix(filepath.ToSlash(rel), "internal/testutil/") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, v := range sharedAccessViolations(string(b)) {
				offenders = append(offenders, rel+": "+v)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("code that can reach the shared zamk_test outside testutil:\n  %s", strings.Join(offenders, "\n  "))
	}
}

func TestSharedAccessDetector(t *testing.T) {
	for _, bad := range []string{
		`dsn := "postgres://zamk:zamk_password@localhost:5433/zamk_test?sslmode=disable"`,
		"const c = `postgresql://u:p@127.0.0.1:5433/zamk_test`",
		`dsn = testutil.CanonicalTestDatabaseDSN`,
		`dsn := os.Getenv("TEST_DATABASE_URL")`,
		`v, _ := os.LookupEnv( "TEST_DATABASE_URL" )`,
	} {
		if len(sharedAccessViolations(bad)) == 0 {
			t.Errorf("not detected: %s", bad)
		}
	}
	for _, ok := range []string{
		`dsn := testutil.GetTestDatabaseURL()`,
		`dsn := testutil.RunnerDatabaseURL()`,
		`dsn = testutil.BlockedTestDatabaseDSN`,
		`// the shared database used to be postgres://u:p@localhost:5433/zamk_test`,
		`DatabaseName: "zamk_test",`,
	} {
		if v := sharedAccessViolations(ok); len(v) != 0 {
			t.Errorf("false positive on %q: %v", ok, v)
		}
	}
}
