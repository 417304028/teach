package generate

import (
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hermesclaw/internal/ai"
	"hermesclaw/internal/model"
	"hermesclaw/internal/rag"
	"hermesclaw/internal/store"
)

type Service struct {
	Store   store.Store
	RAG     rag.Service
	Chat    ai.ChatProvider
	DataDir string
	BaseURL string
	FileTTL time.Duration
}

type Request struct {
	Topic   string              `json:"topic"`
	Query   string              `json:"query"`
	Pages   int                 `json:"pages"`
	Count   int                 `json:"count"`
	Style   string              `json:"style"`
	Filters model.SearchFilters `json:"filters"`
	UserID  string              `json:"user_id"`
}

type Response struct {
	File       model.FileRecord     `json:"file"`
	URL        string               `json:"url"`
	Citations  []Citation           `json:"citations"`
	UsedRAG    bool                 `json:"used_rag"`
	Notice     string               `json:"notice,omitempty"`
	Preview    string               `json:"preview,omitempty"`
	SearchHits []model.SearchResult `json:"search_hits,omitempty"`
	PPTPDF    *Response             `json:"ppt_pdf,omitempty"`
}

type Citation struct {
	MaterialID string  `json:"material_id"`
	SourcePath string  `json:"source_path"`
	Score      float64 `json:"score"`
}

func (s Service) GenerateMindmap(ctx context.Context, req Request) (Response, error) {
	req = normalizeRequest(req)
	results, used, notice := s.retrieve(ctx, req)
	nodes := mindmapNodes(req.Topic, results)
	htmlDoc := renderMindmap(req.Topic, nodes, citations(results), notice)
	file, err := s.writeFile(req.Topic, "mindmap", ".html", "text/html; charset=utf-8", []byte(htmlDoc))
	if err != nil {
		return Response{}, err
	}
	return Response{File: file, URL: s.fileURL(file.ID), Citations: citations(results), UsedRAG: used, Notice: notice, Preview: "导图已生成", SearchHits: results}, nil
}

func (s Service) GeneratePPTX(ctx context.Context, req Request) (Response, error) {
	req = normalizeRequest(req)
	if req.Pages <= 0 {
		req.Pages = 12
	}
	results, used, notice := s.retrieve(ctx, req)

	var slides []Slide
	if s.Chat != nil {
		contentBlock := contextText(results)
		promptContent, _ := BuildSlideContentJSON(req.Topic, req.Pages, contentBlock)
		aiResp, err := s.Chat.Chat(ctx, ai.ChatRequest{
			Messages: []ai.Message{
				{Role: "system", Content: slideSystemPrompt},
				{Role: "user", Content: promptContent},
			},
			Temperature: 0.3,
			MaxTokens:   2000,
			JSONMode:    true,
		})
		if err == nil {
			if parsed, err := ParseSlideContent(aiResp); err == nil && len(parsed) > 0 {
				slides = parsed
				for i := range slides {
					if len(slides[i].Lines) == 0 {
						slides[i].Lines = []string{"请在课堂上补充具体内容"}
					}
				}
			}
		}
	}
	if slides == nil {
		slides = outlineSlides(req.Topic, req.Pages, results, notice)
	}

	if len(results) > 0 {
		slides = append(slides, Slide{Title: "资料引用", Lines: citationLines(results)})
	}

	data, err := BuildPPTX(slides)
	if err != nil {
		return Response{}, err
	}
	file, err := s.writeFile(req.Topic, "ppt", ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", data)
	if err != nil {
		return Response{}, err
	}
	return Response{File: file, URL: s.fileURL(file.ID), Citations: citations(results), UsedRAG: used, Notice: notice, Preview: fmt.Sprintf("已生成 %d 页 PPT", len(slides)), SearchHits: results}, nil
}

func (s Service) GenerateExercises(ctx context.Context, req Request) (Response, error) {
	req = normalizeRequest(req)
	if req.Count <= 0 {
		req.Count = 10
	}
	results, used, notice := s.retrieve(ctx, req)

	knowledgeResults := results
	if req.Filters.Track == "复习" || req.Filters.Track == "" {
		knowledgeFilters := req.Filters
		knowledgeFilters.Track = "新授"
		kr, _, _ := s.retrieveWithFilters(ctx, req.Topic, knowledgeFilters, 6)
		if len(kr) > 0 {
			knowledgeResults = kr
		}
	}

	var exercises []map[string]string
	if s.Chat != nil && len(results) > 0 {
		contentBlock := contextText(results)
		aiResp, err := s.Chat.Chat(ctx, ai.ChatRequest{
			Messages: []ai.Message{
				{Role: "system", Content: exerciseSystemPrompt},
				{Role: "user", Content: fmt.Sprintf("主题：%s\n题目数量：%d\n课程资料：\n%s", req.Topic, req.Count, contentBlock)},
			},
			Temperature: 0.5,
			MaxTokens:   2500,
			JSONMode:    true,
		})
		if err == nil {
			if parsed, err := parseExerciseJSON(aiResp); err == nil && len(parsed) > 0 {
				exercises = parsed
			}
		}
	}

	doc := buildExerciseDocumentFromAI(req.Topic, req.Count, exercises, results, knowledgeResults, notice)
	data, err := BuildDOCX(doc)
	if err != nil {
		return Response{}, err
	}
	file, err := s.writeFile(req.Topic, "exercises", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
	if err != nil {
		return Response{}, err
	}
	return Response{File: file, URL: s.fileURL(file.ID), Citations: citations(results), UsedRAG: used, Notice: notice, Preview: fmt.Sprintf("已生成 %d 道习题", req.Count), SearchHits: results}, nil
}

func buildExerciseDocumentFromAI(topic string, count int, exercises []map[string]string, results []model.SearchResult, knowledgeResults []model.SearchResult, notice string) Doc {
	doc := Doc{Title: topic + " 习题"}
	if notice != "" {
		doc.Sections = append(doc.Sections, DocSection{Heading: "说明", Lines: []string{notice}})
	}

	// 知识点（从新授资料中提取）
	if len(knowledgeResults) > 0 {
		doc.Sections = append(doc.Sections, DocSection{Heading: "知识点", Lines: knowledgePointLines(topic, knowledgeResults)})
	}

	var problemLines, answerLinesList []string
	if len(exercises) > 0 {
		problemLines = formatExercisesFromAI(exercises, 1)
		answerLinesList = formatAnswersFromAI(exercises, 1)
	} else {
		problemLines = exerciseLines(topic, count, results)
		answerLinesList = answerLines(count)
	}

	doc.Sections = append(doc.Sections, DocSection{Heading: "练习题", Lines: problemLines})
	doc.Sections = append(doc.Sections, DocSection{Heading: "参考答案", Lines: answerLinesList})
	doc.Sections = append(doc.Sections, DocSection{Heading: "引用资料", Lines: citationLines(results)})
	return doc
}

func (s Service) GenerateOutline(ctx context.Context, req Request) (Response, error) {
	req = normalizeRequest(req)
	results, used, notice := s.retrieve(ctx, req)
	outline, err := s.buildOutline(ctx, req.Topic, results, notice)
	if err != nil {
		return Response{}, err
	}
	data, err := BuildDOCX(outline)
	if err != nil {
		return Response{}, err
	}
	file, err := s.writeFile(req.Topic, "outline", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
	if err != nil {
		return Response{}, err
	}
	return Response{File: file, URL: s.fileURL(file.ID), Citations: citations(results), UsedRAG: used, Notice: notice, Preview: "教学大纲已生成", SearchHits: results}, nil
}

func (s Service) Answer(ctx context.Context, req Request, question string) (string, []Citation, bool, error) {
	req = normalizeRequest(req)
	if req.Query == "" {
		req.Query = question
	}
	results, used, notice := s.retrieve(ctx, req)
	contextBlock := contextText(results)
	prompt := "你是高中物理课件和作业助手。优先依据给定资料回答；没有资料时必须说明使用通用知识。回答要简洁、可执行，并给出可追溯的依据。"
	answer, err := s.Chat.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: "资料：\n" + contextBlock + "\n\n问题：" + question},
		},
		Temperature: 0.3,
		MaxTokens:   1200,
	})
	if err != nil {
		return "", nil, false, err
	}
	if notice != "" {
		answer = notice + "\n\n" + answer
	}
	return answer, citations(results), used, nil
}

func (s Service) buildOutline(ctx context.Context, topic string, results []model.SearchResult, notice string) (Doc, error) {
	contextBlock := contextText(results)
	prompt := "你是高中物理教研员。请基于资料生成教学大纲，包含教学目标、重难点、课时流程、例题设计、练习安排和课后作业。"
	answer, err := s.Chat.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: "主题：" + topic + "\n资料：\n" + contextBlock},
		},
		Temperature: 0.25,
		MaxTokens:   1800,
	})
	if err != nil {
		return Doc{}, err
	}
	sections := []DocSection{}
	if notice != "" {
		sections = append(sections, DocSection{Heading: "说明", Lines: []string{notice}})
	}
	sections = append(sections, DocSection{Heading: "教学大纲", Lines: nonEmptyLines(answer)})
	sections = append(sections, DocSection{Heading: "引用资料", Lines: citationLines(results)})
	return Doc{Title: topic + " 教学大纲", Sections: sections}, nil
}

func (s Service) retrieve(ctx context.Context, req Request) ([]model.SearchResult, bool, string) {
	return s.retrieveWithFilters(ctx, req.Query, req.Filters, 8)
}

func (s Service) retrieveWithFilters(ctx context.Context, query string, filters model.SearchFilters, limit int) ([]model.SearchResult, bool, string) {
	if query == "" {
		query = filters.Season + " " + filters.Edition
	}
	results, err := s.RAG.Search(ctx, query, filters, limit)
	if err != nil || len(results) == 0 {
		return nil, false, "未检索到课程资料，使用通用知识生成。"
	}
	return results, true, ""
}

func (s Service) writeFile(topic, prefix, ext, mime string, data []byte) (model.FileRecord, error) {
	dir := filepath.Join(s.DataDir, "files", time.Now().Format("20060102"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return model.FileRecord{}, err
	}
	name := safeName(prefix + "_" + topic + ext)
	path := filepath.Join(dir, store.NewID(prefix)+"_"+name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return model.FileRecord{}, err
	}
	info, _ := os.Stat(path)
	file := model.FileRecord{
		Name:      name,
		Path:      path,
		MimeType:  mime,
		ExpiresAt: time.Now().Add(s.FileTTL),
	}
	if info != nil {
		file.SizeBytes = info.Size()
	}
	return s.Store.AddFile(file)
}

func (s Service) fileURL(id string) string {
	return strings.TrimRight(s.BaseURL, "/") + "/api/files/" + id
}

func normalizeRequest(req Request) Request {
	if req.Topic == "" {
		req.Topic = req.Query
	}
	if req.Topic == "" {
		req.Topic = "未命名主题"
	}
	return req
}

func contextText(results []model.SearchResult) string {
	var b strings.Builder
	for i, result := range results {
		b.WriteString(fmt.Sprintf("[%d] %s | %s | %.2f\n", i+1, result.Material.SourcePath, result.Material.Version, result.Score))
		b.WriteString(result.Chunk.Text)
		b.WriteString("\n\n")
	}
	return b.String()
}

func citations(results []model.SearchResult) []Citation {
	out := make([]Citation, 0, len(results))
	seen := map[string]bool{}
	for _, result := range results {
		if seen[result.Material.ID] {
			continue
		}
		seen[result.Material.ID] = true
		out = append(out, Citation{MaterialID: result.Material.ID, SourcePath: result.Material.SourcePath, Score: result.Score})
	}
	return out
}

func nonEmptyLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return []string{"暂无内容。"}
	}
	return lines
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "output"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	return name
}

func xmlEscape(text string) string {
	return html.EscapeString(text)
}
