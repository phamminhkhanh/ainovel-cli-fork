package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProdRunStoreCreateAndList(t *testing.T) {
	dir := t.TempDir()
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	r1, err := ps.create("Alpha", "profiles/a.md", "m1", "p1", 10, 3)
	if err != nil {
		t.Fatalf("create r1: %v", err)
	}
	r2, err := ps.create("Beta", "profiles/b.md", "", "", 0, 0)
	if err != nil {
		t.Fatalf("create r2: %v", err)
	}

	if r1.ID != "run-001" || r2.ID != "run-002" {
		t.Fatalf("unexpected IDs: %s, %s", r1.ID, r2.ID)
	}
	if r2.TargetChapters != 30 {
		t.Fatalf("expected default target 30, got %d", r2.TargetChapters)
	}
	if r2.BudgetUSD != defaultProdRunBudgetUSD {
		t.Fatalf("expected default budget %v, got %v", defaultProdRunBudgetUSD, r2.BudgetUSD)
	}

	list := ps.list()
	if len(list) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(list))
	}
	if list[0].Name != "Alpha" {
		t.Fatalf("expected Alpha first, got %s", list[0].Name)
	}
}

func TestProdRunStorePersistenceAndUncleanShutdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")

	// Seed a store with a running run.
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ps.create("Running", "profiles/r.md", "", "", 5, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := ps.update(r.ID, func(r *ProdRun) {
		r.Status = prodRunRunning
		r.StartedAt = time.Now()
	}); err != nil {
		t.Fatalf("update run: %v", err)
	}

	// Re-open the store to simulate a server restart.
	ps2, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r2 := ps2.get(r.ID)
	if r2.Status != prodRunFailed || r2.StopReason != stopReasonUnclean || !r2.PossiblyOrphaned {
		t.Fatalf("expected failed/unclean/orphaned, got status=%s reason=%s orphaned=%v", r2.Status, r2.StopReason, r2.PossiblyOrphaned)
	}
	if r2.StoppedAt.IsZero() {
		t.Fatal("expected StoppedAt set")
	}

	// The recovered state should have been persisted.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"status": "failed"`) {
		t.Fatal("persisted state should reflect failed status")
	}
}
