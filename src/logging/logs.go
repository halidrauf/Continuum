// Copyright (c) 2026 Khaled Abbas
//
// This source code is licensed under the Business Source License 1.1.
//
// Change Date: 4 years after the first public release of this version.
// Change License: MIT
//
// On the Change Date, this version of the code automatically converts
// to the MIT License. Prior to that date, use is subject to the
// Additional Use Grant. See the LICENSE file for details.

package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "go.opentelemetry.io/otel/continuum/worker"

var (
	meter  = otel.Meter(instrumentationName)
	logger = otelslog.NewLogger(instrumentationName)
	tracer = otel.Tracer(instrumentationName)
)

func Log(content string, level slog.Level) {
	logger.Log(context.Background(), level, content)
}

func InitializeFloatCounter(name, description, unit string) (metric.Float64Counter, error) {
	counter, err := meter.Float64Counter(name,
		metric.WithDescription(description),
		metric.WithUnit(unit))
	if err != nil {
		Log("Failed to create metric: "+err.Error(), slog.LevelError)
		return nil, err
	}
	return counter, nil
}

func UpdateSpanValue(key string, value float64) {
	span := trace.SpanFromContext(context.Background())
	span.SetAttributes(attribute.Float64(key, value))
}