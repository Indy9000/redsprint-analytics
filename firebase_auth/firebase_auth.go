package firebase_auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	cache "github.com/patrickmn/go-cache"
)

// FirebaseConfig proj-short-name:fbproj-name
type FbProjectConfig map[string]string

// TokenCacheEntry stores validated token info
type TokenCacheEntry struct {
	UserID string // e.g., "sub" claim
}

// JWKS represents the Firebase JWKS response structure
type JWKS struct {
	Keys []struct {
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
		Kty string `json:"kty"`
		Alg string `json:"alg"`
		Use string `json:"use"`
	} `json:"keys"`
}

type ContextKey string

// UserIDKey is the specific key for user ID
const UserIDKey ContextKey = "userID"

// Global cache instance
var (
	tokenCache *cache.Cache
	cacheMutex sync.Mutex // For thread-safe cache access
)

func init() {
	// Initialize cache with 24-hour expiration and 1-hour cleanup interval
	tokenCache = cache.New(24*time.Hour, 1*time.Hour)
}

// In firebase_auth.go, update fetchFirebasePublicKeys if necessary
func fetchFirebasePublicKeys(jwksURL string) (map[string]*rsa.PublicKey, error) {
	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch JWKS: status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %v", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue // Skip non-RSA keys
		}
		// Decode base64 URL-encoded modulus (n) and exponent (e)
		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			return nil, fmt.Errorf("failed to decode modulus for kid %s: %v", key.Kid, err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			return nil, fmt.Errorf("failed to decode exponent for kid %s: %v", key.Kid, err)
		}

		// Construct RSA public key
		pubKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		}
		keys[key.Kid] = pubKey
	}
	return keys, nil
}

// Middleware to validate Firebase JWT with caching
func FirebaseAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error": "Missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == "" {
			http.Error(w, `{"error": "Missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Check cache first
		cacheMutex.Lock()
		if cached, found := tokenCache.Get(tokenStr); found {
			cacheMutex.Unlock()
			if entry, ok := cached.(TokenCacheEntry); ok {
				// log.Printf("Cache hit - Authenticated User ID: %s", entry.UserID)
				// Add UID to request context
				ctx := context.WithValue(r.Context(), UserIDKey, entry.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		cacheMutex.Unlock()

		// Cache miss - validate with Firebase
		token, err := verifyFirebaseToken(tokenStr)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Unauthorized: %v"}`, err), http.StatusForbidden)
			return
		}

		// Extract claims and cache the result
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID := claims["sub"].(string)
			log.Printf("Authenticated User ID: %s (from Firebase)", userID)

			// Get expiration time from token
			if exp, ok := claims["exp"].(float64); ok {
				expTime := time.Unix(int64(exp), 0)
				ttl := time.Until(expTime)
				if ttl > 0 {
					cacheMutex.Lock()
					tokenCache.Set(tokenStr, TokenCacheEntry{UserID: userID}, ttl)
					cacheMutex.Unlock()
				}
			}
			// Add UID to request context
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

const JWKSURL = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

// verifyFirebaseToken validates the JWT against Firebase public keys
func verifyFirebaseToken(tokenStr string) (*jwt.Token, error) {
	log.Printf("Fetching JWKS from: %s", JWKSURL)
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid kid in token header")
		}
		keys, err := fetchFirebasePublicKeys(JWKSURL)
		if err != nil {
			return nil, fmt.Errorf("error while executing keyfunc: %v", err)
		}
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("no public key found for kid %s", kid)
		}
		return key, nil
	}

	token, err := jwt.Parse(tokenStr, keyFunc)
	if err != nil {
		return nil, err
	}
	// Rest of the validation logic (iss, aud, etc.)...
	return token, nil
}
