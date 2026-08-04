package web

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *server) handleRadarScan(w http.ResponseWriter, r *http.Request) {
	if !requireRadarJSONPost(w, r) {
		return
	}
	var body struct {
		TargetMarket string `json:"targetMarket"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.TargetMarket != "" && normalizeRadarMarket(body.TargetMarket) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("targetMarket must be vi, es, or en"))
		return
	}
	if !s.radarMu.TryLock() {
		writeErr(w, http.StatusConflict, fmt.Errorf("radar scan already running"))
		return
	}
	defer s.radarMu.Unlock()
	sources := s.radarSources
	if len(sources) == 0 {
		sources = defaultRadarSources()
	}
	analyze := s.radarAnalyze
	if analyze == nil {
		analyze = s.analyzeRadar
	}
	report, err := scanMarketRadar(r.Context(), body.TargetMarket, sources, analyze)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.saveRadarReport(report); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// requireRadarJSONPost protects the paid model call from browser cross-site
// requests. Host validation blocks DNS rebinding, but a malicious page can
// still POST directly to 127.0.0.1 with a valid loopback Host. Requiring JSON
// forces CORS preflight, and Origin/Sec-Fetch-Site reject cross-site requests
// before any source fetch or model spend. Origin may be absent for curl/tests.
func requireRadarJSONPost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeErr(w, http.StatusUnsupportedMediaType, fmt.Errorf("Content-Type must be application/json"))
		return false
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
		writeErr(w, http.StatusForbidden, fmt.Errorf("cross-site request denied"))
		return false
	}
	if rawOrigin := strings.TrimSpace(r.Header.Get("Origin")); rawOrigin != "" {
		origin, err := url.Parse(rawOrigin)
		if err != nil || origin.Host == "" || !strings.EqualFold(origin.Host, r.Host) {
			writeErr(w, http.StatusForbidden, fmt.Errorf("cross-origin request denied"))
			return false
		}
	}
	return true
}

func (s *server) handleRadarLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	data, err := os.ReadFile(filepath.Join(s.radarDir(), "latest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, fmt.Errorf("no radar report yet"))
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var report radarReport
	if err := json.Unmarshal(data, &report); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("parse latest radar report: %w", err))
		return
	}
	if !report.GeneratedAt.IsZero() {
		report.SnapshotAge = time.Since(report.GeneratedAt).Round(time.Second).String()
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *server) radarDir() string {
	return filepath.Join(filepath.Dir(s.store.Dir()), "radar")
}

func (s *server) saveRadarReport(report radarReport) error {
	dir := s.radarDir()
	if err := os.MkdirAll(filepath.Join(dir, "reports"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		return err
	}
	stamp := report.GeneratedAt.UTC().Format("20060102T150405.000000000Z")
	raw, err := json.MarshalIndent(report.Snapshots, "", "  ")
	if err != nil {
		return err
	}
	if err := safeWriteFile(filepath.Join(dir, "raw", "snapshot-"+stamp+".json"), raw); err != nil {
		return fmt.Errorf("save radar snapshot: %w", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := safeWriteFile(filepath.Join(dir, "reports", "report-"+stamp+".json"), data); err != nil {
		return fmt.Errorf("save radar report: %w", err)
	}
	if err := safeWriteFile(filepath.Join(dir, "latest.json"), data); err != nil {
		return fmt.Errorf("save latest radar report: %w", err)
	}
	return nil
}
