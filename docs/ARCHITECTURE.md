# Hermesclaw 架构文档

## 系统概述

Hermesclaw 是一个高中物理课件智能生成平台，通过 RAG + DeepSeek 技术，从本地课程资料库中检索相关内容，生成 PPT、Word 文档、思维导图等教学材料。

## 核心架构

### 分层设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        接入层                                      │
│  ┌──────────┐    ┌──────────────┐    ┌───────────────────────┐ │
│  │ QQ Bot   │    │  微信 Bot    │    │  HTTP API / Admin UI  │ │
│  │(NapCat)  │    │  (Clawbot)   │    │                       │ │
│  └────┬─────┘    └──────┬───────┘    └──────────┬────────────┘ │
│       │                  │                         │               │
└───────┼──────────────────┼─────────────────────────┼───────────────┘
        │                  │                         │
        ▼                  ▼                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                      意图识别层                                   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   Classifier.Classify()                   │   │
│  │                                                          │   │
│  │  Layer 1: 规则匹配 (keyword regex)       confidence≥0.85 │   │
│  │      ↓                                                    │   │
│  │  Layer 2: 向量相似度 (cosine)            confidence≥0.72 │   │
│  │      ↓                                                    │   │
│  │  Layer 3: DeepSeek AI 辅助               confidence≥0.65 │   │
│  │                                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  输出: IntentResult { Intent, Confidence, Topic, LessonNo, ... }  │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      智能决策层                                   │
│                                                                  │
│  if 置信度低 → 追问用户                                         │
│  if 意图明确    → 检查资料库                                     │
│     ├ 有资料 → RAG 检索 → DeepSeek 生成                          │
│     └ 无资料 → DeepSeek 通用能力回答                             │
│  if 意图模糊  → 追问                                             │
│                                                                  │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                     多模态生成层                                  │
│                                                                  │
│  ┌──────────┐  ┌───────────┐  ┌──────────────┐  ┌────────────┐ │
│  │ PPT生成  │  │ Word生成  │  │ 思维导图生成 │  │ 智能回答   │ │
│  │ pptx.go │  │ docx.go   │  │ mindmap.go   │  │ Answer()   │ │
│  └────┬─────┘  └─────┬─────┘  └──────┬───────┘  └─────┬──────┘ │
│       │               │                 │                 │         │
│       └───────────────┴─────────────────┴─────────────────┘         │
│                               │                                     │
│                               ▼                                     │
│                    ┌─────────────────────┐                         │
│                    │   AI Content Gen   │                         │
│                    │   (DeepSeek)        │                         │
│                    └─────────────────────┘                         │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                       存储层                                      │
│                                                                  │
│  ┌────────────────┐          ┌──────────────────────────────┐   │
│  │  PostgreSQL    │          │      文件系统                │   │
│  │  + pgvector   │          │   data/files/YYYYMMDD/       │   │
│  │                │          │   *.pptx / *.docx / *.html  │   │
│  │  materials     │          │   30分钟自动过期清理          │   │
│  │  chunks (vec)  │          └──────────────────────────────┘   │
│  │  jobs          │                                             │
│  │  files         │                                             │
│  │  messages      │                                             │
│  └────────────────┘                                             │
└─────────────────────────────────────────────────────────────────┘
```

### 数据流

```
用户消息
    │
    ▼
Webhook Handler (QQ/微信)
    │
    ▼
消息文本
    │
    ▼
意图识别 ──(置信度<0.72)──▶ 追问用户
    │
    ▼
RAG 检索 ──(无结果)──▶ DeepSeek 通用回答
    │
    ▼
上下文构建 (检索结果 → context block)
    │
    ▼
DeepSeek 生成 (PPT/Word/Mindmap/大纲)
    │
    ▼
文件写入 (data/files/YYYYMMDD/)
    │
    ▼
消息回复 (文件 URL + 摘要)
```

### 模块职责

#### `internal/ai/provider.go`

AI 能力抽象层，支持两种后端：

- **DeepSeek** (`DeepSeekChat`)：调用 DeepSeek API 生成内容
- **本地 Fallback** (`LocalChat`)：无 API Key 时的模板化响应
- **DashScope Embedding**：阿里云向量嵌入服务（1024 维）
- **本地 Embedding** (`LocalEmbedding`)：基于 FNV hash 的伪向量

#### `internal/intent/classifier.go`

三层混合意图识别：

1. **规则层**：正则表达式匹配关键词（置信度 0.88-0.96）
2. **向量层**：计算用户文本与示例的余弦相似度（需 embedder）
3. **AI 层**：DeepSeek API 辅助判断（需 chat provider）

参数提取：季节、课程号、页数、题目数量、教材版本等

#### `internal/rag/rag.go`

RAG 核心服务：

- **IngestPath**：解压 ZIP → 解析 PDF → 分块 → 向量化 → 入库
- **Search**：query 向量化 → pgvector 余弦检索 → 按阈值过滤
- **PDF 提取**：优先使用 `ledongthuc/pdf` 库，fallback 到正则

#### `internal/generate/`

多模态生成服务：

- **PPT**：使用 `outlineSlides` 生成幻灯片结构，AI 填充具体内容
- **Word**：生成 `.docx`（Office Open XML 格式，无需 external libs）
- **Mindmap**：生成带交互的 HTML/SVG 思维导图（可点击展开子节点）

#### `internal/store/`

存储抽象，支持双后端：

- **JSON Store**：`data/store.json`，开发/轻量使用
- **Postgres Store**：PostgreSQL + pgvector，支持向量检索

#### `internal/qq/onebot.go` / `internal/wechat/clawbot.go`

消息接入适配器：

- QQ：解析 OneBot v11 协议事件，转发给 `app.Service`
- 微信：解析 Clawbot webhook 事件，转发给 `app.Service`

#### `internal/httpapi/server.go`

HTTP 接口层：

- `/api/chat` - 聊天接口
- `/api/ingest` - 资料导入
- `/api/generate/*` - 各生成接口
- `/api/files/:id` - 文件下载
- `/onebot/event` - QQ 事件 webhook
- `/clawbot/event` - 微信事件 webhook
- `/admin/*` - 管理后台

## 数据库设计

### 向量检索

使用 `pgvector` 扩展，`chunks` 表存储 1024 维向量：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE chunks (
  embedding vector(1024)
);
CREATE INDEX ON chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

检索时使用 `<=>`（余弦距离）操作符，排序取 top-K。

### 关键表

- `materials`：课程资料元信息（SHA256 去重）
- `chunks`：文本分块 + 向量
- `generation_jobs`：异步生成任务状态
- `files`：生成文件记录（带过期时间）
- `chat_messages`：聊天历史

## 意图识别决策树

```
Classify(text)
    │
    ├─▶ ruleClassify(text)
    │        ├─ keyword "导图" → Mindmap (0.96)
    │        ├─ keyword "PPT/课件" → PPT (0.95)
    │        ├─ keyword "习题/练习" → Exercises (0.94)
    │        ├─ keyword "大纲/教案" → Outline (0.93)
    │        ├─ keyword "搜索/检索" → Search (0.90)
    │        ├─ keyword "上传/导入" → Upload (0.88)
    │        └─ keyword "游戏/互动" → Game (0.86)
    │
    ├─▶ classifyWithVectors(text) [if embedder]
    │        └─ cosine(query, examples) > best
    │
    └─▶ classifyWithAI(text) [if chat && confidence < 0.72]
             └─ DeepSeek JSON → {intent, topic, ...}
```

## 配置管理

所有配置通过环境变量注入，`internal/config/config.go` 统一加载：

- 服务地址、存储方式、数据库连接
- AI API Key 和模型参数
- 消息平台 webhook 配置
- RAG 阈值、文件 TTL 等业务参数

## 部署拓扑

```
┌────────────────────────────────────┐
│           Hermesclaw App            │
│  (hermesclaw serve)                │
│                                    │
│  ┌──────────────────────────────┐  │
│  │   HTTP Server (:8080)       │  │
│  │  ┌──────────────────────┐   │  │
│  │  │ /onebot/event       │   │  │◀─── QQ Bot (NapCat)
│  │  │ /clawbot/event      │   │  │◀─── WeChat (Clawbot)
│  │  │ /api/*              │   │  │◀─── 外部调用
│  │  │ /admin/*            │   │  │
│  │  └──────────────────────┘   │  │
│  └──────────────────────────────┘  │
│                                    │
│  ┌──────────────────────────────┐  │
│  │   Cleanup Loop (每分钟)     │  │
│  │   删除过期文件               │  │
│  └──────────────────────────────┘  │
└───────────────┬────────────────────┘
                │
        ┌───────┴────────┐
        │                │
        ▼                ▼
┌──────────────┐  ┌────────────────────┐
│ PostgreSQL   │  │   文件系统          │
│ + pgvector  │  │   data/files/       │
│  :5432       │  │   data/materials/   │
└──────────────┘  └────────────────────┘
```

## 扩展方向

1. **视频处理层**：接入 FFmpeg + Whisper，支持课堂视频转录和摘要
2. **多用户系统**：引入 JWT 认证，支持学生账号
3. **实时协作**：WebSocket 支持多人同时编辑课件
4. **批处理**：支持批量生成多个讲次的全套材料
5. **质量评估**：生成后自动评分，可人工调整提示词
