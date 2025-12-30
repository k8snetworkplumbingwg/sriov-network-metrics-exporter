// The SR-IOV networks exporter makes metrics from SR-IOV Virtual Functions available in a prometheus format.
// Different classes of metrics are implemented as individual collectors.

package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/collectors"
)

const (
	defaultRateBurst         = 10
	defaultReadHeaderTimeout = 10 * time.Second
)

var (
	addr            = flag.String("web.listen-address", ":9808", "Port to listen on for web interface and telemetry.")
	rateLimit       = flag.Int("web.rate-limit", 1, "Limit for requests per second.")
	rateBurst       = flag.Int("web.rate-burst", defaultRateBurst, "Maximum per second burst rate for requests.")
	metricsEndpoint = "/metrics"
)

func main() {
	parseAndVerifyFlags()

	err := prometheus.Register(collectors.Enabled())
	if err != nil {
		log.Fatalf("collector could not be registered: %v", err)
		return
	}

	// Use the default promhttp handler wrapped with middleware to serve at the metrics endpoint
	handlerWithMiddleware := limitRequests(
		getOnly(
			endpointOnly(
				noBody(promhttp.Handler()), metricsEndpoint)),
		rate.Limit(*rateLimit), *rateBurst)

	server := &http.Server{
		Addr:              *addr,
		Handler:           handlerWithMiddleware,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}
	log.Printf("listening on %v", *addr)
	log.Fatalf("ListenAndServe error: %v", server.ListenAndServe())
}

func parseAndVerifyFlags() {
	flag.Parse()
	verifyFlags()
}

// endpointOnly restricts all responses to 404 where the passed endpoint isn't used. Used to minimize the possible outputs of the server.
func endpointOnly(next http.Handler, endpoint string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpoint {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte{})
			if err != nil {
				log.Print(err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getOnly restricts the possible verbs used in a http request to GET only
func getOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, err := w.Write([]byte{})
			if err != nil {
				log.Print(err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// noBody returns a 400 to any request that contains a body
func noBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != http.NoBody {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte{})
			if err != nil {
				log.Print(err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitRequests sets a rate limit and a burst limit for requests to the endpoint
func limitRequests(next http.Handler, rateLimit rate.Limit, burstLimit int) http.Handler {
	limiter := rate.NewLimiter(rateLimit, burstLimit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func verifyFlags() {
	if err := collectors.ResolveFilepaths(); err != nil {
		log.Panicf("failed to resolve paths\n%v", err)
	}
}
