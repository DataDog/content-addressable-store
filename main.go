package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

const version = "v3"

func main() {
	// Parse flags
	var (
		httpF = flag.String("http", "localhost:8080", "HTTP addr to listen on.")
		dirF  = flag.String("dir", "data", "Directory for storing data.")
	)
	flag.Parse()

	// Start datadog tracer
	tracer.Start(tracer.WithServiceVersion(version))
	defer tracer.Stop()

	// Start datadog profiler with go timeline beta feature enabled
	os.Setenv("DD_PROFILING_EXECUTION_TRACE_ENABLED", "true")
	if err := profiler.Start(profiler.WithVersion(version)); err != nil {
		log.Fatalf("failed to start profiler: %s", err)
	}
	defer profiler.Stop()

	// Initialize data directory and service
	service, err := NewService(*dirF)
	if err != nil {
		log.Fatalf("failed to init service: %s", err)
	}

	// Start http server
	log.Printf("listening on: http://%s", *httpF)
	if err := http.ListenAndServe(*httpF, service); err != nil {
		log.Fatalf("failed to listen: %s", err)
	}
}
