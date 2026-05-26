package generate

import (
	"fmt"
	"html"
	"strings"

	"hermesclaw/internal/model"
)

type mindNode struct {
	Title    string
	Body     string
	Children []mindNode
}

func mindmapNodes(topic string, results []model.SearchResult) []mindNode {
	nodes := []mindNode{}
	if len(results) == 0 {
		return []mindNode{
			{Title: "核心概念", Body: topic, Children: []mindNode{
				{Title: "物理量", Body: "定义与单位"},
				{Title: "适用条件", Body: "何时使用"},
			}},
			{Title: "典型题型", Body: "概念辨析、模型建构、计算应用", Children: []mindNode{
				{Title: "选择题", Body: "辨析类"},
				{Title: "计算题", Body: "综合应用"},
			}},
			{Title: "课堂活动", Body: "例题讲解、分组练习、即时反馈", Children: []mindNode{
				{Title: "例题", Body: "教师演示"},
				{Title: "练习", Body: "学生动手"},
			}},
			{Title: "课后巩固", Body: "基础题、提升题、错题整理", Children: []mindNode{
				{Title: "基础题", Body: "课后作业A"},
				{Title: "提升题", Body: "课后作业B"},
			}},
		}
	}
	seen := map[string]bool{}
	for _, result := range results {
		title := result.Material.LessonTitle
		if title == "" {
			title = result.Material.MaterialKind
		}
		key := title + result.Material.MaterialKind
		if seen[key] {
			continue
		}
		seen[key] = true
		nodes = append(nodes, mindNode{
			Title: title,
			Body:  result.Material.MaterialKind + " · " + result.Material.Version,
			Children: []mindNode{
				{Title: "版本", Body: result.Material.Version},
				{Title: "来源", Body: result.Material.Season},
			},
		})
		if len(nodes) >= 6 {
			break
		}
	}
	return nodes
}

const mindmapHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s 导图</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { background: #f0f4f8; font-family: "Microsoft YaHei", system-ui, sans-serif; color: #1a202c; min-height: 100vh; }
header { background: linear-gradient(135deg, #1e3a8a, #2563eb); color: white; padding: 20px 32px; display: flex; align-items: center; gap: 16px; }
header h1 { font-size: 22px; font-weight: 600; }
header .badge { background: rgba(255,255,255,0.2); padding: 4px 12px; border-radius: 12px; font-size: 13px; }
main { padding: 28px 32px; }
.mind-container { display: flex; align-items: center; justify-content: center; min-height: 70vh; }
.mind-tree { display: flex; gap: 48px; align-items: flex-start; justify-content: center; flex-wrap: wrap; }
.level-1 { position: relative; }
.center-node { background: linear-gradient(135deg, #1e40af, #3b82f6); color: white; padding: 18px 36px; border-radius: 16px; font-size: 18px; font-weight: 600; box-shadow: 0 4px 20px rgba(37,99,235,0.35); cursor: default; text-align: center; min-width: 180px; }
.level-2 { display: flex; flex-direction: column; gap: 16px; margin-top: 0; }
.branch { display: flex; flex-direction: column; gap: 10px; }
.branch-header { display: flex; align-items: center; gap: 10px; }
.branch-line { width: 40px; height: 2px; background: #93c5fd; }
.node-card { background: white; border: 2px solid #dbeafe; border-radius: 12px; padding: 14px 20px; min-width: 200px; cursor: pointer; transition: all 0.2s; position: relative; }
.node-card:hover { border-color: #3b82f6; box-shadow: 0 4px 16px rgba(59,130,246,0.15); transform: translateY(-2px); }
.node-title { font-size: 15px; font-weight: 600; color: #1e40af; margin-bottom: 4px; }
.node-body { font-size: 13px; color: #64748b; }
.toggle { position: absolute; right: 12px; top: 50%%; transform: translateY(-50%%); width: 20px; height: 20px; background: #dbeafe; border-radius: 50%%; display: flex; align-items: center; justify-content: center; font-size: 12px; color: #3b82f6; font-weight: bold; }
.level-3 { margin-left: 28px; margin-top: 8px; display: none; flex-direction: column; gap: 6px; border-left: 2px solid #e2e8f0; padding-left: 12px; }
.level-3.open { display: flex; }
.sub-node { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 8px; padding: 8px 14px; font-size: 13px; }
.sub-node-title { font-weight: 500; color: #475569; }
.sub-node-body { color: #94a3b8; font-size: 12px; margin-top: 2px; }
.citation { background: white; border: 1px solid #e2e8f0; border-radius: 12px; padding: 20px 24px; margin-top: 32px; }
.citation h2 { font-size: 16px; color: #1e40af; margin-bottom: 12px; }
.citation li { margin: 6px 0; color: #475569; font-size: 14px; }
.citation li span { color: #94a3b8; }
.notice { background: #fef3c7; border: 1px solid #fcd34d; color: #92400e; padding: 12px 20px; border-radius: 10px; margin-bottom: 20px; font-size: 14px; }
</style>
</head>
<body>
<header>
  <h1>%s 导图</h1>
  <span class="badge">%s</span>
</header>
<main>
  %s
  <div class="mind-container">
    <div class="mind-tree">
      <div class="level-1">
        <div class="center-node">%s</div>
      </div>
      <div class="level-2">%s</div>
    </div>
  </div>
  <div class="citation">
    <h2>引用资料</h2>
    <ol>%s</ol>
  </div>
</main>
<script>
document.querySelectorAll('.node-card').forEach(function(card) {
  card.addEventListener('click', function() {
    var sub = card.querySelector('.level-3');
    if (!sub) return;
    var isOpen = sub.classList.contains('open');
    sub.classList.toggle('open');
    card.querySelector('.toggle').textContent = isOpen ? '+' : '−';
  });
});
</script>
</body>
</html>`

func renderMindmap(topic string, nodes []mindNode, cites []Citation, notice string) string {
	var branches strings.Builder
	for _, node := range nodes {
		children := renderChildren(node.Children)
		childrenHTML := ""
		if len(node.Children) > 0 {
			childrenHTML = `<div class="level-3"><div class="sub-node"><div class="sub-node-title">` + html.EscapeString(node.Title) + `</div><div class="sub-node-body">` + html.EscapeString(node.Body) + `</div></div>` + children + `</div>`
			branches.WriteString(fmt.Sprintf(`<div class="branch"><div class="branch-header"><div class="branch-line"></div><div class="node-card" onclick="this.querySelector('.level-3')&&(this.querySelector('.level-3').classList.toggle('open'),this.querySelector('.toggle').textContent=this.querySelector('.level-3').classList.contains('open')?'−':'+')"><div class="node-title">%s</div><div class="node-body">%s</div><div class="toggle">+</div>%s</div></div></div>`,
				html.EscapeString(node.Title), html.EscapeString(node.Body), childrenHTML))
		} else {
			branches.WriteString(fmt.Sprintf(`<div class="branch"><div class="branch-header"><div class="branch-line"></div><div class="node-card"><div class="node-title">%s</div><div class="node-body">%s</div></div></div></div>`,
				html.EscapeString(node.Title), html.EscapeString(node.Body)))
		}
	}

	var citesHTML strings.Builder
	for _, cite := range cites {
		citesHTML.WriteString(fmt.Sprintf("<li>%s <span>%.2f</span></li>", html.EscapeString(cite.SourcePath), cite.Score))
	}
	if citesHTML.Len() == 0 {
		citesHTML.WriteString("<li>未命中课程资料</li>")
	}

	noticeHTML := ""
	if notice != "" {
		noticeHTML = `<div class="notice">` + html.EscapeString(notice) + `</div>`
	}

	citationCount := len(cites)
	citationLabel := fmt.Sprintf("%d 个来源", citationCount)
	if citationCount == 0 {
		citationLabel = "通用生成"
	}

	return fmt.Sprintf(mindmapHTML,
		html.EscapeString(topic),
		html.EscapeString(topic),
		citationLabel,
		noticeHTML,
		html.EscapeString(topic),
		branches.String(),
		citesHTML.String(),
	)
}

func renderChildren(children []mindNode) string {
	var b strings.Builder
	for _, child := range children {
		b.WriteString(fmt.Sprintf(`<div class="sub-node"><div class="sub-node-title">%s</div><div class="sub-node-body">%s</div></div>`,
			html.EscapeString(child.Title), html.EscapeString(child.Body)))
	}
	return b.String()
}

func noticeHTML(notice string) string {
	if notice == "" {
		return ""
	}
	return `<p class="notice">` + html.EscapeString(notice) + `</p>`
}
