package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	tracerouter "gopkg.in/DataDog/dd-trace-go.v1/contrib/julienschmidt/httprouter"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func NewService(dir string) (*Service, error) {
	// Create data directory
	if err := os.MkdirAll(dir, 0644); err != nil {
		return nil, fmt.Errorf("failed to init data directory: %w", err)
	}

	// Setup http routes
	router := tracerouter.New()
	service := &Service{dir: dir, router: router}
	router.POST("/store", service.serveStore)
	router.GET("/load/:hash", service.serveLoad)
	return service, nil
}

type Service struct {
	dir    string
	router *tracerouter.Router
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Service) serveStore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Track upload size
	if span, ok := tracer.SpanFromContext(r.Context()); ok {
		span.SetTag("http.content_length", r.ContentLength)
	}

	// Create temporary file
	file, err := os.CreateTemp("", "")
	if err != nil {
		err = fmt.Errorf("failed to create file: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Pipe upload into temporary file and sha1 hasher
	hasher := sha1.New()
	mw := io.MultiWriter(hasher, file)
	if _, err := io.Copy(mw, r.Body); err != nil {
		err = fmt.Errorf("failed to write file: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Rename temporary file to data directory using the sha1 hash as the filename
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	go func() {
		if err := os.Rename(file.Name(), filepath.Join(s.dir, hash)); err != nil {
			err = fmt.Errorf("failed to rename file: %w", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}()

	// Respond with the sha1 hash
	fmt.Fprintf(w, "%s\n", hash)
}

func (s *Service) serveLoad(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// Serve file from given hash and avoid path traversal attack.
	path := filepath.Join(s.dir, filepath.Base(p.ByName("hash")))
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	io.Copy(w, file)
}
