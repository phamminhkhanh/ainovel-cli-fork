package web

import (
	"fmt"
	"time"
)

// Health strip for a production run: a compact, at-a-glance answer to "is this
// run on track / should I review it?" It is 100% derived from fields the runner
// already polls (chapters, reviews, rewrites, cost) — no new detection, no
// engine coupling. Thresholds live here as a single, testable source of truth
// rather than being scattered in the frontend.

// healthLevel is the traffic-light state of a single metric or the run overall.
type healthLevel string

const (
	healthIdle healthLevel = "idle" // not enough data yet (gray)
	healthGood healthLevel = "good" // on track (green)
	healthWarn healthLevel = "warn" // worth a look (yellow)
	healthBad  healthLevel = "bad"  // needs attention (red)
)

// severity orders levels so the overall status can be the worst actionable one.
func (l healthLevel) severity() int {
	switch l {
	case healthGood:
		return 1
	case healthWarn:
		return 2
	case healthBad:
		return 3
	default: // idle
		return 0
	}
}

// healthMetric is one chip in the strip. Key is a stable identifier the
// frontend maps to a Vietnamese label; Value is the already-formatted display
// string. Keeping display labels in the frontend (consistent with the rest of
// app-production.js) and the logic/levels here keeps the thresholds testable.
type healthMetric struct {
	Key   string      `json:"key"`
	Value string      `json:"value"`
	Level healthLevel `json:"level"`
}

// runHealth is the computed strip attached to a ProdRun in API responses.
type runHealth struct {
	Overall healthLevel    `json:"overall"`
	Metrics []healthMetric `json:"metrics"`
}

// Thresholds — documented so the strip's judgement is inspectable.
const (
	// Rewrite rate needs a minimum sample before it means anything.
	healthMinReviewsForRewrite = 3
	rewriteRateWarn            = 0.25 // > 25% of reviews demand a rewrite
	rewriteRateBad             = 0.50 // > 50% is systemic quality drift

	// Cost-per-chapter is compared against the budget's own expectation
	// (budget / target chapters). A run costing far more per chapter than it
	// budgeted for is trending toward a blowout.
	healthMinChaptersForCost = 2
	costPaceWarn             = 1.2 // 20% over expected per-chapter cost
	costPaceBad              = 2.0 // double the expected per-chapter cost

	// Budget consumption relative to the cap.
	budgetUsedWarn = 0.80 // 80% of budget spent
)

// computeRunHealth derives the health strip from a run's polled stats. It is a
// pure function: same input always yields the same output, and it never mutates
// the run. Metrics that lack enough data are marked idle and do not drag the
// overall status down.
func computeRunHealth(r *ProdRun) runHealth {
	if r == nil {
		return runHealth{Overall: healthIdle}
	}

	metrics := []healthMetric{
		progressMetric(r),
		rewriteRateMetric(r),
		costPaceMetric(r),
		budgetMetric(r),
		persistMetric(r),
	}

	overall := healthIdle
	for _, m := range metrics {
		// Progress is informational only — a run early in its life is not
		// "bad", it just hasn't produced enough to judge. Only the actionable
		// metrics drive the overall light.
		if m.Key == "progress" {
			continue
		}
		if m.Level.severity() > overall.severity() {
			overall = m.Level
		}
	}
	return runHealth{Overall: overall, Metrics: metrics}
}

func progressMetric(r *ProdRun) healthMetric {
	pct := 0
	if r.TargetChapters > 0 {
		pct = int(float64(r.Chapters) / float64(r.TargetChapters) * 100)
		if pct > 100 {
			pct = 100
		}
	}
	level := healthIdle
	if r.Chapters > 0 {
		level = healthGood
	}
	return healthMetric{
		Key:   "progress",
		Value: fmt.Sprintf("%d/%d (%d%%)", r.Chapters, r.TargetChapters, pct),
		Level: level,
	}
}

func rewriteRateMetric(r *ProdRun) healthMetric {
	if r.Reviews < healthMinReviewsForRewrite {
		return healthMetric{Key: "rewrite_rate", Value: "\u2014", Level: healthIdle}
	}
	rate := float64(r.Rewrites) / float64(r.Reviews)
	level := healthGood
	switch {
	case rate > rewriteRateBad:
		level = healthBad
	case rate > rewriteRateWarn:
		level = healthWarn
	}
	return healthMetric{
		Key:   "rewrite_rate",
		Value: fmt.Sprintf("%d%%", int(rate*100+0.5)),
		Level: level,
	}
}

func costPaceMetric(r *ProdRun) healthMetric {
	if r.Chapters < healthMinChaptersForCost || r.CostUSD <= 0 {
		return healthMetric{Key: "cost_pace", Value: "\u2014", Level: healthIdle}
	}
	perCh := r.CostUSD / float64(r.Chapters)
	level := healthGood
	if r.TargetChapters > 0 && r.BudgetUSD > 0 {
		expected := r.BudgetUSD / float64(r.TargetChapters)
		if expected > 0 {
			ratio := perCh / expected
			switch {
			case ratio > costPaceBad:
				level = healthBad
			case ratio > costPaceWarn:
				level = healthWarn
			}
		}
	}
	return healthMetric{
		Key:   "cost_pace",
		Value: fmt.Sprintf("$%.3f/ch", perCh),
		Level: level,
	}
}

func budgetMetric(r *ProdRun) healthMetric {
	// Idle until money is actually spent: 0% of budget used on a not-yet-run
	// job is "no data", not "healthy". Without this a queued run would read
	// green overall before it has produced anything.
	if r.BudgetUSD <= 0 || r.CostUSD <= 0 {
		return healthMetric{Key: "budget", Value: "\u2014", Level: healthIdle}
	}
	used := r.CostUSD / r.BudgetUSD
	level := healthGood
	switch {
	case used >= 1.0:
		level = healthBad
	case used >= budgetUsedWarn:
		level = healthWarn
	}
	return healthMetric{
		Key:   "budget",
		Value: fmt.Sprintf("%d%%", int(used*100+0.5)),
		Level: level,
	}
}

// persistMetric báo trạng thái lưu jobs.json. Khi persist OK → idle (ẩn chip,
// không nhiễu). Khi có PersistError (thường Windows file lock do IDE mở jobs.json)
// → bad (đỏ) nếu lỗi mới (<5 phút), warn (vàng) nếu lỗi cũ không tái diễn.
func persistMetric(r *ProdRun) healthMetric {
	if r.PersistError == "" {
		return healthMetric{Key: "persist", Value: "ok", Level: healthIdle}
	}
	age := time.Since(r.PersistErrorAt)
	level := healthBad
	if age > 5*time.Minute {
		level = healthWarn
	}
	return healthMetric{
		Key:   "persist",
		Value: "file lock",
		Level: level,
	}
}

// prodRunView wraps a ProdRun with its computed health for API responses. The
// embedded pointer promotes all ProdRun JSON fields to the top level, so the
// frontend still reads run.chapters/run.status directly and gains run.health.
type prodRunView struct {
	*ProdRun
	Health runHealth `json:"health"`
}

func newProdRunView(r *ProdRun) prodRunView {
	return prodRunView{ProdRun: r, Health: computeRunHealth(r)}
}

func newProdRunViews(rs []*ProdRun) []prodRunView {
	views := make([]prodRunView, 0, len(rs))
	for _, r := range rs {
		views = append(views, newProdRunView(r))
	}
	return views
}
