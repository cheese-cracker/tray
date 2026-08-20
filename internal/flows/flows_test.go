// Package flows holds nothing. Its only job is to fail the build when FLOWS.md and
// the tests that back it have drifted apart — a promise with no test, or a flow test
// no promise names. Either way the document has stopped being true, which is worse
// than not having written it.
package flows

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	flowsDoc  = "../../FLOWS.md"
	shellFile = "../../scripts/check-tray.sh"
	uiFlows   = "../ui/flows_test.go"
)

// A row is `| F1 | what must keep working | ` + "`held by`" + ` |`.
var (
	rowRe   = regexp.MustCompile("(?m)^\\|\\s*([FT]\\d+)\\s*\\|.*\\|\\s*`([^`]+)`\\s*\\|\\s*$")
	headRe  = regexp.MustCompile(`(?m)^head_ "([^"]+)"`)
	testPre = "TestFlow"
)

// documented returns the flow ids and the test each one names.
func documented(t *testing.T) map[string]string {
	t.Helper()
	body, err := os.ReadFile(flowsDoc)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, m := range rowRe.FindAllStringSubmatch(string(body), -1) {
		if was, dup := rows[m[1]]; dup {
			t.Errorf("FLOWS.md lists %s twice: %q and %q", m[1], was, m[2])
		}
		rows[m[1]] = m[2]
	}
	if len(rows) == 0 {
		t.Fatalf("no flow rows parsed out of %s — the table format changed", flowsDoc)
	}
	return rows
}

// shellFlows are the `head_ "F1 · capture"` blocks in the bash suite.
func shellFlows(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(shellFile)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range headRe.FindAllStringSubmatch(string(body), -1) {
		out = append(out, m[1])
	}
	return out
}

// goFlows are the TestFlow… functions in the teatest suite. Parsed rather than
// grepped, so a name inside a comment or a string can't satisfy a row.
func goFlows(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), uiFlows, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, testPre) {
			out = append(out, fn.Name.Name)
		}
	}
	return out
}

func TestEveryFlowRowNamesARealTest(t *testing.T) {
	rows := documented(t)
	real := map[string]bool{}
	for _, name := range append(shellFlows(t), goFlows(t)...) {
		real[name] = true
	}
	for _, id := range sorted(rows) {
		if !real[rows[id]] {
			t.Errorf("FLOWS.md %s names %q, which no test provides", id, rows[id])
		}
	}
}

func TestEveryFlowTestHasARow(t *testing.T) {
	named := map[string]string{}
	for id, held := range documented(t) {
		named[held] = id
	}
	for _, name := range shellFlows(t) {
		if named[name] == "" {
			t.Errorf("%s runs %q, which FLOWS.md does not promise", shellFile, name)
		}
	}
	for _, name := range goFlows(t) {
		if named[name] == "" {
			t.Errorf("%s defines %s, which FLOWS.md does not promise", uiFlows, name)
		}
	}
}

// F rows must be held by the shell suite and T rows by the Go suite, or the table's
// two halves stop meaning anything.
func TestFlowIdsMatchTheirSuite(t *testing.T) {
	shell := map[string]bool{}
	for _, name := range shellFlows(t) {
		shell[name] = true
	}
	rows := documented(t)
	for _, id := range sorted(rows) {
		switch {
		case strings.HasPrefix(id, "F") && !shell[rows[id]]:
			t.Errorf("%s is an F row but %q is not a check-tray.sh flow", id, rows[id])
		case strings.HasPrefix(id, "T") && !strings.HasPrefix(rows[id], testPre):
			t.Errorf("%s is a T row but %q is not a %s… function", id, rows[id], testPre)
		}
	}
}

func sorted(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
