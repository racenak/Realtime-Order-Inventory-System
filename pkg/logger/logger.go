package logger

import (
	"context"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger bọc zap.Logger để mở rộng thêm các hàm tiện ích
type Logger struct {
	*zap.Logger
}

// Zap returns the underlying *zap.Logger for use with packages that require it.
func (l *Logger) Zap() *zap.Logger {
	return l.Logger
}

// InfoContext ghi log Info kèm theo TraceID/SpanID tự động từ context (nếu có)
func (l *Logger) InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.Info(msg, append(fields, traceFields(ctx)...)...)
}

// WarnContext ghi log Warn kèm theo TraceID/SpanID tự động từ context (nếu có)
func (l *Logger) WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.Warn(msg, append(fields, traceFields(ctx)...)...)
}

// ErrorContext ghi log Error kèm theo TraceID/SpanID tự động từ context (nếu có)
func (l *Logger) ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {
	l.Error(msg, append(fields, traceFields(ctx)...)...)
}

// traceFields extracts TraceID and SpanID from context if available
func traceFields(ctx context.Context) []zap.Field {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return nil
	}
	sc := span.SpanContext()
	if !sc.TraceID().IsValid() {
		return nil
	}
	fields := []zap.Field{
		zap.String("trace_id", sc.TraceID().String()),
	}
	if sc.SpanID().IsValid() {
		fields = append(fields, zap.String("span_id", sc.SpanID().String()))
	}
	return fields
}

// New khởi tạo Zap Logger kết hợp đẩy log ra Stdout và gửi qua OTLP gRPC tới OTel Collector
func InitLogger(ctx context.Context, service string, otelEndpoint string) (*Logger, func(), error) {
	// 1. Cấu hình Encoder & Core cho Stdout (Console)
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	stdoutLogger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.Fields(zap.String("service", service)),
	)
	if err != nil {
		return nil, nil, err
	}

	// Nếu không truyền otelEndpoint, chỉ dùng Stdout logger mặc định
	if otelEndpoint == "" {
		return &Logger{stdoutLogger}, func() {}, nil
	}

	// 2. Khởi tạo OTLP Log Exporter (gRPC)
	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(otelEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, err
	}

	// 3. Tạo Resource gắn thông tin service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(service),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	// 4. Khởi tạo OTel LoggerProvider
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	// 5. Tạo Zap Core cho OTel (Gửi log về Collector và tự gắn TraceID/SpanID từ Context)
	otelCore := otelzap.NewCore(service, otelzap.WithLoggerProvider(lp))

	// 6. Kết hợp (Tee) cả 2 Core: Stdout Core + OTel Core
	combinedCore := zapcore.NewTee(stdoutLogger.Core(), otelCore)

	finalLogger := zap.New(
		combinedCore,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.Fields(zap.String("service", service)),
	)

	// Hàm cleanup gọi defer ở main() để flush log trước khi stop app
	cleanup := func() {
		_ = lp.Shutdown(ctx)
		_ = exporter.Shutdown(ctx)
		_ = finalLogger.Sync()
	}

	return &Logger{finalLogger}, cleanup, nil
}
