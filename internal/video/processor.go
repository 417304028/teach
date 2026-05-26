package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TaskType string

const (
	TaskTranscribe TaskType = "transcribe"
	TaskSummarize  TaskType = "summarize"
	TaskClip       TaskType = "clip"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusSucceeded TaskStatus = "succeeded"
	StatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID         string     `json:"id"`
	Type       TaskType   `json:"type"`
	Status     TaskStatus `json:"status"`
	InputPath  string     `json:"input_path"`
	OutputPath string     `json:"output_path,omitempty"`
	Params     TaskParams `json:"params,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt time.Time  `json:"finished_at,omitempty"`
	Progress   int        `json:"progress"`
}

type TaskParams struct {
	Language       string `json:"language,omitempty"`
	Model          string `json:"model,omitempty"`
	StartTime      string `json:"start_time,omitempty"`
	EndTime        string `json:"end_time,omitempty"`
	ClipDuration   string `json:"clip_duration,omitempty"`
	SummaryLength  int    `json:"summary_length,omitempty"`
	WhisperModel   string `json:"whisper_model,omitempty"`
	FFmpegPreset   string `json:"ffmpeg_preset,omitempty"`
}

type Processor struct {
	dataDir    string
	whisperCmd string
}

func New(dataDir string) *Processor {
	return &Processor{
		dataDir:    dataDir,
		whisperCmd: findWhisperCmd(),
	}
}

func findWhisperCmd() string {
	paths := []string{"whisper", "whisper.exe", "C:\\whisper.exe", "/usr/local/bin/whisper"}
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return "whisper"
}

type ProcessResult struct {
	Task     Task
	Subtitles string
	Summary  string
	Clips    []string
}

func (p *Processor) Process(ctx context.Context, task Task) (ProcessResult, error) {
	switch task.Type {
	case TaskTranscribe:
		return p.transcribe(ctx, task)
	case TaskSummarize:
		return p.summarize(ctx, task)
	case TaskClip:
		return p.clip(ctx, task)
	default:
		return ProcessResult{}, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func (p *Processor) transcribe(ctx context.Context, task Task) (ProcessResult, error) {
	result := ProcessResult{Task: task}

	if p.whisperCmd == "" || !isCommandAvailable(p.whisperCmd) {
		return result, fmt.Errorf("whisper command not found: install whisper or set PATH")
	}

	args := []string{
		"--model", paramOr(task.Params.WhisperModel, "base"),
		"--language", paramOr(task.Params.Language, "zh"),
		"--output_dir", p.dataDir,
		"--output_format", "srt",
		"--fp16", "False",
		task.InputPath,
	}

	cmd := exec.CommandContext(ctx, p.whisperCmd, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("whisper failed: %w, output: %s", err, string(output))
	}

	baseName := strings.TrimSuffix(filepath.Base(task.InputPath), filepath.Ext(task.InputPath))
	srtPath := filepath.Join(p.dataDir, baseName+".srt")
	if data, err := osReadFile(srtPath); err == nil {
		result.Subtitles = string(data)
	}

	task.Status = StatusSucceeded
	task.OutputPath = srtPath
	task.FinishedAt = time.Now()
	task.Progress = 100
	result.Task = task
	return result, nil
}

func (p *Processor) summarize(ctx context.Context, task Task) (ProcessResult, error) {
	result := ProcessResult{Task: task}

	subtitles, err := osReadFile(task.InputPath)
	if err != nil {
		return result, fmt.Errorf("failed to read subtitles: %w", err)
	}

	lines := strings.Split(string(subtitles), "\n")
	var sb strings.Builder
	count := 0
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "-->") && !isNumericLine(line) {
			sb.WriteString(strings.TrimSpace(line))
			sb.WriteString(" ")
			count++
			if count > 200 {
				break
			}
		}
	}

	summaryLen := task.Params.SummaryLength
	if summaryLen <= 0 {
		summaryLen = 300
	}
	text := strings.TrimSpace(sb.String())
	if len(text) > 2000 {
		text = text[:2000]
	}
	result.Summary = text

	task.Status = StatusSucceeded
	task.FinishedAt = time.Now()
	task.Progress = 100
	result.Task = task
	return result, nil
}

func (p *Processor) clip(ctx context.Context, task Task) (ProcessResult, error) {
	result := ProcessResult{Task: task}

	if !isCommandAvailable("ffmpeg") {
		return result, fmt.Errorf("ffmpeg not found: install ffmpeg and add to PATH")
	}

	start := task.Params.StartTime
	if start == "" {
		start = "00:00:00"
	}
	duration := task.Params.ClipDuration
	if duration == "" {
		duration = "60"
	}

	outputName := fmt.Sprintf("clip_%s_%s.mp4", task.ID, time.Now().Format("150405"))
	outputPath := filepath.Join(p.dataDir, "clips", outputName)

	args := []string{
		"-y",
		"-ss", start,
		"-i", task.InputPath,
		"-t", duration,
		"-c:v", "libx264",
		"-preset", paramOr(task.Params.FFmpegPreset, "fast"),
		"-c:a", "aac",
		"-b:a", "128k",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if _, err := cmd.CombinedOutput(); err != nil {
		return result, fmt.Errorf("ffmpeg clip failed: %w", err)
	}

	task.Status = StatusSucceeded
	task.OutputPath = outputPath
	task.FinishedAt = time.Now()
	task.Progress = 100
	result.Task = task
	result.Clips = []string{outputPath}
	return result, nil
}

func (p *Processor) BatchProcess(ctx context.Context, tasks []Task) []ProcessResult {
	results := make([]ProcessResult, len(tasks))
	for i, task := range tasks {
		result, err := p.Process(ctx, task)
		if err != nil {
			result.Task.Status = StatusFailed
			result.Task.Error = err.Error()
		}
		results[i] = result
	}
	return results
}

func TaskToJSON(task Task) string {
	data, _ := json.Marshal(task)
	return string(data)
}

func paramOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func isNumericLine(line string) bool {
	line = strings.TrimSpace(line)
	for _, r := range line {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(line) > 0
}

func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func osReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
