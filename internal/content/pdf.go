package content

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

var literalStringPattern = regexp.MustCompile(`\(([^()\r\n]{2,300})\)`)

func ExtractPDFTextBestEffort(data []byte) string {
	if len(data) == 0 || !bytes.HasPrefix(data, []byte("%PDF")) {
		return ""
	}
	text, err := extractPDFReader(data)
	if err == nil && strings.TrimSpace(text) != "" {
		return text
	}
	return extractPDFLiteral(data)
}

func extractPDFReader(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	r, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return "", err
	}
	var out strings.Builder
	numPages := r.NumPage()
	maxPages := 100
	if numPages < maxPages {
		maxPages = numPages
	}
	for i := 1; i <= maxPages; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err == nil && strings.TrimSpace(text) != "" {
			out.WriteString(text)
			out.WriteString("\n")
		}
	}
	return out.String(), nil
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
