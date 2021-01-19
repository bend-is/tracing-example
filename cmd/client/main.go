package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	uuid "github.com/satori/go.uuid"
	"go.opencensus.io/trace"

	// for pg driver
	// "go.opencensus.io/plugin/ochttp"
	"contrib.go.opencensus.io/exporter/jaeger/propagation"
)

const (
	envTracingHost = "http://localhost:14268/api/traces"
	envPgConn      = "postgres://user:pass@localhost:9241/mydb?sslmode=disable"
	waitTime       = time.Second * 30
)

func main() {
	var exporter *jaeger.Exporter
	{
		var err error
		exporter, err = jaeger.NewExporter(jaeger.Options{
			CollectorEndpoint: envTracingHost,
			Username:          os.Getenv("TRACING_USER"),
			Password:          os.Getenv("TRACING_PASS"),
			Process: jaeger.Process{
				ServiceName: "github.com/korjavin/tracing-example/client",
			},
		})
		if err != nil {
			log.Fatalf("Can't register tracing exporter, error%v", err)
		}
		log.Println("Using jaeger exporter for tracing")
		trace.RegisterExporter(exporter)
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	} // Create exporter.

	id := uuid.NewV4()
	clientCtx, span := trace.StartSpan(context.Background(), fmt.Sprintf("Request: %s", id))
	defer span.End()
	span.Annotate(nil, "Hi, I'm from client")
	span.AddAttributes(trace.Int64Attribute("to_server", 1))

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/test", nil)
	if err != nil {
		log.Fatalf("[FATAL]  %v", err)
	}
	(&propagation.HTTPFormat{}).SpanContextToRequest(span.SpanContext(), req)

	go func() {
		_, err = http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("[FATAL]  %v", err)
		}
	}()

	LongFunc(clientCtx)
	span.End()
	exporter.Flush()
	time.Sleep(time.Second * 10)
	log.Printf("[INFO] Done")
}

func LongFunc(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "Long func")
	defer span.End()
	time.Sleep(waitTime)
	ctx, span1 := trace.StartSpan(ctx, "Sub func") // TODO exclude some operations and see difference on compare tool
	time.Sleep(waitTime)
	span1.End()
	time.Sleep(waitTime)
}
