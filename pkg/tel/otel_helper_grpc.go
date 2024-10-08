package tel

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func InitTracerGRPC() *sdktrace.TracerProvider {
	OTEL_OTLP_GRPC_ENDPOINT := os.Getenv("OTEL_OTLP_GRPC_ENDPOINT")

	if OTEL_OTLP_GRPC_ENDPOINT == "" {
		OTEL_OTLP_GRPC_ENDPOINT = "<host>:<port>" //without trailing slash
	}

	OTEL_OTLP_GRPC_ENDPOINT = "127.0.0.1:5081"

	otlptracegrpc.NewClient()

	otlpGRPCExporter, err := otlptracegrpc.New(context.TODO(),
		otlptracegrpc.WithInsecure(), // use http & not https
		otlptracegrpc.WithEndpoint("127.0.0.1:5081"),
		//otlptracegrpc.w ("/api/prabhat-org2/traces"),
		otlptracegrpc.WithHeaders(map[string]string{
			"Authorization": "Basic YWRtaW46Q29tcGxleHBhc3MjMTIz",
			//"Authorization": "Bearer some-auth-token",
			"zinc-org-id": "org1",
		}),
	)

	if err != nil {
		fmt.Println("Error creating HTTP OTLP exporter: ", err)
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		// the service name used to display traces in backends
		semconv.ServiceNameKey.String("user-info"),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(otlpGRPCExporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}
