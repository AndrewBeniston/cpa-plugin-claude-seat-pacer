package runtime

import (
	"testing"
	"time"

	"github.com/yuya-iwabuchi/cpa-plugin-claude-seat-pacer/internal/model"
)

// The order travels from the YAML block to the pick: on a pool where the plan
// favours the seat with its week ahead, soonest-reset takes the seat that
// resets first, and the decision log carries the cost in days.
func TestColdPickUnderSoonestReset(t *testing.T) {
	pool := func(tp *testPlugin) {
		tp.quota.Put(seat("seat-late", testNow,
			model.Window{Kind: model.WindowWeekly, Utilization: 0, ResetsAt: testNow.Add(6 * 24 * time.Hour), Duration: model.WeeklyDuration}))
		tp.quota.Put(seat("seat-soon", testNow,
			model.Window{Kind: model.WindowWeekly, Utilization: 0.95, ResetsAt: testNow.Add(24 * time.Hour), Duration: model.WeeklyDuration}))
	}

	plan := newTestPlugin(t, testConfigYAML)
	pool(plan)
	if resp := plan.pick(t, pickRequest("claude-opus-5", "k", "seat-late", "seat-soon")); resp.AuthID != "seat-late" {
		t.Fatalf("plan response = %+v, want seat-late, furthest behind its plan", resp)
	}

	soonest := newTestPlugin(t, testConfigYAML+"pace:\n  order: soonest-reset\n")
	pool(soonest)
	if resp := soonest.pick(t, pickRequest("claude-opus-5", "k", "seat-late", "seat-soon")); resp.AuthID != "seat-soon" {
		t.Fatalf("soonest-reset response = %+v, want seat-soon, which resets first", resp)
	}
	d := soonest.lastDecision(t)
	if d.Kind != model.DecisionColdPick {
		t.Fatalf("decision = %+v, want a cold pick", d)
	}
	for _, s := range d.Scores {
		if s.CostUnit != model.CostUnitDays {
			t.Errorf("logged %s cost unit = %q, want days", s.AuthID, s.CostUnit)
		}
	}
}
