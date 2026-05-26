package generate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"hermesclaw/internal/model"
)

type Slide struct {
	Title string
	Lines []string
}

func BuildPPTX(slides []Slide) ([]byte, error) {
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":               contentTypesPPTX(len(slides)),
		"_rels/.rels":                       packageRels("ppt/presentation.xml"),
		"docProps/app.xml":                  appProps("Hermesclaw"),
		"docProps/core.xml":                 coreProps("Hermesclaw PPT"),
		"ppt/presentation.xml":              presentationXML(len(slides)),
		"ppt/_rels/presentation.xml.rels":   presentationRels(len(slides)),
		"ppt/theme/theme1.xml":              themeXML(),
		"ppt/slideMasters/slideMaster1.xml": slideMasterXML(),
	}
	for i, slide := range slides {
		files[fmt.Sprintf("ppt/slides/slide%d.xml", i+1)] = slideXML(slide)
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

func outlineSlides(topic string, pages int, results []model.SearchResult, notice string) []Slide {
	if pages < 3 {
		pages = 3
	}
	slides := []Slide{{Title: topic, Lines: []string{"课程目标", "知识结构", "典型题型", "课堂练习"}}}
	if notice != "" {
		slides = append(slides, Slide{Title: "生成说明", Lines: []string{notice}})
	}
	points := keyPointsFromResults(topic, results)
	for len(slides) < pages-1 {
		point := points[(len(slides)-1)%len(points)]
		slides = append(slides, Slide{Title: point, Lines: []string{
			"核心概念：明确物理量和适用条件",
			"模型方法：先画过程图，再列关系式",
			"易错提醒：区分对象、过程和状态量",
			"课堂练习：设置一道基础题和一道提升题",
		}})
	}
	slides = append(slides, Slide{Title: "资料引用", Lines: citationLines(results)})
	return slides
}

type SlideContentRequest struct {
	Topic    string
	PageNo   int
	Total    int
	Context  string
	SlideType string
}

const slideSystemPrompt = `你是高中物理教研专家。请为每一页PPT生成精炼的内容，输出JSON数组格式：
[
  {
    "title": "页面标题（不超过20字）",
    "bullets": ["要点1", "要点2", "要点3"],
    "example": "典型例题或具体数据（可空）",
    "note": "易错提醒或教学提示（可空）"
  }
]
每页内容要精炼，禁止大段文字，使用bullet points。封面页只需title和bullets。`

func BuildSlideContentJSON(topic string, totalPages int, context string) (string, error) {
	prompt := fmt.Sprintf("课程主题：%s\n总页数：%d\n课程资料摘要：\n%s\n\n请为每一页生成标题和要点内容。", topic, totalPages, context)
	return slideSystemPrompt + "\n" + prompt, nil
}

func ParseSlideContent(content string) ([]Slide, error) {
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid JSON array in AI response")
	}
	jsonStr := strings.TrimSpace(content[start : end+1])

	type slideJSON struct {
		Title    string   `json:"title"`
		Bullets  []string `json:"bullets"`
		Example  string   `json:"example"`
		Note     string   `json:"note"`
	}
	var slides []slideJSON
	if err := json.Unmarshal([]byte(jsonStr), &slides); err != nil {
		return nil, fmt.Errorf("failed to parse slide JSON: %w", err)
	}
	result := make([]Slide, 0, len(slides))
	for _, s := range slides {
		lines := s.Bullets
		if s.Example != "" {
			lines = append(lines, "【例】"+s.Example)
		}
		if s.Note != "" {
			lines = append(lines, "【提醒】"+s.Note)
		}
		result = append(result, Slide{Title: s.Title, Lines: lines})
	}
	return result, nil
}

func jsonUnmarshalSafe(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func contentTypesPPTX(slides int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/><Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	for i := 1; i <= slides; i++ {
		b.WriteString(fmt.Sprintf(`<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i))
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func presentationXML(slides int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rIdMaster"/></p:sldMasterIdLst><p:sldIdLst>`)
	for i := 1; i <= slides; i++ {
		b.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 255+i, i))
	}
	b.WriteString(`</p:sldIdLst><p:sldSz cx="12192000" cy="6858000" type="screen16x9"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`)
	return b.String()
}

func presentationRels(slides int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 1; i <= slides; i++ {
		b.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i, i))
	}
	b.WriteString(`<Relationship Id="rIdMaster" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/><Relationship Id="rIdTheme" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/></Relationships>`)
	return b.String()
}

func slideXML(slide Slide) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	b.WriteString(textBox(2, "Title", 640000, 420000, 10900000, 760000, 34, slide.Title, true))
	body := strings.Join(slide.Lines, "\n")
	b.WriteString(textBox(3, "Body", 900000, 1450000, 10400000, 4400000, 22, body, false))
	b.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
	return b.String()
}

func textBox(id int, name string, x, y, cx, cy int, size int, text string, bold bool) string {
	var b strings.Builder
	boldXML := ""
	if bold {
		boldXML = ` b="1"`
	}
	b.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr><p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>`, id, name, x, y, cx, cy))
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(fmt.Sprintf(`<a:p><a:r><a:rPr lang="zh-CN" sz="%d00"%s><a:latin typeface="Microsoft YaHei"/><a:ea typeface="Microsoft YaHei"/></a:rPr><a:t>%s</a:t></a:r></a:p>`, size, boldXML, xmlEscape(line)))
	}
	b.WriteString(`</p:txBody></p:sp>`)
	return b.String()
}

func slideMasterXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst/></p:sldMaster>`
}

func themeXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Hermesclaw"><a:themeElements><a:clrScheme name="Hermesclaw"><a:dk1><a:srgbClr val="172033"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="F8FBFF"/></a:lt2><a:accent1><a:srgbClr val="1F6FEB"/></a:accent1><a:accent2><a:srgbClr val="38A169"/></a:accent2><a:accent3><a:srgbClr val="D69E2E"/></a:accent3><a:accent4><a:srgbClr val="DD6B20"/></a:accent4><a:accent5><a:srgbClr val="805AD5"/></a:accent5><a:accent6><a:srgbClr val="319795"/></a:accent6><a:hlink><a:srgbClr val="1F6FEB"/></a:hlink><a:folHlink><a:srgbClr val="805AD5"/></a:folHlink></a:clrScheme><a:fontScheme name="Hermesclaw"><a:majorFont><a:latin typeface="Microsoft YaHei"/><a:ea typeface="Microsoft YaHei"/></a:majorFont><a:minorFont><a:latin typeface="Microsoft YaHei"/><a:ea typeface="Microsoft YaHei"/></a:minorFont></a:fontScheme><a:fmtScheme name="Hermesclaw"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`
}
