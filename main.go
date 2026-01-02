package main

import (
	"analytics/apps"
	fba "analytics/firebase_auth"
	"analytics/tracker"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func corsMiddlewareWrapper(appMgr *apps.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// log.Printf("CORS Middleware: Received %s request for %s", r.Method, r.URL.Path)
			origin := r.Header.Get("Origin")
			// log.Printf("CORS Middleware: Origin header: %q", origin)

			if origin == "" {
				// log.Println("CORS Middleware: No Origin header, proceeding without CORS headers")
				next.ServeHTTP(w, r)
				return
			}

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				// log.Println("CORS Middleware: Handling OPTIONS preflight request")
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.WriteHeader(http.StatusOK)
				return
			}

			// Validate API key for non-OPTIONS requests
			apiKey := r.Header.Get("X-API-Key")
			// log.Printf("CORS Middleware: X-API-Key: %q", apiKey)
			if apiKey == "" {
				log.Println("CORS Middleware: Missing X-API-Key header")
				http.Error(w, "Missing API key", http.StatusUnauthorized)
				return
			}

			// Get app by API key
			// log.Println("CORS Middleware: Attempting to fetch app by API key")
			app, err := appMgr.GetAppByAPIKey(apiKey)
			if err != nil {
				log.Printf("CORS Middleware: Failed to get app for API key: %v", err)
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}
			// log.Printf("CORS Middleware: Found app ID: %s, AllowedOrigins: %v", app.ID, app.AllowedOrigins)

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range app.AllowedOrigins {
				if allowedOrigin == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				log.Printf("CORS Middleware: Origin %q not allowed for app %s (allowed: %v)", origin, app.ID, app.AllowedOrigins)
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			// log.Printf("CORS Middleware: Origin %q allowed for app %s", origin, app.ID)

			// Set CORS headers
			log.Println("CORS Middleware: Setting CORS headers")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Pass to the next handler
			// log.Println("CORS Middleware: Passing to next handler")
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	log.Print("loading config")

	// Initialize config
	apps, err := apps.NewManager("./app-metadata.json")
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	corsMiddleware := corsMiddlewareWrapper(apps)

	// app, err := firebase.NewApp(context.Background(), nil,
	// 	option.WithCredentialsFile("redsprint-analytics-firebase-adminsdk.json"))
	// if err != nil {
	// 	log.Fatalf("Error initializing Firebase app: %v", err)
	// }
	// firebaseAuth, err := app.Auth(context.Background())
	// if err != nil {
	// 	log.Fatalf("Error initializing Firebase auth: %v", err)
	// }
	// firestore, err := app.Firestore(context.Background())
	// if err != nil {
	// 	log.Fatalf("Error initializing Firestore: %v", err)
	// }
	tracker := tracker.NewEventTracker(apps)
	// Set up the router
	mux := http.NewServeMux()

	mux.Handle("/analytics/api/v1/apps", corsMiddleware(fba.FirebaseAuthMiddleware(apps.ListAppsHandler())))
	mux.Handle("/analytics/api/v1/apps/", corsMiddleware(fba.FirebaseAuthMiddleware(apps.CrudHandler())))
	mux.Handle("/analytics/api/v1/track", corsMiddleware(tracker.PostHandler()))

	const port = "8115"
	// Create an HTTP server
	server := &http.Server{
		Addr:    "127.0.0.1:" + port,
		Handler: mux,
	}

	_, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	go gracefulShutdown(server, cancel, &wg)

	log.Println("Starting server on :" + port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// gracefulShutdown handles shutdown signals and waits for active goroutines to finish
func gracefulShutdown(server *http.Server, cancel context.CancelFunc, wg *sync.WaitGroup) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) // Catch termination signals
	<-c
	log.Println("Shutdown signal received...")

	// Initiate graceful shutdown
	cancel()  // Signal goroutines to stop
	wg.Wait() // Wait for all goroutines to finish

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down server: %v", err)
	}
	log.Println("Server gracefully stopped.")
}
