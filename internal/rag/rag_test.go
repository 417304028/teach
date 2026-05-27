package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hermesclaw/internal/model"
)

type mockEmbedder struct{}

func (m *mockEmbedder) Dimensions() int { return 1024 }

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, 1024)
		for j := range vec {
			vec[j] = 0.01 * float64(i+j)
		}
		result[i] = vec
	}
	return result, nil
}

type mockStore struct {
	materials []model.Material
	chunks    []model.Chunk
	searched  []model.SearchResult
}

func (m *mockStore) AddMessage(msg model.ChatMessage) error     { return nil }
func (m *mockStore) CreateJob(job model.Job) (model.Job, error) { return job, nil }
func (m *mockStore) UpdateJob(job model.Job) error              { return nil }
func (m *mockStore) ListJobs(limit int) ([]model.Job, error)    { return nil, nil }
func (m *mockStore) GetFile(id string) (model.FileRecord, bool, error) {
	return model.FileRecord{}, false, nil
}
func (m *mockStore) AddFile(file model.FileRecord) (model.FileRecord, error)      { return file, nil }
func (m *mockStore) ListFiles(limit int) ([]model.FileRecord, error)              { return nil, nil }
func (m *mockStore) DeleteExpiredFiles(now time.Time) ([]model.FileRecord, error) { return nil, nil }
func (m *mockStore) UpsertMaterial(material model.Material) (model.Material, bool, error) {
	for _, existing := range m.materials {
		if existing.SHA256 == material.SHA256 {
			return existing, false, nil
		}
	}
	m.materials = append(m.materials, material)
	return material, true, nil
}
func (m *mockStore) AddChunk(chunk model.Chunk) (model.Chunk, error) {
	m.chunks = append(m.chunks, chunk)
	return chunk, nil
}
func (m *mockStore) SearchChunks(queryVector []float64, filters model.SearchFilters, limit int) ([]model.SearchResult, error) {
	return m.searched, nil
}
func (m *mockStore) SearchMaterials(query string, filters model.SearchFilters, limit int) ([]model.Material, error) {
	return m.materials, nil
}
func (m *mockStore) ListMaterials() ([]model.Material, error) { return m.materials, nil }
func (m *mockStore) ListChunks() ([]model.Chunk, error)       { return m.chunks, nil }
func (m *mockStore) Stats() model.Stats                       { return model.Stats{} }

func TestIngestPath_DirNotFound(t *testing.T) {
	svc := Service{Store: &mockStore{}, Embedder: &mockEmbedder{}}
	_, err := svc.IngestPath(context.Background(), "/nonexistent/directory")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestIngestPath_ZipNotFound(t *testing.T) {
	svc := Service{Store: &mockStore{}, Embedder: &mockEmbedder{}}
	_, err := svc.IngestPath(context.Background(), "/nonexistent.zip")
	if err == nil {
		t.Error("expected error for nonexistent zip")
	}
}

func TestIngestPath_UnsupportedFile(t *testing.T) {
	dir := t.TempDir()
	unsupportedFile := filepath.Join(dir, "test_unsupported.txt")
	if err := os.WriteFile(unsupportedFile, []byte("not a pdf or zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{Store: &mockStore{}, Embedder: &mockEmbedder{}}
	_, err := svc.IngestPath(context.Background(), unsupportedFile)
	if err == nil {
		t.Error("expected error for unsupported file type")
	}
	if err != nil && !containsString(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got: %v", err)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSearch_FallbackToMaterials(t *testing.T) {
	store := &mockStore{
		materials: []model.Material{
			{LessonTitle: "动能定理", Season: "春季课"},
		},
		searched: []model.SearchResult{},
	}
	svc := Service{
		Store:     store,
		Embedder:  &mockEmbedder{},
		Threshold: 0.8,
	}
	results, err := svc.Search(context.Background(), "动能定理", model.SearchFilters{}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected fallback results from materials")
	}
}

func TestSearch_WithFilters(t *testing.T) {
	store := &mockStore{
		materials: []model.Material{
			{LessonTitle: "动能定理", Season: "春季课", LessonNo: 10},
		},
		searched: []model.SearchResult{},
	}
	svc := Service{
		Store:     store,
		Embedder:  &mockEmbedder{},
		Threshold: 0.8,
	}
	results, err := svc.Search(context.Background(), "动能", model.SearchFilters{Season: "春季课", LessonNo: 10}, 5)
	if err != nil {
		t.Fatal(err)
	}
	_ = results
}

func TestSearch_Limit(t *testing.T) {
	store := &mockStore{
		searched: []model.SearchResult{
			{Material: model.Material{LessonTitle: "A"}, Score: 0.9},
			{Material: model.Material{LessonTitle: "B"}, Score: 0.8},
			{Material: model.Material{LessonTitle: "C"}, Score: 0.7},
			{Material: model.Material{LessonTitle: "D"}, Score: 0.6},
		},
	}
	svc := Service{
		Store:     store,
		Embedder:  &mockEmbedder{},
		Threshold: 0.1,
	}
	results, err := svc.Search(context.Background(), "测试", model.SearchFilters{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results after limit, got %d", len(results))
	}
}
