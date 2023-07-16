package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"
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
	if err := MultiCopy(r.Body, hasher, file); err != nil {
		err = fmt.Errorf("failed to hash or write to file: %w", err)
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

// MultiCopy copies data from r to all writers. Both reads and writes are done
// concurrently while trying to minimize buffer sizes.
func MultiCopy(r io.Reader, writers ...io.Writer) error {
	var write = make([]chan []byte, len(writers))
	for i := 0; i < len(writers); i++ {
		write[i] = make(chan []byte, 10)
	}

	var eg errgroup.Group
	for i := 0; i < len(writers); i++ {
		i := i
		eg.Go(func() error {
			for buf := range write[i] {
				if _, err := writers[i].Write(buf); err != nil {
					return err
				}
			}
			return nil
		})
	}

	for {
		// Read into the buf
		buf := make([]byte, 32*1024)
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Dispatch buf to all writers
		for i := 0; i < len(writers); i++ {
			write[i] <- buf[0:n]
		}
	}

	// Signal all writers to finish and wait for them
	for i := 0; i < len(writers); i++ {
		close(write[i])
	}
	return eg.Wait()
}
