package web

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type radarEntry struct {
	Rank     int    `json:"rank"`
	Title    string `json:"title"`
	Author   string `json:"author,omitempty"`
	Category string `json:"category,omitempty"`
	List     string `json:"list,omitempty"`
}

type radarSnapshot struct {
	Source    string       `json:"source"`
	Market    string       `json:"market"`
	FetchedAt time.Time    `json:"fetchedAt"`
	Entries   []radarEntry `json:"entries"`
	Error     string       `json:"error,omitempty"`
}

type radarSource interface {
	ID() string
	Market() string
	Fetch(context.Context) ([]radarEntry, error)
}

type radarRecommendation struct {
	Genre              string   `json:"genre"`
	Concept            string   `json:"concept"`
	Momentum           float64  `json:"momentum"`
	Saturation         float64  `json:"saturation"`
	Transferability    float64  `json:"transferability"`
	EstimatedLagMonths []int    `json:"estimatedLagMonths"`
	Reasoning          string   `json:"reasoning"`
	BenchmarkTitles    []string `json:"benchmarkTitles"`
	LocalizationRisks  []string `json:"localizationRisks"`
	Evidence           []string `json:"evidence"`
}

type radarReport struct {
	GeneratedAt     time.Time             `json:"generatedAt"`
	SourceMarket    string                `json:"sourceMarket"`
	TargetMarket    string                `json:"targetMarket"`
	AnalysisMode    string                `json:"analysisMode"`
	LiveSources     int                   `json:"liveSources"`
	TotalSources    int                   `json:"totalSources"`
	FailedSources   []string              `json:"failedSources"`
	SnapshotAge     string                `json:"snapshotAge"`
	MarketSummary   string                `json:"marketSummary"`
	Recommendations []radarRecommendation `json:"recommendations"`
	Snapshots       []radarSnapshot       `json:"snapshots"`
}

type radarAnalyzer func(context.Context, string, string, []radarSnapshot) (radarReport, error)

func scanMarketRadar(ctx context.Context, targetMarket string, sources []radarSource, analyze radarAnalyzer) (radarReport, error) {
	targetMarket = normalizeRadarMarket(targetMarket)
	if targetMarket == "" {
		targetMarket = "vi"
	}

	type result struct {
		index int
		snap  radarSnapshot
	}
	results := make(chan result, len(sources))
	for i, source := range sources {
		go func(index int, src radarSource) {
			entries, err := src.Fetch(ctx)
			snap := radarSnapshot{Source: src.ID(), Market: src.Market(), FetchedAt: time.Now(), Entries: entries}
			if err != nil {
				snap.Error = err.Error()
			}
			results <- result{index: index, snap: snap}
		}(i, source)
	}

	ordered := make([]result, 0, len(sources))
	for range sources {
		ordered = append(ordered, <-results)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })
	snapshots := make([]radarSnapshot, 0, len(ordered))
	for _, item := range ordered {
		snapshots = append(snapshots, item.snap)
	}

	report, err := analyze(ctx, "cn", targetMarket, snapshots)
	if err != nil {
		return radarReport{}, err
	}
	// Source-health metadata is backend-owned. The model only supplies market
	// analysis fields; never let malformed output or prompt injection pre-seed
	// counters that decide live/partial/fallback mode and confidence caps.
	report.GeneratedAt = time.Time{}
	report.SourceMarket = ""
	report.TargetMarket = ""
	report.AnalysisMode = ""
	report.LiveSources = 0
	report.TotalSources = 0
	report.FailedSources = nil
	report.SnapshotAge = ""
	report.Snapshots = nil
	report.SourceMarket = "cn"
	report.TargetMarket = targetMarket
	report.GeneratedAt = time.Now()
	report.TotalSources = len(snapshots)
	report.Snapshots = snapshots
	for _, snap := range snapshots {
		if len(snap.Entries) > 0 {
			report.LiveSources++
		}
		if snap.Error != "" || len(snap.Entries) == 0 {
			report.FailedSources = append(report.FailedSources, snap.Source)
		}
	}
	switch {
	case report.LiveSources == 0:
		report.AnalysisMode = "model_knowledge_fallback"
		capRadarConfidence(&report, 0.35)
	case report.LiveSources < report.TotalSources || len(report.FailedSources) > 0:
		report.AnalysisMode = "partial_live"
	default:
		report.AnalysisMode = "live"
	}
	report.SnapshotAge = "0s"
	if err := normalizeAndValidateRadarReport(&report); err != nil {
		return radarReport{}, err
	}
	return report, nil
}

func normalizeRadarMarket(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vi", "vn", "vietnam", "vietnamese", "tiếng việt":
		return "vi"
	case "es", "spanish", "español":
		return "es"
	case "en", "english":
		return "en"
	default:
		return ""
	}
}

func capRadarConfidence(report *radarReport, max float64) {
	for i := range report.Recommendations {
		if report.Recommendations[i].Transferability > max {
			report.Recommendations[i].Transferability = max
		}
		if report.Recommendations[i].Momentum > max {
			report.Recommendations[i].Momentum = max
		}
	}
}

func normalizeAndValidateRadarReport(report *radarReport) error {
	if strings.TrimSpace(report.MarketSummary) == "" {
		return fmt.Errorf("radar report missing marketSummary")
	}
	if len(report.Recommendations) < 3 || len(report.Recommendations) > 5 {
		return fmt.Errorf("radar report must have 3-5 recommendations")
	}

	type fact struct {
		source string
		entry  radarEntry
	}
	facts := make(map[string][]fact)
	for _, snapshot := range report.Snapshots {
		for _, entry := range snapshot.Entries {
			key := strings.ToLower(strings.TrimSpace(entry.Title))
			facts[key] = append(facts[key], fact{source: snapshot.Source, entry: entry})
		}
	}
	for i := range report.Recommendations {
		rec := &report.Recommendations[i]
		if strings.TrimSpace(rec.Genre) == "" || strings.TrimSpace(rec.Concept) == "" || strings.TrimSpace(rec.Reasoning) == "" {
			return fmt.Errorf("radar recommendation %d missing genre, concept, or reasoning", i+1)
		}
		if len(rec.EstimatedLagMonths) != 2 || rec.EstimatedLagMonths[0] < 1 || rec.EstimatedLagMonths[1] < rec.EstimatedLagMonths[0] || rec.EstimatedLagMonths[1] > 24 {
			return fmt.Errorf("radar recommendation %d has invalid lag range", i+1)
		}
		validated := make([]string, 0, len(rec.BenchmarkTitles))
		evidence := make([]string, 0, len(rec.BenchmarkTitles))
		seen := make(map[string]struct{})
		for _, title := range rec.BenchmarkTitles {
			key := strings.ToLower(strings.TrimSpace(title))
			items := facts[key]
			if len(items) == 0 {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			validated = append(validated, items[0].entry.Title)
			for _, item := range items {
				evidence = append(evidence, fmt.Sprintf("%s/%s #%d: %s", item.source, item.entry.List, item.entry.Rank, item.entry.Title))
			}
		}
		rec.BenchmarkTitles = validated
		rec.Evidence = evidence
		if report.AnalysisMode != "model_knowledge_fallback" && len(validated) == 0 {
			return fmt.Errorf("radar recommendation %d has no benchmark title from live snapshots", i+1)
		}
	}
	sort.SliceStable(report.Recommendations, func(i, j int) bool {
		if report.Recommendations[i].Transferability == report.Recommendations[j].Transferability {
			return report.Recommendations[i].Momentum > report.Recommendations[j].Momentum
		}
		return report.Recommendations[i].Transferability > report.Recommendations[j].Transferability
	})
	return nil
}
