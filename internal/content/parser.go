package content

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"hermesclaw/internal/model"
)

var lessonPattern = regexp.MustCompile(`第\s*([0-9一二三四五六七八九十]+)\s*讲\s*(.*)`)

func ParseMaterialPath(sourcePath string) model.Material {
	clean := filepath.ToSlash(sourcePath)
	parts := splitPath(clean)
	fileName := ""
	if len(parts) > 0 {
		fileName = parts[len(parts)-1]
	}

	m := model.Material{
		SourcePath:   clean,
		Season:       firstSeason(parts),
		Edition:      firstEdition(parts),
		Track:        firstTrack(parts),
		MaterialKind: materialKind(parts, fileName),
		Version:      version(fileName),
	}

	lessonPart := firstLessonPart(parts)
	m.LessonNo, m.LessonTitle = parseLesson(lessonPart)
	if m.LessonTitle == "" {
		m.LessonTitle = titleFromFile(fileName)
	}
	if m.MaterialKind == "" {
		m.MaterialKind = "讲义"
	}
	if m.Version == "" {
		m.Version = "未知"
	}
	if m.Edition == "" {
		m.Edition = "未知教材"
	}
	if m.Season == "" {
		m.Season = "未知课程"
	}
	return m
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	raw := strings.Split(path, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func firstSeason(parts []string) string {
	for _, part := range parts {
		if strings.HasSuffix(part, "课") {
			return part
		}
	}
	return ""
}

func firstEdition(parts []string) string {
	for _, part := range parts {
		if strings.Contains(part, "教版") || strings.Contains(part, "版") {
			if !strings.Contains(part, "学生版") && !strings.Contains(part, "教师版") && !strings.Contains(part, "答案版") {
				return part
			}
		}
	}
	return ""
}

func firstTrack(parts []string) string {
	for _, part := range parts {
		if part == "复习" || part == "新授" {
			return part
		}
	}
	return ""
}

func firstLessonPart(parts []string) string {
	for _, part := range parts {
		if lessonPattern.MatchString(part) {
			return part
		}
	}
	return ""
}

func parseLesson(part string) (int, string) {
	matches := lessonPattern.FindStringSubmatch(part)
	if len(matches) != 3 {
		return 0, ""
	}
	return parseChineseNumber(matches[1]), strings.TrimSpace(matches[2])
}

func parseChineseNumber(raw string) int {
	raw = strings.TrimSpace(raw)
	if value, err := strconv.Atoi(raw); err == nil {
		return value
	}
	table := map[rune]int{'零': 0, '一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	if raw == "十" {
		return 10
	}
	runes := []rune(raw)
	if len(runes) == 2 && runes[0] == '十' {
		return 10 + table[runes[1]]
	}
	if len(runes) == 2 && runes[1] == '十' {
		return table[runes[0]] * 10
	}
	if len(runes) == 3 && runes[1] == '十' {
		return table[runes[0]]*10 + table[runes[2]]
	}
	return 0
}

func materialKind(parts []string, fileName string) string {
	for _, part := range parts {
		switch {
		case strings.Contains(part, "题集"):
			return "题集"
		case strings.Contains(part, "讲义"):
			return "讲义"
		case strings.Contains(part, "期末复习"):
			return "期末复习"
		}
	}
	if strings.Contains(fileName, "题集") {
		return "题集"
	}
	return ""
}

func version(fileName string) string {
	switch {
	case strings.Contains(fileName, "教师版"):
		return "教师版"
	case strings.Contains(fileName, "学生版"):
		return "学生版"
	case strings.Contains(fileName, "答案版"):
		return "答案版"
	default:
		return ""
	}
}

func titleFromFile(fileName string) string {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	name = strings.ReplaceAll(name, "题集：", "")
	for _, suffix := range []string{"(学生版)", "(教师版)", "(答案版)", "（学生版）", "（教师版）", "（答案版）"} {
		name = strings.ReplaceAll(name, suffix, "")
	}
	name = strings.ReplaceAll(name, " (1)", "")
	return strings.TrimSpace(name)
}
