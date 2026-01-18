package apps

import (
	"analytics/models"
	"log"
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
	done         chan struct{}
	stopOnce     sync.Once
}

// NewEventCache creates a new EventCache with advance routine.
// lastMinute is set to current minute + 1 to provide a bucket for events
// with minor clock skew (up to 1 minute ahead of server time).
func NewEventCache() *EventCache {
	now := time.Now().UTC().Truncate(time.Minute)
	cache := &EventCache{
		buckets:      [CacheWindowMinutes][]models.Event{},
		currentIndex: 0,
		lastMinute:   now.Add(time.Minute),
		done:         make(chan struct{}),
	}
	go cache.advance()
	return cache
}

// Add adds an event to the appropriate minute bucket.
// Cache time is controlled ONLY by the advance() goroutine (server time).
// Event timestamps NEVER advance the cache.
// lastMinute is 1 minute ahead of real time, so events with minor clock skew
// naturally fall within the window without special handling.
func (c *EventCache) Add(event *models.Event) {
	if event == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	eventTime := event.Timestamp.UTC().Truncate(time.Minute)
	oldestAllowed := c.lastMinute.Add(-CacheWindowMinutes * time.Minute)

	// Discard events outside the cache window
	if eventTime.Before(oldestAllowed) {
		log.Printf("EventCache.Add: Event too old, discarding. eventTime=%s, oldestAllowed=%s, lastMinute=%s",
			eventTime.Format(time.RFC3339), oldestAllowed.Format(time.RFC3339), c.lastMinute.Format(time.RFC3339))
		return
	}
	if eventTime.After(c.lastMinute) {
		log.Printf("EventCache.Add: Event too far in future, discarding. eventTime=%s, lastMinute=%s",
			eventTime.Format(time.RFC3339), c.lastMinute.Format(time.RFC3339))
		return
	}

	diffMinutes := int(c.lastMinute.Sub(eventTime) / time.Minute)
	index := (c.currentIndex - diffMinutes + CacheWindowMinutes) % CacheWindowMinutes
	c.buckets[index] = append(c.buckets[index], *event)
	log.Printf("EventCache.Add: Added event to bucket %d (appID=%s, eventTime=%s, lastMinute=%s, currentIndex=%d)",
		index, event.AppID, eventTime.Format(time.RFC3339), c.lastMinute.Format(time.RFC3339), c.currentIndex)
}

func (c *EventCache) GetEventsSince(startMinutes int64) []models.Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Initialize as empty slice (not nil) so JSON encodes as [] not null
	events := make([]models.Event, 0)
	cacheLastMinutes := toMinutesSinceEpoch(c.lastMinute)

	// Count total events in all buckets for debugging
	totalEventsInCache := 0
	for i := 0; i < CacheWindowMinutes; i++ {
		totalEventsInCache += len(c.buckets[i])
	}
	log.Printf("GetEventsSince: cacheLastMinutes=%d, startMinutes=%d, currentIndex=%d, totalEventsInCache=%d",
		cacheLastMinutes, startMinutes, c.currentIndex, totalEventsInCache)

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
// It aligns to clock minute boundaries to avoid initial gap issues.
func (c *EventCache) advance() {
	// Wait until the next minute boundary before starting
	now := time.Now().UTC()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	waitDuration := nextMinute.Sub(now)

	select {
	case <-c.done:
		return
	case <-time.After(waitDuration):
	}

	advanceEx := func() {
		// Advance once after wait to sync lastMinute to current minute + 1
		now = time.Now().UTC().Truncate(time.Minute)
		target := now.Add(time.Minute)
		c.mu.Lock()
		for c.lastMinute.Before(target) {
			c.currentIndex = (c.currentIndex + 1) % CacheWindowMinutes
			c.buckets[c.currentIndex] = []models.Event{}
			c.lastMinute = c.lastMinute.Add(time.Minute)
		}
		c.mu.Unlock()
	}
	
	advanceEx()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			advanceEx()
		}
	}
}

// Stop signals the advance goroutine to exit. Safe to call multiple times.
func (c *EventCache) Stop() {
	c.stopOnce.Do(func() {
		close(c.done)
	})
}

func toMinutesSinceEpoch(t time.Time) int64 {
	return t.Unix() / 60
}

// Helper function to convert minutes since epoch to time
func fromMinutesSinceEpoch(minutes int64) time.Time {
	return time.Unix(minutes*60, 0).UTC()
}
