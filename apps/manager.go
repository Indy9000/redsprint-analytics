package apps

import (
	"analytics/models"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// // PURPOSE: manage a list of apps
// used for
// - checking the api-key in tracking call
// - add/delete/list apps in admin page
// //
type Manager struct {
	path     string
	data     *Data
	caches   map[string]*EventCache // Per-app caches
	dataMu   sync.RWMutex           // Protects data
	cachesMu sync.RWMutex           // Protects caches
}

type Data struct {
	Apps map[string]*App `json:"apps"`
}

type App struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}

func NewManager(path string) (*Manager, error) {
	m := &Manager{
		path: path,
		data: &Data{
			Apps: make(map[string]*App),
		},
		caches: make(map[string]*EventCache),
	}

	// Try to load the config file
	if err := m.load(); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create a new one
			if err := m.save(); err != nil {
				return nil, fmt.Errorf("create new config file: %w", err)
			}
		} else {
			// Other errors during loading
			return nil, fmt.Errorf("load apps metadata: %w", err)
		}
	}

	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.data)
}

func (m *Manager) save() error {
	// NO need for lock because it is accessed under LOCK when app is added

	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func (m *Manager) AddEvent(event *models.Event) {
	m.cachesMu.Lock()
	defer m.cachesMu.Unlock()

	// Initialize cache for app if not exists
	if _, exists := m.caches[event.AppID]; !exists {
		m.caches[event.AppID] = NewEventCache()
	}
	m.caches[event.AppID].Add(event)
}

func (m *Manager) CreateApp(name string) (*App, error) {
	m.dataMu.Lock()
	defer m.dataMu.Unlock()

	// Generate ID by hashing the name
	hash := sha256.Sum256([]byte(name))
	id := fmt.Sprintf("%x", hash)[:8] // Use first 8 characters of hex-encoded hash

	// Check if app with this ID already exists
	if _, exists := m.data.Apps[id]; exists {
		return nil, fmt.Errorf("app with name %s already exists (ID: %s)", name, id)
	}

	// Generate API key
	apiKey, err := GenerateUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("unable to create API key: %w", err)
	}

	// Create new App
	app := &App{
		ID:        id,
		Name:      name,
		APIKey:    apiKey,
		CreatedAt: time.Now().UTC(),
	}

	// Store the app
	m.data.Apps[id] = app

	// Save to config file
	if err := m.save(); err != nil {
		delete(m.data.Apps, id) // Rollback on save failure
		return nil, fmt.Errorf("save app: %w", err)
	}

	return app, nil
}

func (m *Manager) GetAppByAPIKey(apiKey string) (*App, error) {
	m.dataMu.RLock()
	defer m.dataMu.RUnlock()

	for _, app := range m.data.Apps {
		if app.APIKey == apiKey {
			return app, nil
		}
	}

	return nil, fmt.Errorf("invalid API key")
}

func (m *Manager) ListApps() []*App {
	m.dataMu.RLock()
	defer m.dataMu.RUnlock()

	apps := make([]*App, 0, len(m.data.Apps))
	for _, app := range m.data.Apps {
		apps = append(apps, app)
	}

	return apps
}

func GenerateUUIDv7() (string, error) {
	uuid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}
