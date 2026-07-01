package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeReviewsReturnsChapterAndGlobal(t *testing.T) {
	eng := newFakeEngine(t, 3)
	// Progress: đã tới chương 2.
	writeFile(t, eng.dir, "meta/progress.json", `{"current_chapter":2,"completed_chapters":[1,2]}`)
	// Review từng chương + một review global.
	writeFile(t, eng.dir, "reviews/01.json", `{"chapter":1,"scope":"chapter","verdict":"accept","summary":"ok"}`)
	writeFile(t, eng.dir, "reviews/02.json", `{"chapter":2,"scope":"chapter","verdict":"rewrite","summary":"cần sửa","dimensions":[{"dimension":"pacing","score":40,"verdict":"fail"}]}`)
	writeFile(t, eng.dir, "reviews/02-global.json", `{"chapter":2,"scope":"arc","verdict":"polish","summary":"vòng cung"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/reviews", nil)
	rec := httptest.NewRecorder()
	serveReviews(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body reviewsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Reviews) != 2 {
		t.Fatalf("reviews len = %d, want 2", len(body.Reviews))
	}
	if body.Reviews[1].Verdict != "rewrite" || len(body.Reviews[1].Dimensions) != 1 {
		t.Fatalf("unexpected chapter-2 review: %+v", body.Reviews[1])
	}
	if body.Global == nil || body.Global.Scope != "arc" {
		t.Fatalf("global review missing/wrong: %+v", body.Global)
	}
}

func TestServeReviewsEmptyWhenNoProgress(t *testing.T) {
	eng := newFakeEngine(t, 0)

	req := httptest.NewRequest(http.MethodGet, "/api/reviews", nil)
	rec := httptest.NewRecorder()
	serveReviews(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body reviewsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Reviews) != 0 || body.Global != nil {
		t.Fatalf("expected empty result, got %+v", body)
	}
}

func TestServeReviewsCorruptJSONReturns500(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "meta/progress.json", `{"current_chapter":1}`)
	writeFile(t, eng.dir, "reviews/01.json", `{invalid`)

	req := httptest.NewRequest(http.MethodGet, "/api/reviews", nil)
	rec := httptest.NewRecorder()
	serveReviews(eng, rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestServeForeshadowReturnsEntries(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "foreshadow_ledger.json", `[{"id":"f1","description":"thanh kiếm cổ","planted_at":3,"status":"planted"},{"id":"f2","description":"lời tiên tri","planted_at":1,"status":"resolved","resolved_at":8}]`)

	req := httptest.NewRequest(http.MethodGet, "/api/foreshadow", nil)
	rec := httptest.NewRecorder()
	serveForeshadow(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body foreshadowResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(body.Entries))
	}
	if body.Entries[0].ID != "f1" || body.Entries[1].ResolvedAt != 8 {
		t.Fatalf("unexpected entries: %+v", body.Entries)
	}
}

func TestServeForeshadowEmptyWhenMissing(t *testing.T) {
	eng := newFakeEngine(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/foreshadow", nil)
	rec := httptest.NewRecorder()
	serveForeshadow(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body foreshadowResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(body.Entries))
	}
}
