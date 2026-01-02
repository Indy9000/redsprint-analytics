package apps

import (
	"analytics/models"
	"sync"
	"time"
)

const (
	CacheWindowMinutes = 30
)

// EventCache stores events in a m-minute circular buffer, with each bucket for one minute.
type EventCache struct {
	buckets      [CacheWindowMinutes][]models.Event
	currentIndex int
	lastMinute   time.Time
	mu           sync.RWMutex
}

// NewEventCache creates a new EventCache with advance routine.
func NewEventCache() *EventCache {
	now := time.Now().UTC().Truncate(time.Minute)
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   now,
	}
	go cache.advance()
	return cache
}

// Add adds an event to the appropriate minute bucket.
func (c *EventCache) Add(event *models.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	eventTime := event.Timestamp.UTC().Truncate(time.Minute)
	if eventTime.Before(c.lastMinute.Add(-(CacheWindowMinutes - 1) * time.Minute)) {
		// Too old, discard
		return
	}
	if eventTime.After(c.lastMinute) {
		// Future event, add to current bucket
		eventTime = c.lastMinute
	}

	diffMinutes := int(c.lastMinute.Sub(eventTime) / time.Minute)
	index := (c.currentIndex - diffMinutes + CacheWindowMinutes) % CacheWindowMinutes
	c.buckets[index] = append(c.buckets[index], *event)
}

func (c *EventCache) GetEventsSince(startMinutes int64) []models.Event {
	var events []models.Event
	cacheLastMinutes := toMinutesSinceEpoch(c.lastMinute)

	// Iterate through all buckets in the circular buffer
	for i := 0; i < CacheWindowMinutes; i++ {
		// Calculate the actual bucket index in the circular buffer
		bucketIndex := (c.currentIndex - i + CacheWindowMinutes) % CacheWindowMinutes
		// Calculate the minute this bucket represents
		bucketMinutes := cacheLastMinutes - int64(i)

		// Skip buckets that are before our startMinutes
		if bucketMinutes < startMinutes {
			continue
		}

		for _, event := range c.buckets[bucketIndex] {
			eventMinutes := toMinutesSinceEpoch(event.Timestamp)
			if eventMinutes >= startMinutes {
				events = append(events, event)
			}
		}
	}

	return events
}

// advance shifts the buffer every minute to evict old data.
func (c *EventCache) advance() {
	for range time.Tick(time.Minute) {
		c.mu.Lock()
		c.currentIndex = (c.currentIndex + 1) % CacheWindowMinutes
		c.buckets[c.currentIndex] = []models.Event{}
		c.lastMinute = c.lastMinute.Add(time.Minute)
		c.mu.Unlock()
	}
}

func toMinutesSinceEpoch(t time.Time) int64 {
	return t.Unix() / 60
}

// Helper function to convert minutes since epoch to time
func fromMinutesSinceEpoch(minutes int64) time.Time {
	return time.Unix(minutes*60, 0).UTC()
}
