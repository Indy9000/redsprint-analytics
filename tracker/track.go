package tracker

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"analytics/apps"
	"analytics/models"

	"github.com/google/uuid"
)

type EventTracker struct {
	appMgr *apps.Manager
}

func NewEventTracker(appMgr *apps.Manager) *EventTracker {
	return &EventTracker{
		appMgr: appMgr,
	}
}

func (h *EventTracker) PostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)
		log.Printf("PostHandler: %s %s from %s", r.Method, r.URL.Path, clientIP)

		if r.Method != http.MethodPost {
			log.Printf("PostHandler: method not allowed: %s from %s", r.Method, clientIP)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			log.Printf("PostHandler: missing API key from %s", clientIP)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		app, err := h.appMgr.GetAppByAPIKey(apiKey)
		if err != nil {
			log.Printf("PostHandler: invalid API key (%s) from %s", apiKey, clientIP)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var event models.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			log.Printf("PostHandler: failed to decode event from %s (app=%s): %v", clientIP, app.ID, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		h.enrichEvent(&event, app.ID, r)

		log.Printf("PostHandler: event received (app=%s, eventID=%s, type=%s, name=%s, userID=%s, sessionID=%s)",
			app.ID, event.EventID, event.EventType, event.EventName, event.User.ID, event.User.SessionID)

		h.appMgr.AddEvent(&event)
		if err := h.saveEvent(&event, app.ID); err != nil {
			log.Printf("PostHandler: failed to save event (app=%s, eventID=%s): %v", app.ID, event.EventID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// log.Printf("PostHandler: event saved (app=%s, eventID=%s)", app.ID, event.EventID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"event_id": event.EventID,
		})
	}
}

// saveEvent saves an event as a JSON file in data/<app-id>/<utc-date-YYYYMMDD>/<eventid>.json.
func (h *EventTracker) saveEvent(event *models.Event, appID string) error {
	dateStr := time.Now().UTC().Format("20060102")
	filePath := filepath.Join("data", appID, dateStr, fmt.Sprintf("%s.json", event.EventID))
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directories for %s: %w", filePath, err)
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if err := os.WriteFile(filePath, eventJSON, 0644); err != nil {
		return fmt.Errorf("save event to %s: %w", filePath, err)
	}
	return nil
}

const (
	// MaxTimestampDrift is the maximum allowed difference between event timestamp and server time
	// Events with timestamps outside this range will be corrected to server time
	MaxTimestampDrift = 5 * time.Minute
)

func (h *EventTracker) enrichEvent(event *models.Event, appID string, r *http.Request) {
	// Set app ID
	event.AppID = appID

	// Generate event ID if missing
	if event.EventID == "" {
		event.EventID = uuid.Must(uuid.NewV7()).String()
	}

	// Set or validate timestamp
	serverNow := time.Now().UTC()
	if event.Timestamp.IsZero() {
		event.Timestamp = serverNow
	} else {
		// Check for unreasonable timestamps and correct them
		drift := event.Timestamp.Sub(serverNow)
		if drift > MaxTimestampDrift {
			log.Printf("enrichEvent: Event timestamp too far in future (drift=%v), correcting to server time. Original=%s, Corrected=%s",
				drift, event.Timestamp.Format(time.RFC3339), serverNow.Format(time.RFC3339))
			event.Timestamp = serverNow
		} else if drift < -MaxTimestampDrift {
			log.Printf("enrichEvent: Event timestamp too far in past (drift=%v), correcting to server time. Original=%s, Corrected=%s",
				drift, event.Timestamp.Format(time.RFC3339), serverNow.Format(time.RFC3339))
			event.Timestamp = serverNow
		}
	}

	// Extract client IP
	clientIP := extractClientIP(r)

	// Set location if not provided
	if event.Location == nil {
		event.Location = &models.LocationInfo{
			IP: clientIP,
			// TODO: Add GeoIP lookup
		}
	}

	// For web events, capture user agent
	if event.Device.Platform == "web" && event.Web != nil {
		if event.Web.UserAgent == "" {
			event.Web.UserAgent = r.UserAgent()
		}
		if event.Web.Referrer == "" {
			event.Web.Referrer = r.Referer()
		}
	}
}

func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
