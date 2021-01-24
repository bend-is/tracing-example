package main

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/jaeger/propagation"

	"contrib.go.opencensus.io/integrations/ocsql"
	"github.com/opencensus-integrations/redigo/redis"
	"go.opencensus.io/trace"

	"github.com/lib/pq" // for pg driver
)

const (
	envTracingHost = "http://localhost:14268/api/traces"
	envPgConn      = "postgres://user:pass@localhost:9241/mydb?sslmode=disable"
	envRedisHost   = "localhost:6380"
)

func main() {
	rand.Seed(time.Now().UnixNano())

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

	redisPool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", envRedisHost)
		},
	}

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		sc, ok := (&propagation.HTTPFormat{}).SpanContextFromRequest(r)
		if ok {
			log.Printf("[INFO] Got span context %v", sc)
		}

		ctx, span := trace.StartSpanWithRemoteParent(context.Background(), "server", sc)

		doRedisStaff(ctx, redisPool)

		rows, err := db.QueryContext(ctx, "select count(*) from test")
		if err != nil {
			log.Fatalf("db: %v", err)
		}

		if rows.Next() {
			var count string
			err := rows.Scan(&count)
			if err != nil {
				log.Fatalf("error during fetch the sql query result: %v", err)
			}
			span.AddAttributes(trace.StringAttribute("count result", count))
			rows.Close()
		}

		_, spanSleep := trace.StartSpan(ctx, "sleep")
		time.Sleep(time.Second * 2)
		spanSleep.End()

		_, err = db.ExecContext(ctx, "insert into test values($1,$2)", 1, "test")
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		defer span.End()

		if rand.Intn(100) <= 20 {
			span.SetStatus(trace.Status{Code: 500, Message: "Internal Error"})
			w.WriteHeader(500)
			return
		}

		span.SetStatus(trace.Status{Code: 200, Message: "OK"})
		w.WriteHeader(200)
	})
	http.ListenAndServe(":8080", nil)
}

func doRedisStaff(ctx context.Context, pool *redis.Pool) {
	conn := pool.GetWithContext(ctx).(redis.ConnWithContext)
	defer conn.CloseContext(ctx)

	setCtx, setSpan := trace.StartSpan(ctx, "set in redis")
	_, err := conn.DoContext(setCtx, "SET", "test", "hello world")
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	setSpan.End()

	getCtx, getSpan := trace.StartSpan(ctx, "get from redis")
	_, err = conn.DoContext(getCtx, "GET", "test")
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	getSpan.End()

	delCtx, delSpan := trace.StartSpan(ctx, "delete from redis")
	_, err = conn.DoContext(delCtx, "DEL", "test")
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	delSpan.End()
}
