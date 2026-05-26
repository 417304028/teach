package chunk

import (
	"strings"
	"unicode/utf8"
)

type Config struct {
	ChunkSize    int
	ChunkOverlap int
	MinChunkSize int
}

var DefaultConfig = Config{
	ChunkSize:    1200,
	ChunkOverlap: 200,
	MinChunkSize: 100,
}

func ChunkText(text string, cfg Config) []string {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = DefaultConfig.ChunkSize
	}
	if cfg.MinChunkSize <= 0 {
		cfg.MinChunkSize = DefaultConfig.MinChunkSize
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	paragraphs := strings.Split(text, "\n")
	chunks := []string{}
	var current strings.Builder
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(para)+1 > cfg.ChunkSize && current.Len() > 0 {
			chunk := strings.TrimSpace(current.String())
			if utf8.RuneCountInString(chunk) >= cfg.MinChunkSize {
				chunks = append(chunks, chunk)
			}
			current.Reset()
			if cfg.ChunkOverlap > 0 {
				prev := chunk
				runes := []rune(prev)
				overlapLen := cfg.ChunkOverlap
				if overlapLen > len(runes) {
					overlapLen = len(runes)
				}
				current.WriteString(string(runes[len(runes)-overlapLen:]))
			}
		}
		current.WriteString(para)
		current.WriteString("\n")
	}
	if last := strings.TrimSpace(current.String()); last != "" && utf8.RuneCountInString(last) >= cfg.MinChunkSize {
		chunks = append(chunks, last)
	}
	return chunks
}

func EstimateTokens(text string) int {
	runes := utf8.RuneCountInString(text)
	if runes == 0 {
		return 0
	}
	return runes/2 + 1
}
