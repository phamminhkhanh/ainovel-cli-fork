package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/store"
)

type fakeRadarSource struct {
	id      string
	entries []radarEntry
	err     error
}

func testRadarReport(title string) radarReport {
	recs := make([]radarRecommendation, 3)
	for i := range recs {
		recs[i] = radarRecommendation{
			Genre: "mystery", Concept: fmt.Sprintf("concept %d", i+1),
			Momentum: .8 - float64(i)*.1, Saturation: .4,
			Transferability: .7 - float64(i)*.1, EstimatedLagMonths: []int{6, 12},
			Reasoning: "portable serial hook", BenchmarkTitles: []string{title},
		}
	}
	return radarReport{MarketSummary: "market", Recommendations: recs}
}

func (s fakeRadarSource) ID() string     { return s.id }
func (s fakeRadarSource) Market() string { return "cn" }
func (s fakeRadarSource) Fetch(context.Context) ([]radarEntry, error) {
	return s.entries, s.err
}

func TestScanMarketRadarPartialLive(t *testing.T) {
	analyze := func(_ context.Context, source, target string, snapshots []radarSnapshot) (radarReport, error) {
		if source != "cn" || target != "vi" || len(snapshots) != 2 {
			t.Fatalf("unexpected analyzer input: %s %s %d", source, target, len(snapshots))
		}
		return testRadarReport("A"), nil
	}
	report, err := scanMarketRadar(context.Background(), "vi", []radarSource{
		fakeRadarSource{id: "fanqie", entries: []radarEntry{{Rank: 1, Title: "A"}}},
		fakeRadarSource{id: "qidian", err: errors.New("blocked")},
	}, analyze)
	if err != nil {
		t.Fatal(err)
	}
	if report.AnalysisMode != "partial_live" || report.LiveSources != 1 || len(report.FailedSources) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestScanMarketRadarFallbackCapsConfidence(t *testing.T) {
	report, err := scanMarketRadar(context.Background(), "es", []radarSource{
		fakeRadarSource{id: "fanqie", err: errors.New("down")},
	}, func(context.Context, string, string, []radarSnapshot) (radarReport, error) {
		report := testRadarReport("")
		report.MarketSummary = "fallback"
		for i := range report.Recommendations {
			report.Recommendations[i].BenchmarkTitles = nil
			report.Recommendations[i].Momentum = .9
			report.Recommendations[i].Transferability = .8
		}
		return report, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.AnalysisMode != "model_knowledge_fallback" {
		t.Fatalf("mode = %s", report.AnalysisMode)
	}
	if got := report.Recommendations[0].Transferability; got != .35 {
		t.Fatalf("transferability = %v", got)
	}
}

func TestScanMarketRadarIgnoresModelOwnedSourceMetadata(t *testing.T) {
	report, err := scanMarketRadar(context.Background(), "vi", []radarSource{
		fakeRadarSource{id: "fanqie", err: errors.New("down")},
		fakeRadarSource{id: "qidian", err: errors.New("blocked")},
	}, func(context.Context, string, string, []radarSnapshot) (radarReport, error) {
		report := testRadarReport("")
		for i := range report.Recommendations {
			report.Recommendations[i].BenchmarkTitles = nil
			report.Recommendations[i].Momentum = .9
			report.Recommendations[i].Transferability = .8
		}
		// Simulate malformed/model-injected backend metadata. None of these
		// values may survive orchestration.
		report.LiveSources = 10
		report.TotalSources = 99
		report.FailedSources = []string{"fake"}
		report.AnalysisMode = "live"
		return report, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.AnalysisMode != "model_knowledge_fallback" || report.LiveSources != 0 || report.TotalSources != 2 {
		t.Fatalf("backend metadata not recomputed: %+v", report)
	}
	if len(report.FailedSources) != 2 || report.FailedSources[0] != "fanqie" || report.FailedSources[1] != "qidian" {
		t.Fatalf("failed sources = %#v", report.FailedSources)
	}
	if got := report.Recommendations[0].Transferability; got != .35 {
		t.Fatalf("fallback confidence cap bypassed: %v", got)
	}
}

func TestScanMarketRadarCountsDegradedSourceAsLive(t *testing.T) {
	report, err := scanMarketRadar(context.Background(), "vi", []radarSource{
		fakeRadarSource{id: "fanqie", entries: []radarEntry{{Rank: 1, Title: "A"}}, err: errors.New("dark-horse list failed")},
		fakeRadarSource{id: "qidian", err: errors.New("blocked")},
	}, func(context.Context, string, string, []radarSnapshot) (radarReport, error) {
		return testRadarReport("A"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.AnalysisMode != "partial_live" || report.LiveSources != 1 || len(report.FailedSources) != 2 {
		t.Fatalf("unexpected degraded report: %+v", report)
	}
}

func TestRadarEvidencePreservesDuplicateTitleFacts(t *testing.T) {
	report := testRadarReport("A")
	report.AnalysisMode = "live"
	report.Snapshots = []radarSnapshot{
		{Source: "fanqie", Entries: []radarEntry{{Rank: 1, Title: "A", List: "hot"}}},
		{Source: "qidian", Entries: []radarEntry{{Rank: 3, Title: "A", List: "rank"}}},
	}
	if err := normalizeAndValidateRadarReport(&report); err != nil {
		t.Fatal(err)
	}
	if len(report.Recommendations[0].Evidence) != 2 {
		t.Fatalf("evidence = %#v", report.Recommendations[0].Evidence)
	}
}

func TestParseRadarReportClampsScores(t *testing.T) {
	report, err := parseRadarReport("```json\n{\"marketSummary\":\"x\",\"recommendations\":[{\"genre\":\"g\",\"concept\":\"c\",\"momentum\":2,\"saturation\":-1,\"transferability\":0.8}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	got := report.Recommendations[0]
	if got.Momentum != 1 || got.Saturation != 0 || got.Transferability != .8 {
		t.Fatalf("scores not clamped: %+v", got)
	}
}

func TestRadarOutputLanguage(t *testing.T) {
	cases := map[string]string{
		"vi": "Vietnamese (tiếng Việt tự nhiên, dễ hiểu; giữ nguyên tên sách Trung Quốc)",
		"es": "Neutral Spanish (español neutro; keep Chinese book titles unchanged)",
		"en": "English (keep Chinese book titles unchanged)",
	}
	for market, want := range cases {
		if got := radarOutputLanguage(market); got != want {
			t.Fatalf("radarOutputLanguage(%q) = %q, want %q", market, got, want)
		}
	}
}

func TestRadarRoutesScanAndLatest(t *testing.T) {
	dir := t.TempDir()
	s := &server{
		store:        store.NewStore(filepath.Join(dir, "output", "novel")),
		radarSources: []radarSource{fakeRadarSource{id: "fanqie", entries: []radarEntry{{Rank: 1, Title: "A"}}}},
		radarAnalyze: func(context.Context, string, string, []radarSnapshot) (radarReport, error) {
			return testRadarReport("A"), nil
		},
	}
	if err := s.store.Init(); err != nil {
		t.Fatal(err)
	}
	handler := s.mux()
	req := httptest.NewRequest(http.MethodPost, "/api/radar/scan", bytes.NewReader([]byte(`{"targetMarket":"vi"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("scan status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var report radarReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.AnalysisMode != "live" || report.LiveSources != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	latestReq := httptest.NewRequest(http.MethodGet, "/api/radar/latest", nil)
	latestRec := httptest.NewRecorder()
	handler.ServeHTTP(latestRec, latestReq)
	if latestRec.Code != http.StatusOK {
		t.Fatalf("latest status = %d, body=%s", latestRec.Code, latestRec.Body.String())
	}
	matches, err := filepath.Glob(filepath.Join(s.radarDir(), "raw", "snapshot-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("raw snapshots = %d", len(matches))
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/radar/scan", bytes.NewReader([]byte(`{"targetMarket":"xx"}`)))
	badReq.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid market status = %d", badRec.Code)
	}
}

func TestRadarScanRejectsCrossSiteAndNonJSON(t *testing.T) {
	s := &server{}
	nonJSON := httptest.NewRequest(http.MethodPost, "/api/radar/scan", strings.NewReader(`{}`))
	nonJSONRec := httptest.NewRecorder()
	s.handleRadarScan(nonJSONRec, nonJSON)
	if nonJSONRec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-JSON status = %d", nonJSONRec.Code)
	}

	crossSite := httptest.NewRequest(http.MethodPost, "/api/radar/scan", strings.NewReader(`{}`))
	crossSite.Header.Set("Content-Type", "application/json")
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	crossSiteRec := httptest.NewRecorder()
	s.handleRadarScan(crossSiteRec, crossSite)
	if crossSiteRec.Code != http.StatusForbidden {
		t.Fatalf("cross-site status = %d", crossSiteRec.Code)
	}
}

func TestFetchQidianRadarParsesAndDeduplicates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<a href="//book.qidian.com/info/1">Alpha</a><a href="//book.qidian.com/info/2">Alpha</a><a href="//book.qidian.com/info/3">Beta &amp; Co</a>`))
	}))
	defer srv.Close()

	client := srv.Client()
	originalTransport := client.Transport
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return originalTransport.RoundTrip(req)
	})
	entries, err := fetchQidianRadar(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[1].Title != "Beta & Co" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestFetchFanqieRadarParsesBothLists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		list := "hot"
		if r.URL.Query().Get("side_type") == "13" {
			list = "dark"
		}
		_, _ = fmt.Fprintf(w, `{"data":{"result":[{"book_name":"%s Book","author":"A","category":"Fantasy"}]}}`, list)
	}))
	defer srv.Close()

	client := srv.Client()
	originalTransport := client.Transport
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return originalTransport.RoundTrip(req)
	})
	entries, err := fetchFanqieRadar(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].List != "hot" || entries[1].List != "dark_horse" {
		t.Fatalf("entries = %+v", entries)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
