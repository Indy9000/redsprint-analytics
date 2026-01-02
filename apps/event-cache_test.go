package apps

import (
	"analytics/models"
	"testing"
	"time"
)

func TestNewEventCache(t *testing.T) {
	cache := NewEventCache()

	if cache.currentIndex != 0 {
		t.Errorf("Expected currentIndex to be 0, got %d", cache.currentIndex)
	}

	// Check that lastMinute is recent and truncated to minute
	now := time.Now().UTC().Truncate(time.Minute)
	if cache.lastMinute.Sub(now).Abs() > time.Minute {
		t.Errorf("Expected lastMinute to be close to now, got %v", cache.lastMinute)
	}

	// Check all buckets are empty
	for i, bucket := range cache.buckets {
		if len(bucket) != 0 {
			t.Errorf("Expected bucket %d to be empty, got %d events", i, len(bucket))
		}
	}
}

func TestEventCacheAdd(t *testing.T) {
	// Create cache without the advance goroutine for predictable testing
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
	}

	tests := []struct {
		name        string
		eventTime   time.Time
		expectAdded bool
		expectIndex int
	}{
		{
			name:        "current minute event",
			eventTime:   baseTime,
			expectAdded: true,
			expectIndex: 0,
		},
		{
			name:        "5 minutes ago",
			eventTime:   baseTime.Add(-5 * time.Minute),
			expectAdded: true,
			expectIndex: (0 - 5 + CacheWindowMinutes) % CacheWindowMinutes, // 25
		},
		{
			name:        "29 minutes ago (oldest valid)",
			eventTime:   baseTime.Add(-29 * time.Minute),
			expectAdded: true,
			expectIndex: (0 - 29 + CacheWindowMinutes) % CacheWindowMinutes, // 1
		},
		{
			name:        "30 minutes ago (too old)",
			eventTime:   baseTime.Add(-30 * time.Minute),
			expectAdded: false,
			expectIndex: -1,
		},
		{
			name:        "future event (should go to current)",
			eventTime:   baseTime.Add(5 * time.Minute),
			expectAdded: true,
			expectIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear buckets
			for i := range cache.buckets {
				cache.buckets[i] = []models.Event{}
			}

			event := &models.Event{
				EventID:   "test-" + tt.name,
				Timestamp: tt.eventTime,
			}

			cache.Add(event)

			if tt.expectAdded {
				if len(cache.buckets[tt.expectIndex]) != 1 {
					t.Errorf("Expected 1 event in bucket %d, got %d", tt.expectIndex, len(cache.buckets[tt.expectIndex]))
				}
				if cache.buckets[tt.expectIndex][0].EventID != event.EventID {
					t.Errorf("Expected event ID %s, got %s", event.EventID, cache.buckets[tt.expectIndex][0].EventID)
				}
			} else {
				// Check no buckets have the event
				totalEvents := 0
				for _, bucket := range cache.buckets {
					totalEvents += len(bucket)
				}
				if totalEvents != 0 {
					t.Errorf("Expected no events to be added, but found %d events", totalEvents)
				}
			}
		})
	}
}

func TestEventCacheGetEventsSince(t *testing.T) {
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
	}

	// Add events at different times
	events := []*models.Event{
		{EventID: "event-now", Timestamp: baseTime},
		{EventID: "event-5min", Timestamp: baseTime.Add(-5 * time.Minute)},
		{EventID: "event-10min", Timestamp: baseTime.Add(-10 * time.Minute)},
		{EventID: "event-25min", Timestamp: baseTime.Add(-25 * time.Minute)},
	}

	for _, event := range events {
		cache.Add(event)
	}

	tests := []struct {
		name           string
		startMinutes   int64
		expectedEvents []string
	}{
		{
			name:           "get all events",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-30 * time.Minute)),
			expectedEvents: []string{"event-now", "event-5min", "event-10min", "event-25min"},
		},
		{
			name:           "get events from 10 minutes ago",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-10 * time.Minute)),
			expectedEvents: []string{"event-now", "event-5min", "event-10min"},
		},
		{
			name:           "get only recent events",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(-5 * time.Minute)),
			expectedEvents: []string{"event-now", "event-5min"},
		},
		{
			name:           "get only current events",
			startMinutes:   toMinutesSinceEpoch(baseTime),
			expectedEvents: []string{"event-now"},
		},
		{
			name:           "get future events (none)",
			startMinutes:   toMinutesSinceEpoch(baseTime.Add(5 * time.Minute)),
			expectedEvents: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.GetEventsSince(tt.startMinutes)

			if len(result) != len(tt.expectedEvents) {
				t.Errorf("Expected %d events, got %d", len(tt.expectedEvents), len(result))
			}

			// Create a map for easy lookup
			resultIDs := make(map[string]bool)
			for _, event := range result {
				resultIDs[event.EventID] = true
			}

			for _, expectedID := range tt.expectedEvents {
				if !resultIDs[expectedID] {
					t.Errorf("Expected event %s not found in results", expectedID)
				}
			}
		})
	}
}

func TestEventCacheCircularBuffer(t *testing.T) {
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
	}

	// Add event to current bucket
	event1 := &models.Event{EventID: "event1", Timestamp: baseTime}
	cache.Add(event1)

	// Verify it's in the right place
	if len(cache.buckets[0]) != 1 {
		t.Fatalf("Expected 1 event in bucket 0, got %d", len(cache.buckets[0]))
	}

	// Manually advance the cache (simulate time passing)
	cache.currentIndex = (cache.currentIndex + 1) % CacheWindowMinutes
	cache.buckets[cache.currentIndex] = []models.Event{}
	cache.lastMinute = cache.lastMinute.Add(time.Minute)

	// Add another event (should go to new current bucket)
	event2 := &models.Event{EventID: "event2", Timestamp: cache.lastMinute}
	cache.Add(event2)

	// Verify both events are retrievable
	events := cache.GetEventsSince(toMinutesSinceEpoch(baseTime.Add(-5 * time.Minute)))
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Verify the first event is now considered "1 minute old"
	events = cache.GetEventsSince(toMinutesSinceEpoch(cache.lastMinute))
	if len(events) != 1 {
		t.Errorf("Expected 1 recent event, got %d", len(events))
	}
	if events[0].EventID != "event2" {
		t.Errorf("Expected event2, got %s", events[0].EventID)
	}
}

func TestEventCacheAdvance(t *testing.T) {
	// This test is harder to test automatically due to the goroutine
	// but we can test the manual advance logic
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   baseTime,
	}

	// Add event to current bucket
	event := &models.Event{EventID: "test", Timestamp: baseTime}
	cache.Add(event)

	originalIndex := cache.currentIndex
	originalTime := cache.lastMinute

	// Manually trigger what advance() does
	cache.mu.Lock()
	cache.currentIndex = (cache.currentIndex + 1) % CacheWindowMinutes
	cache.buckets[cache.currentIndex] = []models.Event{}
	cache.lastMinute = cache.lastMinute.Add(time.Minute)
	cache.mu.Unlock()

	// Verify state changed
	if cache.currentIndex != (originalIndex+1)%CacheWindowMinutes {
		t.Errorf("Expected currentIndex to advance to %d, got %d", (originalIndex+1)%CacheWindowMinutes, cache.currentIndex)
	}
	if !cache.lastMinute.Equal(originalTime.Add(time.Minute)) {
		t.Errorf("Expected lastMinute to advance by 1 minute, got %v", cache.lastMinute)
	}
	if len(cache.buckets[cache.currentIndex]) != 0 {
		t.Errorf("Expected current bucket to be empty after advance, got %d events", len(cache.buckets[cache.currentIndex]))
	}
}

func TestToMinutesSinceEpoch(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected int64
	}{
		{
			name:     "unix epoch",
			time:     time.Unix(0, 0).UTC(),
			expected: 0,
		},
		{
			name:     "one hour later",
			time:     time.Unix(3600, 0).UTC(),
			expected: 60,
		},
		{
			name:     "with seconds and milliseconds",
			time:     time.Unix(3665, 123000000).UTC(), // 1 hour, 1 minute, 5 seconds, 123ms
			expected: 61,                               // Should truncate to 61 minutes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toMinutesSinceEpoch(tt.time)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestFromMinutesSinceEpoch(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int64
		expected time.Time
	}{
		{
			name:     "zero minutes",
			minutes:  0,
			expected: time.Unix(0, 0).UTC(),
		},
		{
			name:     "60 minutes",
			minutes:  60,
			expected: time.Unix(3600, 0).UTC(),
		},
		{
			name:     "arbitrary time",
			minutes:  100,
			expected: time.Unix(6000, 0).UTC(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromMinutesSinceEpoch(tt.minutes)
			if !result.Equal(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
