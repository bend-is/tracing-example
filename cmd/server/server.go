package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/jaeger/propagation"

	"contrib.go.opencensus.io/integrations/ocsql"
	"go.opencensus.io/trace"

	"github.com/lib/pq" // for pg driver
)

const (
	envTracingHost = "http://localhost:14268/api/traces"
	envPgConn      = "postgres://user:pass@localhost:9241/mydb?sslmode=disable"
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
				ServiceName: "github.com/korjavin/tracing-example/server",
			},
		})
		if err != nil {
			log.Fatalf("Can't register tracing exporter, error%v", err)
		}
		log.Println("Using jaeger exporter for tracing")
		trace.RegisterExporter(exporter)
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	} // Create exporter.

	connector, err := pq.NewConnector(envPgConn)
	if err != nil {
		log.Fatalf("unable to create our postgres connector: %v\n", err)
	}

	// Wrap the driver.Connector with ocsql.
	cnn := ocsql.WrapConnector(connector, ocsql.WithAllTraceOptions())
	db := sql.OpenDB(cnn)

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := (&propagation.HTTPFormat{}).SpanContextFromRequest(r)
		if ok {
			log.Printf("[INFO] Got span context %v", sc)
		}
		ctx, span := trace.StartSpanWithRemoteParent(context.Background(), "server", sc)
		// TODO Add count to span attributes
		_, err := db.ExecContext(ctx, "select count(*) from test")
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		_, spanSleep := trace.StartSpan(ctx, "sleep")
		time.Sleep(time.Second * 10)
		spanSleep.End()

		_, err = db.ExecContext(ctx, "insert into test values($1,$2)", 1, "test")
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		defer span.End()
		w.WriteHeader(200) // TODO mark result in the span
		// TODO fail ~ 20% of requests
	})
	http.ListenAndServe(":8080", nil)
}
