package content

import (
	"archive/zip"
	"io"
	"regexp"
	"strings"
)

var literalStringPattern = regexp.MustCompile(`\(([^()\r\n]{2,300})\)`)

func ExtractPDFTextBestEffort(data []byte) string {
	if len(data) == 0 || !strings.HasPrefix(string(data), "%PDF") {
		return ""
	}
	return extractPDFLiteral(data)
}

func SanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "")
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
	if strings.Contains(text, "http") || strings.Contains(text, "Adobe") {
		return false
	}
	letters := 0
	for _, r := range text {
		if r > 127 || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			letters++
		}
	}
	return letters >= 3
}

func ReadZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
