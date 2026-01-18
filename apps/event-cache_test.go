package apps

import (
	"analytics/models"
	"testing"
	"time"
)

func TestNewEventCache(t *testing.T) {
	cache := NewEventCache()
	defer cache.Stop()

	if cache.currentIndex != 0 {
		t.Errorf("Expected currentIndex to be 0, got %d", cache.currentIndex)
	}

	// Check that lastMinute is 1 minute ahead of now (truncated to minute)
	now := time.Now().UTC().Truncate(time.Minute)
	expected := now.Add(time.Minute)
	if cache.lastMinute.Sub(expected).Abs() > time.Minute {
		t.Errorf("Expected lastMinute to be ~1 minute ahead of now, got %v (expected ~%v)", cache.lastMinute, expected)
	}

	// Check all buckets are empty
	for i, bucket := range cache.buckets {
		if len(bucket) != 0 {
			t.Errorf("Expected bucket %d to be empty, got %d events", i, len(bucket))
		}
	}
}

func TestEventCacheAdd(t *testing.T) {
	// baseTime represents "now" (real server time)
	// lastMinute is set 1 minute ahead to provide buffer for clock skew
	baseTime := time.Date(2025, 8, 24, 12, 0, 0, 0, time.UTC)
	lastMinute := baseTime.Add(time.Minute) // 12:01

	tests := []struct {
		name                 string
		eventTime            time.Time
		expectAdded          bool
		expectIndex          int
		expectedCurrentIndex int
	}{
		{
			name:                 "current minute event",
			eventTime:            baseTime, // 12:00, diffMinutes=1
			expectAdded:          true,
			expectIndex:          (0 - 1 + CacheWindowMinutes) % CacheWindowMinutes, // 29
			expectedCurrentIndex: 0,
		},
		{
			name:                 "5 minutes ago",
			eventTime:            baseTime.Add(-5 * time.Minute), // 11:55, diffMinutes=6
			expectAdded:          true,
			expectIndex:          (0 - 6 + CacheWindowMinutes) % CacheWindowMinutes, // 24
			expectedCurrentIndex: 0,
		},
		{
			name:                 "30 minutes ago (oldest valid)",
			eventTime:            baseTime.Add(-29 * time.Minute), // 11:31, diffMinutes=30
			expectAdded:          true,
			expectIndex:          (0 - 30 + CacheWindowMinutes) % CacheWindowMinutes, // 0
			expectedCurrentIndex: 0,
		},
		{
			name:                 "31 minutes ago (too old)",
			eventTime:            baseTime.Add(-30 * time.Minute), // 11:30, before oldestAllowed (11:31)
			expectAdded:          false,
			expectIndex:          -1,
			expectedCurrentIndex: 0,
		},
		{
			name:                 "future event under 1 min (minor clock skew - accept)",
			eventTime:            baseTime.Add(30 * time.Second), // 12:00:30 truncates to 12:00
			expectAdded:          true,
			expectIndex:          (0 - 1 + CacheWindowMinutes) % CacheWindowMinutes, // 29
			expectedCurrentIndex: 0,
		},
		{
			name:                 "future event at lastMinute (accept)",
			eventTime:            lastMinute, // 12:01, exactly at lastMinute
			expectAdded:          true,
			expectIndex:          0, // diffMinutes=0
			expectedCurrentIndex: 0,
		},
		{
			name:                 "future event beyond lastMinute (reject)",
			eventTime:            lastMinute.Add(1 * time.Minute), // 12:02, after lastMinute
			expectAdded:          false,
			expectIndex:          -1,
			expectedCurrentIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh cache for each test case
			// lastMinute is 1 minute ahead of "now" (baseTime)
			cache := &EventCache{
				buckets:      [CacheWindowMinutes][]models.Event{},
				currentIndex: 0,
				lastMinute:   lastMinute,
			}

			event := &models.Event{
				EventID:   "test-" + tt.name,
				Timestamp: tt.eventTime,
			}

			cache.Add(event)

			// Verify cache state
			if cache.currentIndex != tt.expectedCurrentIndex {
				t.Errorf("Expected currentIndex %d, got %d", tt.expectedCurrentIndex, cache.currentIndex)
			}

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
	lastMinute := baseTime.Add(time.Minute) // 12:01
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   lastMinute,
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
	lastMinute := baseTime.Add(time.Minute) // 12:01
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   lastMinute,
	}

	// Add event at baseTime (12:00) - goes to bucket 29 (diffMinutes=1)
	event1 := &models.Event{EventID: "event1", Timestamp: baseTime}
	cache.Add(event1)

	// Verify it's in the right place (bucket 29)
	if len(cache.buckets[29]) != 1 {
		t.Fatalf("Expected 1 event in bucket 29, got %d", len(cache.buckets[29]))
	}

	// Manually advance the cache (simulate time passing)
	cache.currentIndex = (cache.currentIndex + 1) % CacheWindowMinutes
	cache.buckets[cache.currentIndex] = []models.Event{}
	cache.lastMinute = cache.lastMinute.Add(time.Minute) // now 12:02

	// Add another event at new lastMinute (12:02) - goes to bucket 1 (diffMinutes=0)
	event2 := &models.Event{EventID: "event2", Timestamp: cache.lastMinute}
	cache.Add(event2)

	// Verify both events are retrievable
	events := cache.GetEventsSince(toMinutesSinceEpoch(baseTime.Add(-5 * time.Minute)))
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Verify event2 is retrievable when querying from its timestamp
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
	lastMinute := baseTime.Add(time.Minute) // 12:01
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   lastMinute,
	}

	// Add event at baseTime (12:00) - goes to bucket 29
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

func TestEventCacheRealTime(t *testing.T) {
	// Test 1: Create cache and add event immediately (same second)
	t.Run("add event immediately after cache creation", func(t *testing.T) {
		cache := NewEventCache()
		defer cache.Stop()

		now := time.Now().UTC()
		event := &models.Event{
			EventID:   "immediate-event",
			Timestamp: now,
		}

		cache.Add(event)

		// Count total events
		totalEvents := 0
		for _, bucket := range cache.buckets {
			totalEvents += len(bucket)
		}

		if totalEvents != 1 {
			t.Errorf("Expected 1 event in cache, got %d", totalEvents)
		}

		// Event should be in bucket at index currentIndex - 1 (since lastMinute is 1 ahead)
		// With currentIndex = 0, that's bucket 29
		expectedBucket := (cache.currentIndex - 1 + CacheWindowMinutes) % CacheWindowMinutes
		if len(cache.buckets[expectedBucket]) != 1 {
			t.Errorf("Expected event in bucket %d, but found %d events there", expectedBucket, len(cache.buckets[expectedBucket]))
		}
	})

	// Test 2: Stale cache (lastMinute = yesterday) rejects current events
	t.Run("stale cache rejects current events", func(t *testing.T) {
		yesterday := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Minute)
		cache := &EventCache{
			buckets:      [CacheWindowMinutes][]models.Event{},
			currentIndex: 0,
			lastMinute:   yesterday,
		}

		now := time.Now().UTC()
		event := &models.Event{
			EventID:   "current-event",
			Timestamp: now,
		}

		cache.Add(event)

		// Event should be REJECTED - it's far in the future relative to stale lastMinute
		totalEvents := 0
		for _, bucket := range cache.buckets {
			totalEvents += len(bucket)
		}

		if totalEvents != 0 {
			t.Errorf("Expected 0 events (stale cache should reject future events), got %d", totalEvents)
		}
	})

	// Test 3: Event with timestamp 2 minutes in future is rejected
	t.Run("event 2 minutes in future is rejected", func(t *testing.T) {
		cache := NewEventCache()
		defer cache.Stop()

		futureTime := time.Now().UTC().Add(2 * time.Minute)
		event := &models.Event{
			EventID:   "future-event",
			Timestamp: futureTime,
		}

		cache.Add(event)

		// Event should be REJECTED - beyond lastMinute (which is only 1 min ahead)
		totalEvents := 0
		for _, bucket := range cache.buckets {
			totalEvents += len(bucket)
		}

		if totalEvents != 0 {
			t.Errorf("Expected 0 events (future event should be rejected), got %d", totalEvents)
		}
	})
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
