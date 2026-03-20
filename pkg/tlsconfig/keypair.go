package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// KeypairReloader holds a TLS certificate that can be reloaded at runtime.
// It watches the cert and key files for changes using fsnotify, handling
// Kubernetes Secret volume mount symlink rotations.
type KeypairReloader struct {
	certMutex sync.RWMutex
	cert      *tls.Certificate
	certPath  string
	keyPath   string
	watcher   *fsnotify.Watcher
}

// NewKeypairReloader creates a new reloader, loads the initial keypair,
// and starts watching the files for changes.
func NewKeypairReloader(certPath, keyPath string) (*KeypairReloader, error) {
	r := &KeypairReloader{
		certPath: certPath,
		keyPath:  keyPath,
	}
	if err := r.reload(); err != nil {
		return nil, fmt.Errorf("initial certificate load: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}
	r.watcher = watcher

	// Watch the directories containing the cert and key files.
	// This handles Kubernetes Secret mounts where the files are symlinks
	// that get atomically replaced.
	certDir := filepath.Dir(certPath)
	keyDir := filepath.Dir(keyPath)
	if err := watcher.Add(certDir); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("watching cert directory %s: %w", certDir, err)
	}
	if certDir != keyDir {
		if err := watcher.Add(keyDir); err != nil {
			_ = watcher.Close()
			return nil, fmt.Errorf("watching key directory %s: %w", keyDir, err)
		}
	}

	go r.watchLoop()

	return r, nil
}

func (r *KeypairReloader) reload() error {
	cert, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return err
	}
	r.certMutex.Lock()
	r.cert = &cert
	r.certMutex.Unlock()
	return nil
}

func (r *KeypairReloader) watchLoop() {
	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			// Reload on any write or create event in the watched directory.
			// Kubernetes Secret mounts trigger Create events when symlinks are rotated.
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				if err := r.reload(); err != nil {
					log.Printf("failed to reload TLS certificate: %v", err)
				} else {
					log.Printf("TLS certificate reloaded")
				}
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("certificate watcher error: %v", err)
		}
	}
}

// GetCertificateFunc returns a function suitable for tls.Config.GetCertificate.
func (r *KeypairReloader) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
		r.certMutex.RLock()
		defer r.certMutex.RUnlock()
		return r.cert, nil
	}
}

// Close stops the file watcher.
func (r *KeypairReloader) Close() error {
	if r.watcher != nil {
		return r.watcher.Close()
	}
	return nil
}
