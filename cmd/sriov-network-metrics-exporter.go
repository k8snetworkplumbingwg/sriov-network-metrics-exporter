// The SR-IOV networks exporter makes metrics from SR-IOV Virtual Functions available in a prometheus format.
// Different classes of metrics are implemented as individual collectors.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/stdr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"

	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/collectors"
	"github.com/k8snetworkplumbingwg/sriov-network-metrics-exporter/pkg/tlsconfig"
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

	tlsCertFile = flag.String("tls-cert-file", "",
		"File containing the x509 certificate for HTTPS. If empty, server runs in plain HTTP mode.")
	tlsKeyFile      = flag.String("tls-private-key-file", "", "File containing the x509 private key matching --tls-cert-file.")
	tlsCipherSuites = flag.String("tls-cipher-suites", "", "Comma-separated list of TLS cipher suites (IANA names). If empty, uses defaults.")
	tlsCurves       = flag.String("tls-curve-preferences", "", "Comma-separated list of TLS curve preferences. If empty, uses defaults.")
	tlsMinVer       = flag.String("tls-min-version", "",
		"Minimum TLS version (VersionTLS12, VersionTLS13). If empty, defaults to VersionTLS12.")
	enableHTTP2 = flag.Bool("enable-http2", false,
		"Enable HTTP/2 for the metrics server. Disabled by default (CVE-2023-39325 mitigation).")
	enableAuthNAuthZ = flag.Bool("authentication-and-authorization", false,
		"Enable Kubernetes TokenReview/SubjectAccessReview for the metrics endpoint.")
)

func main() {
	parseAndVerifyFlags()

	err := prometheus.Register(collectors.Enabled())
	if err != nil {
		log.Fatalf("collector could not be registered: %v", err)
		return
	}

	// Use the default promhttp handler wrapped with middleware to serve at the metrics endpoint
	var handler = limitRequests(
		getOnly(
			endpointOnly(
				noBody(promhttp.Handler()), metricsEndpoint)),
		rate.Limit(*rateLimit), *rateBurst)

	if *enableAuthNAuthZ {
		handler, err = newAuthHandler(handler)
		if err != nil {
			log.Fatalf("failed to create auth handler: %v", err)
		}
		log.Print("authentication and authorization enabled")
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	if *tlsCertFile != "" {
		if *tlsKeyFile == "" {
			log.Fatal("--tls-private-key-file is required when --tls-cert-file is set")
		}

		var opts []tlsconfig.Option
		if *tlsCipherSuites != "" {
			ciphers, err := tlsconfig.CipherNamesToIDs(strings.Split(*tlsCipherSuites, ","))
			if err != nil {
				log.Fatalf("invalid --tls-cipher-suites: %v", err)
			}
			opts = append(opts, tlsconfig.WithCipherSuites(ciphers))
		}
		if *tlsCurves != "" {
			curves, err := tlsconfig.CurveNamesToIDs(strings.Split(*tlsCurves, ","))
			if err != nil {
				log.Fatalf("invalid --tls-curve-preferences: %v", err)
			}
			opts = append(opts, tlsconfig.WithCurvePreferences(curves))
		}
		if *tlsMinVer != "" {
			v, err := tlsconfig.TLSVersionToGo(*tlsMinVer)
			if err != nil {
				log.Fatalf("invalid --tls-min-version: %v", err)
			}
			opts = append(opts, tlsconfig.WithMinVersion(v))
		}
		opts = append(opts, tlsconfig.WithHTTP2(*enableHTTP2))

		tlsCfg, reloader, err := tlsconfig.NewTLSConfig(*tlsCertFile, *tlsKeyFile, opts...)
		if err != nil {
			log.Fatalf("failed to configure TLS: %v", err)
		}
		server.TLSConfig = tlsCfg
		log.Printf("listening on %v (HTTPS)", *addr)
		err = server.ListenAndServeTLS("", "")
		_ = reloader.Close()
		log.Fatalf("ListenAndServeTLS error: %v", err)
	} else {
		log.Printf("listening on %v (HTTP)", *addr)
		log.Fatalf("ListenAndServe error: %v", server.ListenAndServe())
	}
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

// newAuthHandler wraps an http.Handler with Kubernetes authentication (TokenReview)
// and authorization (SubjectAccessReview) using controller-runtime's
// WithAuthenticationAndAuthorization filter.
func newAuthHandler(handler http.Handler) (http.Handler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("getting in-cluster config: %w", err)
	}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client: %w", err)
	}

	filter, err := filters.WithAuthenticationAndAuthorization(config, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating auth filter: %w", err)
	}

	return filter(stdr.New(log.Default()), handler)
}
