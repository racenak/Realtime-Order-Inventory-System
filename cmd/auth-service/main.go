package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/racenak/Realtime-Order-Inventory-System/pkg/logger"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/tracing"
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

const authServiceName = "auth-service"

var (
	jwtSecret []byte
	issuer    string
	audience  string
	log       *logger.Logger
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Khởi tạo Logger (Xuất Stdout + Gửi OTLP về otel-collector)
	otelCollectorAddr := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	logClient, cleanupLogger, err := logger.InitLogger(ctx, "auth-service", otelCollectorAddr)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanupLogger()
	log = logClient

	// 2. Kiểm tra JWT Secret
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is required")
	}
	jwtSecret = []byte(secret)
	issuer = getEnv("JWT_ISSUER", "order-inventory-system")
	audience = getEnv("JWT_AUDIENCE", "order-inventory-api")

	// 3. Khởi tạo OpenTelemetry Tracer
	shutdownTracer, err := tracing.InitTracer(ctx, "auth-service", otelCollectorAddr)
	if err != nil {
		log.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer func() { _ = shutdownTracer(context.Background()) }()
	}

	port := getEnv("HTTP_PORT", "8083")

	http.Handle("/verify", metricsHTTP(authServiceName, traceHandler("auth.verify", handleVerify)))
	http.Handle("/health", metricsHTTP(authServiceName, http.HandlerFunc(handleHealth)))
	http.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%s", port)
	server := &http.Server{
		Addr:    addr,
		Handler: nil,
	}

	log.Info("Auth service starting", zap.String("addr", addr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down auth service...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Auth service stopped cleanly")
}

func traceHandler(name string, next http.HandlerFunc) http.HandlerFunc {
	tracer := otel.Tracer("auth-service")
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, "HTTP POST /verify",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("auth.handler", name),
			),
		)
		defer span.End()

		next(w, r.WithContext(ctx))
	}
}

func handleVerify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authHeader := r.Header.Get("Authorization")

	if authHeader == "" {
		log.WarnContext(ctx, "Missing authorization header")
		http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		log.WarnContext(ctx, "Invalid authorization header format")
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
		log.WarnContext(ctx, "Invalid or expired token", zap.Error(err))
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		log.ErrorContext(ctx, "Failed to parse custom claims")
		http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
		return
	}

	log.InfoContext(ctx, "Token verified successfully",
		zap.String("user_id", claims.UserID),
		zap.String("role", claims.Role),
	)

	// Set user info in response headers for Traefik to forward
	w.Header().Set("X-User-Id", claims.UserID)
	w.Header().Set("X-User-Role", claims.Role)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"user_id":"%s","role":"%s"}`, claims.UserID, claims.Role)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func metricsHTTP(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &metricsStatusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", sw.status)
		metrics.HTTPRequestsTotal.WithLabelValues(service, r.Method, r.URL.Path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(service, r.Method, r.URL.Path).Observe(duration)
	})
}

type metricsStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
