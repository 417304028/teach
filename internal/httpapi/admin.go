package httpapi

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
)

func adminFuncs() template.FuncMap {
	return template.FuncMap{
		"statusClass": statusClass,
		"fileSize":    fileSize,
		"fileType":    fileType,
		"ext":         ext,
		"first":       first,
		"jsEscape":    jsEscape,
	}
}

func first(n int, s []any) []any {
	if n > len(s) {
		return s
	}
	return s[:n]
}

func jsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

var adminTemplate = template.Must(template.New("admin").Funcs(adminFuncs()).Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Hermesclaw 管理后台</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { background: #f0f4f8; color: #1a202c; font-family: "Microsoft YaHei", system-ui, sans-serif; min-height: 100vh; }
    header { background: linear-gradient(135deg, #1e3a8a, #2563eb); color: white; padding: 0 28px; display: flex; align-items: center; justify-content: space-between; }
    .logo { font-size: 18px; font-weight: 700; padding: 16px 0; }
    nav { display: flex; gap: 0; }
    nav a { color: rgba(255,255,255,0.8); text-decoration: none; padding: 16px 18px; font-size: 14px; display: block; border-bottom: 3px solid transparent; }
    nav a:hover, nav a.active { color: white; border-bottom-color: #60a5fa; background: rgba(255,255,255,0.05); }
    main { max-width: 1280px; margin: 0 auto; padding: 28px; }
    h1 { font-size: 22px; margin-bottom: 20px; color: #1e40af; }
    .page { display: none; }
    .page.active { display: block; }

    .stats { display: grid; grid-template-columns: repeat(5, 1fr); gap: 16px; margin-bottom: 28px; }
    .stat-card { background: white; border: 1px solid #dbeafe; border-radius: 12px; padding: 20px 24px; position: relative; overflow: hidden; }
    .stat-card::before { content: ''; position: absolute; top: 0; left: 0; right: 0; height: 3px; border-radius: 12px 12px 0 0; }
    .stat-card.blue::before { background: #3b82f6; }
    .stat-card.green::before { background: #10b981; }
    .stat-card.amber::before { background: #f59e0b; }
    .stat-card.purple::before { background: #8b5cf6; }
    .stat-card.red::before { background: #ef4444; }
    .stat-card .label { font-size: 13px; color: #64748b; margin-bottom: 8px; }
    .stat-card .value { font-size: 30px; font-weight: 700; color: #1e3a8a; }
    .stat-card .sub { font-size: 12px; color: #94a3b8; margin-top: 4px; }

    .toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; background: white; padding: 14px 18px; border: 1px solid #dbeafe; border-radius: 10px; }
    .search-box { display: flex; align-items: center; gap: 8px; flex: 1; }
    .search-box input { flex: 1; padding: 8px 14px; border: 1px solid #e2e8f0; border-radius: 8px; font-size: 14px; font-family: inherit; outline: none; }
    .search-box input:focus { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59,130,246,0.1); }
    select { padding: 8px 12px; border: 1px solid #e2e8f0; border-radius: 8px; font-size: 14px; font-family: inherit; background: white; cursor: pointer; }
    .btn { padding: 8px 16px; border-radius: 8px; font-size: 13px; font-weight: 500; cursor: pointer; border: none; transition: all 0.2s; font-family: inherit; }
    .btn-primary { background: #2563eb; color: white; }
    .btn-primary:hover { background: #1d4ed8; }
    .btn-ghost { background: transparent; color: #64748b; border: 1px solid #e2e8f0; }
    .btn-ghost:hover { background: #f1f5f9; }

    .card { background: white; border: 1px solid #dbeafe; border-radius: 12px; overflow: hidden; }
    .card-header { padding: 14px 20px; border-bottom: 1px solid #f1f5f9; display: flex; justify-content: space-between; align-items: center; }
    .card-header h2 { font-size: 16px; font-weight: 600; color: #1e40af; }
    table { width: 100%; border-collapse: collapse; }
    thead tr { background: #f8fafc; }
    th { padding: 10px 16px; text-align: left; font-size: 12px; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #e2e8f0; white-space: nowrap; }
    td { padding: 12px 16px; font-size: 14px; border-bottom: 1px solid #f1f5f9; vertical-align: middle; }
    tbody tr:hover { background: #f8fafc; }
    tbody tr:last-child td { border-bottom: none; }

    .badge { display: inline-block; padding: 3px 10px; border-radius: 12px; font-size: 12px; font-weight: 500; }
    .badge-success { background: #d1fae5; color: #065f46; }
    .badge-warning { background: #fef3c7; color: #92400e; }
    .badge-error { background: #fee2e2; color: #991b1b; }
    .badge-info { background: #dbeafe; color: #1e40af; }
    .badge-gray { background: #f1f5f9; color: #475569; }

    .path-cell { font-family: monospace; font-size: 13px; color: #475569; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

    .actions { display: flex; gap: 6px; }
    .action-btn { padding: 4px 10px; border-radius: 6px; font-size: 12px; cursor: pointer; border: 1px solid #e2e8f0; background: white; color: #475569; font-family: inherit; }
    .action-btn:hover { background: #f1f5f9; border-color: #3b82f6; color: #2563eb; }
    .action-btn.danger:hover { border-color: #ef4444; color: #dc2626; }

    .preview-modal { display: none; position: fixed; inset: 0; z-index: 1000; background: rgba(0,0,0,0.6); align-items: center; justify-content: center; }
    .preview-modal.open { display: flex; }
    .preview-content { background: white; border-radius: 14px; max-width: 900px; width: 90vw; max-height: 80vh; overflow: auto; }
    .preview-header { padding: 18px 24px; border-bottom: 1px solid #e2e8f0; display: flex; justify-content: space-between; align-items: center; }
    .preview-header h3 { font-size: 16px; color: #1e40af; }
    .preview-close { width: 32px; height: 32px; border-radius: 8px; border: none; background: #f1f5f9; cursor: pointer; font-size: 18px; display: flex; align-items: center; justify-content: center; }
    .preview-close:hover { background: #e2e8f0; }
    .preview-body { padding: 24px; font-size: 14px; line-height: 1.8; color: #374151; }
    .preview-body pre { white-space: pre-wrap; word-break: break-all; font-family: "Microsoft YaHei", monospace; background: #f8fafc; padding: 16px; border-radius: 8px; border: 1px solid #e2e8f0; }

    .tabs { display: flex; gap: 4px; background: white; border: 1px solid #dbeafe; border-radius: 10px; padding: 4px; margin-bottom: 20px; }
    .tab { padding: 8px 20px; border-radius: 8px; font-size: 14px; font-weight: 500; cursor: pointer; border: none; background: transparent; color: #64748b; font-family: inherit; transition: all 0.2s; }
    .tab:hover { background: #f1f5f9; }
    .tab.active { background: #2563eb; color: white; }

    .empty-state { text-align: center; padding: 48px; color: #94a3b8; font-size: 15px; }
    .empty-state svg { margin-bottom: 12px; opacity: 0.4; }

    .row-count { font-size: 13px; color: #94a3b8; }
    .time-ago { font-size: 12px; color: #94a3b8; }
  </style>
</head>
<body>
  <header>
    <div class="logo">Hermesclaw 管理后台</div>
    <nav>
      <a href="#" data-page="dashboard" class="active" onclick="switchPage(this)">Dashboard</a>
      <a href="#" data-page="materials" onclick="switchPage(this)">课程资料</a>
      <a href="#" data-page="jobs" onclick="switchPage(this)">生成任务</a>
      <a href="#" data-page="files" onclick="switchPage(this)">生成文件</a>
    </nav>
  </header>

  <main>
    <div id="page-dashboard" class="page active">
      <h1>系统概览</h1>
      <div class="stats">
        <div class="stat-card blue"><div class="label">课程资料</div><div class="value">{{.Data.Stats.Materials}}</div><div class="sub">PDF 文档</div></div>
        <div class="stat-card green"><div class="label">文本分块</div><div class="value">{{.Data.Stats.Chunks}}</div><div class="sub">已向量化</div></div>
        <div class="stat-card amber"><div class="label">生成任务</div><div class="value">{{.Data.Stats.Jobs}}</div><div class="sub">总任务数</div></div>
        <div class="stat-card purple"><div class="label">生成文件</div><div class="value">{{.Data.Stats.Files}}</div><div class="sub">有效文件</div></div>
        <div class="stat-card blue"><div class="label">聊天记录</div><div class="value">{{.Data.Stats.Messages}}</div><div class="sub">对话数</div></div>
      </div>
      <div class="card">
        <div class="card-header"><h2>最近任务</h2><span class="row-count">{{len .Data.Jobs}} 条</span></div>
        <table>
          <thead><tr><th>时间</th><th>类型</th><th>状态</th><th>用户</th><th>消息</th></tr></thead>
          <tbody>
            {{range first 10 .Data.Jobs}}
            <tr>
              <td><span class="time-ago">{{.CreatedAt}}</span></td>
              <td><span class="badge badge-info">{{.Type}}</span></td>
              <td><span class="badge {{statusClass .Status}}">{{.Status}}</span></td>
              <td>{{.UserID}}</td>
              <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{.Message}}</td>
            </tr>
            {{else}}
            <tr><td colspan="5"><div class="empty-state">暂无任务记录</div></td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>

    <div id="page-materials" class="page">
      <h1>课程资料库</h1>
      <div class="toolbar">
        <div class="search-box">
          <input type="text" id="material-search" placeholder="搜索课程、章节、版本..." onkeyup="filterTable('material-table', this.value)">
        </div>
        <select onchange="filterTable('material-table', document.getElementById('material-search').value, this.value)">
          <option value="">全部季节</option>
          <option value="春季课">春季课</option>
          <option value="秋季课">秋季课</option>
          <option value="暑假课">暑假课</option>
          <option value="寒假课">寒假课</option>
        </select>
        <select onchange="filterTable('material-table', document.getElementById('material-search').value, '', this.value)">
          <option value="">全部类型</option>
          <option value="讲义">讲义</option>
          <option value="题集">题集</option>
          <option value="期末复习">期末复习</option>
        </select>
        <span class="row-count" id="material-count"></span>
      </div>
      <div class="card">
        <table id="material-table">
          <thead><tr><th>季节</th><th>讲次</th><th>标题</th><th>类型</th><th>版本</th><th>来源路径</th></tr></thead>
          <tbody>
            {{range .Data.Materials}}
            <tr data-season="{{.Season}}" data-kind="{{.MaterialKind}}">
              <td><span class="badge badge-gray">{{.Season}}</span></td>
              <td>第{{.LessonNo}}讲</td>
              <td><strong>{{.LessonTitle}}</strong></td>
              <td><span class="badge badge-info">{{.MaterialKind}}</span></td>
              <td>{{.Version}}</td>
              <td class="path-cell" title="{{.SourcePath}}">{{.SourcePath}}</td>
            </tr>
            {{else}}
            <tr><td colspan="6"><div class="empty-state">暂无资料，请先导入课程</div></td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>

    <div id="page-jobs" class="page">
      <h1>生成任务</h1>
      <div class="toolbar">
        <div class="search-box">
          <input type="text" id="job-search" placeholder="搜索任务类型、用户..." onkeyup="filterTable('job-table', this.value)">
        </div>
        <select onchange="filterTable('job-table', document.getElementById('job-search').value, this.value)">
          <option value="">全部状态</option>
          <option value="succeeded">成功</option>
          <option value="failed">失败</option>
          <option value="pending">等待中</option>
          <option value="running">进行中</option>
        </select>
        <span class="row-count">{{len .Data.Jobs}} 条任务</span>
      </div>
      <div class="card">
        <table id="job-table">
          <thead><tr><th>时间</th><th>类型</th><th>状态</th><th>用户</th><th>参数</th><th>结果</th></tr></thead>
          <tbody>
            {{range .Data.Jobs}}
            <tr data-status="{{.Status}}">
              <td><span class="time-ago">{{.CreatedAt}}</span></td>
              <td><span class="badge badge-info">{{.Type}}</span></td>
              <td><span class="badge {{statusClass .Status}}">{{.Status}}</span></td>
              <td>{{.UserID}}</td>
              <td style="font-size:12px;color:#64748b">{{.Params.topic}}</td>
              <td>
                {{if .FileID}}<a href="/api/files/{{.FileID}}" class="action-btn">下载</a>{{end}}
                {{if .Error}}<button class="action-btn danger" onclick="showError('{{jsEscape .Error}}')">错误</button>{{end}}
              </td>
            </tr>
            {{else}}
            <tr><td colspan="6"><div class="empty-state">暂无任务记录</div></td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>

    <div id="page-files" class="page">
      <h1>生成文件</h1>
      <div class="toolbar">
        <div class="search-box">
          <input type="text" id="file-search" placeholder="搜索文件名..." onkeyup="filterTable('file-table', this.value)">
        </div>
        <select onchange="filterTable('file-table', document.getElementById('file-search').value, this.value)">
          <option value="">全部类型</option>
          <option value="pptx">PPTX</option>
          <option value="docx">DOCX</option>
          <option value="html">HTML</option>
        </select>
        <span class="row-count">{{len .Data.Files}} 个文件</span>
      </div>
      <div class="card">
        <table id="file-table">
          <thead><tr><th>文件名</th><th>大小</th><th>类型</th><th>过期时间</th><th>创建时间</th><th>操作</th></tr></thead>
          <tbody>
            {{range .Data.Files}}
            <tr data-ext="{{ext .Name}}">
              <td><strong>{{.Name}}</strong></td>
              <td>{{fileSize .SizeBytes}}</td>
              <td><span class="badge badge-gray">{{fileType .Name}}</span></td>
              <td><span class="time-ago">{{.ExpiresAt}}</span></td>
              <td><span class="time-ago">{{.CreatedAt}}</span></td>
              <td>
                <div class="actions">
                  <a href="/api/files/{{.ID}}" class="action-btn">下载</a>
                  {{if eq (ext .Name) "html"}}
                  <button class="action-btn" onclick="previewFile('/api/files/{{.ID}}', 'html')">预览</button>
                  {{end}}
                  {{if eq (ext .Name) "docx"}}
                  <button class="action-btn" onclick="previewFile('/api/files/{{.ID}}', 'docx')">预览</button>
                  {{end}}
                </div>
              </td>
            </tr>
            {{else}}
            <tr><td colspan="6"><div class="empty-state">暂无文件</div></td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>
  </main>

  <div id="preview-modal" class="preview-modal">
    <div class="preview-content">
      <div class="preview-header">
        <h3 id="preview-title">文件预览</h3>
        <button class="preview-close" onclick="closePreview()">×</button>
      </div>
      <div class="preview-body" id="preview-body"></div>
    </div>
  </div>

  <script>
    function switchPage(el) {
      document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
      document.querySelectorAll('nav a').forEach(a => a.classList.remove('active'));
      var page = el.getAttribute('data-page');
      document.getElementById('page-' + page).classList.add('active');
      el.classList.add('active');
    }

    function filterTable(tableId, keyword, extra) {
      var table = document.getElementById(tableId);
      if (!table) return;
      var rows = table.querySelectorAll('tbody tr');
      var count = 0;
      keyword = keyword.toLowerCase();
      rows.forEach(function(row) {
        var text = row.textContent.toLowerCase();
        var show = text.includes(keyword);
        if (extra && extra !== '') {
          var attrs = ['data-season','data-kind','data-status','data-ext'];
          var found = attrs.some(function(attr) {
            return row.getAttribute(attr) === extra;
          });
          show = show && found;
        }
        row.style.display = show ? '' : 'none';
        if (show) count++;
      });
      var countEl = document.getElementById(tableId.replace('-table', '-count'));
      if (countEl) countEl.textContent = count + ' 条';
    }

    async function previewFile(url, type) {
      var modal = document.getElementById('preview-modal');
      var body = document.getElementById('preview-body');
      var title = document.getElementById('preview-title');
      modal.classList.add('open');
      title.textContent = '文件预览';
      body.innerHTML = '<div style="text-align:center;padding:40px;color:#94a3b8;">加载中...</div>';
      try {
        var resp = await fetch(url);
        if (type === 'html') {
          var text = await resp.text();
          body.innerHTML = '<div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:8px;padding:16px;max-height:60vh;overflow:auto"><pre>' + escapeHtml(text.substring(0, 5000)) + '</pre></div>';
        } else {
          body.innerHTML = '<div style="text-align:center;padding:40px;color:#94a3b8;">DOCX 文件请下载后查看，或点击 <a href="' + url + '" style="color:#2563eb">下载</a></div>';
        }
      } catch(e) {
        body.innerHTML = '<div style="color:#ef4444;padding:20px;">预览失败：' + e.message + '</div>';
      }
    }

    function closePreview() {
      document.getElementById('preview-modal').classList.remove('open');
    }

    function showError(msg) {
      alert('错误: ' + msg);
    }

    function escapeHtml(str) {
      return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
    }
  </script>
</body>
</html>`))

func renderAdmin(w http.ResponseWriter, title string, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = adminTemplate.Execute(w, map[string]any{"Title": title, "Data": data})
}

func statusClass(status string) string {
	switch status {
	case "succeeded":
		return "badge-success"
	case "failed":
		return "badge-error"
	case "pending":
		return "badge-warning"
	case "running":
		return "badge-info"
	default:
		return "badge-gray"
	}
}

func fileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
}

func fileType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".pptx":
		return "PPT"
	case ".docx":
		return "Word"
	case ".html":
		return "HTML"
	case ".pdf":
		return "PDF"
	default:
		return strings.TrimPrefix(ext, ".")
	}
}

func ext(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}
