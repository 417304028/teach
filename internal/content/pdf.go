package content

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

var literalStringPattern = regexp.MustCompile(`\(([^()\r\n]{2,300})\)`)

func ExtractPDFTextBestEffort(data []byte) string {
	if len(data) == 0 || !strings.HasPrefix(string(data), "%PDF") {
		return ""
	}
	return extractPDFText(data)
}

func SanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "")
}

func extractPDFText(data []byte) string {
	text, good := extractPDFReader(data)
	if good {
		return text
	}
	pyText := extractPDFWithPython(data)
	if pyText != "" {
		return pyText
	}
	return extractPDFLiteral(data)
}

func extractPDFReader(data []byte) (string, bool) {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return "", false
	}
	r := bytes.NewReader(data)
	reader, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return "", false
	}

	var out strings.Builder
	totalPages := reader.NumPage()
	if totalPages == 0 {
		return "", false
	}

	goodChars := 0
	totalChars := 0

	for i := 1; i <= totalPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		cleaned := cleanPDFText(content)
		if len(cleaned) > 0 {
			out.WriteString(cleaned)
			out.WriteString("\n")
			for _, r := range cleaned {
				totalChars++
				if isReadableChar(r) {
					goodChars++
				}
			}
		}
	}

	result := out.String()
	if totalChars == 0 {
		return result, false
	}
	goodRatio := float64(goodChars) / float64(totalChars)
	return result, goodRatio > 0.3
}

func cleanPDFText(text string) string {
	var b strings.Builder
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if isNoiseLine(line) {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func isNoiseLine(line string) bool {
	if len(line) < 3 {
		return true
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, "chromium") || strings.Contains(lower, "agpl") ||
		strings.Contains(lower, "adobereader") || strings.Contains(lower, "http://") ||
		strings.Contains(lower, "https://") {
		return true
	}
	readable := 0
	for _, r := range line {
		if isReadableChar(r) {
			readable++
		}
	}
	if len(line) > 0 && float64(readable)/float64(len(line)) < 0.3 {
		return true
	}
	return false
}

func isReadableChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) ||
		unicode.Is(unicode.Han, r) ||
		unicode.IsPunct(r) ||
		r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
		(r >= 0x3000 && r <= 0x9FFF) ||
		(r >= 0xFF00 && r <= 0xFFEF)
}

func extractPDFLiteral(data []byte) string {
	matches := literalStringPattern.FindAllSubmatch(data, 2000)
	var out strings.Builder
	for _, match := range matches {
		text := strings.TrimSpace(string(match[1]))
		if looksUseful(text) {
			text = strings.ReplaceAll(text, `\(`, "(")
			text = strings.ReplaceAll(text, `\)`, ")")
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func looksUseful(text string) bool {
	if len(text) < 4 {
		return false
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "http") || strings.Contains(lower, "adobe") {
		return false
	}
	if strings.HasPrefix(text, "_") && strings.HasSuffix(text, "_") {
		return false
	}
	readable := 0
	for _, r := range text {
		if r > 127 || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			readable++
		}
	}
	return readable >= 3
}

func ReadZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func extractPDFWithPython(data []byte) string {
	tmpDir, err := os.MkdirTemp("", "hermesclaw_pdf_")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return ""
	}

	scriptPath := findPdfExtractScript()
	if scriptPath == "" {
		return ""
	}

	cmd := exec.Command("python", scriptPath, tmpFile, "--max-pages", "50")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	var result struct {
		Text     string  `json:"text"`
		Error    string  `json:"error"`
		Fallback bool    `json:"fallback"`
		Ratio    float64 `json:"ratio"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return ""
	}
	if result.Error != "" && !result.Fallback {
		return ""
	}
	return strings.TrimSpace(result.Text)
}

func findPdfExtractScript() string {
	candidates := []string{
		"tools/pdf_extract.py",
		"../tools/pdf_extract.py",
		"../../tools/pdf_extract.py",
		filepath.Join("tools", "pdf_extract.py"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "tools", "pdf_extract.py"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "tools", "pdf_extract.py"))
	}
	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}
	absCandidates := []string{
		filepath.Join("D:", "study", "teach", "teach", "tools", "pdf_extract.py"),
	}
	for _, path := range absCandidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func SanitizePythonOutput(text string) string {
	return fmt.Sprintf("%s", text)
}

func TruncateString(s string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 3000
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "...(已截断)"
}

func CountChineseChars(s string) int {
	count := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || (r >= 0x3000 && r <= 0x9FFF) {
			count++
		}
	}
	return count
}

func TrimNonReadable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == utf8.RuneError {
			continue
		}
		if isReadableChar(r) || r == '\n' || r == '\r' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
