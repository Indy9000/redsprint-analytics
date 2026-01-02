package apps

import (
	"analytics/models"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGetEvents(t *testing.T) {
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)

	// Create real EventCache without advance goroutine
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
		mu:           sync.RWMutex{},
	}

	// Add events to cache
	cacheEvents := []*models.Event{
		{EventID: "cache-event1", Timestamp: baseTime},
		{EventID: "cache-event2", Timestamp: baseTime.Add(-10 * time.Minute)},
	}

	for _, event := range cacheEvents {
		cache.Add(event)
	}

	// Setup temporary directory for disk events
	tempDir := t.TempDir()
	appID := "test-app"
	dateStr := baseTime.Format("20060102")
	dirPath := filepath.Join(tempDir, "data", appID, dateStr)
	os.MkdirAll(dirPath, 0755)

	// Write a disk event (outside cache window)
	diskEvent := models.Event{
		EventID:   "disk-event1",
		Timestamp: baseTime.Add(-40 * time.Minute),
	}
	data, _ := json.Marshal(diskEvent)
	os.WriteFile(filepath.Join(dirPath, "disk-event1.json"), data, 0644)

	manager := &Manager{
		caches:   make(map[string]*EventCache),
		cachesMu: sync.RWMutex{},
	}
	manager.caches[appID] = cache

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	tests := []struct {
		name           string
		startMinutes   int64
		expectSource   string // "cache" or "disk"
		expectCount    int
		expectEventIDs []string
	}{
		{
			name:           "recent time - should hit cache",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-15 * time.Minute)),
			expectSource:   "cache",
			expectCount:    2,
			expectEventIDs: []string{"cache-event1", "cache-event2"},
		},
		{
			name:           "old time - should hit disk",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-45 * time.Minute)),
			expectSource:   "disk",
			expectCount:    1,
			expectEventIDs: []string{"disk-event1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := manager.getEvents(appID, tt.startMinutes)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(events) != tt.expectCount {
				t.Errorf("Expected %d events, got %d", tt.expectCount, len(events))
			}

			// Check event IDs
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

func TestGetEventsIntegrationWithNoCache(t *testing.T) {
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)

	// Setup temporary directory for disk events
	tempDir := t.TempDir()
	appID := "no-cache-app"
	dateStr := baseTime.Format("20060102")
	dirPath := filepath.Join(tempDir, "data", appID, dateStr)
	os.MkdirAll(dirPath, 0755)

	// Write disk events
	diskEvents := []models.Event{
		{EventID: "disk1", Timestamp: baseTime.Add(-5 * time.Minute)},
		{EventID: "disk2", Timestamp: baseTime.Add(-35 * time.Minute)},
	}

	for _, event := range diskEvents {
		data, _ := json.Marshal(event)
		os.WriteFile(filepath.Join(dirPath, event.EventID+".json"), data, 0644)
	}

	manager := &Manager{
		caches:   make(map[string]*EventCache),
		cachesMu: sync.RWMutex{},
	}
	// No cache for this app

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	t.Run("no cache - should go to disk", func(t *testing.T) {
		startMinutes := toMinutesSinceEpoch(baseTime.Add(-10 * time.Minute))
		events, err := manager.getEvents(appID, startMinutes)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}

		if events[0].EventID != "disk1" {
			t.Errorf("Expected disk1, got %s", events[0].EventID)
		}
	})
}
