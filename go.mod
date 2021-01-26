module github.com/korjavin/tracing-example

go 1.15

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	contrib.go.opencensus.io/integrations/ocsql v0.1.7
	github.com/lib/pq v1.9.0
	github.com/opencensus-integrations/redigo v2.0.1+incompatible
	go.opencensus.io v0.22.5
)
