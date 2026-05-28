package generate

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"hermesclaw/internal/model"
)

func TestBuildPPTX(t *testing.T) {
	slides := []Slide{
		{Title: "动能定理", Lines: []string{"目标", "例题"}},
		{Title: "核心概念", Lines: []string{"动能", "功", "能量守恒"}},
	}
	data, err := BuildPPTX(slides)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("PPTX data is empty")
	}
	assertZipContains(t, data, "ppt/presentation.xml")
	assertZipContains(t, data, "ppt/slides/slide1.xml")
	assertZipContains(t, data, "ppt/slides/slide2.xml")
	assertZipContains(t, data, "ppt/theme/theme1.xml")
}

func TestBuildPPTX_CoverPage(t *testing.T) {
	slides := []Slide{
		{Title: "机械能守恒定律", Lines: []string{"第10讲", "人教版"}},
	}
	data, err := BuildPPTX(slides)
	if err != nil {
		t.Fatal(err)
	}
	assertZipContains(t, data, "ppt/slides/slide1.xml")
}

func TestBuildDOCX(t *testing.T) {
	doc := Doc{
		Title: "动能定理 习题",
		Sections: []DocSection{
			{Heading: "练习", Lines: []string{"第一题", "第二题"}},
		},
	}
	data, err := BuildDOCX(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("DOCX data is empty")
	}
	assertZipContains(t, data, "word/document.xml")
	assertZipContains(t, data, "word/styles.xml")
}

func TestBuildDOCX_MultipleSections(t *testing.T) {
	doc := Doc{
		Title: "教学大纲",
		Sections: []DocSection{
			{Heading: "教学目标", Lines: []string{"理解动能定理", "掌握能量守恒"}},
			{Heading: "重难点", Lines: []string{"动能与功的关系", "机械能守恒条件"}},
			{Heading: "课时流程", Lines: []string{"复习引入（5分钟）", "概念讲解（15分钟）", "例题分析（20分钟）"}},
		},
	}
	data, err := BuildDOCX(doc)
	if err != nil {
		t.Fatal(err)
	}
	assertZipContains(t, data, "word/document.xml")
}

func TestOutlineSlides(t *testing.T) {
	slides := outlineSlides("动能定理", 6, nil, "")
	if len(slides) < 3 {
		t.Fatalf("expected at least 3 slides, got %d", len(slides))
	}
	if slides[0].Title != "动能定理" {
		t.Errorf("first slide title = %q, want %q", slides[0].Title, "动能定理")
	}
}

func TestOutlineSlides_WithResults(t *testing.T) {
	results := []model.SearchResult{
		{
			Material: model.Material{LessonTitle: "动能定理的应用", MaterialKind: "讲义", Version: "教师版"},
		},
		{
			Material: model.Material{LessonTitle: "机械能守恒", MaterialKind: "题集", Version: "学生版"},
		},
	}
	slides := outlineSlides("机械能", 5, results, "")
	if len(slides) < 3 {
		t.Fatalf("expected at least 3 slides, got %d", len(slides))
	}
}

func TestOutlineSlides_WithNotice(t *testing.T) {
	slides := outlineSlides("动能定理", 4, nil, "未检索到课程资料")
	found := false
	for _, s := range slides {
		if strings.Contains(s.Title, "生成说明") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected notice slide not found")
	}
}

func TestKeyPointsFromResults(t *testing.T) {
	results := []model.SearchResult{
		{Material: model.Material{LessonTitle: "动能定理"}},
		{Material: model.Material{LessonTitle: "机械能守恒"}},
	}
	points := keyPointsFromResults("机械能", results)
	if len(points) < 2 {
		t.Errorf("expected at least 2 key points, got %d", len(points))
	}
}

func TestKeyPointsFromResults_Empty(t *testing.T) {
	points := keyPointsFromResults("未知主题", nil)
	if len(points) == 0 {
		t.Error("expected fallback key points for empty results")
	}
}

func TestMindmapNodes_Empty(t *testing.T) {
	nodes := mindmapNodes("动能定理", nil)
	if len(nodes) == 0 {
		t.Error("expected fallback nodes for empty results")
	}
}

func TestMindmapNodes_WithResults(t *testing.T) {
	results := []model.SearchResult{
		{Material: model.Material{LessonTitle: "动能定理", MaterialKind: "讲义", Version: "教师版"}},
		{Material: model.Material{LessonTitle: "机械能守恒", MaterialKind: "题集", Version: "学生版"}},
	}
	nodes := mindmapNodes("机械能", results)
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestMindmapNodes_Deduplication(t *testing.T) {
	results := []model.SearchResult{
		{Material: model.Material{LessonTitle: "动能定理", MaterialKind: "讲义"}},
		{Material: model.Material{LessonTitle: "动能定理", MaterialKind: "讲义"}},
		{Material: model.Material{LessonTitle: "动能定理", MaterialKind: "题集"}},
	}
	nodes := mindmapNodes("动能定理", results)
	if len(nodes) != 2 {
		t.Errorf("expected 2 unique nodes after dedup, got %d", len(nodes))
	}
}

func TestRenderMindmap(t *testing.T) {
	nodes := []mindNode{
		{Title: "核心概念", Body: "动能定理", Children: []mindNode{
			{Title: "定义", Body: "Ek = 1/2mv^2"},
			{Title: "单位", Body: "焦耳(J)"},
		}},
	}
	cites := []Citation{
		{SourcePath: "春季课/人教版/第10讲/讲义.pdf", Score: 0.85},
	}
	html := renderMindmap("动能定理", nodes, cites, "")
	if !strings.Contains(html, "动能定理") {
		t.Error("rendered HTML missing topic")
	}
	if !strings.Contains(html, "核心概念") {
		t.Error("rendered HTML missing node title")
	}
	if !strings.Contains(html, "春季课") {
		t.Error("rendered HTML missing citation")
	}
}

func TestRenderMindmap_WithNotice(t *testing.T) {
	nodes := mindmapNodes("动能定理", nil)
	html := renderMindmap("动能定理", nodes, nil, "未检索到课程资料，使用通用知识生成。")
	if !strings.Contains(html, "notice") {
		t.Error("rendered HTML missing notice")
	}
}

func TestParseSlideContent(t *testing.T) {
	json := `[{"title":"封面","bullets":["动能定理","第10讲"],"example":"","note":""},{"title":"核心概念","bullets":["动能","功","能量"],"example":"小球从斜面滑下","note":"注意正负功"}]`
	slides, err := ParseSlideContent(json)
	if err != nil {
		t.Fatalf("ParseSlideContent failed: %v", err)
	}
	if len(slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(slides))
	}
	if slides[0].Title != "封面" {
		t.Errorf("first slide title = %q", slides[0].Title)
	}
	if len(slides[1].Lines) != 4 {
		t.Errorf("second slide should have 4 lines (3 bullets + example), got %d", len(slides[1].Lines))
	}
}

func TestParseSlideContent_Invalid(t *testing.T) {
	_, err := ParseSlideContent("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseSlideContent_ExampleAndNote(t *testing.T) {
	json := `[{"title":"易错提醒","bullets":["判断守恒条件"],"example":"光滑水平面","note":"忽略空气阻力"}]`
	slides, err := ParseSlideContent(json)
	if err != nil {
		t.Fatalf("ParseSlideContent failed: %v", err)
	}
	if len(slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(slides))
	}
	hasExample := false
	hasNote := false
	for _, line := range slides[0].Lines {
		if strings.Contains(line, "【例】") {
			hasExample = true
		}
		if strings.Contains(line, "【提醒】") {
			hasNote = true
		}
	}
	if !hasExample || !hasNote {
		t.Error("expected example and note in lines")
	}
}

func TestSafeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.pdf", "normal.pdf"},
		{"file/with/slash.pdf", "file_with_slash.pdf"},
		{"file:with:colons.pdf", "file_with_colons.pdf"},
		{"very-long-name-that-exceeds-eighty-characters-because-this-test-is-intentionally-long-to-check-truncation.pdf", ""},
	}
	for _, tt := range tests {
		result := safeName(tt.input)
		if tt.expected != "" && result != tt.expected {
			t.Errorf("safeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNonEmptyLines(t *testing.T) {
	lines := nonEmptyLines("line1\n\nline2\n  \nline3")
	if len(lines) != 3 {
		t.Errorf("expected 3 non-empty lines, got %d", len(lines))
	}
}

func TestNonEmptyLines_Empty(t *testing.T) {
	lines := nonEmptyLines("")
	if len(lines) != 1 || lines[0] != "暂无内容。" {
		t.Errorf("expected fallback for empty input")
	}
}

func TestXmlEscape(t *testing.T) {
	result := xmlEscape("<Test>Title & \"Quote\"")
	if result == "<Test>Title & \"Quote\"" {
		t.Error("xmlEscape should escape special characters")
	}
	if !strings.Contains(result, "&lt;") {
		t.Error("xmlEscape missing &lt;")
	}
}

func assertZipContains(t *testing.T, data []byte, name string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name == name {
			return
		}
	}
	t.Fatalf("zip did not contain %s", name)
}
