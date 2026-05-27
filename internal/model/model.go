package model

import "time"

type Intent string

const (
	IntentChat        Intent = "chat"
	IntentMindmap     Intent = "mindmap"
	IntentPPT         Intent = "ppt"
	IntentExercises   Intent = "exercises"
	IntentOutline     Intent = "outline"
	IntentSearch      Intent = "search"
	IntentUpload      Intent = "upload"
	IntentGame        Intent = "game"
	IntentUnknown     Intent = "unknown"
	IntentGenerateAll Intent = "generate_all"
)

type IntentResult struct {
	Intent             Intent            `json:"intent"`
	Confidence         float64           `json:"confidence"`
	NeedsClarification bool              `json:"needs_clarification"`
	Clarification      string            `json:"clarification,omitempty"`
	Topic              string            `json:"topic,omitempty"`
	Season             string            `json:"season,omitempty"`
	Edition            string            `json:"edition,omitempty"`
	Track              string            `json:"track,omitempty"`
	LessonNo           int               `json:"lesson_no,omitempty"`
	Pages              int               `json:"pages,omitempty"`
	Count              int               `json:"count,omitempty"`
	Style              string            `json:"style,omitempty"`
	Params             map[string]string `json:"params,omitempty"`
}

type Material struct {
	ID           string    `json:"id"`
	Season       string    `json:"season"`
	Edition      string    `json:"edition"`
	Track        string    `json:"track"`
	LessonNo     int       `json:"lesson_no"`
	LessonTitle  string    `json:"lesson_title"`
	MaterialKind string    `json:"material_kind"`
	Version      string    `json:"version"`
	SourcePath   string    `json:"source_path"`
	StoredPath   string    `json:"stored_path"`
	SHA256       string    `json:"sha256"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
}

type Chunk struct {
	ID         string    `json:"id"`
	MaterialID string    `json:"material_id"`
	Text       string    `json:"text"`
	Page       int       `json:"page"`
	TokenCount int       `json:"token_count"`
	Embedding  []float64 `json:"embedding"`
	CreatedAt  time.Time `json:"created_at"`
}

type SearchFilters struct {
	Season       string `json:"season,omitempty"`
	Edition      string `json:"edition,omitempty"`
	Track        string `json:"track,omitempty"`
	LessonNo     int    `json:"lesson_no,omitempty"`
	MaterialKind string `json:"material_kind,omitempty"`
	Version      string `json:"version,omitempty"`
}

type SearchResult struct {
	Chunk    Chunk    `json:"chunk"`
	Material Material `json:"material"`
	Score    float64  `json:"score"`
}

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID        string            `json:"id"`
	Type      Intent            `json:"type"`
	Status    JobStatus         `json:"status"`
	Message   string            `json:"message,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
	Params    map[string]string `json:"params,omitempty"`
	FileID    string            `json:"file_id,omitempty"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type FileRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	ExpiresAt time.Time `json:"expires_at"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Channel   string    `json:"channel"`
	Message   string    `json:"message"`
	Response  string    `json:"response"`
	Intent    Intent    `json:"intent"`
	CreatedAt time.Time `json:"created_at"`
}

type Stats struct {
	Materials int `json:"materials"`
	Chunks    int `json:"chunks"`
	Jobs      int `json:"jobs"`
	Files     int `json:"files"`
	Messages  int `json:"messages"`
}
