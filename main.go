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
			origin := r.Header.Get("Origin")

			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.WriteHeader(http.StatusOK)
				return
			}

			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				log.Printf("CORS: missing API key from %s (origin=%s)", r.RemoteAddr, origin)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			app, err := appMgr.GetAppByAPIKey(apiKey)
			if err != nil {
				log.Printf("CORS: invalid API key from %s (origin=%s)", r.RemoteAddr, origin)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range app.AllowedOrigins {
				if allowedOrigin == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				log.Printf("CORS: origin %q not allowed for app %s from %s", origin, app.ID, r.RemoteAddr)
				w.WriteHeader(http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

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
