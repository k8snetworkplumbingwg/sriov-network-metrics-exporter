package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"strings"
)

// DefaultCipherSuites contains GCM-only ciphers with forward secrecy.
var DefaultCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
}

var DefaultCurvePreferences = []tls.CurveID{
	tls.CurveP521,
	tls.CurveP384,
}

const DefaultMinTLSVersion = tls.VersionTLS12

var cipherNameToID = map[string]uint16{
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":               tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":               tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
}

var curveNameToID = map[string]tls.CurveID{
	"X25519":    tls.X25519,
	"CurveP256": tls.CurveP256,
	"CurveP384": tls.CurveP384,
	"CurveP521": tls.CurveP521,
}

var tlsVersionMap = map[string]uint16{
	"VersionTLS12": tls.VersionTLS12,
	"VersionTLS13": tls.VersionTLS13,
}

// CipherNamesToIDs converts IANA-style cipher suite names to Go tls constants.
func CipherNamesToIDs(names []string) ([]uint16, error) {
	ids := make([]uint16, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		id, ok := cipherNameToID[name]
		if !ok {
			return nil, fmt.Errorf("unknown cipher suite: %q", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// CurveNamesToIDs converts curve name strings to Go tls.CurveID values.
func CurveNamesToIDs(names []string) ([]tls.CurveID, error) {
	ids := make([]tls.CurveID, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		id, ok := curveNameToID[name]
		if !ok {
			return nil, fmt.Errorf("unknown curve: %q", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// TLSVersionToGo converts a version string to a Go tls version constant.
func TLSVersionToGo(version string) (uint16, error) {
	v, ok := tlsVersionMap[strings.TrimSpace(version)]
	if !ok {
		return 0, fmt.Errorf("unknown TLS version: %q (valid: VersionTLS12, VersionTLS13)", version)
	}
	return v, nil
}

// NewTLSConfig builds a tls.Config from the provided options, applying defaults
// for any unset values.
func NewTLSConfig(certFile, keyFile string, opts ...Option) (*tls.Config, *KeypairReloader, error) {
	cfg := &tlsOptions{
		cipherSuites:     DefaultCipherSuites,
		curvePreferences: DefaultCurvePreferences,
		minVersion:       DefaultMinTLSVersion,
		enableHTTP2:      false,
	}
	for _, o := range opts {
		o(cfg)
	}

	reloader, err := NewKeypairReloader(certFile, keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("loading TLS keypair: %w", err)
	}

	tlsCfg := &tls.Config{ //nolint:gosec // MinVersion is configurable, defaults to TLS 1.2.
		GetCertificate:   reloader.GetCertificateFunc(),
		CipherSuites:     cfg.cipherSuites,
		CurvePreferences: cfg.curvePreferences,
		MinVersion:       cfg.minVersion,
	}

	if !cfg.enableHTTP2 {
		// Disable HTTP/2 (CVE-2023-39325 mitigation)
		tlsCfg.NextProtos = []string{"http/1.1"}
	}

	return tlsCfg, reloader, nil
}

type tlsOptions struct {
	cipherSuites     []uint16
	curvePreferences []tls.CurveID
	minVersion       uint16
	enableHTTP2      bool
}

// Option configures TLS settings.
type Option func(*tlsOptions)

func WithCipherSuites(suites []uint16) Option {
	return func(o *tlsOptions) { o.cipherSuites = suites }
}

func WithCurvePreferences(curves []tls.CurveID) Option {
	return func(o *tlsOptions) { o.curvePreferences = curves }
}

func WithMinVersion(v uint16) Option {
	return func(o *tlsOptions) { o.minVersion = v }
}

func WithHTTP2(enabled bool) Option {
	return func(o *tlsOptions) { o.enableHTTP2 = enabled }
}
