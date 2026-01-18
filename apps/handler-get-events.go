package apps

import (
	"analytics/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) GetEventsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("GetEventsHandler: Received %s request for %s", r.Method, r.URL.Path)

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		appID, startMinutes, err := m.parseRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		events, err := m.getEvents(appID, startMinutes)
		if err != nil {
			log.Printf("GetEventsHandler: Error getting events: %v", err)
			http.Error(w, "Failed to get events", http.StatusInternalServerError)
			return
		}

		m.sendResponse(w, events, appID)
	}
}

func (m *Manager) parseRequest(r *http.Request) (string, int64, error) {
	appID, err := extractAppID(r.URL.Path)
	if err != nil {
		return "", 0, fmt.Errorf("invalid app ID")
	}

	// Default: last 30 minutes
	startMinutes := toMinutesSinceEpoch(time.Now().UTC()) - CacheWindowMinutes

	if startStr := r.URL.Query().Get("start-minutes-since-epoch"); startStr != "" {
		parsed, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("invalid start-minutes-since-epoch format")
		}
		startMinutes = parsed
		// log.Printf("GetEventsHandler: Parsed start minutes: %d (%s)",
		// 	startMinutes, fromMinutesSinceEpoch(startMinutes).Format(time.RFC3339))
	}

	return appID, startMinutes, nil
}

func (m *Manager) getEvents(appID string, startMinutes int64) ([]models.Event, error) {
	// log.Printf("GetEventsHandler: Looking for events since minute %d (%s)",
	// 	startMinutes, fromMinutesSinceEpoch(startMinutes).Format(time.RFC3339))

	// Try cache first
	if events, found := m.getEventsFromCache(appID, startMinutes); found {
		return events, nil
	}

	// Fallback to disk
	return m.getEventsFromDisk(appID, startMinutes)
}

func (m *Manager) getEventsFromCache(appID string, startMinutes int64) ([]models.Event, bool) {
	m.cachesMu.RLock()
	cache, exists := m.caches[appID]
	m.cachesMu.RUnlock()

	if !exists {
		return nil, false
	}

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	// Check if startMinutes is within cache window using cache's lastMinute
	cacheLastMinutes := toMinutesSinceEpoch(cache.lastMinute)
	cacheWindowStart := cacheLastMinutes - (CacheWindowMinutes - 1)

	// Cache miss if request is older than cache window
	if startMinutes < cacheWindowStart {
		return nil, false
	}

	// Cache miss if request is newer than cache's lastMinute
	// This can happen if the advance() goroutine falls behind real time
	if startMinutes > cacheLastMinutes {
		log.Printf("GetEventsHandler: Cache miss - startMinutes (%d) > cacheLastMinutes (%d), falling back to disk",
			startMinutes, cacheLastMinutes)
		return nil, false
	}

	// Let the cache handle its own iteration logic
	events := cache.GetEventsSince(startMinutes)

	log.Printf("GetEventsHandler: Retrieved %d events from cache", len(events))
	return events, true
}

func (m *Manager) getEventsFromDisk(appID string, startMinutes int64) ([]models.Event, error) {
	startTime := fromMinutesSinceEpoch(startMinutes)
	// log.Printf("GetEventsHandler: Scanning disk for events since minute %d (%s)",
	// 	startMinutes, startTime.Format(time.RFC3339))

	// Initialize as empty slice (not nil) so JSON encodes as [] not null
	events := make([]models.Event, 0)
	startDate := startTime.Truncate(24 * time.Hour)
	endDate := time.Now().UTC().Truncate(24 * time.Hour)

	for date := startDate; !date.After(endDate); date = date.Add(24 * time.Hour) {
		dayEvents, err := m.getEventsFromDay(appID, date, startMinutes)
		if err != nil {
			return nil, err
		}
		events = append(events, dayEvents...)
	}

	// log.Printf("GetEventsHandler: Retrieved %d events from disk", len(events))
	return events, nil
}

func (m *Manager) getEventsFromDay(appID string, date time.Time, startMinutes int64) ([]models.Event, error) {
	dir := filepath.Join("data", appID, date.Format("20060102"))
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		// log.Printf("GetEventsHandler: Directory %s does not exist", dir)
		return []models.Event{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %v", dir, err)
	}

	// log.Printf("GetEventsHandler: Found %d files in %s", len(files), dir)

	if len(files) > 10000 {
		return nil, fmt.Errorf("too many files in %s, please narrow time range", dir)
	}

	// Initialize as empty slice (not nil) so JSON encodes as [] not null
	events := make([]models.Event, 0)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		event, err := m.readEventFile(filepath.Join(dir, file.Name()))
		if err != nil {
			log.Printf("GetEventsHandler: Failed to read event %s: %v", file.Name(), err)
			continue
		}

		eventMinutes := toMinutesSinceEpoch(event.Timestamp)
		if eventMinutes >= startMinutes {
			events = append(events, event)
		}
	}

	return events, nil
}

func (m *Manager) readEventFile(filePath string) (models.Event, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return models.Event{}, err
	}

	var event models.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return models.Event{}, err
	}

	return event, nil
}

func (m *Manager) sendResponse(w http.ResponseWriter, events []models.Event, appID string) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		log.Printf("GetEventsHandler: Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	// log.Printf("GetEventsHandler: Returned %d events for app %s", len(events), appID)
}

// func (m *Manager) GetEventsHandler() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		log.Printf("GetEventsHandler: Received %s request for %s", r.Method, r.URL.Path)

// 		if r.Method != http.MethodGet {
// 			log.Printf("GetEventsHandler: Method %s not allowed", r.Method)
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		appID, err := extractAppID(r.URL.Path)
// 		if err != nil {
// 			log.Println("GetEventsHandler: Invalid app ID")
// 			http.Error(w, "Invalid app ID", http.StatusBadRequest)
// 			return
// 		}

// 		// Get Firebase user ID from context (set by FirebaseAuthMiddleware)
// 		// userID, ok := r.Context().Value("user_id").(string)
// 		// if !ok {
// 		// 	log.Println("GetEventsHandler: Missing user ID in context")
// 		// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		// 	return
// 		// }

// 		// Check app existence and ownership
// 		// m.dataMu.RLock()
// 		// app, exists := m.data.Apps[appID]
// 		// if !exists {
// 		// 	m.dataMu.RUnlock()
// 		// 	log.Printf("GetEventsHandler: App ID %s not found", appID)
// 		// 	http.Error(w, "App not found", http.StatusNotFound)
// 		// 	return
// 		// }
// 		// if app.OwnerID != userID {
// 		// 	m.dataMu.RUnlock()
// 		// 	log.Printf("GetEventsHandler: User %s not authorized for app %s", userID, appID)
// 		// 	http.Error(w, "Forbidden", http.StatusForbidden)
// 		// 	return
// 		// }
// 		// m.dataMu.RUnlock()

// 		// Parse start-utc-datetime
// 		startTime := time.Now().UTC().Add(-30 * time.Minute) // Default: last 30 minutes
// 		if startStr := r.URL.Query().Get("start-minutes-since-epoch"); startStr != "" {
// 			startMinutes, err := strconv.ParseInt(startStr, 10, 64)
// 			if err != nil {
// 				log.Printf("GetEventsHandler: Invalid start-minutes-since-epoch: %v", err)
// 				http.Error(w, "Invalid start-minutes-since-epoch format", http.StatusBadRequest)
// 				return
// 			}
// 			startTime = time.Unix(startMinutes*60, 0).UTC()
// 			log.Printf("GetEventsHandler: Parsed start time: %s (from minutes: %d)", startTime.Format(time.RFC3339), startMinutes)
// 		}

// 		log.Printf("GetEventsHandler: Looking for events since %s", startTime.Format(time.RFC3339))

// 		// Collect events
// 		var events []models.Event
// 		cacheHit := false

// 		// Check cache - if startTime is recent enough to be in cache
// 		thirtyMinutesAgo := time.Now().UTC().Add(-30 * time.Minute)
// 		m.cachesMu.RLock()
// 		cache, exists := m.caches[appID]
// 		if exists && !startTime.Before(thirtyMinutesAgo) { // FIX: Logic was inverted
// 			cache.mu.RLock()
// 			for i := range 30 {
// 				// FIX: Bucket time calculation - oldest bucket is at index 0
// 				bucketTime := cache.lastMinute.Add(-time.Duration(29-i) * time.Minute)
// 				log.Printf("GetEventsHandler: Checking bucket %d at time %s", i, bucketTime.Format(time.RFC3339))

// 				for _, event := range cache.buckets[i] {
// 					if !event.Timestamp.Before(startTime) {
// 						events = append(events, event)
// 					}
// 				}
// 			}
// 			cache.mu.RUnlock()
// 			cacheHit = true
// 			log.Printf("GetEventsHandler: Retrieved %d events from cache", len(events))
// 		}
// 		m.cachesMu.RUnlock()

// 		// If cache miss or startTime is older than cache window, scan disk
// 		if !cacheHit || startTime.Before(thirtyMinutesAgo) {
// 			log.Printf("GetEventsHandler: Scanning disk for events since %s", startTime.Format(time.RFC3339))

// 			startDate := startTime.UTC().Truncate(24 * time.Hour)
// 			endDate := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)

// 			for date := startDate; !date.After(endDate); date = date.Add(24 * time.Hour) {
// 				dir := filepath.Join("data", appID, date.Format("20060102"))
// 				files, err := os.ReadDir(dir)
// 				if os.IsNotExist(err) {
// 					log.Printf("GetEventsHandler: Directory %s does not exist", dir)
// 					continue
// 				}
// 				if err != nil {
// 					log.Printf("GetEventsHandler: Failed to read directory %s: %v", dir, err)
// 					http.Error(w, "Failed to read events", http.StatusInternalServerError)
// 					return
// 				}

// 				log.Printf("GetEventsHandler: Found %d files in %s", len(files), dir)

// 				// Limit to prevent overload
// 				if len(files) > 10000 { // Arbitrary threshold
// 					log.Printf("GetEventsHandler: Too many files in %s", dir)
// 					http.Error(w, "Too many events, please narrow time range", http.StatusTooManyRequests)
// 					return
// 				}

// 				for _, file := range files {
// 					if !strings.HasSuffix(file.Name(), ".json") {
// 						continue
// 					}
// 					data, err := os.ReadFile(filepath.Join(dir, file.Name()))
// 					if err != nil {
// 						log.Printf("GetEventsHandler: Failed to read file %s: %v", file.Name(), err)
// 						continue
// 					}
// 					var event models.Event
// 					if err := json.Unmarshal(data, &event); err != nil {
// 						log.Printf("GetEventsHandler: Failed to parse event %s: %v", file.Name(), err)
// 						continue
// 					}
// 					if !event.Timestamp.Before(startTime) {
// 						events = append(events, event)
// 					}
// 				}
// 			}
// 			log.Printf("GetEventsHandler: Retrieved %d events from disk", len(events))
// 		}

// 		w.Header().Set("Content-Type", "application/json")
// 		if err := json.NewEncoder(w).Encode(events); err != nil {
// 			log.Printf("GetEventsHandler: Failed to encode response: %v", err)
// 			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 			return
// 		}
// 		log.Printf("GetEventsHandler: Returned %d events for app %s", len(events), appID)
// 	}
// }

// extractAppID extracts the app ID from a URL path like /analytics/api/v1/apps/<appID>/events.
func extractAppID(path string) (string, error) {
	const prefix = "/analytics/api/v1/apps/"
	if !strings.HasPrefix(path, prefix) {
		log.Printf("extractAppID: Invalid path %s, expected prefix %s", path, prefix)
		return "", fmt.Errorf("invalid path")
	}

	// Trim prefix and take part before next /
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		log.Println("extractAppID: Missing app ID")
		return "", fmt.Errorf("missing app ID")
	}

	// Split on first / to get appID
	appID := strings.SplitN(rest, "/", 2)[0]
	if appID == "" {
		log.Println("extractAppID: Empty app ID")
		return "", fmt.Errorf("empty app ID")
	}

	return appID, nil
}
