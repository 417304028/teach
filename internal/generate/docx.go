package generate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"hermesclaw/internal/model"
)

type DocSection struct {
	Heading string
	Lines   []string
}

type Doc struct {
	Title    string
	Sections []DocSection
}

func BuildDOCX(doc Doc) ([]byte, error) {
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml": contentTypesDocx(),
		"_rels/.rels":         packageRels("word/document.xml"),
		"docProps/app.xml":    appProps("Hermesclaw"),
		"docProps/core.xml":   coreProps(doc.Title),
		"word/document.xml":   documentXML(doc),
		"word/styles.xml":     stylesXML(),
	}
	for name, body := range files {
		if err := addZipFile(zw, name, []byte(body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildExerciseDocument(topic string, count int, results []model.SearchResult, notice string) Doc {
	doc := Doc{Title: topic + " 习题"}
	if notice != "" {
		doc.Sections = append(doc.Sections, DocSection{Heading: "说明", Lines: []string{notice}})
	}
	doc.Sections = append(doc.Sections, DocSection{Heading: "练习题", Lines: exerciseLines(topic, count, results)})
	doc.Sections = append(doc.Sections, DocSection{Heading: "参考答案", Lines: answerLines(count)})
	doc.Sections = append(doc.Sections, DocSection{Heading: "引用资料", Lines: citationLines(results)})
	return doc
}

const exerciseSystemPrompt = `你是高中物理教研专家。请为学生生成练习题，要求：
1. 涵盖选择题、填空题、计算题多种题型
2. 难度分基础题和提升题
3. 每道题要有完整题干
4. 输出JSON数组格式：
[{"type":"选择题","stem":"题干","options":["A选项","B选项","C选项","D选项"],"answer":"B","difficulty":"基础"},{"type":"计算题","stem":"题干","answer":"答案","difficulty":"提升"}]
只输出JSON数组，不要其他文字。`

func parseExerciseJSON(text string) ([]map[string]string, error) {
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid JSON")
	}
	jsonStr := strings.TrimSpace(text[start : end+1])
	var exercises []map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &exercises); err != nil {
		return nil, err
	}
	return exercises, nil
}

func formatExercisesFromAI(exercises []map[string]string, startNum int) []string {
	lines := []string{}
	num := startNum
	for _, ex := range exercises {
		qtype, _ := ex["type"]
		stem, _ := ex["stem"]
		difficulty, hasDiff := ex["difficulty"]
		prefix := ""
		if hasDiff && difficulty != "" {
			prefix = "[" + difficulty + "] "
		}
		switch qtype {
		case "选择题":
			opts, _ := ex["options"]
			lines = append(lines, fmt.Sprintf("%d. %s%s", num, prefix, stem))
			if opts != "" {
				lines = append(lines, opts)
			}
		case "填空题":
			lines = append(lines, fmt.Sprintf("%d. %s%s", num, prefix, stem))
		case "计算题":
			lines = append(lines, fmt.Sprintf("%d. %s%s", num, prefix, stem))
		default:
			lines = append(lines, fmt.Sprintf("%d. %s%s", num, prefix, stem))
		}
		num++
	}
	return lines
}

func formatAnswersFromAI(exercises []map[string]string, startNum int) []string {
	lines := []string{}
	num := startNum
	for _, ex := range exercises {
		answer, _ := ex["answer"]
		qtype, _ := ex["type"]
		if answer == "" {
			answer = "（略）"
		}
		switch qtype {
		case "选择题":
			lines = append(lines, fmt.Sprintf("%d. %s", num, answer))
		case "填空题":
			lines = append(lines, fmt.Sprintf("%d. %s", num, answer))
		case "计算题":
			解析, _ := ex["解析"]
			if 解析 != "" {
				lines = append(lines, fmt.Sprintf("%d. %s", num, answer))
				lines = append(lines, fmt.Sprintf("   解析：%s", 解析))
			} else {
				lines = append(lines, fmt.Sprintf("%d. %s", num, answer))
			}
		default:
			lines = append(lines, fmt.Sprintf("%d. %s", num, answer))
		}
		num++
	}
	return lines
}

func exerciseLines(topic string, count int, results []model.SearchResult) []string {
	lines := make([]string, 0, count*2)
	keyPoints := keyPointsFromResults(topic, results)
	for i := 1; i <= count; i++ {
		point := keyPoints[(i-1)%len(keyPoints)]
		lines = append(lines, fmt.Sprintf("%d. 关于%s，下列说法或计算过程是否正确？请写出判断依据。", i, point))
		lines = append(lines, "A. 只需套用公式即可  B. 需要先分析物理过程  C. 与受力或能量关系无关  D. 无法判断")
	}
	return lines
}

func answerLines(count int) []string {
	lines := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		lines = append(lines, fmt.Sprintf("%d. B。解析：先识别研究对象和过程，再选择对应规律，避免直接套公式。", i))
	}
	return lines
}

func citationLines(results []model.SearchResult) []string {
	if len(results) == 0 {
		return []string{"未命中课程资料。"}
	}
	lines := []string{}
	seen := map[string]bool{}
	for _, result := range results {
		if seen[result.Material.SourcePath] {
			continue
		}
		seen[result.Material.SourcePath] = true
		lines = append(lines, fmt.Sprintf("%s（相似度 %.2f）", result.Material.SourcePath, result.Score))
	}
	return lines
}

func keyPointsFromResults(topic string, results []model.SearchResult) []string {
	points := []string{}
	for _, result := range results {
		if result.Material.LessonTitle != "" {
			points = append(points, result.Material.LessonTitle)
		}
	}
	if len(points) == 0 {
		points = append(points, topic, "核心概念", "典型模型", "易错点")
	}
	return points
}

func knowledgePointLines(topic string, results []model.SearchResult) []string {
	if len(results) == 0 {
		return []string{"（未检索到新授课知识点）"}
	}
	lines := []string{}
	seen := map[string]bool{}
	for _, result := range results {
		// Use the chunk text to build knowledge points.
		// Truncate long text to avoid overly verbose output.
		text := result.Chunk.Text
		if len(text) > 600 {
			text = text[:600] + "..."
		}
		source := result.Material.LessonTitle
		if source == "" {
			source = result.Material.SourcePath
		}
		key := source + text[:min(50, len(text))]
		if seen[key] {
			continue
		}
		seen[key] = true
		lines = append(lines, fmt.Sprintf("【%s】%s", source, text))
	}
	if len(lines) == 0 {
		lines = []string{"（未检索到新授课知识点）"}
	}
	return lines
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func documentXML(doc Doc) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	b.WriteString(paragraph(doc.Title, "Title"))
	for _, section := range doc.Sections {
		b.WriteString(paragraph(section.Heading, "Heading1"))
		for _, line := range section.Lines {
			b.WriteString(paragraph(line, "Normal"))
		}
	}
	b.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`)
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func paragraph(text, style string) string {
	return `<w:p><w:pPr><w:pStyle w:val="` + style + `"/></w:pPr><w:r><w:t xml:space="preserve">` + xmlEscape(text) + `</w:t></w:r></w:p>`
}

func contentTypesDocx() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/></Types>`
}

func stylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:rFonts w:ascii="Microsoft YaHei" w:eastAsia="Microsoft YaHei"/><w:sz w:val="22"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:rPr><w:b/><w:rFonts w:ascii="Microsoft YaHei" w:eastAsia="Microsoft YaHei"/><w:sz w:val="36"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="Heading 1"/><w:rPr><w:b/><w:rFonts w:ascii="Microsoft YaHei" w:eastAsia="Microsoft YaHei"/><w:sz w:val="28"/></w:rPr></w:style></w:styles>`
}
