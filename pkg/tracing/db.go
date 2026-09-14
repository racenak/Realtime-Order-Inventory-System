package tracing

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const dbSpanName = "db.query"

type TracedDB struct {
	db      *sqlx.DB
	tracer  trace.Tracer
	version string
}

func NewTracedDB(db *sqlx.DB, serviceName string) *TracedDB {
	return &TracedDB{
		db:     db,
		tracer: otel.Tracer(serviceName),
	}
}

func (t *TracedDB) DB() *sqlx.DB {
	return t.db
}

func (t *TracedDB) startSpan(ctx context.Context, query string) (context.Context, trace.Span) {
	ctx, span := t.tracer.Start(ctx, dbSpanName,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.statement", query),
	)
	return ctx, span
}

func (t *TracedDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	ctx, span := t.startSpan(ctx, query)
	defer span.End()

	result, err := t.db.ExecContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return result, err
}

func (t *TracedDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, span := t.startSpan(ctx, query)
	defer span.End()

	rows, err := t.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return rows, err
}

func (t *TracedDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	ctx, span := t.startSpan(ctx, query)
	defer span.End()

	return t.db.QueryRowContext(ctx, query, args...)
}

func (t *TracedDB) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	ctx, span := t.startSpan(ctx, query)
	defer span.End()

	result, err := t.db.NamedExecContext(ctx, query, arg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return result, err
}

func (t *TracedDB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	ctx, span := t.startSpan(ctx, query)
	defer span.End()

	err := t.db.SelectContext(ctx, dest, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func (t *TracedDB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	ctx, span := t.startSpan(ctx, query)
	defer span.End()

	err := t.db.GetContext(ctx, dest, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
