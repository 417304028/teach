package generate

import (
	"strconv"
	"strings"

	"hermesclaw/internal/model"
)

const homeworkSystemPrompt = `你是高中物理教研专家。请为学生生成课后作业，要求：
1. 作业只包含题目，不包含答案
2. 涵盖选择题、填空题、计算题多种题型
3. 难度分基础题和提升题
4. 题目要有完整题干，选择题要有选项
5. 输出JSON数组格式：
[{"type":"选择题","stem":"题干","options":["A选项","B选项","C选项","D选项"],"difficulty":"基础"},{"type":"计算题","stem":"题干","difficulty":"提升"}]
只输出JSON数组，不要其他文字。`

func buildHomeworkDocument(topic string, count int, exercises []map[string]string, results []model.SearchResult, knowledgeResults []model.SearchResult, notice string) Doc {
	doc := Doc{Title: topic + " 课后作业"}
	if notice != "" {
		doc.Sections = append(doc.Sections, DocSection{Heading: "说明", Lines: []string{notice}})
	}

	if len(knowledgeResults) > 0 {
		doc.Sections = append(doc.Sections, DocSection{Heading: "知识点回顾", Lines: knowledgePointLines(topic, knowledgeResults)})
	}

	var problemLines []string
	if len(exercises) > 0 {
		problemLines = formatHomeworkFromAI(exercises, 1)
	} else {
		problemLines = homeworkLines(topic, count, results)
	}

	doc.Sections = append(doc.Sections, DocSection{Heading: "作业题", Lines: problemLines})
	doc.Sections = append(doc.Sections, DocSection{Heading: "引用资料", Lines: citationLines(results)})
	return doc
}

func formatHomeworkFromAI(exercises []map[string]string, startNum int) []string {
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
			lines = append(lines, "")
			lines = append(lines, strconv.Itoa(num)+". "+prefix+stem)
			if opts != "" {
				for _, opt := range splitOptions(opts) {
					lines = append(lines, "  "+opt)
				}
			}
		case "填空题":
			lines = append(lines, "")
			lines = append(lines, strconv.Itoa(num)+". "+prefix+stem)
		case "计算题":
			lines = append(lines, "")
			lines = append(lines, strconv.Itoa(num)+". "+prefix+stem)
		default:
			lines = append(lines, "")
			lines = append(lines, strconv.Itoa(num)+". "+prefix+stem)
		}
		num++
	}
	return lines
}

func splitOptions(opts string) []string {
	lines := []string{}
	for _, opt := range []string{"A", "B", "C", "D"} {
		prefix := opt + "."
		if idx := strings.Index(opts, prefix); idx >= 0 {
			end := strings.IndexFunc(opts[idx+2:], func(r rune) bool { return r < 'A' || r > 'Z' })
			if end < 0 {
				end = len(opts) - idx - 2
			}
			line := strings.TrimSpace(opts[idx : idx+2+end])
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	if len(lines) == 0 {
		for _, line := range strings.Split(opts, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func homeworkLines(topic string, count int, results []model.SearchResult) []string {
	lines := []string{}
	for i := 1; i <= count; i++ {
		lines = append(lines, "")
		lines = append(lines, strconv.Itoa(i+1)+". "+topic+" 相关练习题")
	}
	return lines
}
