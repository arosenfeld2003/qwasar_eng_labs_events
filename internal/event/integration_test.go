package event

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFromDatasetFile loads a real dataset file and validates all events.
func TestLoadFromDatasetFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	datasetDir := filepath.Join("..", "..", "datasets")
	entries, err := os.ReadDir(datasetDir)
	if err != nil {
		t.Skipf("datasets directory not accessible: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(datasetDir, entry.Name())
			events, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile(%s): %v", path, err)
			}
			if len(events) == 0 {
				t.Fatalf("expected events in %s, got 0", entry.Name())
			}

			for i, e := range events {
				if e.ID <= 0 {
					t.Errorf("event %d: invalid ID %d", i, e.ID)
				}
				if e.Type == "" {
					t.Errorf("event %d (id=%d): empty event type", i, e.ID)
				}
				if !e.Priority.IsValid() {
					t.Errorf("event %d (id=%d): invalid priority %q", i, e.ID, e.Priority)
				}
			}
		})
	}
}

// TestDatasetTeamRouting verifies every event in each dataset maps to a known team.
func TestDatasetTeamRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	datasetDir := filepath.Join("..", "..", "datasets")
	entries, err := os.ReadDir(datasetDir)
	if err != nil {
		t.Skipf("datasets directory not accessible: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(datasetDir, entry.Name())
			events, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile: %v", err)
			}

			teamCounts := make(map[Team]int)
			for _, e := range events {
				team := e.Team()
				if team == TeamUnknown {
					t.Errorf("event %d (%s): routed to unknown team", e.ID, e.Type)
				}
				teamCounts[team]++
			}

			if len(teamCounts) == 0 {
				t.Fatal("no team routing occurred")
			}

			t.Logf("Team distribution for %s:", entry.Name())
			for team, count := range teamCounts {
				t.Logf("  %s: %d events", team, count)
			}
		})
	}
}
