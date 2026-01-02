package apps

import (
	"analytics/models"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEventsFromDisk(t *testing.T) {
	// Create temporary directory for test data
	tempDir := t.TempDir()

	manager := &Manager{}

	// Create test events
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)
	events := []models.Event{
		{EventID: "event1", Timestamp: baseTime},
		{EventID: "event2", Timestamp: baseTime.Add(-30 * time.Minute)},
		{EventID: "event3", Timestamp: baseTime.Add(-2 * time.Hour)}, // Should be filtered out
	}

	// Create directory structure and write test files
	appID := "test-app"
	dateStr := baseTime.Format("20060102")
	dirPath := filepath.Join(tempDir, "data", appID, dateStr)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Write event files
	for i, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("Failed to marshal event: %v", err)
		}

		filename := filepath.Join(dirPath, event.EventID+".json")
		err = os.WriteFile(filename, data, 0644)
		if err != nil {
			t.Fatalf("Failed to write event file %d: %v", i, err)
		}
	}

	// Temporarily change working directory for the test
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	tests := []struct {
		name           string
		startMinutes   int64
		expectCount    int
		expectEventIDs []string
	}{
		{
			name:           "get all recent events",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-1 * time.Hour)),
			expectCount:    2,
			expectEventIDs: []string{"event1", "event2"},
		},
		{
			name:           "get only very recent events",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-15 * time.Minute)),
			expectCount:    1,
			expectEventIDs: []string{"event1"},
		},
		{
			name:           "get events from future (none)",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(1 * time.Hour)),
			expectCount:    0,
			expectEventIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := manager.getEventsFromDisk(appID, tt.startMinutes)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(events) != tt.expectCount {
				t.Errorf("Expected %d events, got %d", tt.expectCount, len(events))
			}

			// Check that we got the right events
			eventIDs := make(map[string]bool)
			for _, event := range events {
				eventIDs[event.EventID] = true
			}

			for _, expectedID := range tt.expectEventIDs {
				if !eventIDs[expectedID] {
					t.Errorf("Expected event %s not found", expectedID)
				}
			}
		})
	}
}

func TestGetEventsFromDiskErrorCases(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name        string
		setupFunc   func() string // Returns temp dir path
		appID       string
		expectError bool
	}{
		{
			name: "nonexistent app directory",
			setupFunc: func() string {
				return t.TempDir()
			},
			appID:       "nonexistent-app",
			expectError: false, // Should return empty, not error
		},
		{
			name: "invalid JSON file",
			setupFunc: func() string {
				tempDir := t.TempDir()
				appID := "test-app"
				dateStr := time.Now().Format("20060102")
				dirPath := filepath.Join(tempDir, "data", appID, dateStr)
				os.MkdirAll(dirPath, 0755)

				// Create invalid JSON file
				os.WriteFile(filepath.Join(dirPath, "invalid.json"), []byte("invalid json"), 0644)
				return tempDir
			},
			appID:       "test-app",
			expectError: false, // Should skip invalid files, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := tt.setupFunc()

			oldWd, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(oldWd)

			startMinutes := toMinutesSinceEpoch(time.Now().Add(-1 * time.Hour))
			events, err := manager.getEventsFromDisk(tt.appID, startMinutes)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Should return empty slice (could be nil, that's ok too)
			if events == nil {
				// This is actually fine - Go slices can be nil and still be valid empty slices
				if len(events) != 0 {
					t.Errorf("Expected empty slice, got length %d", len(events))
				}
			} else if len(events) != 0 {
				t.Errorf("Expected empty slice, got %d events", len(events))
			}
		})
	}
}
