package pace

import (
	"math"
	"testing"
	"time"

	"github.com/yuya-iwabuchi/cpa-plugin-claude-seat-pacer/internal/model"
)

func soonestCfg() model.PaceConfig {
	cfg := model.Defaults().Pace
	cfg.Order = model.OrderSoonestReset
	return cfg
}

// The readings from a live pool on 2 Nov 2026 at 17:27 UTC. seat-a had just
// reset; seat-b was ahead of its plan with half its week left to spend.
func liveResetPool(t *testing.T, now time.Time) map[string]model.AuthSnapshot {
	return snapshots(
		seat("seat-a", now,
			window(model.WindowSession, "", 0.78, at(t, "2026-11-02T18:00:00Z")),
			window(model.WindowWeekly, "", 0.01, at(t, "2026-11-09T17:00:00Z")),
			window(model.WindowWeeklyScoped, model.FamilyFable, 0, at(t, "2026-11-09T17:00:00Z")),
		),
		seat("seat-b", now,
			window(model.WindowWeekly, "", 0.50, at(t, "2026-11-07T08:00:00Z")),
			window(model.WindowWeeklyScoped, model.FamilyFable, 0.08, at(t, "2026-11-07T08:00:00Z")),
		),
	)
}

func TestSoonestResetPicksTheSeatThatResetsFirst(t *testing.T) {
	now := at(t, "2026-11-02T17:27:00Z")
	pool := liveResetPool(t, now)
	ids := []string{"seat-a", "seat-b"}

	if got := rankedIDs(Rank(model.Defaults().Pace, pool, ids, opus, now))[0]; got != "seat-a" {
		t.Fatalf("plan order picked %q, want seat-a: seat-b runs ahead of its plan", got)
	}
	ranked := Rank(soonestCfg(), pool, ids, opus, now)
	if got := rankedIDs(ranked)[0]; got != "seat-b" {
		t.Fatalf("soonest-reset picked %q, want seat-b, which resets on 7 Nov", got)
	}
	want := at(t, "2026-11-07T08:00:00Z").Sub(now).Hours() / 24
	if got := scoreOf(t, ranked, "seat-b").Cost; math.Abs(got-want) > tolerance {
		t.Errorf("seat-b cost = %v, want %v days to its reset", got, want)
	}
}

// A heavily used seat near its reset still wins over a lightly used seat with
// days left, which is where a bent plan curve stops agreeing with the rule.
func TestSoonestResetIgnoresHowMuchEachSeatHasUsed(t *testing.T) {
	now := testNow
	pool := snapshots(
		seat("late", now, weekly(now, 6.0/7, 0.80)),
		seat("early", now, weekly(now, 2.0/7, 0.10)),
	)
	if got := rankedIDs(Rank(soonestCfg(), pool, []string{"early", "late"}, opus, now))[0]; got != "late" {
		t.Fatalf("picked %q, want late, whose window resets first", got)
	}
}

// A full seat is gated as under the plan order, so the next reset takes over
// and the full seat waits for its own.
func TestSoonestResetMovesOnWhenTheFirstSeatIsFull(t *testing.T) {
	now := testNow
	pool := snapshots(
		seat("full", now, weekly(now, 6.0/7, 1.0)),
		seat("next", now, weekly(now, 3.0/7, 0.40)),
		seat("last", now, weekly(now, 1.0/7, 0.0)),
	)
	ranked := Rank(soonestCfg(), pool, []string{"full", "next", "last"}, opus, now)
	if got := rankedIDs(ranked); got[0] != "next" || got[1] != "last" || got[2] != "full" {
		t.Fatalf("ranked %v, want [next last full]", got)
	}
	if s := scoreOf(t, ranked, "full"); s.Eligible || s.Reason != model.ReasonSpent {
		t.Errorf("full eligible=%v reason=%q, want spent", s.Eligible, s.Reason)
	}
}

// A Fable request weighs the Fable cap, so a cap that resets before the
// all-models window sets the seat's place in the order.
func TestSoonestResetReadsTheFamilyCapForItsFamily(t *testing.T) {
	now := testNow
	pool := snapshots(
		seat("cap-soon", now,
			weekly(now, 1.0/7, 0.10),
			paced(model.WindowWeeklyScoped, model.FamilyFable, now, 6.0/7, 0.20)),
		seat("other", now, weekly(now, 4.0/7, 0.30)),
	)
	ids := []string{"cap-soon", "other"}
	if got := rankedIDs(Rank(soonestCfg(), pool, ids, fable, now))[0]; got != "cap-soon" {
		t.Errorf("fable request picked %q, want cap-soon, whose Fable cap resets first", got)
	}
	if got := rankedIDs(Rank(soonestCfg(), pool, ids, opus, now))[0]; got != "other" {
		t.Errorf("opus request picked %q, want other: the Fable cap does not bear on Opus", got)
	}
}

// A seat with no weekly cap keeps its plan cost on its 5-hour window and
// still ranks first, as under the plan order.
func TestSoonestResetLeavesSessionOnlySeatsFirst(t *testing.T) {
	now := testNow
	pool := snapshots(
		seat("session", now, paced(model.WindowSession, "", now, 0.5, 0.2)),
		seat("capped", now, weekly(now, 6.0/7, 0.1)),
	)
	ranked := Rank(soonestCfg(), pool, []string{"capped", "session"}, opus, now)
	if got := rankedIDs(ranked)[0]; got != "session" {
		t.Fatalf("picked %q, want the session-only seat", got)
	}
	plan := Rank(model.Defaults().Pace, pool, []string{"session"}, opus, now)[0].Cost
	if got := scoreOf(t, ranked, "session").Cost; got != plan {
		t.Errorf("session-only cost = %v, want its plan cost %v", got, plan)
	}
}

func TestSoonestResetCountsAPastResetAsNow(t *testing.T) {
	now := testNow
	pool := snapshots(seat("past", now, window(model.WindowWeekly, "", 0.5, now.Add(-time.Minute))))
	if got := Rank(soonestCfg(), pool, []string{"past"}, opus, now)[0].Cost; got != 0 {
		t.Errorf("cost = %v, want 0 for a reset already past", got)
	}
}
