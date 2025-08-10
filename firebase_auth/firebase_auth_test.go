package firebase_auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cache "github.com/patrickmn/go-cache"
)

// Mock handler for testing middleware
func mockHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

// TestFirebaseAuthMiddleware tests token extraction and cache behavior
func TestFirebaseAuthMiddleware(t *testing.T) {
	// cfg := FirebaseConfig{
	// 	ProjectID: "thoughtflow-8744c",
	// }

	tests := []struct {
		name           string
		authHeader     string
		cacheSetup     func(string) // Pass tokenStr to cacheSetup
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Missing Authorization Header",
			authHeader:     "",
			cacheSetup:     func(tokenStr string) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error": "Missing or invalid Authorization header"}`,
		},
		{
			name:           "Invalid Bearer Token (Empty)",
			authHeader:     "Bearer ",
			cacheSetup:     func(tokenStr string) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error": "Missing or invalid Authorization header"}`,
		},
		{
			name:       "Cache Miss (Valid Token Format)",
			authHeader: "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6Im1vY2sta2lkIn0.eyJpc3MiOiJodHRwczovL3NlY3VyZXRva2VuLmdvb2dsZS5jb20vdGhvdWdodGZs",
			cacheSetup: func(tokenStr string) {
				cacheMutex.Lock()
				tokenCache.Flush() // Ensure cache is empty
				cacheMutex.Unlock()
			},
			expectedStatus: http.StatusForbidden, // Will fail validation, but we’re testing extraction
			expectedBody:   `{"error": "Unauthorized: token is malformed: token contains an invalid number of segments"}`,
		},
		{
			name:       "Cache Hit (Valid Token Format)",
			authHeader: "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6Im1vY2sta2lkIn0.eyJpc3MiOiJodHRwczovL3NlY3VyZXRva2VuLmdvb2dsZS5jb20vdGhvdWdodGZs",
			cacheSetup: func(tokenStr string) {
				cacheMutex.Lock()
				tokenCache.Set(tokenStr, TokenCacheEntry{UserID: "test-user"}, cache.DefaultExpiration)
				cacheMutex.Unlock()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request and recorder
			req := httptest.NewRequest("GET", "/api/inference", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()

			// Setup cache with the extracted token
			tokenStr := strings.TrimPrefix(tt.authHeader, "Bearer ")
			tt.cacheSetup(tokenStr)

			// Apply middleware
			handler := FirebaseAuthMiddleware(http.HandlerFunc(mockHandler))
			handler.ServeHTTP(rr, req)

			// Check results
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if strings.TrimSpace(rr.Body.String()) != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

// TestCacheInvalidation tests cache expiration
func TestCacheInvalidation(t *testing.T) {
	// cfg := FirebaseConfig{
	// 	ProjectID: "thoughtflow-8744c",
	// }

	// Use a short TTL for testing invalidation
	shortCache := cache.New(1*time.Second, 1*time.Second)
	originalCache := tokenCache
	tokenCache = shortCache
	defer func() { tokenCache = originalCache }() // Restore original cache

	// Test token
	tokenStr := "eyJhbGciOiJSUzI1NiIsImtpZCI6Im1vY2sta2lkIn0.eyJpc3MiOiJodHRwczovL3NlY3VyZXRva2VuLmdvb2dsZS5jb20vdGhvdWdodGZs"

	// Step 1: Populate cache
	cacheMutex.Lock()
	tokenCache.Set(tokenStr, TokenCacheEntry{UserID: "test-user"}, cache.DefaultExpiration)
	cacheMutex.Unlock()

	// Step 2: Immediate request (cache hit)
	req := httptest.NewRequest("GET", "/api/inference", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()
	handler := FirebaseAuthMiddleware(http.HandlerFunc(mockHandler))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected cache hit status 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "OK" {
		t.Errorf("expected body 'OK', got %q", rr.Body.String())
	}

	// Step 3: Wait for cache to expire
	time.Sleep(2 * time.Second)

	// Step 4: Request after expiration (cache miss, will attempt Firebase validation)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected cache miss status 403 after expiration, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Unauthorized") {
		t.Errorf("expected 'Unauthorized' error after cache expiration, got %q", rr.Body.String())
	}
}
