package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

var (
	jwtSecret []byte
	issuer    string
	audience  string
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	jwtSecret = []byte(secret)
	issuer = getEnv("JWT_ISSUER", "order-inventory-system")
	audience = getEnv("JWT_AUDIENCE", "order-inventory-api")

	port := getEnv("HTTP_PORT", "8083")

	http.HandleFunc("/verify", handleVerify)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Auth service starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleVerify(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
		return
	}

	tokenString := parts[1]

	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		},
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithLeeway(30*time.Second),
	)

	if err != nil || !token.Valid {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
		return
	}

	// Set user info in response headers for Traefik to forward
	w.Header().Set("X-User-Id", claims.UserID)
	w.Header().Set("X-User-Role", claims.Role)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`{"user_id":"%s","role":"%s"}`, claims.UserID, claims.Role)))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
