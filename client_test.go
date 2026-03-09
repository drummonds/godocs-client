package godocsclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newDemoServer returns an httptest.Server that mimics the godocs API.
func newDemoServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/document/upload", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		destPath := r.FormValue("path")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(UploadResult{
			Path: destPath,
			ULID: "01JTEST000000000000000000",
			Name: header.Filename,
			Hash: "abc123",
			ID:   1,
		})
	})

	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Tag{
			{ID: 1, Name: "invoice", Color: "#3498db", Description: ""},
			{ID: 2, Name: "receipt", Color: "#e74c3c", Description: ""},
		})
	})

	mux.HandleFunc("POST /api/tags", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Tag{ID: 99, Name: req["name"], Color: req["color"]})
	})

	mux.HandleFunc("POST /api/documents/{ulid}/tags", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("PUT /api/document/{ulid}/metadata", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(mux)
}

func TestUpload(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	// Create a temp file to upload
	tmp := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(tmp, []byte("fake pdf content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := c.Upload(tmp, "inbox/test.pdf")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.ULID == "" {
		t.Error("expected non-empty ULID")
	}
	if result.Path != "inbox/test.pdf" {
		t.Errorf("path = %q, want %q", result.Path, "inbox/test.pdf")
	}
	if result.Name != "test.pdf" {
		t.Errorf("name = %q, want %q", result.Name, "test.pdf")
	}
}

func TestUploadBytes(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	result, err := c.UploadBytes([]byte("hello world"), "hello.txt", "docs/hello.txt")
	if err != nil {
		t.Fatalf("UploadBytes: %v", err)
	}
	if result.Name != "hello.txt" {
		t.Errorf("name = %q, want %q", result.Name, "hello.txt")
	}
	if result.Path != "docs/hello.txt" {
		t.Errorf("path = %q, want %q", result.Path, "docs/hello.txt")
	}
}

func TestUploadDuplicate(t *testing.T) {
	// Server that always returns 409 Conflict
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(UploadResult{
			ULID: "01JEXISTING0000000000000",
			Name: "dup.pdf",
			Hash: "existinghash",
			ID:   42,
		})
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	result, err := c.UploadBytes([]byte("dup"), "dup.pdf", "")
	if err != nil {
		t.Fatalf("UploadBytes duplicate: %v", err)
	}
	if !result.Duplicate {
		t.Error("expected Duplicate=true")
	}
	if result.ID != 42 {
		t.Errorf("ID = %d, want 42", result.ID)
	}
}

func TestGetTags(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	tags, err := c.GetTags()
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(tags))
	}
	if tags[0].Name != "invoice" {
		t.Errorf("tags[0].Name = %q, want %q", tags[0].Name, "invoice")
	}
}

func TestCreateTag(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	tag, err := c.CreateTag("new-tag")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if tag.Name != "new-tag" {
		t.Errorf("Name = %q, want %q", tag.Name, "new-tag")
	}
	if tag.ID != 99 {
		t.Errorf("ID = %d, want 99", tag.ID)
	}
}

func TestEnsureTag_Existing(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	id, err := c.EnsureTag("invoice")
	if err != nil {
		t.Fatalf("EnsureTag: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}

func TestEnsureTag_New(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	id, err := c.EnsureTag("brand-new")
	if err != nil {
		t.Fatalf("EnsureTag: %v", err)
	}
	if id != 99 {
		t.Errorf("id = %d, want 99", id)
	}
}

func TestEnsureTag_Cached(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	id1, _ := c.EnsureTag("invoice")
	id2, _ := c.EnsureTag("invoice")
	if id1 != id2 {
		t.Errorf("cache miss: %d != %d", id1, id2)
	}
}

func TestAddTag(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	if err := c.AddTag("01JTEST000000000000000000", 1); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
}

func TestUpdateMetadata(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	author := "test-author"
	now := time.Now()
	err := c.UpdateMetadata("01JTEST000000000000000000", MetadataUpdate{
		Author:      &author,
		CreatedDate: &now,
	})
	if err != nil {
		t.Fatalf("UpdateMetadata: %v", err)
	}
}

func TestUploadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	_, err := c.UploadBytes([]byte("data"), "f.txt", "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestUploadFileNotFound(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()
	c := NewClient(srv.URL)

	_, err := c.Upload("/nonexistent/file.pdf", "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestServerDown(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // nothing listening

	_, err := c.UploadBytes([]byte("data"), "f.txt", "")
	if err == nil {
		t.Fatal("expected connection error")
	}

	_, err = c.GetTags()
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestAddTag_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	err := c.AddTag("01JBADULID", 1)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCreateTag_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "conflict", http.StatusConflict)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	_, err := c.CreateTag("dup-tag")
	if err == nil {
		t.Fatal("expected error for non-201")
	}
}

func TestUpdateMetadata_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	err := c.UpdateMetadata("01JBAD", MetadataUpdate{})
	if err == nil {
		t.Fatal("expected error for 400")
	}
}

// TestDemoServerRequestValidation verifies the demo server echoes back
// what we send, useful for catching regressions in the test harness itself.
func TestDemoServerRequestValidation(t *testing.T) {
	srv := newDemoServer()
	defer srv.Close()

	// Verify upload echoes the dest path
	c := NewClient(srv.URL)
	res, err := c.UploadBytes([]byte("x"), "a.txt", "custom/path/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != "custom/path/a.txt" {
		t.Errorf("path = %q, want %q", res.Path, "custom/path/a.txt")
	}

	// Verify tags endpoint returns fixture data
	resp, err := c.HTTPClient.Get(srv.URL + "/api/tags")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tags []Tag
	json.Unmarshal(body, &tags)
	if len(tags) != 2 {
		t.Errorf("fixture tags count = %d, want 2", len(tags))
	}
}
