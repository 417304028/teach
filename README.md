# Hermesclaw 课件生成平台

基于 DeepSeek + RAG 的高中物理智能课件生成系统。接收 QQ/微信消息，自动识别意图，从课程资料库检索内容，生成 PPT/Word/思维导图等教学材料。

## 架构概览

```
┌─────────────┐     ┌──────────────────────────────────────────────────┐
│  QQ Bot     │     │              Hermesclaw 服务                        │
│  (NapCat)   │────▶│  ┌──────────┐  ┌───────────┐  ┌──────────────┐  │
└─────────────┘     │  │Webhook   │  │ 意图识别层 │  │ 智能决策层   │  │
                    │  │ Handler  │─▶│ (keyword  │─▶│ (资料库优先  │
┌─────────────┐     │  └──────────┘  │ +vector   │  │  +DeepSeek)  │  │
│  微信       │     │                 │ +AI三层)  │  └──────┬───────┘  │
│  (Clawbot)  │────▶│                 └───────────┘         │          │
└─────────────┘     │                                      ▼          │
                    │                 ┌──────────────────────────────┐ │
                    │                 │     多模态生成层             │ │
                    │                 │  PPT / Word / 思维导图       │ │
                    │                 └──────────────────────────────┘ │
                    │                                              │     │
                    │  ┌──────────┐  ┌─────────────┐  ┌──────────┐ │
                    │  │PostgreSQL│  │   RAG       │  │ 文件存储 │ │
                    │  │+pgvector │◀─│ (向量检索)  │  │(30minTTL)│ │
                    │  └──────────┘  └─────────────┘  └──────────┘ │
                    └──────────────────────────────────────────────────┘
```

## 目录结构

```
teach/
├── cmd/hermesclaw/main.go      # CLI 入口
├── internal/
│   ├── ai/provider.go         # AI 抽象层（DeepSeek / 本地 fallback）
│   ├── app/service.go         # 应用服务（消息处理入口）
│   ├── chunk/chunk.go         # 文本分块
│   ├── config/config.go        # 配置管理
│   ├── content/               # 内容解析（PDF、路径解析）
│   │   ├── chunk.go           # 分块函数
│   │   ├── parser.go          # 路径元信息解析
│   │   └── pdf.go             # PDF 文本提取
│   ├── generate/               # 多模态生成
│   │   ├── service.go         # 生成服务入口
│   │   ├── docx.go           # Word 文档生成
│   │   ├── pptx.go           # PPT 生成
│   │   └── mindmap.go        # 思维导图生成
│   ├── httpapi/               # HTTP 接口
│   │   ├── server.go         # 路由与处理
│   │   └── admin.go          # 管理后台
│   ├── intent/               # 意图识别
│   │   └── classifier.go     # 三层混合判断
│   ├── log/logger.go         # 结构化日志
│   ├── model/                # 数据模型
│   │   ├── model.go          # 核心模型
│   │   └── user.go           # 用户模型
│   ├── qq/onebot.go          # QQ/OneBot 接入
│   ├── rag/rag.go            # RAG 检索服务
│   ├── store/                # 存储抽象
│   │   ├── store.go          # JSON Store 实现
│   │   └── postgres.go       # PostgreSQL 实现
│   └── wechat/clawbot.go     # 微信 Clawbot 接入
├── db/migrations/             # 数据库迁移
│   └── 001_init.sql
├── docker-compose.yml         # Docker 部署
├── Dockerfile                 # 多阶段构建
├── .env.example              # 环境变量模板
└── go.mod
```

## 快速开始

### 前置依赖

- Go 1.21+
- Docker & Docker Compose（可选）
- PostgreSQL 16 + pgvector（可选，本地 JSON 模式也可运行）
- DeepSeek API Key（用于生成内容）
- 阿里云 DashScope API Key（用于向量嵌入，可选）

### 方式一：Docker 部署（推荐）

```bash
cd teach

# 复制并编辑配置
cp .env.example .env
# 编辑 .env，填入 DEEPSEEK_API_KEY 和 DASHSCOPE_API_KEY

# 启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f app
```

服务地址：`http://localhost:8080`
管理后台：`http://localhost:8080/admin`（默认账号 admin / change-me）

### 方式二：本地运行

```bash
cd teach

# 启动 PostgreSQL（Docker）
docker run -d \
  --name hermesclaw_pg \
  -e POSTGRES_DB=hermesclaw \
  -e POSTGRES_USER=hermesclaw \
  -e POSTGRES_PASSWORD=hermesclaw \
  -p 5432:5432 \
  pgvector/pgvector:pg16

# 初始化数据库（自动执行）
# 如果需要手动初始化：
# psql postgres://hermesclaw:hermesclaw@localhost:5432/hermesclaw -f db/migrations/001_init.sql

# 运行
go mod tidy
go run ./cmd/hermesclaw serve

# 后台导入课程资料
hermesclaw ingest -path ../春季课.zip
```

### 方式三：本地 JSON 模式（无需数据库）

```bash
cd teach
HERMESCLAW_STORE=json go run ./cmd/hermesclaw serve
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HERMESCLAW_ADDR` | `:8080` | 服务监听地址 |
| `HERMESCLAW_DATA_DIR` | `data` | 数据目录 |
| `HERMESCLAW_STORE` | `postgres` | 存储方式：`postgres` 或 `json` |
| `HERMESCLAW_DATABASE_URL` | - | PostgreSQL 连接串 |
| `HERMESCLAW_FILE_TTL` | `30m` | 生成文件过期时间 |
| `HERMESCLAW_RAG_THRESHOLD` | `0.60` | 向量检索相似度阈值 |
| `HERMESCLAW_LOG_LEVEL` | `info` | 日志级别 |
| `DEEPSEEK_API_KEY` | - | DeepSeek API Key |
| `DEEPSEEK_MODEL` | `deepseek-chat` | DeepSeek 模型 |
| `DASHSCOPE_API_KEY` | - | 阿里云 DashScope API Key（向量嵌入） |

## API 接口

### 聊天

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user1","channel":"test","message":"生成春季课第10讲的PPT"}'
```

### 导入课程资料

```bash
curl -X POST http://localhost:8080/api/ingest \
  -H "Content-Type: application/json" \
  -d '{"path":"/path/to/spring.zip"}'
```

### 生成思维导图

```bash
curl -X POST http://localhost:8080/api/generate/mindmap \
  -H "Content-Type: application/json" \
  -d '{"topic":"动能定理","filters":{"season":"春季课"}}'
```

### 生成 PPT

```bash
curl -X POST http://localhost:8080/api/generate/ppt \
  -H "Content-Type: application/json" \
  -d '{"topic":"动能定理","pages":12,"filters":{"season":"春季课","lesson_no":10}}'
```

### 生成习题

```bash
curl -X POST http://localhost:8080/api/generate/exercises \
  -H "Content-Type: application/json" \
  -d '{"topic":"动能定理","count":10}'
```

### 生成教学大纲

```bash
curl -X POST http://localhost:8080/api/generate/outline \
  -H "Content-Type: application/json" \
  -d '{"topic":"动能定理","filters":{"season":"春季课","lesson_no":10}}'
```

## 支持的意图

| 意图 | 触发关键词 | 说明 |
|------|-----------|------|
| `mindmap` | 导图、思维导图 | 生成 HTML 思维导图 |
| `ppt` | PPT、课件、幻灯片 | 生成 PPT 文件 |
| `exercises` | 习题、练习、题集、作业 | 生成 Word 习题集 |
| `outline` | 大纲、教学设计、教案 | 生成教学大纲 Word |
| `search` | 搜索、检索 | 检索资料库 |
| `chat` | 其他 | 智能问答 |
| `upload` | 上传、导入 | 导入课程资料 |

## 数据导入

将课程资料按以下目录结构组织：

```
课程压缩包.zip
├── 春季课/
│   ├── 人教版/
│   │   ├── 新授/
│   │   │   ├── 第1讲 运动的描述/
│   │   │   │   ├── 讲义/动能定理的应用(教师版).pdf
│   │   │   │   └── 题集/动能定理题集(学生版).pdf
│   │   │   └── ...
│   │   └── 复习/
│   └── ...
├── 暑假课/
├── 秋季课/
└── 寒假课/
```

支持 PDF 文件，支持按季节（第几课）、教材版本（新授/复习）自动分类。

## 视频处理层

视频处理模块框架已预留（`internal/video/processor.go`），未来计划支持：

- 视频转录（Whisper / 阿里云 ASR）
- 视频内容摘要
- 课堂片段截取
