package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	service, err := NewService(t.TempDir())
	require.NoError(t, err)
	data := randomBytes(t, 10*1024*1024)
	hash := fmt.Sprintf("%x", sha1.Sum(data))

	t.Run("StoreAndLoad", func(t *testing.T) {
		// Check that a file doesn't exist for our hash yet
		req := httptest.NewRequest("GET", "/load/"+hash, nil)
		rec := httptest.NewRecorder()
		service.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code)

		// Upload the file and check it produces the right hash
		req = httptest.NewRequest("POST", "/store", bytes.NewReader(data))
		rec = httptest.NewRecorder()
		service.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, hash+"\n", rec.Body.String())

		// Check that the file with our hash exists now
		req = httptest.NewRequest("GET", "/load/"+hash, nil)
		rec = httptest.NewRecorder()
		service.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, data, rec.Body.Bytes())
	})
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}
