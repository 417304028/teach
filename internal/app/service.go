package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"hermesclaw/internal/generate"
	"hermesclaw/internal/intent"
	"hermesclaw/internal/model"
	"hermesclaw/internal/store"
)

type Service struct {
	Store     store.Store
	Intent    intent.Classifier
	Generator generate.Service
	MaxTokens int
}

type MessageResponse struct {
	Text        string                   `json:"text"`
	Intent      model.IntentResult       `json:"intent"`
	File        *model.FileRecord       `json:"file,omitempty"`
	URL         string                   `json:"url,omitempty"`
	Citations   []generate.Citation      `json:"citations,omitempty"`
	UsedRAG     bool                     `json:"used_rag"`
	Files       []generate.GenerateAllResponse `json:"files,omitempty"`
}

func (s Service) HandleMessage(ctx context.Context, userID, channel, text string) (MessageResponse, error) {
	intentResult := s.Intent.Classify(ctx, text)
	if intentResult.NeedsClarification {
		resp := MessageResponse{Text: intentResult.Clarification, Intent: intentResult}
		_ = s.Store.AddMessage(model.ChatMessage{UserID: userID, Channel: channel, Message: text, Response: resp.Text, Intent: intentResult.Intent})
		return resp, nil
	}
	req := requestFromIntent(intentResult, userID, text)
	switch intentResult.Intent {
	case model.IntentMindmap:
		genResp, err := s.runJob(ctx, userID, intentResult, func() (generate.Response, error) {
			return s.Generator.GenerateMindmap(ctx, req)
		})
		return s.finishGenerated(userID, channel, text, intentResult, genResp, err)
	case model.IntentPPT:
		genResp, err := s.runJob(ctx, userID, intentResult, func() (generate.Response, error) {
			return s.Generator.GeneratePPTX(ctx, req)
		})
		return s.finishGenerated(userID, channel, text, intentResult, genResp, err)
	case model.IntentExercises:
		genResp, err := s.runJob(ctx, userID, intentResult, func() (generate.Response, error) {
			return s.Generator.GenerateExercises(ctx, req)
		})
		return s.finishGenerated(userID, channel, text, intentResult, genResp, err)
	case model.IntentOutline:
		genResp, err := s.runJob(ctx, userID, intentResult, func() (generate.Response, error) {
			return s.Generator.GenerateOutline(ctx, req)
		})
		return s.finishGenerated(userID, channel, text, intentResult, genResp, err)
	case model.IntentGenerateAll:
		genResp, err := s.runGenerateAll(ctx, userID, intentResult)
		return s.finishGenerateAll(userID, channel, text, intentResult, genResp, err)
	case model.IntentSearch:
		results, err := s.Generator.RAG.Search(ctx, req.Query, req.Filters, 8)
		if err != nil {
			return MessageResponse{}, err
		}
		answer := formatSearchResults(results)
		_ = s.Store.AddMessage(model.ChatMessage{UserID: userID, Channel: channel, Message: text, Response: answer, Intent: intentResult.Intent})
		return MessageResponse{Text: answer, Intent: intentResult, UsedRAG: len(results) > 0}, nil
	case model.IntentUpload:
		answer := "请调用 POST /api/ingest 并传入课程压缩包或目录路径；QQ 上传文件自动入库会在下一阶段接入。"
		_ = s.Store.AddMessage(model.ChatMessage{UserID: userID, Channel: channel, Message: text, Response: answer, Intent: intentResult.Intent})
		return MessageResponse{Text: answer, Intent: intentResult}, nil
	default:
		answer, cites, used, err := s.Generator.Answer(ctx, req, text)
		if err != nil {
			return MessageResponse{}, err
		}
		_ = s.Store.AddMessage(model.ChatMessage{UserID: userID, Channel: channel, Message: text, Response: answer, Intent: intentResult.Intent})
		return MessageResponse{Text: answer, Intent: intentResult, Citations: cites, UsedRAG: used}, nil
	}
}

func (s Service) runJob(ctx context.Context, userID string, intentResult model.IntentResult, fn func() (generate.Response, error)) (generate.Response, error) {
	job, err := s.Store.CreateJob(model.Job{
		Type:   intentResult.Intent,
		Status: model.JobRunning,
		UserID: userID,
		Params: map[string]string{
			"topic":     intentResult.Topic,
			"lesson_no": strconv.Itoa(intentResult.LessonNo),
		},
	})
	if err != nil {
		return generate.Response{}, err
	}
	resp, err := fn()
	if err != nil {
		job.Status = model.JobFailed
		job.Error = err.Error()
		_ = s.Store.UpdateJob(job)
		return generate.Response{}, err
	}
	job.Status = model.JobSucceeded
	job.FileID = resp.File.ID
	job.Message = resp.Preview
	_ = s.Store.UpdateJob(job)
	return resp, nil
}

func (s Service) runGenerateAll(ctx context.Context, userID string, intentResult model.IntentResult) (generate.GenerateAllResponse, error) {
	req := requestFromIntent(intentResult, userID, intentResult.Topic)
	job, err := s.Store.CreateJob(model.Job{
		Type:   model.IntentGenerateAll,
		Status: model.JobRunning,
		UserID: userID,
		Params: map[string]string{
			"topic":     intentResult.Topic,
			"lesson_no": strconv.Itoa(intentResult.LessonNo),
		},
	})
	if err != nil {
		return generate.GenerateAllResponse{}, err
	}
	resp, err := s.Generator.GenerateAll(ctx, req)
	if err != nil {
		job.Status = model.JobFailed
		job.Error = err.Error()
		_ = s.Store.UpdateJob(job)
		return generate.GenerateAllResponse{}, err
	}
	job.Status = model.JobSucceeded
	job.Message = fmt.Sprintf("生成了 %d 个文件", countFiles(resp))
	_ = s.Store.UpdateJob(job)
	return resp, nil
}

func countFiles(r generate.GenerateAllResponse) int {
	n := 0
	if r.ExercisesDOCX != nil {
		n++
	}
	if r.ExercisesPDF != nil {
		n++
	}
	if r.HomeworkDOCX != nil {
		n++
	}
	if r.HomeworkPDF != nil {
		n++
	}
	if r.PPTX != nil {
		n++
	}
	if r.PPTX != nil && r.PPTX.PPTPDF != nil {
		n++
	}
	if r.Mindmap != nil {
		n++
	}
	return n
}

func (s Service) finishGenerateAll(userID, channel, text string, intentResult model.IntentResult, genResp generate.GenerateAllResponse, err error) (MessageResponse, error) {
	if err != nil {
		return MessageResponse{}, err
	}
	var b strings.Builder
	b.WriteString("已生成以下文件：\n")
	if genResp.ExercisesDOCX != nil {
		b.WriteString(fmt.Sprintf("练习题(含答案) DOCX: %s\n", genResp.ExercisesDOCX.URL))
	}
	if genResp.ExercisesPDF != nil {
		b.WriteString(fmt.Sprintf("练习题(含答案) PDF: %s\n", genResp.ExercisesPDF.URL))
	}
	if genResp.HomeworkDOCX != nil {
		b.WriteString(fmt.Sprintf("课后作业 DOCX: %s\n", genResp.HomeworkDOCX.URL))
	}
	if genResp.HomeworkPDF != nil {
		b.WriteString(fmt.Sprintf("课后作业 PDF: %s\n", genResp.HomeworkPDF.URL))
	}
	if genResp.PPTX != nil {
		b.WriteString(fmt.Sprintf("课件 PPTX: %s\n", genResp.PPTX.URL))
	}
	if genResp.PPTX != nil && genResp.PPTX.PPTPDF != nil {
		b.WriteString(fmt.Sprintf("课件 PDF: %s\n", genResp.PPTX.PPTPDF.URL))
	}
	if genResp.Mindmap != nil {
		b.WriteString(fmt.Sprintf("知识点导图: %s\n", genResp.Mindmap.URL))
	}
	if len(genResp.Errors) > 0 {
		b.WriteString("\n部分文件生成失败：\n")
		for _, e := range genResp.Errors {
			b.WriteString("- " + e + "\n")
		}
	}
	if genResp.Notice != "" {
		b.WriteString("\n注意: " + genResp.Notice)
	}
	answer := b.String()
	_ = s.Store.AddMessage(model.ChatMessage{UserID: userID, Channel: channel, Message: text, Response: answer, Intent: intentResult.Intent})
	return MessageResponse{Text: answer, Intent: intentResult, Files: []generate.GenerateAllResponse{genResp}, Citations: genResp.Citations, UsedRAG: genResp.UsedRAG}, nil
}

func (s Service) finishGenerated(userID, channel, text string, intentResult model.IntentResult, genResp generate.Response, err error) (MessageResponse, error) {
	if err != nil {
		return MessageResponse{}, err
	}
	answer := genResp.Preview + "\n" + genResp.URL
	if genResp.Notice != "" {
		answer = genResp.Notice + "\n" + answer
	}
	_ = s.Store.AddMessage(model.ChatMessage{UserID: userID, Channel: channel, Message: text, Response: answer, Intent: intentResult.Intent})
	return MessageResponse{Text: answer, Intent: intentResult, File: &genResp.File, URL: genResp.URL, Citations: genResp.Citations, UsedRAG: genResp.UsedRAG}, nil
}

func requestFromIntent(result model.IntentResult, userID, original string) generate.Request {
	query := result.Topic
	if query == "" {
		query = original
	}
	return generate.Request{
		Topic:  query,
		Query:  query,
		Pages:  result.Pages,
		Count:  result.Count,
		Style:  result.Style,
		UserID: userID,
		Filters: model.SearchFilters{
			Season:   result.Season,
			Edition:  result.Edition,
			Track:    result.Track,
			LessonNo: result.LessonNo,
		},
	}
}

func formatSearchResults(results []model.SearchResult) string {
	if len(results) == 0 {
		return "未检索到相关课程资料。"
	}
	var b strings.Builder
	b.WriteString("检索到以下资料：")
	for i, result := range results {
		b.WriteString(fmt.Sprintf("\n%d. %s（%s/%s，相似度 %.2f）", i+1, result.Material.SourcePath, result.Material.MaterialKind, result.Material.Version, result.Score))
	}
	return b.String()
}
