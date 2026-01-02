package apps

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseRequest(t *testing.T) {
	manager := &Manager{}

	tests := []struct {
		name          string
		path          string
		queryParams   map[string]string
		expectAppID   string
		expectMinutes int64
		expectError   bool
	}{
		{
			name:          "valid path no query params",
			path:          "/analytics/api/v1/apps/test123/events",
			queryParams:   map[string]string{},
			expectAppID:   "test123",
			expectMinutes: toMinutesSinceEpoch(time.Now().UTC()) - CacheWindowMinutes,
			expectError:   false,
		},
		{
			name:          "valid path with start time",
			path:          "/analytics/api/v1/apps/abc456/events",
			queryParams:   map[string]string{"start-minutes-since-epoch": "1000000"},
			expectAppID:   "abc456",
			expectMinutes: 1000000,
			expectError:   false,
		},
		{
			name:        "invalid app ID path",
			path:        "/invalid/path",
			queryParams: map[string]string{},
			expectError: true,
		},
		{
			name:        "invalid start time format",
			path:        "/analytics/api/v1/apps/test123/events",
			queryParams: map[string]string{"start-minutes-since-epoch": "invalid"},
			expectError: true,
		},
		{
			name:          "negative start time",
			path:          "/analytics/api/v1/apps/test123/events",
			queryParams:   map[string]string{"start-minutes-since-epoch": "-100"},
			expectAppID:   "test123",
			expectMinutes: -100,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock request
			req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)

			// Add query parameters
			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			appID, startMinutes, err := manager.parseRequest(req)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if appID != tt.expectAppID {
				t.Errorf("Expected appID %s, got %s", tt.expectAppID, appID)
			}

			// For default time, allow some tolerance (test execution time)
			if tt.queryParams["start-minutes-since-epoch"] == "" {
				expectedDefault := toMinutesSinceEpoch(time.Now().UTC()) - CacheWindowMinutes
				if startMinutes < expectedDefault-1 || startMinutes > expectedDefault+1 {
					t.Errorf("Expected startMinutes around %d, got %d", expectedDefault, startMinutes)
				}
			} else {
				if startMinutes != tt.expectMinutes {
					t.Errorf("Expected startMinutes %d, got %d", tt.expectMinutes, startMinutes)
				}
			}
		})
	}
}
