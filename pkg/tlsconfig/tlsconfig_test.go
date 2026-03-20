package tlsconfig

import (
	"crypto/tls"
	"testing"
)

func TestCipherNamesToIDs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []uint16
		wantErr bool
	}{
		{
			name:  "valid ciphers",
			input: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
		},
		{
			name:  "with whitespace",
			input: []string{" TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 "},
			want:  []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		{
			name:    "unknown cipher",
			input:   []string{"TLS_UNKNOWN_CIPHER"},
			wantErr: true,
		},
		{
			name:  "empty input",
			input: []string{},
			want:  []uint16{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CipherNamesToIDs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CipherNamesToIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("CipherNamesToIDs() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("CipherNamesToIDs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCurveNamesToIDs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []tls.CurveID
		wantErr bool
	}{
		{
			name:  "valid curves",
			input: []string{"CurveP521", "CurveP384"},
			want:  []tls.CurveID{tls.CurveP521, tls.CurveP384},
		},
		{
			name:  "X25519",
			input: []string{"X25519"},
			want:  []tls.CurveID{tls.X25519},
		},
		{
			name:    "unknown curve",
			input:   []string{"UnknownCurve"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CurveNamesToIDs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CurveNamesToIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("CurveNamesToIDs() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("CurveNamesToIDs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTLSVersionToGo(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint16
		wantErr bool
	}{
		{name: "TLS 1.2", input: "VersionTLS12", want: tls.VersionTLS12},
		{name: "TLS 1.3", input: "VersionTLS13", want: tls.VersionTLS13},
		{name: "with whitespace", input: " VersionTLS12 ", want: tls.VersionTLS12},
		{name: "unknown", input: "VersionTLS11", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TLSVersionToGo(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TLSVersionToGo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("TLSVersionToGo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultCipherSuitesAreGCMOnly(t *testing.T) {
	for _, id := range DefaultCipherSuites {
		suite := cipherSuiteByID(id)
		if suite == nil {
			t.Errorf("default cipher suite %d not found in Go's tls package", id)
			continue
		}
		// All default ciphers should use ECDHE (forward secrecy)
		if suite.Name[:len("TLS_ECDHE")] != "TLS_ECDHE" {
			t.Errorf("default cipher %s does not use ECDHE", suite.Name)
		}
	}
}

func cipherSuiteByID(id uint16) *tls.CipherSuite {
	for _, s := range tls.CipherSuites() {
		if s.ID == id {
			return s
		}
	}
	return nil
}
