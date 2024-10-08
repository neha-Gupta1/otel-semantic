package tel

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func InitTracerHTTP() *sdktrace.TracerProvider {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	OTEL_OTLP_HTTP_ENDPOINT := os.Getenv("OTEL_OTLP_HTTP_ENDPOINT")

	if OTEL_OTLP_HTTP_ENDPOINT == "" {
		OTEL_OTLP_HTTP_ENDPOINT = "localhost:5080" //without trailing slash
	}

	otlptracehttp.NewClient()

	otlpHTTPExporter, err := otlptracehttp.New(context.TODO(),
		otlptracehttp.WithInsecure(), // use http & not https
		otlptracehttp.WithEndpoint(OTEL_OTLP_HTTP_ENDPOINT),
		otlptracehttp.WithURLPath("/api/default/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			// update this with your API key or default username and password for OpenObserve
			"Authorization": "Basic cm9vdEBleGFtcGxlLmNvbTpDb21wbGV4cGFzcyMxMjMK",
		}),
	)

	if err != nil {
		fmt.Println("Error creating HTTP OTLP exporter: ", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		// the service name used to display traces in backends
		semconv.ServiceNameKey.String("user-info"),
		semconv.ServiceVersionKey.String("0.0.1"),
		attribute.String("environment", "test"),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(otlpHTTPExporter),
	)
	otel.SetTracerProvider(tp)

	return tp
}
