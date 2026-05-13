# CLAUDE.md

This file guides Claude Code when working in this repository.

## ⚠️ 上下文保护规则 (Context Protection)

**严禁自动读取以下大型文档文件，除非用户明确要求：**
- `API.md` (1417行/47KB) — 完整 API 接口文档
- `API.en.md` (1408行/46KB) — 英文 API 文档
- `docs/prompt-compatibility.md` (428行/27KB) — Prompt 兼容性文档
- `README.MD` (438行/23KB) — 主 README
- `README.en.md` — 英文 README
- `audit_report_*.md` — 自动生成的安全审计报告
- `docs/DEPLOY.md`, `docs/DEPLOY.en.md` — 部署文档

**需要查阅上述文档时，使用 @API.md 等引用语法，让用户选择性读取。**

---

## 项目概述

DS2API v4.3.0 — 将 DeepSeek Web 对话转换为 OpenAI / Claude / Gemini 兼容 API 的 Go 后端服务。

- **语言**: Go 1.26.0
- **Web 路由**: chi/v5
- **前端管理台**: React (源码在 `webui/`，构建到 `static/admin`)
- **Vercel 桥接**: `api/chat-stream.js`

## 核心架构

```
客户端 SDK (OpenAI/Claude/Gemini 格式)
        ↓
  chi Router (中间件链)
        ↓
  HTTP API Surface (openai/ claude/ gemini/ admin/)
        ↓
  PromptCompat 内核 (API → DeepSeek 网页纯文本上下文)
        ↓
  DeepSeek Web API (client_auth → client_completion → SSE 流式)
```

### 核心数据流

1. **请求转换** (`internal/httpapi/`): OpenAI/Claude/Gemini 格式 → 内部标准请求
2. **Prompt 兼容** (`internal/promptcompat/`): 标准请求 → DeepSeek 网页纯文本 prompt
3. **DeepSeek 客户端** (`internal/deepseek/`): 认证、会话管理、HTTP 请求、SSE 解析
4. **Tool Call 处理** (`internal/toolcall/` + `internal/toolstream/`): DSML/XML/JSON 多格式工具调用解析
5. **响应渲染** (`internal/format/`): 流式输出 → 目标 API 格式

## 关键源文件 (按重要性)

### 入口
| 文件 | 作用 |
|------|------|
| `cmd/ds2api/main.go` | 主程序入口，HTTP 服务器启动 |
| `cmd/ds2api-tests/main.go` | 测试套件入口 |

### 核心运行时 (~1000行级)
| 文件 | 行数 | 作用 |
|------|------|------|
| `internal/server/router.go` | ~317 | chi 路由注册，中间件挂载 |
| `internal/promptcompat/promptcompat.go` | ~1035 | Prompt 构建核心入口 |
| `internal/httpapi/claude/stream_runtime_core.go` | ~300+ | Claude 流式运行时 |
| `internal/httpapi/openai/chat/chat_stream_runtime.go` | ~300+ | OpenAI Chat 流式 |
| `internal/httpapi/openai/responses/responses_stream_runtime_core.go` | ~300+ | Responses 流式 |

### Tool Call 解析
| 文件 | 行数 | 作用 |
|------|------|------|
| `internal/toolcall/toolcalls_parse.go` | ~274 | 工具调用解析入口 |
| `internal/toolcall/toolcalls_scan.go` | ~289 | 工具调用扫描 |
| `internal/toolcall/toolcalls_dsml.go` | — | DSML 格式解析 |
| `internal/toolcall/toolcalls_xml.go` | — | XML 格式解析 |
| `internal/toolstream/tool_sieve_core.go` | — | 流式工具调用筛选 |
| `internal/toolstream/tool_sieve_xml.go` | — | 流式 XML 筛选 |

### DeepSeek 客户端
| 文件 | 作用 |
|------|------|
| `internal/deepseek/client/client_core.go` | 核心客户端 |
| `internal/deepseek/client/client_auth.go` | 认证 (Bearer/JWT) |
| `internal/deepseek/client/client_completion.go` | 对话请求 |
| `internal/deepseek/client/client_session.go` | 会话管理 |
| `internal/deepseek/protocol/sse.go` | SSE 协议常量 |
| `internal/deepseek/protocol/constants.go` | 协议常量 |

### 配置系统
| 文件 | 行数 | 作用 |
|------|------|------|
| `internal/config/config.go` | ~373 | 配置主结构 |
| `internal/config/store.go` | ~300 | 配置持久化 |
| `internal/config/models.go` | ~372 | 模型别名/路由 |
| `internal/config/account.go` | — | 账号管理 |
| `config.example.json` | — | 配置示例 |

### API 路由层
| 目录 | 作用 |
|------|------|
| `internal/httpapi/claude/` | `/anthropic/*` 路由 |
| `internal/httpapi/openai/chat/` | `/v1/chat/completions` |
| `internal/httpapi/openai/responses/` | `/v1/responses` |
| `internal/httpapi/gemini/` | `/v1beta/models/*` |
| `internal/httpapi/admin/` | 管理 API + WebUI |

### SSE 解析
| 文件 | 作用 |
|------|------|
| `internal/sse/parser.go` | SSE 行解析 |
| `internal/sse/stream.go` | SSE 流管理 |
| `internal/sse/consumer.go` | SSE 事件消费 |
| `internal/sse/dedupe.go` | 去重 |

### 测试套件
| 文件 | 行数 | 作用 |
|------|------|------|
| `internal/testsuite/runner_core.go` | ~290 | 测试运行器核心 |
| `internal/testsuite/cases.go` | — | 测试用例注册 |
| `internal/testsuite/runner_cases_claude.go` | — | Claude 测试用例 |
| `internal/testsuite/runner_cases_openai.go` | — | OpenAI 测试用例 |

## 目录结构

```
ds2api/
├── cmd/                    # 可执行入口
│   ├── ds2api/main.go      # 主服务
│   └── ds2api-tests/       # 测试套件
├── internal/               # 私有 Go 包
│   ├── account/            # 账号池 (获取/限制/等待)
│   ├── auth/               # API Key / Bearer 鉴权
│   ├── chathistory/        # 对话历史存储
│   ├── claudeconv/         # Claude 格式转换
│   ├── config/             # 配置管理 (文件/环境变量)
│   ├── deepseek/           # DeepSeek Web API 客户端
│   │   ├── client/         # HTTP 客户端实现
│   │   ├── protocol/       # SSE 常量/协议
│   │   └── transport/      # TLS 传输层
│   ├── devcapture/         # 开发抓包
│   ├── format/             # 输出格式渲染
│   ├── httpapi/            # HTTP API 路由处理器
│   │   ├── admin/          # 管理 API
│   │   ├── claude/         # Claude 兼容端点
│   │   ├── gemini/         # Gemini 兼容端点
│   │   └── openai/         # OpenAI 兼容端点
│   ├── prompt/             # Prompt 模板
│   ├── promptcompat/       # Prompt 兼容内核
│   ├── rawsample/          # 原始样本
│   ├── server/             # HTTP 服务器配置
│   ├── sse/                # SSE 流解析
│   ├── stream/             # 流引擎
│   ├── testsuite/          # API 测试框架
│   ├── textclean/          # 文本清理
│   ├── toolcall/           # 工具调用解析
│   ├── toolstream/         # 流式工具调用筛选
│   ├── translatorcliproxy/ # CLI Proxy 翻译器
│   ├── util/               # 通用工具函数
│   ├── version/            # 版本号
│   └── webui/              # WebUI 静态托管
├── api/                    # Vercel Node 桥接
├── webui/                  # React 管理台源码
├── docs/                   # 文档 (读前检查大小)
├── tests/                  # 测试脚本
├── scripts/                # 构建/检查脚本
├── dist/                   # 构建输出
├── pow/                    # PoW 相关
├── Dockerfile
├── docker-compose.yml
└── config.example.json     # 配置模板
```

## 开发命令

```bash
# 构建
go build -o dist/ds2api ./cmd/ds2api

# 运行 (需要 config.json)
cp config.example.json config.json
# 编辑 config.json 填入 DeepSeek 账号和 API Key
go run ./cmd/ds2api

# Docker
docker compose up -d

# 测试
go test ./internal/...

# 运行 API 测试套件
go run ./cmd/ds2api-tests

# Lint (按 AGENTS.md 要求)
gofmt -w ./...
./scripts/lint.sh
./tests/scripts/run-unit-all.sh

# WebUI 构建
npm run build --prefix webui

# 验证质量门
./scripts/lint.sh && ./tests/scripts/check-refactor-line-gate.sh && ./tests/scripts/run-unit-all.sh
```

## 配置关键字段

- `accounts[]` — DeepSeek 网页账号 (email/password 或 mobile/password)
- `keys[]` / `api_keys[]` — 对外 API Key
- `model_aliases` — 模型名映射 (如 `gpt-4o` → `deepseek-v4-flash`)
- `runtime.account_max_inflight` — 单账号最大并发
- `runtime.global_max_inflight` — 全局最大并发

## 代码风格

- Go: 遵循 `gofmt`，错误不忽略，I/O cleanup 错误显式处理
- 提交前必须通过 lint + unit test + refactor gate
- 参考 `AGENTS.md` 了解完整的 PR Gate 规则
