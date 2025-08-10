package apps

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func (m *Manager) AddAppHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("AddAppHandler Received %s request for %s", r.Method, r.URL.Path)

		// Check for POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse JSON request body
		var req struct {
			Name           string   `json:"name"`
			AllowedOrigins []string `json:"allowed_origins"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate name
		if req.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		// Create the app
		app, err := m.CreateApp(req.Name, req.AllowedOrigins)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create app: %v", err), http.StatusInternalServerError)
			return
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		// Respond with the created app
		resp := App{
			ID:             app.ID,
			Name:           app.Name,
			APIKey:         app.APIKey,
			CreatedAt:      app.CreatedAt,
			AllowedOrigins: app.AllowedOrigins,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}
