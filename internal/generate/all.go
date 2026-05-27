package generate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"hermesclaw/internal/ai"
	"hermesclaw/internal/model"
	"hermesclaw/internal/store"
)

type GenerateAllResponse struct {
	ExercisesDOCX *Response  `json:"exercises_docx,omitempty"`
	ExercisesPDF  *Response  `json:"exercises_pdf,omitempty"`
	HomeworkDOCX  *Response  `json:"homework_docx,omitempty"`
	HomeworkPDF   *Response  `json:"homework_pdf,omitempty"`
	PPTX          *Response  `json:"pptx,omitempty"`
	PPTPDF       *Response  `json:"ppt_pdf,omitempty"`
	Mindmap      *Response  `json:"mindmap,omitempty"`
	Notice       string     `json:"notice,omitempty"`
	UsedRAG      bool       `json:"used_rag"`
	Citations    []Citation `json:"citations,omitempty"`
	Errors       []string   `json:"errors,omitempty"`
}

func (s Service) GenerateAll(ctx context.Context, req Request) (GenerateAllResponse, error) {
	resp := GenerateAllResponse{}
	now := time.Now()
	dateDir := now.Format("2006-01-02")
	baseDir := filepath.Join(s.DataDir, "output", dateDir)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return resp, fmt.Errorf("创建输出目录失败: %w", err)
	}

	knowledgeResults := []model.SearchResult{}
	results := []model.SearchResult{}

	knowledgeResults, resp.UsedRAG, resp.Notice = s.retrieveWithFilters(ctx, req.Topic, req.Filters, 8)

	if req.Filters.Track == "复习" || req.Filters.Track == "" {
		kf := req.Filters
		kf.Track = "新授"
		kr, _, _ := s.retrieveWithFilters(ctx, req.Topic, kf, 6)
		if len(kr) > 0 {
			knowledgeResults = kr
		}
	}
	if len(knowledgeResults) == 0 {
		knowledgeResults = results
	}

	resp.Citations = citations(knowledgeResults)
	if len(resp.Citations) == 0 {
		resp.Citations = citations(results)
	}

	if req.Count <= 0 {
		req.Count = 10
	}
	if req.Pages <= 0 {
		req.Pages = 12
	}

	allResults := append(results, knowledgeResults...)
	contextBlock := contextText(allResults)

	// 1. 生成练习题 DOCX
	exercises, err := s.generateExercisesFromAI(ctx, req, contextBlock)
	if err != nil {
		resp.Errors = append(resp.Errors, "练习题生成: "+err.Error())
	}
	doc := s.buildExerciseDocumentForAll(req.Topic, req.Count, exercises, allResults, knowledgeResults, resp.Notice)
	data, err := BuildDOCX(doc)
	if err != nil {
		resp.Errors = append(resp.Errors, "练习题DOCX构建: "+err.Error())
	} else {
		file, err := s.writeFileTo(baseDir, req.Topic, "练习题_含答案", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
		if err == nil {
			docxResp := Response{File: file, URL: s.fileURL(file.ID), UsedRAG: resp.UsedRAG, Notice: resp.Notice}
			resp.ExercisesDOCX = &docxResp
		}
		pdfData, err := convertDocxToPDF(data, filepath.Join(baseDir, "练习题_含答案.docx"))
		if err == nil && pdfData != nil {
			pdfFile, err := s.writeFileTo(baseDir, req.Topic, "练习题_含答案", ".pdf", "application/pdf", pdfData)
			if err == nil {
				pdfResp := Response{File: pdfFile, URL: s.fileURL(pdfFile.ID), UsedRAG: resp.UsedRAG}
				resp.ExercisesPDF = &pdfResp
			}
		}
	}

	// 2. 生成作业 PDF（无答案）
	hwExercises := exercises
	hwDoc := buildHomeworkDocument(req.Topic, req.Count, hwExercises, allResults, knowledgeResults, resp.Notice)
	hwData, err := BuildDOCX(hwDoc)
	if err != nil {
		resp.Errors = append(resp.Errors, "作业DOCX构建: "+err.Error())
	} else {
		file, err := s.writeFileTo(baseDir, req.Topic, "课后作业", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", hwData)
		if err == nil {
			docxResp := Response{File: file, URL: s.fileURL(file.ID), UsedRAG: resp.UsedRAG}
			resp.HomeworkDOCX = &docxResp
		}
		hwPDFData, err := convertDocxToPDF(hwData, filepath.Join(baseDir, "课后作业.docx"))
		if err == nil && hwPDFData != nil {
			pdfFile, err := s.writeFileTo(baseDir, req.Topic, "课后作业", ".pdf", "application/pdf", hwPDFData)
			if err == nil {
				pdfResp := Response{File: pdfFile, URL: s.fileURL(pdfFile.ID), UsedRAG: resp.UsedRAG}
				resp.HomeworkPDF = &pdfResp
			}
		}
	}

	// 3. 生成课件 PPTX
	slides := s.buildSlidesForAll(ctx, req, contextBlock, dateDir, baseDir)
	if slides != nil {
		resp.PPTX = slides
	}

	// 4. 生成导图 HTML
	mindmapResp, err := s.GenerateMindmap(ctx, req)
	if err == nil {
		resp.Mindmap = &mindmapResp
	} else {
		resp.Errors = append(resp.Errors, "导图生成: "+err.Error())
	}

	return resp, nil
}

func (s Service) generateExercisesFromAI(ctx context.Context, req Request, contextBlock string) ([]map[string]string, error) {
	if s.Chat == nil || contextBlock == "" {
		return nil, nil
	}
	aiResp, err := s.Chat.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: exerciseSystemPrompt},
			{Role: "user", Content: fmt.Sprintf("主题：%s\n题目数量：%d\n课程资料：\n%s", req.Topic, req.Count, contextBlock)},
		},
		Temperature: 0.5,
		MaxTokens:   2500,
		JSONMode:    true,
	})
	if err != nil {
		return nil, err
	}
	parsed, err := parseExerciseJSON(aiResp)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (s Service) buildExerciseDocumentForAll(topic string, count int, exercises []map[string]string, results, knowledgeResults []model.SearchResult, notice string) Doc {
	doc := Doc{Title: topic + " 习题"}
	if notice != "" {
		doc.Sections = append(doc.Sections, DocSection{Heading: "说明", Lines: []string{notice}})
	}
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

func (s Service) buildSlidesForAll(ctx context.Context, req Request, contextBlock, dateDir, baseDir string) *Response {
	if s.Chat == nil {
		return nil
	}
	promptContent, _ := BuildSlideContentJSON(req.Topic, req.Pages, contextBlock)
	aiResp, err := s.Chat.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: slideSystemPrompt},
			{Role: "user", Content: promptContent},
		},
		Temperature: 0.3,
		MaxTokens:   2000,
		JSONMode:    true,
	})
	var slides []Slide
	if err == nil {
		slides, _ = ParseSlideContent(aiResp)
	}
	if slides == nil {
		slides = outlineSlides(req.Topic, req.Pages, nil, "")
	}
	data, err := BuildPPTX(slides)
	if err != nil {
		return nil
	}
	file, err := s.writeFileTo(baseDir, req.Topic, "课件", ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", data)
	if err != nil {
		return nil
	}
	pdfData, err := convertPPTXToPDF(data, filepath.Join(baseDir, "课件.pptx"))
	if err == nil && pdfData != nil {
		pdfFile, err := s.writeFileTo(baseDir, req.Topic, "课件", ".pdf", "application/pdf", pdfData)
		if err == nil {
			pdfResp := Response{File: pdfFile, URL: s.fileURL(pdfFile.ID)}
			return &Response{File: file, URL: s.fileURL(file.ID), PPTPDF: &pdfResp}
		}
	}
	resp := Response{File: file, URL: s.fileURL(file.ID)}
	return &resp
}

func (s Service) writeFileTo(dir, topic, prefix, ext, mime string, data []byte) (model.FileRecord, error) {
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

func convertDocxToPDF(docxData []byte, docxPath string) ([]byte, error) {
	docxPath = docxPath + ".tmp.docx"
	if err := os.WriteFile(docxPath, docxData, 0o644); err != nil {
		return nil, err
	}
	defer os.Remove(docxPath)

	soffice, err := findLibreOffice()
	if err != nil {
		return nil, fmt.Errorf("未找到LibreOffice: %w", err)
	}

	dir := filepath.Dir(docxPath)
	cmd := exec.Command(soffice, "--headless", "--convert-to", "pdf", "--outdir", dir, docxPath)
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("LibreOffice转换失败: %w", err)
	}

	pdfPath := docxPath[:len(docxPath)-4] + ".pdf"
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("读取PDF失败: %w", err)
	}
	os.Remove(pdfPath)
	return pdfData, nil
}

func convertPPTXToPDF(pptxData []byte, pptxPath string) ([]byte, error) {
	pptxPath = pptxPath + ".tmp.pptx"
	if err := os.WriteFile(pptxPath, pptxData, 0o644); err != nil {
		return nil, err
	}
	defer os.Remove(pptxPath)

	soffice, err := findLibreOffice()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(pptxPath)
	cmd := exec.Command(soffice, "--headless", "--convert-to", "pdf", "--outdir", dir, pptxPath)
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("LibreOffice转换失败: %w", err)
	}

	pdfPath := strings.TrimSuffix(pptxPath, ".tmp.pptx") + ".pdf"
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("读取PDF失败: %w", err)
	}
	os.Remove(pdfPath)
	return pdfData, nil
}

func findLibreOffice() (string, error) {
	paths := []string{
		"soffice",
		"libreoffice",
		"C:/Program Files/LibreOffice/program/soffice.exe",
		"C:/Program Files (x86)/LibreOffice/program/soffice.exe",
		"/Applications/LibreOffice.app/Contents/MacOS/soffice",
		"/usr/bin/soffice",
		"/usr/local/bin/soffice",
	}
	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return p, nil
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("LibreOffice not found")
}
