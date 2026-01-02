package apps

import (
	"analytics/models"
	"sync"
	"testing"
	"time"
)

func TestGetEventsFromCache(t *testing.T) {
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)

	// Create real EventCache without the advance goroutine
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
		mu:           sync.RWMutex{},
	}

	// Add test events
	events := []*models.Event{
		{EventID: "event1", Timestamp: baseTime},
		{EventID: "event2", Timestamp: baseTime.Add(-5 * time.Minute)},
		{EventID: "event3", Timestamp: baseTime.Add(-15 * time.Minute)},
	}

	for _, event := range events {
		cache.Add(event)
	}

	manager := &Manager{
		caches:   make(map[string]*EventCache),
		cachesMu: sync.RWMutex{},
	}
	manager.caches["test-app"] = cache

	tests := []struct {
		name           string
		appID          string
		startMinutes   int64
		expectFound    bool
		expectCount    int
		expectEventIDs []string
	}{
		{
			name:           "cache hit - get all recent events",
			appID:          "test-app",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-20 * time.Minute)),
			expectFound:    true,
			expectCount:    3,
			expectEventIDs: []string{"event1", "event2", "event3"},
		},
		{
			name:           "cache hit - get only very recent events",
			appID:          "test-app",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-3 * time.Minute)),
			expectFound:    true,
			expectCount:    1,
			expectEventIDs: []string{"event1"},
		},
		{
			name:         "cache miss - app doesn't exist",
			appID:        "nonexistent-app",
			startMinutes: toMinutesSinceEpoch(baseTime.Add(-10 * time.Minute)),
			expectFound:  false,
			expectCount:  0,
		},
		{
			name:         "cache miss - start time too old",
			appID:        "test-app",
			startMinutes: toMinutesSinceEpoch(baseTime.Add(-40 * time.Minute)),
			expectFound:  false,
			expectCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, found := manager.getEventsFromCache(tt.appID, tt.startMinutes)

			if found != tt.expectFound {
				t.Errorf("Expected found=%t, got found=%t", tt.expectFound, found)
			}

			if len(events) != tt.expectCount {
				t.Errorf("Expected %d events, got %d", tt.expectCount, len(events))
			}

			if tt.expectFound {
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
			}
		})
	}
}

func TestGetEventsFromCacheCircularBuffer(t *testing.T) {
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)

	// Test the real EventCache circular buffer behavior
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
		mu:           sync.RWMutex{},
	}

	// Add events at specific times to test circular buffer
	events := []*models.Event{
		{EventID: "current", Timestamp: baseTime},
		{EventID: "minus5", Timestamp: baseTime.Add(-5 * time.Minute)},
		{EventID: "minus29", Timestamp: baseTime.Add(-29 * time.Minute)},
	}

	for _, event := range events {
		cache.Add(event)
	}

	manager := &Manager{
		caches:   make(map[string]*EventCache),
		cachesMu: sync.RWMutex{},
	}
	manager.caches["test-app"] = cache

	t.Run("verify circular buffer placement", func(t *testing.T) {
		// Manually check that events are in the right buckets
		cache.mu.RLock()
		defer cache.mu.RUnlock()

		// Current minute should be at currentIndex (0)
		if len(cache.buckets[0]) != 1 || cache.buckets[0][0].EventID != "current" {
			t.Errorf("Current event not in bucket 0, got %d events", len(cache.buckets[0]))
		}

		// 5 minutes ago should be at index (0-5+30)%30 = 25
		expectedIndex5 := (0 - 5 + CacheWindowMinutes) % CacheWindowMinutes
		if len(cache.buckets[expectedIndex5]) != 1 || cache.buckets[expectedIndex5][0].EventID != "minus5" {
			t.Errorf("5-minute event not in bucket %d, got %d events", expectedIndex5, len(cache.buckets[expectedIndex5]))
		}

		// 29 minutes ago should be at index (0-29+30)%30 = 1
		expectedIndex29 := (0 - 29 + CacheWindowMinutes) % CacheWindowMinutes
		if len(cache.buckets[expectedIndex29]) != 1 || cache.buckets[expectedIndex29][0].EventID != "minus29" {
			t.Errorf("29-minute event not in bucket %d, got %d events", expectedIndex29, len(cache.buckets[expectedIndex29]))
		}
	})

	t.Run("test GetEventsSince with circular buffer", func(t *testing.T) {
		// Test that GetEventsSince correctly traverses the circular buffer
		startMinutes := toMinutesSinceEpoch(baseTime.Add(-30 * time.Minute))
		events := cache.GetEventsSince(startMinutes)

		if len(events) != 3 {
			t.Errorf("Expected 3 events, got %d", len(events))
		}

		eventIDs := make(map[string]bool)
		for _, event := range events {
			eventIDs[event.EventID] = true
		}

		expectedIDs := []string{"current", "minus5", "minus29"}
		for _, expectedID := range expectedIDs {
			if !eventIDs[expectedID] {
				t.Errorf("Expected event %s not found", expectedID)
			}
		}
	})
}
