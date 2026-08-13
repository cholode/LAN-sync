package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"lan-im-go/internal/storage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type fakeStorageProvider struct{}

func (f *fakeStorageProvider) PreSignedUploadURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://minio.test/lan-im-files/" + key + "?X-Amz-Signature=fake", nil
}

func (f *fakeStorageProvider) Save(_ context.Context, key string, _ io.Reader, size int64) (*storage.UploadResult, error) {
	return &storage.UploadResult{Key: key, Size: size}, nil
}

func (f *fakeStorageProvider) GetDownloadURL(_ context.Context, key string) (string, error) {
	return "http://minio.test/lan-im-files/" + key + "?X-Amz-Signature=download-fake", nil
}

func (f *fakeStorageProvider) Delete(_ context.Context, _ string) error { return nil }

func (f *fakeStorageProvider) BackendType() storage.Backend { return storage.BackendMinIO }

func setupTestRouter() *gin.Engine {
	r := gin.New()
	r.GET("/download/*filepath", DownloadFile)

	auth := r.Group("/api/v1")
	auth.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	{
		auth.POST("/files/presign", PreSignUploadHandler)
	}
	return r
}

func TestPreSignUploadHandler(t *testing.T) {
	oldStorage := Storage
	Storage = &fakeStorageProvider{}
	defer func() { Storage = oldStorage }()

	r := setupTestRouter()
	body := `{"filename":"test.png","file_type":"png","file_size":1024}`
	req := httptest.NewRequest("POST", "/api/v1/files/presign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("presign failed: %d %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["upload_url"] == "" {
		t.Fatalf("missing upload_url: %s", w.Body.String())
	}
	if resp["object_key"] == "" {
		t.Fatalf("missing object_key: %s", w.Body.String())
	}
	t.Logf("presign OK: %s", w.Body.String())
}

func TestDownloadFileRedirectsToObjectStorage(t *testing.T) {
	oldStorage := Storage
	Storage = &fakeStorageProvider{}
	defer func() { Storage = oldStorage }()

	r := setupTestRouter()
	req := httptest.NewRequest("GET", "/download/2026-08-13/1/123_test.png", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("download status = %d, want %d: %s", w.Code, http.StatusTemporaryRedirect, w.Body.String())
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "2026-08-13/1/123_test.png") {
		t.Fatalf("unexpected redirect location: %s", location)
	}
	t.Logf("download redirect OK: %s", location)
}

func TestDownloadFileRejectsInvalidKey(t *testing.T) {
	oldStorage := Storage
	Storage = &fakeStorageProvider{}
	defer func() { Storage = oldStorage }()

	r := setupTestRouter()
	req := httptest.NewRequest("GET", "/download/../secret.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusTemporaryRedirect {
		t.Fatalf("invalid key should not redirect: %d", w.Code)
	}
}
