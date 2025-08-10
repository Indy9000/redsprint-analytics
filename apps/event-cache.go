package apps

import (
	"analytics/models"
	"sync"
	"time"
)

// EventCache stores events in a 30-minute circular buffer, with each bucket for one minute.
type EventCache struct {
	buckets      [30][]models.Event
	currentIndex int
	lastMinute   time.Time
	mu           sync.RWMutex
}

// NewEventCache creates a new EventCache with advance routine.
func NewEventCache() *EventCache {
	now := time.Now().UTC().Truncate(time.Minute)
	cache := &EventCache{
		buckets:      [30][]models.Event{},
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
	if eventTime.Before(c.lastMinute.Add(-29 * time.Minute)) {
		// Too old, discard
		return
	}
	if eventTime.After(c.lastMinute) {
		// Future event, add to current bucket
		eventTime = c.lastMinute
	}

	diffMinutes := int(c.lastMinute.Sub(eventTime) / time.Minute)
	index := (c.currentIndex - diffMinutes + 30) % 30
	c.buckets[index] = append(c.buckets[index], *event)
}

// advance shifts the buffer every minute to evict old data.
func (c *EventCache) advance() {
	for range time.Tick(time.Minute) {
		c.mu.Lock()
		c.currentIndex = (c.currentIndex + 1) % 30
		c.buckets[c.currentIndex] = []models.Event{}
		c.lastMinute = c.lastMinute.Add(time.Minute)
		c.mu.Unlock()
	}
}
