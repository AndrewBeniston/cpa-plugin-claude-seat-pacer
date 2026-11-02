package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPageCostText runs the page's cost labels under node: a cost in days
// prints as the wait to its reset, and a plan cost, with or without the
// unit field, keeps its signed figure whatever order the plugin runs.
func TestPageCostText(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	// The one-line helpers have no closing brace on a line of their own, so
	// each is read as its line.
	var oneLiners strings.Builder
	for _, name := range []string{"num", "scored", "sessionOnly", "counted", "daysCost"} {
		line := regexp.MustCompile(`\n  function ` + name + `\(.*\}\n`).Find(indexHTML)
		if line == nil {
			t.Fatalf("page has no one-line function %s", name)
		}
		oneLiners.Write(line)
	}
	src := oneLiners.String() + pageFunctions(t, "signed", "durText", "costTenths", "soonestFormulaText", "costText") + `
var fails = [];
function eq(name, got, want) {
  if (got !== want) fails.push(name + ": got " + JSON.stringify(got) + ", want " + JSON.stringify(want));
}
var planScore = { cost: 0.124, windows: [{ kind: "seven_day", weight: 1, slack: -0.124 }] };
var legacyScore = { cost: -0.05, windows: [{ kind: "seven_day", weight: 1, slack: 0.05 }] };
var dayScore = { cost: 4.593, cost_unit: "days", windows: [{ kind: "seven_day", weight: 1, slack: -0.1 }] };
var pastScore = { cost: 0, cost_unit: "days", windows: [{ kind: "seven_day", weight: 1, slack: 0 }] };

eq("days", costText(dayScore), "resets in 4d 14h");
eq("reset now", costText(pastScore), "resets in 0s");
eq("plan", costText(planScore), "+12.4");
eq("legacy plan, no unit field", costText(legacyScore), "−5.0");
eq("unscored", costText({ cost: 1, windows: [] }), "—");
eq("formula", soonestFormulaText(true).indexOf("no weekly cap first") > 0, true);
eq("formula, none eligible", soonestFormulaText(false), ": no seat eligible");

if (fails.length) { console.log(fails.join("\n")); process.exit(1); }
`
	file := filepath.Join(t.TempDir(), "cost.js")
	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(node, file).CombinedOutput(); err != nil {
		t.Errorf("page cost text: %v\n%s", err, out)
	}
}
