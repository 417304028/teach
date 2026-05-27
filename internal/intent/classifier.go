package intent

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"hermesclaw/internal/ai"
	"hermesclaw/internal/model"
)

type Classifier struct {
	Chat     ai.ChatProvider
	Embedder ai.EmbeddingProvider
}

type example struct {
	Intent model.Intent
	Text   string
}

var (
	pagesPattern  = regexp.MustCompile(`([0-9]{1,2})\s*页`)
	countPattern  = regexp.MustCompile(`([0-9]{1,3})\s*(道|题|个)`)
	lessonPattern = regexp.MustCompile(`第\s*([0-9一二三四五六七八九十]+)\s*讲`)
	examples      = []example{
		{model.IntentMindmap, "生成这一讲的思维导图"},
		{model.IntentMindmap, "把动能定理整理成知识导图"},
		{model.IntentPPT, "做一份十二页课件"},
		{model.IntentPPT, "根据资料生成上课用的PPT"},
		{model.IntentExercises, "出一套带答案的练习题"},
		{model.IntentExercises, "生成课后作业和答案解析"},
		{model.IntentOutline, "生成教学大纲"},
		{model.IntentOutline, "整理本讲授课提纲和教学目标"},
		{model.IntentSearch, "检索资料库中相关讲义"},
		{model.IntentUpload, "把这个文件导入知识库"},
		{model.IntentGame, "做一个课堂互动闯关游戏"},
		{model.IntentChat, "解释一下这个知识点"},
	}
)

func New(chat ai.ChatProvider, embedders ...ai.EmbeddingProvider) Classifier {
	var embedder ai.EmbeddingProvider
	if len(embedders) > 0 {
		embedder = embedders[0]
	}
	return Classifier{Chat: chat, Embedder: embedder}
}

func (c Classifier) Classify(ctx context.Context, text string) model.IntentResult {
	text = strings.TrimSpace(text)
	result := ruleClassify(text)
	if result.Confidence >= 0.85 {
		return result
	}
	if c.Embedder != nil {
		if vectorResult, err := c.classifyWithVectors(ctx, text); err == nil && vectorResult.Confidence > result.Confidence {
			vectorResult = mergeParams(vectorResult, result)
			if vectorResult.Confidence >= 0.72 {
				return vectorResult
			}
			result = vectorResult
		}
	}
	if c.Chat != nil && (result.Intent == model.IntentUnknown || result.Confidence < 0.72) {
		if aiResult, err := c.classifyWithAI(ctx, text); err == nil && aiResult.Intent != "" {
			return mergeParams(aiResult, result)
		}
	}
	return result
}

func ruleClassify(text string) model.IntentResult {
	lower := strings.ToLower(text)
	result := model.IntentResult{
		Intent:     model.IntentUnknown,
		Confidence: 0.3,
		Topic:      extractTopic(text),
		Pages:      extractPages(text),
		Count:      extractCount(text),
		LessonNo:   extractLessonNo(text),
		Season:     extractSeason(text),
		Track:      extractTrack(text),
		Edition:    extractEdition(text),
		Params:     map[string]string{},
	}

	// 一键生成：检测「题目做的不好/需要生成题目+答案+知识点」
	if containsAny(lower, "题目做的不好", "做的不好", "这部分题目", "出题", "生成题目", "需要出题", "需要生成题目") &&
		(containsAny(lower, "不好", "不好", "差", "弱") || containsAny(lower, "生成", "出", "需要", "做")) {
		result.Intent = model.IntentGenerateAll
		result.Confidence = 0.95
		return result
	}

	switch {
	case containsAny(lower, "导图", "思维导图", "mindmap"):
		result.Intent = model.IntentMindmap
		result.Confidence = 0.96
	case containsAny(lower, "ppt", "课件", "幻灯片", "演示文稿"):
		result.Intent = model.IntentPPT
		result.Confidence = 0.95
	case containsAny(lower, "习题", "题集", "练习", "选择题", "填空题", "作业", "卷子"):
		result.Intent = model.IntentExercises
		result.Confidence = 0.94
	case containsAny(lower, "大纲", "教学设计", "教案", "授课提纲"):
		result.Intent = model.IntentOutline
		result.Confidence = 0.93
	case containsAny(lower, "搜索", "查找", "检索", "有没有", "有哪些"):
		result.Intent = model.IntentSearch
		result.Confidence = 0.90
	case containsAny(lower, "上传", "入库", "导入", "知识库"):
		result.Intent = model.IntentUpload
		result.Confidence = 0.88
	case containsAny(lower, "游戏", "互动", "闯关"):
		result.Intent = model.IntentGame
		result.Confidence = 0.86
	case text != "":
		result.Intent = model.IntentChat
		result.Confidence = 0.65
	}
	if result.Intent != model.IntentChat && result.Intent != model.IntentUnknown && result.Topic == "" && result.LessonNo == 0 {
		result.NeedsClarification = true
		result.Clarification = "你想基于哪一讲或哪个主题生成？例如：生成春季课第10讲动能定理导图。"
	}
	if result.Pages == 0 && result.Intent == model.IntentPPT {
		result.Pages = 12
	}
	if result.Count == 0 && result.Intent == model.IntentExercises {
		result.Count = 10
	}
	return result
}

func (c Classifier) classifyWithVectors(ctx context.Context, text string) (model.IntentResult, error) {
	inputs := make([]string, 0, len(examples)+1)
	inputs = append(inputs, text)
	for _, item := range examples {
		inputs = append(inputs, item.Text)
	}
	vectors, err := c.Embedder.Embed(ctx, inputs)
	if err != nil {
		return model.IntentResult{}, err
	}
	best := model.IntentUnknown
	bestScore := -1.0
	for i, item := range examples {
		score := cosine(vectors[0], vectors[i+1])
		if score > bestScore {
			bestScore = score
			best = item.Intent
		}
	}
	result := ruleClassify(text)
	result.Intent = best
	result.Confidence = bestScore
	return result, nil
}

func (c Classifier) classifyWithAI(ctx context.Context, text string) (model.IntentResult, error) {
	prompt := `请把用户请求分类为 chat、mindmap、ppt、exercises、outline、search、upload、game、unknown 之一，并抽取 topic、season、lesson_no、pages、count。只输出 JSON。`
	resp, err := c.Chat.Chat(ctx, ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: text},
		},
		Temperature: 0,
		JSONMode:    true,
	})
	if err != nil {
		return model.IntentResult{}, err
	}
	var out model.IntentResult
	if err := json.Unmarshal([]byte(resp), &out); err != nil {
		return model.IntentResult{}, err
	}
	if out.Intent == "" {
		out.Intent = model.IntentUnknown
	}
	return out, nil
}

func mergeParams(primary, fallback model.IntentResult) model.IntentResult {
	if primary.Topic == "" {
		primary.Topic = fallback.Topic
	}
	if primary.Season == "" {
		primary.Season = fallback.Season
	}
	if primary.Edition == "" {
		primary.Edition = fallback.Edition
	}
	if primary.Track == "" {
		primary.Track = fallback.Track
	}
	if primary.LessonNo == 0 {
		primary.LessonNo = fallback.LessonNo
	}
	if primary.Pages == 0 {
		primary.Pages = fallback.Pages
	}
	if primary.Count == 0 {
		primary.Count = fallback.Count
	}
	if primary.Params == nil {
		primary.Params = fallback.Params
	}
	return primary
}

func containsAny(text string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(text, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func extractPages(text string) int {
	matches := pagesPattern.FindStringSubmatch(text)
	if len(matches) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(matches[1])
	return value
}

func extractCount(text string) int {
	matches := countPattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0
	}
	value, _ := strconv.Atoi(matches[1])
	return value
}

func extractLessonNo(text string) int {
	matches := lessonPattern.FindStringSubmatch(text)
	if len(matches) != 2 {
		return 0
	}
	return parseNumber(matches[1])
}

func extractSeason(text string) string {
	for _, season := range []string{"春季课", "暑假课", "秋季课", "寒假课"} {
		if strings.Contains(text, season) {
			return season
		}
	}
	return ""
}

func extractTrack(text string) string {
	if strings.Contains(text, "复习") {
		return "复习"
	}
	if strings.Contains(text, "新授") {
		return "新授"
	}
	return ""
}

func extractEdition(text string) string {
	if strings.Contains(text, "人教版") {
		return "人教版"
	}
	return ""
}

func extractTopic(text string) string {
	clean := text
	for _, word := range []string{"生成", "帮我", "请", "做一个", "做", "一份", "导图", "思维导图", "PPT", "ppt", "课件", "习题", "题集", "练习", "搜索", "查找", "大纲", "教学设计", "教案", "出一套", "带答案"} {
		clean = strings.ReplaceAll(clean, word, " ")
	}
	clean = pagesPattern.ReplaceAllString(clean, " ")
	clean = countPattern.ReplaceAllString(clean, " ")
	clean = lessonPattern.ReplaceAllString(clean, " ")
	clean = strings.Join(strings.Fields(clean), " ")
	runes := []rune(strings.TrimSpace(clean))
	if len(runes) > 40 {
		return string(runes[:40])
	}
	return string(runes)
}

func parseNumber(raw string) int {
	if value, err := strconv.Atoi(raw); err == nil {
		return value
	}
	table := map[rune]int{'零': 0, '一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	runes := []rune(raw)
	if raw == "十" {
		return 10
	}
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

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (sqrt(na) * sqrt(nb))
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 12; i++ {
		x = 0.5 * (x + v/x)
	}
	return x
}

func FiltersFromIntent(result model.IntentResult) model.SearchFilters {
	return model.SearchFilters{
		Season:   result.Season,
		Edition:  result.Edition,
		Track:    result.Track,
		LessonNo: result.LessonNo,
	}
}
