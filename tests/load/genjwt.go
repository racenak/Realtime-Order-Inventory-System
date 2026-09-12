package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Iss    string `json:"iss"`
	Aud    string `json:"aud"`
	Iat    int64  `json:"iat"`
	Exp    int64  `json:"exp"`
}

func b64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "changeme"
	}

	header, _ := json.Marshal(Header{Alg: "HS256", Typ: "JWT"})
	now := time.Now().Unix()
	claims, _ := json.Marshal(Claims{
		UserID: "load-test-user",
		Email:  "loadtest@example.com",
		Role:   "admin",
		Iss:    "order-inventory-system",
		Aud:    "order-inventory-api",
		Iat:    now,
		Exp:    now + 3600,
	})

	headerB64 := b64url(header)
	claimsB64 := b64url(claims)
	unsigned := headerB64 + "." + claimsB64

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	sig := b64url(mac.Sum(nil))

	fmt.Println(unsigned + "." + sig)
}
