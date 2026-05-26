package content

import (
	"strings"
	"unicode/utf8"
)

func BuildFallbackText(mTitle, sourcePath, kind, version string) string {
	parts := []string{}
	if mTitle != "" {
		parts = append(parts, "主题："+mTitle)
	}
	if kind != "" {
		parts = append(parts, "资料类型："+kind)
	}
	if version != "" {
		parts = append(parts, "版本："+version)
	}
	if sourcePath != "" {
		parts = append(parts, "来源："+sourcePath)
	}
	return strings.Join(parts, "\n")
}

func ChunkText(text string, maxRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxRunes <= 0 {
		maxRunes = 1200
	}
	paragraphs := strings.Split(text, "\n")
	chunks := []string{}
	var current strings.Builder
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(para)+1 > maxRunes && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}
		current.WriteString(para)
		current.WriteString("\n")
	}
	if strings.TrimSpace(current.String()) != "" {
		chunks = append(chunks, strings.TrimSpace(current.String()))
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
