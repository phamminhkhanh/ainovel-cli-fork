package web

import "testing"

// metricByKey finds a metric in a health strip for assertions.
func metricByKey(h runHealth, key string) (healthMetric, bool) {
	for _, m := range h.Metrics {
		if m.Key == key {
			return m, true
		}
	}
	return healthMetric{}, false
}

func TestComputeRunHealthNil(t *testing.T) {
	if got := computeRunHealth(nil); got.Overall != healthIdle {
		t.Fatalf("nil run: overall = %q, want idle", got.Overall)
	}
}

func TestComputeRunHealthQueuedIsIdle(t *testing.T) {
	r := &ProdRun{Status: prodRunQueued, TargetChapters: 50, BudgetUSD: 5}
	h := computeRunHealth(r)
	if h.Overall != healthIdle {
		t.Fatalf("queued run overall = %q, want idle (no data yet)", h.Overall)
	}
	if m, ok := metricByKey(h, "progress"); !ok || m.Level != healthIdle {
		t.Fatalf("progress metric = %+v, want idle", m)
	}
}

func TestRewriteRateLevels(t *testing.T) {
	cases := []struct {
		name             string
		reviews, rewrite int
		want             healthLevel
	}{
		{"too few reviews", 2, 2, healthIdle},
		{"low rate good", 10, 1, healthGood},
		{"warn band", 10, 3, healthWarn},
		{"bad band", 10, 6, healthBad},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &ProdRun{Reviews: c.reviews, Rewrites: c.rewrite}
			m, ok := metricByKey(computeRunHealth(r), "rewrite_rate")
			if !ok {
				t.Fatal("rewrite_rate metric missing")
			}
			if m.Level != c.want {
				t.Fatalf("reviews=%d rewrites=%d level=%q, want %q", c.reviews, c.rewrite, m.Level, c.want)
			}
		})
	}
}

func TestCostPaceLevels(t *testing.T) {
	// Budget 10 over 100 chapters => expected $0.10/chapter.
	base := func(chapters int, cost float64) *ProdRun {
		return &ProdRun{Chapters: chapters, CostUSD: cost, TargetChapters: 100, BudgetUSD: 10}
	}
	cases := []struct {
		name string
		run  *ProdRun
		want healthLevel
	}{
		{"one chapter idle", base(1, 0.5), healthIdle},
		{"on budget good", base(10, 1.0), healthGood},     // $0.10/ch == expected
		{"slightly over warn", base(10, 1.5), healthWarn}, // $0.15/ch = 1.5x
		{"blowout bad", base(10, 2.5), healthBad},         // $0.25/ch = 2.5x
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, ok := metricByKey(computeRunHealth(c.run), "cost_pace")
			if !ok {
				t.Fatal("cost_pace metric missing")
			}
			if m.Level != c.want {
				t.Fatalf("level=%q, want %q", m.Level, c.want)
			}
		})
	}
}

func TestBudgetLevels(t *testing.T) {
	cases := []struct {
		name         string
		cost, budget float64
		want         healthLevel
	}{
		{"no budget idle", 1, 0, healthIdle},
		{"under good", 3, 10, healthGood},
		{"near cap warn", 8.5, 10, healthWarn},
		{"over cap bad", 10, 10, healthBad},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &ProdRun{CostUSD: c.cost, BudgetUSD: c.budget}
			m, ok := metricByKey(computeRunHealth(r), "budget")
			if !ok {
				t.Fatal("budget metric missing")
			}
			if m.Level != c.want {
				t.Fatalf("cost=%.1f budget=%.1f level=%q, want %q", c.cost, c.budget, m.Level, c.want)
			}
		})
	}
}

func TestOverallIsWorstActionable(t *testing.T) {
	// Healthy chapters/progress but budget blown => overall must be bad.
	r := &ProdRun{
		Status:         prodRunRunning,
		Chapters:       40,
		TargetChapters: 50,
		Reviews:        20,
		Rewrites:       1, // good rewrite rate
		CostUSD:        10,
		BudgetUSD:      10, // 100% used => bad
	}
	h := computeRunHealth(r)
	if h.Overall != healthBad {
		t.Fatalf("overall = %q, want bad (budget exhausted)", h.Overall)
	}
}

func TestProgressDoesNotDriveOverall(t *testing.T) {
	// Everything actionable is good/idle; progress good. Overall should be good,
	// never dragged by progress being informational.
	r := &ProdRun{
		Status:         prodRunRunning,
		Chapters:       5,
		TargetChapters: 50,
		Reviews:        5,
		Rewrites:       0,
		CostUSD:        0.4,
		BudgetUSD:      5,
	}
	h := computeRunHealth(r)
	if h.Overall == healthBad {
		t.Fatalf("overall = %q, should not be bad on a healthy early run", h.Overall)
	}
}

func TestViewEmbedsHealthAndFields(t *testing.T) {
	r := &ProdRun{ID: "run-001", Status: prodRunRunning, Chapters: 3, TargetChapters: 10}
	v := newProdRunView(r)
	if v.ID != "run-001" {
		t.Fatalf("embedded ID = %q, want run-001", v.ID)
	}
	if len(v.Health.Metrics) == 0 {
		t.Fatal("view health has no metrics")
	}
}
