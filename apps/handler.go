package apps

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func (m *Manager) CrudHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			m.CreateAppHandler()(w, r)
		case http.MethodPut:
			m.UpdateAppHandler()(w, r)
		case http.MethodDelete:
			m.DeleteAppHandler()(w, r)
		case http.MethodGet:
			if strings.HasSuffix(r.URL.Path, "/events") {
				m.GetEventsHandler()(w, r)
			} else {
				log.Printf("Main: Invalid GET path %s", r.URL.Path)
				http.Error(w, "Invalid path", http.StatusBadRequest)
			}
		default:
			log.Printf("Main: Method %s not allowed for %s", r.Method, r.URL.Path)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// ListAppsHandler returns all apps.
func (m *Manager) ListAppsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("ListAppsHandler: Received %s request for %s", r.Method, r.URL.Path)

		if r.Method != http.MethodGet {
			log.Printf("ListAppsHandler: Method %s not allowed", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		m.dataMu.RLock()
		apps := make([]App, 0, len(m.data.Apps))
		for _, app := range m.data.Apps {
			apps = append(apps, *app)
		}
		m.dataMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(apps); err != nil {
			log.Printf("ListAppsHandler: Failed to encode response: %v", err)
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
		log.Println("ListAppsHandler: Listed all apps")
	}
}

// CreateAppHandler creates a new app.
func (m *Manager) CreateAppHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("CreateAppHandler: Received %s request for %s", r.Method, r.URL.Path)

		if r.Method != http.MethodPost {
			log.Printf("CreateAppHandler: Method %s not allowed", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Name           string   `json:"name"`
			AllowedOrigins []string `json:"allowed_origins"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("CreateAppHandler: Invalid request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			log.Println("CreateAppHandler: Name is required")
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		app, err := m.CreateApp(req.Name, req.AllowedOrigins)
		if err != nil {
			log.Printf("CreateAppHandler: Failed to create app: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create app: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		resp := App{
			ID:             app.ID,
			Name:           app.Name,
			APIKey:         app.APIKey,
			CreatedAt:      app.CreatedAt,
			AllowedOrigins: app.AllowedOrigins,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("CreateAppHandler: Failed to encode response: %v", err)
		}
		log.Printf("CreateAppHandler: Created app ID: %s", app.ID)
	}
}

// UpdateAppHandler updates an app by ID.
func (m *Manager) UpdateAppHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("UpdateAppHandler: Received %s request for %s", r.Method, r.URL.Path)

		if r.Method != http.MethodPut {
			log.Printf("UpdateAppHandler: Method %s not allowed", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		appID := strings.TrimPrefix(r.URL.Path, "/analytics/api/v1/apps/")
		if appID == "" || appID == r.URL.Path {
			log.Println("UpdateAppHandler: Missing app ID")
			http.Error(w, "Missing app ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Name           string   `json:"name"`
			AllowedOrigins []string `json:"allowed_origins"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("UpdateAppHandler: Invalid request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			log.Println("UpdateAppHandler: Name is required")
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		m.dataMu.Lock()
		app, exists := m.data.Apps[appID]
		if !exists {
			m.dataMu.Unlock()
			log.Printf("UpdateAppHandler: App ID %s not found", appID)
			http.Error(w, "App not found", http.StatusNotFound)
			return
		}

		app.Name = req.Name
		app.AllowedOrigins = req.AllowedOrigins
		m.data.Apps[appID] = app
		if err := m.save(); err != nil {
			m.dataMu.Unlock()
			log.Printf("UpdateAppHandler: Failed to save app: %v", err)
			http.Error(w, "Failed to save app", http.StatusInternalServerError)
			return
		}
		m.dataMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(app); err != nil {
			log.Printf("UpdateAppHandler: Failed to encode response: %v", err)
		}
		log.Printf("UpdateAppHandler: Updated app ID: %s", appID)
	}
}

// DeleteAppHandler deletes an app by ID.
func (m *Manager) DeleteAppHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("DeleteAppHandler: Received %s request for %s", r.Method, r.URL.Path)

		if r.Method != http.MethodDelete {
			log.Printf("DeleteAppHandler: Method %s not allowed", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		appID := strings.TrimPrefix(r.URL.Path, "/analytics/api/v1/apps/")
		if appID == "" || appID == r.URL.Path {
			log.Println("DeleteAppHandler: Missing app ID")
			http.Error(w, "Missing app ID", http.StatusBadRequest)
			return
		}

		m.dataMu.Lock()
		if _, exists := m.data.Apps[appID]; !exists {
			m.dataMu.Unlock()
			log.Printf("DeleteAppHandler: App ID %s not found", appID)
			http.Error(w, "App not found", http.StatusNotFound)
			return
		}

		delete(m.data.Apps, appID)
		m.cachesMu.Lock()
		delete(m.caches, appID)
		m.cachesMu.Unlock()

		if err := m.save(); err != nil {
			m.dataMu.Unlock()
			log.Printf("DeleteAppHandler: Failed to save after delete: %v", err)
			http.Error(w, "Failed to save after delete", http.StatusInternalServerError)
			return
		}
		m.dataMu.Unlock()

		w.WriteHeader(http.StatusNoContent)
		log.Printf("DeleteAppHandler: Deleted app ID: %s", appID)
	}
}

/*
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



*/
