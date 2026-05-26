package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"hermesclaw/internal/model"
)

type Store interface {
	UpsertMaterial(material model.Material) (model.Material, bool, error)
	AddChunk(chunk model.Chunk) (model.Chunk, error)
	SearchChunks(queryVector []float64, filters model.SearchFilters, limit int) ([]model.SearchResult, error)
	SearchMaterials(query string, filters model.SearchFilters, limit int) ([]model.Material, error)
	ListMaterials() ([]model.Material, error)
	ListChunks() ([]model.Chunk, error)
	CreateJob(job model.Job) (model.Job, error)
	UpdateJob(job model.Job) error
	ListJobs(limit int) ([]model.Job, error)
	AddFile(file model.FileRecord) (model.FileRecord, error)
	GetFile(id string) (model.FileRecord, bool, error)
	ListFiles(limit int) ([]model.FileRecord, error)
	DeleteExpiredFiles(now time.Time) ([]model.FileRecord, error)
	AddMessage(message model.ChatMessage) error
	Stats() model.Stats
}

type JSONStore struct {
	path string
	mu   sync.Mutex
	db   database
}

type database struct {
	Materials map[string]model.Material    `json:"materials"`
	Chunks    map[string]model.Chunk       `json:"chunks"`
	Jobs      map[string]model.Job         `json:"jobs"`
	Files     map[string]model.FileRecord  `json:"files"`
	Messages  map[string]model.ChatMessage `json:"messages"`
}

func OpenJSON(path string) (*JSONStore, error) {
	s := &JSONStore{path: path, db: newDatabase()}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.db); err != nil {
			return nil, err
		}
		s.ensureMaps()
	}
	return s, nil
}

func newDatabase() database {
	return database{
		Materials: map[string]model.Material{},
		Chunks:    map[string]model.Chunk{},
		Jobs:      map[string]model.Job{},
		Files:     map[string]model.FileRecord{},
		Messages:  map[string]model.ChatMessage{},
	}
}

func (s *JSONStore) ensureMaps() {
	if s.db.Materials == nil {
		s.db.Materials = map[string]model.Material{}
	}
	if s.db.Chunks == nil {
		s.db.Chunks = map[string]model.Chunk{}
	}
	if s.db.Jobs == nil {
		s.db.Jobs = map[string]model.Job{}
	}
	if s.db.Files == nil {
		s.db.Files = map[string]model.FileRecord{}
	}
	if s.db.Messages == nil {
		s.db.Messages = map[string]model.ChatMessage{}
	}
}

func (s *JSONStore) saveLocked() error {
	tmp := s.path + ".tmp"
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *JSONStore) UpsertMaterial(material model.Material) (model.Material, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.db.Materials {
		if existing.SHA256 == material.SHA256 && material.SHA256 != "" {
			return existing, false, nil
		}
	}
	now := time.Now()
	if material.ID == "" {
		material.ID = NewID("mat")
	}
	if material.CreatedAt.IsZero() {
		material.CreatedAt = now
	}
	s.db.Materials[material.ID] = material
	return material, true, s.saveLocked()
}

func (s *JSONStore) AddChunk(chunk model.Chunk) (model.Chunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if chunk.ID == "" {
		chunk.ID = NewID("chk")
	}
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now()
	}
	s.db.Chunks[chunk.ID] = chunk
	return chunk, s.saveLocked()
}

func (s *JSONStore) SearchChunks(queryVector []float64, filters model.SearchFilters, limit int) ([]model.SearchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 8
	}
	results := []model.SearchResult{}
	for _, chunk := range s.db.Chunks {
		material, ok := s.db.Materials[chunk.MaterialID]
		if !ok || !matchesFilters(material, filters) {
			continue
		}
		score := cosine(queryVector, chunk.Embedding)
		if score == 0 {
			score = lexicalScore("", chunk.Text, material)
		}
		results = append(results, model.SearchResult{Chunk: chunk, Material: material, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s *JSONStore) SearchMaterials(query string, filters model.SearchFilters, limit int) ([]model.Material, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	type scored struct {
		m     model.Material
		score float64
	}
	scoredMaterials := []scored{}
	for _, material := range s.db.Materials {
		if !matchesFilters(material, filters) {
			continue
		}
		score := lexicalScore(query, material.LessonTitle+" "+material.SourcePath, material)
		if query == "" || score > 0 {
			scoredMaterials = append(scoredMaterials, scored{m: material, score: score})
		}
	}
	sort.Slice(scoredMaterials, func(i, j int) bool {
		if scoredMaterials[i].score == scoredMaterials[j].score {
			return scoredMaterials[i].m.SourcePath < scoredMaterials[j].m.SourcePath
		}
		return scoredMaterials[i].score > scoredMaterials[j].score
	})
	out := make([]model.Material, 0, min(limit, len(scoredMaterials)))
	for i, item := range scoredMaterials {
		if i >= limit {
			break
		}
		out = append(out, item.m)
	}
	return out, nil
}

func (s *JSONStore) ListMaterials() ([]model.Material, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Material, 0, len(s.db.Materials))
	for _, material := range s.db.Materials {
		out = append(out, material)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourcePath < out[j].SourcePath })
	return out, nil
}

func (s *JSONStore) ListChunks() ([]model.Chunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Chunk, 0, len(s.db.Chunks))
	for _, chunk := range s.db.Chunks {
		out = append(out, chunk)
	}
	return out, nil
}

func (s *JSONStore) CreateJob(job model.Job) (model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if job.ID == "" {
		job.ID = NewID("job")
	}
	if job.Status == "" {
		job.Status = model.JobPending
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	s.db.Jobs[job.ID] = job
	return job, s.saveLocked()
}

func (s *JSONStore) UpdateJob(job model.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.UpdatedAt = time.Now()
	s.db.Jobs[job.ID] = job
	return s.saveLocked()
}

func (s *JSONStore) ListJobs(limit int) ([]model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]model.Job, 0, len(s.db.Jobs))
	for _, job := range s.db.Jobs {
		out = append(out, job)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *JSONStore) AddFile(file model.FileRecord) (model.FileRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if file.ID == "" {
		file.ID = NewID("file")
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = now
	}
	s.db.Files[file.ID] = file
	return file, s.saveLocked()
}

func (s *JSONStore) GetFile(id string) (model.FileRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, ok := s.db.Files[id]
	return file, ok, nil
}

func (s *JSONStore) ListFiles(limit int) ([]model.FileRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]model.FileRecord, 0, len(s.db.Files))
	for _, file := range s.db.Files {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *JSONStore) DeleteExpiredFiles(now time.Time) ([]model.FileRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expired := []model.FileRecord{}
	for id, file := range s.db.Files {
		if !file.Pinned && !file.ExpiresAt.IsZero() && now.After(file.ExpiresAt) {
			expired = append(expired, file)
			delete(s.db.Files, id)
		}
	}
	if len(expired) == 0 {
		return nil, nil
	}
	return expired, s.saveLocked()
}

func (s *JSONStore) AddMessage(message model.ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if message.ID == "" {
		message.ID = NewID("msg")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}
	s.db.Messages[message.ID] = message
	return s.saveLocked()
}

func (s *JSONStore) Stats() model.Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.Stats{
		Materials: len(s.db.Materials),
		Chunks:    len(s.db.Chunks),
		Jobs:      len(s.db.Jobs),
		Files:     len(s.db.Files),
		Messages:  len(s.db.Messages),
	}
}

func matchesFilters(material model.Material, filters model.SearchFilters) bool {
	if filters.Season != "" && material.Season != filters.Season {
		return false
	}
	if filters.Edition != "" && material.Edition != filters.Edition {
		return false
	}
	if filters.Track != "" && material.Track != filters.Track {
		return false
	}
	if filters.LessonNo != 0 && material.LessonNo != filters.LessonNo {
		return false
	}
	if filters.MaterialKind != "" && material.MaterialKind != filters.MaterialKind {
		return false
	}
	if filters.Version != "" && material.Version != filters.Version {
		return false
	}
	return true
}

func lexicalScore(query string, text string, material model.Material) float64 {
	query = strings.TrimSpace(strings.ToLower(query))
	text = strings.ToLower(text + " " + material.Season + " " + material.Track + " " + material.MaterialKind + " " + material.Version)
	if query == "" {
		return 0.1
	}
	score := 0.0
	for _, token := range strings.Fields(query) {
		if strings.Contains(text, token) {
			score += 1
		}
	}
	for _, r := range query {
		if r > 127 && strings.ContainsRune(text, r) {
			score += 0.08
		}
	}
	if strings.Contains(text, query) {
		score += 2
	}
	return score
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (sqrt(na) * sqrt(nb))
}

func sqrt(v float64) float64 {
	// Newton's method keeps this package free of math imports in older tiny builds.
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 12; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}

func NewID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "_" + time.Now().Format("20060102150405")
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
