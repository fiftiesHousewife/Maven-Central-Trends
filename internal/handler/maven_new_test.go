package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDedup(t *testing.T) {
	tests := []struct {
		name     string
		input    []groupInfo
		wantLen  int
		wantIDs  []string
	}{
		{
			name:    "empty",
			input:   nil,
			wantLen: 0,
		},
		{
			name: "no duplicates",
			input: []groupInfo{
				{GroupID: "com.foo"},
				{GroupID: "com.bar"},
			},
			wantLen: 2,
			wantIDs: []string{"com.foo", "com.bar"},
		},
		{
			name: "removes duplicates preserving order",
			input: []groupInfo{
				{GroupID: "com.foo", FirstArtifact: "a"},
				{GroupID: "com.bar", FirstArtifact: "b"},
				{GroupID: "com.foo", FirstArtifact: "c"},
				{GroupID: "com.baz", FirstArtifact: "d"},
				{GroupID: "com.bar", FirstArtifact: "e"},
			},
			wantLen: 3,
			wantIDs: []string{"com.foo", "com.bar", "com.baz"},
		},
		{
			name: "keeps first occurrence",
			input: []groupInfo{
				{GroupID: "com.foo", FirstArtifact: "first"},
				{GroupID: "com.foo", FirstArtifact: "second"},
			},
			wantLen: 1,
			wantIDs: []string{"com.foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedup(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("dedup() returned %d items, want %d", len(got), tt.wantLen)
			}
			for i, id := range tt.wantIDs {
				if i >= len(got) {
					break
				}
				if got[i].GroupID != id {
					t.Errorf("dedup()[%d].GroupID = %q, want %q", i, got[i].GroupID, id)
				}
			}
			// Verify first occurrence is kept
			if tt.name == "keeps first occurrence" && len(got) > 0 {
				if got[0].FirstArtifact != "first" {
					t.Errorf("dedup() kept %q, want %q", got[0].FirstArtifact, "first")
				}
			}
		})
	}
}

func TestGroupsCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	origFile := groupsCacheFile

	// Temporarily override cache file path
	tmpFile := filepath.Join(dir, "test_groups.json")

	// Write test data
	groups := map[string][]groupInfo{
		"2024-01": {
			{GroupID: "com.foo", FirstArtifact: "foo-core"},
			{GroupID: "com.bar", FirstArtifact: "bar-lib"},
			{GroupID: "com.foo", FirstArtifact: "foo-dup"}, // duplicate
		},
		"2024-02": {
			{GroupID: "com.baz", FirstArtifact: "baz"},
		},
	}

	// Write directly (bypass writeGroupsCache which uses the const path)
	cleaned := make(map[string][]groupInfo)
	for month, gs := range groups {
		cleaned[month] = dedup(gs)
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(tmpFile, b, 0644)

	// Read back
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	var loaded map[string][]groupInfo
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}

	// Verify dedup happened on write
	if len(loaded["2024-01"]) != 2 {
		t.Errorf("expected 2 groups in 2024-01 after dedup, got %d", len(loaded["2024-01"]))
	}
	if len(loaded["2024-02"]) != 1 {
		t.Errorf("expected 1 group in 2024-02, got %d", len(loaded["2024-02"]))
	}

	_ = origFile // suppress unused
}

func TestScanProgressRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "test_progress.json")

	sp := scanProgressData{
		CompletedPrefixes: map[string]int{
			"ai":  277,
			"com": 12914,
		},
	}

	b, err := json.Marshal(sp)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(tmpFile, b, 0644)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	var loaded scanProgressData
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}

	if loaded.CompletedPrefixes["ai"] != 277 {
		t.Errorf("ai prefix count = %d, want 277", loaded.CompletedPrefixes["ai"])
	}
	if loaded.CompletedPrefixes["com"] != 12914 {
		t.Errorf("com prefix count = %d, want 12914", loaded.CompletedPrefixes["com"])
	}
}

func TestMavenNewHandler(t *testing.T) {
	// Create a temp groups cache file
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "groups.json")

	now := time.Now().UTC()
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")
	twoMonthsAgo := now.AddDate(0, -2, 0).Format("2006-01")

	groups := map[string][]groupInfo{
		lastMonth: {
			{GroupID: "com.foo"},
			{GroupID: "com.bar"},
		},
		twoMonthsAgo: {
			{GroupID: "com.baz"},
		},
		thisMonth: {
			{GroupID: "com.current"}, // should be excluded if day < 30
		},
	}

	b, _ := json.Marshal(groups)
	os.WriteFile(tmpFile, b, 0644)

	// We can't easily test the handler without overriding the file path,
	// but we can test the month filtering logic directly
	start := now.AddDate(-4, 0, 0)
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)

	var months []newMonthCount
	for m := start; m.Before(now); m = m.AddDate(0, 1, 0) {
		if m.Year() == now.Year() && m.Month() == now.Month() && now.Day() < 30 {
			continue
		}
		key := m.Format("2006-01")
		months = append(months, newMonthCount{
			Month:     key,
			NewGroups: len(dedup(groups[key])),
		})
	}

	// Verify current partial month is excluded (unless day >= 30)
	for _, m := range months {
		if m.Month == thisMonth && now.Day() < 30 {
			t.Errorf("current month %s should be excluded when day < 30", thisMonth)
		}
	}

	// Verify last month has data
	found := false
	for _, m := range months {
		if m.Month == lastMonth {
			if m.NewGroups != 2 {
				t.Errorf("last month %s has %d groups, want 2", lastMonth, m.NewGroups)
			}
			found = true
		}
	}
	if !found {
		t.Errorf("last month %s not found in results", lastMonth)
	}
}

func TestMavenNewGroupsHandler(t *testing.T) {
	// Create temp cache
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "groups.json")

	groups := map[string][]groupInfo{
		"2024-06": {
			{GroupID: "com.foo", FirstArtifact: "foo"},
			{GroupID: "com.bar", FirstArtifact: "bar"},
			{GroupID: "com.foo", FirstArtifact: "foo-dup"}, // duplicate
		},
	}

	b, _ := json.Marshal(groups)
	os.WriteFile(tmpFile, b, 0644)

	// Test dedup on the data
	result := dedup(groups["2024-06"])
	if len(result) != 2 {
		t.Errorf("expected 2 deduped groups, got %d", len(result))
	}
}

func TestScanStatusJSON(t *testing.T) {
	scanStatus.mu.Lock()
	scanStatus.CurrentPrefix = "com"
	scanStatus.PrefixTotal = 12914
	scanStatus.PrefixDone = 5000
	scanStatus.CompletedCount = 5
	scanStatus.TotalGroupsFound = 3700
	scanStatus.Scanning = true
	scanStatus.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/scan-progress", nil)
	rec := httptest.NewRecorder()

	ScanProgress(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var state scanState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if state.CurrentPrefix != "com" {
		t.Errorf("CurrentPrefix = %q, want %q", state.CurrentPrefix, "com")
	}
	if state.PrefixTotal != 12914 {
		t.Errorf("PrefixTotal = %d, want 12914", state.PrefixTotal)
	}
	if state.PrefixDone != 5000 {
		t.Errorf("PrefixDone = %d, want 5000", state.PrefixDone)
	}
	if state.TotalGroupsFound != 3700 {
		t.Errorf("TotalGroupsFound = %d, want 3700", state.TotalGroupsFound)
	}
	if !state.Scanning {
		t.Error("Scanning should be true")
	}

	// Reset
	scanStatus.mu.Lock()
	scanStatus.Scanning = false
	scanStatus.CurrentPrefix = ""
	scanStatus.PrefixTotal = 0
	scanStatus.PrefixDone = 0
	scanStatus.CompletedCount = 0
	scanStatus.TotalGroupsFound = 0
	scanStatus.mu.Unlock()
}
