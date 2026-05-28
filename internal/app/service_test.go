package app

import (
	"context"
	"testing"

	"hermesclaw/internal/intent"
	"hermesclaw/internal/model"
)

type mockStore struct {
	messages     []model.ChatMessage
	jobs         []model.Job
	lastJobID    string
	lastJobFile  string
}

func (m *mockStore) AddMessage(msg model.ChatMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockStore) CreateJob(job model.Job) (model.Job, error) {
	job.ID = "job-test-1"
	m.jobs = append(m.jobs, job)
	m.lastJobID = job.ID
	return job, nil
}

func (m *mockStore) UpdateJob(job model.Job) error {
	for i := range m.jobs {
		if m.jobs[i].ID == job.ID {
			m.jobs[i] = job
			break
		}
	}
	return nil
}

func (m *mockStore) ListJobs(limit int) ([]model.Job, error)  { return m.jobs, nil }
func (m *mockStore) GetFile(id string) (model.FileRecord, bool, error) {
	return model.FileRecord{ID: id, Name: "test.pptx", Path: "/tmp/test.pptx"}, true, nil
}
func (m *mockStore) AddFile(file model.FileRecord) (model.FileRecord, error) {
	file.ID = "file-test-1"
	m.lastJobFile = file.ID
	return file, nil
}
func (m *mockStore) ListFiles(limit int) ([]model.FileRecord, error) { return nil, nil }
func (m *mockStore) DeleteExpiredFiles(now time.Time) ([]model.FileRecord, error) { return nil, nil }
func (m *mockStore) SearchChunks(queryVector []float64, filters model.SearchFilters, limit int) ([]model.SearchResult, error) { return nil, nil }
func (m *mockStore) SearchMaterials(query string, filters model.SearchFilters, limit int) ([]model.Material, error) { return nil, nil }
func (m *mockStore) UpsertMaterial(material model.Material) (model.Material, bool, error) { return material, true, nil }
func (m *mockStore) AddChunk(chunk model.Chunk) (model.Chunk, error) { return chunk, nil }
func (m *mockStore) ListMaterials() ([]model.Material, error) { return nil, nil }
func (m *mockStore) ListChunks() ([]model.Chunk, error) { return nil, nil }
func (m *mockStore) Stats() model.Stats { return model.Stats{} }

func TestRequestFromIntent(t *testing.T) {
	result := model.IntentResult{
		Intent:   model.IntentPPT,
		Topic:    "动能定理",
		Season:   "春季课",
		LessonNo: 10,
		Pages:    12,
		Count:    5,
	}
	req := requestFromIntent(result, "user1", "生成PPT")
	if req.Topic != "动能定理" {
		t.Errorf("Topic = %q", req.Topic)
	}
	if req.Pages != 12 {
		t.Errorf("Pages = %d", req.Pages)
	}
	if req.Filters.Season != "春季课" {
		t.Errorf("Filters.Season = %q", req.Filters.Season)
	}
	if req.Filters.LessonNo != 10 {
		t.Errorf("Filters.LessonNo = %d", req.Filters.LessonNo)
	}
}

func TestRequestFromIntent_FallbackToOriginal(t *testing.T) {
	result := model.IntentResult{
		Intent: model.IntentChat,
		Topic:  "",
	}
	req := requestFromIntent(result, "user1", "动能定理是什么")
	if req.Topic != "动能定理是什么" {
		t.Errorf("Topic should fall back to original text, got %q", req.Topic)
	}
}

func TestFormatSearchResults_Empty(t *testing.T) {
	result := formatSearchResults(nil)
	if result != "未检索到相关课程资料。" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestFormatSearchResults_WithData(t *testing.T) {
	results := []model.SearchResult{
		{
			Material: model.Material{SourcePath: "春季课/人教版/第10讲/讲义.pdf", MaterialKind: "讲义", Version: "教师版"},
			Score:    0.85,
		},
		{
			Material: model.Material{SourcePath: "春季课/人教版/第10讲/题集.pdf", MaterialKind: "题集", Version: "学生版"},
			Score:    0.72,
		},
	}
	result := formatSearchResults(results)
	if len(results) == 0 {
		t.Error("expected non-empty result")
	}
	if !containsString(result, "春季课") {
		t.Error("result missing source path")
	}
	if !containsString(result, "相似度") {
		t.Error("result missing similarity score")
	}
}

func TestHandleMessage_UploadIntent(t *testing.T) {
	svc := Service{
		Intent: intent.New(nil, nil),
	}
	resp, err := svc.HandleMessage(context.Background(), "user1", "test", "上传春季课资料")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Intent.Intent != model.IntentUpload {
		t.Errorf("Intent = %s", resp.Intent.Intent)
	}
}

func TestHandleMessage_SearchIntent(t *testing.T) {
	svc := Service{
		Intent: intent.New(nil, nil),
	}
	resp, err := svc.HandleMessage(context.Background(), "user1", "test", "搜索动能定理相关内容")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Intent.Intent != model.IntentSearch {
		t.Errorf("Intent = %s", resp.Intent.Intent)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
