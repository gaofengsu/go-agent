# go-agent 项目说明

> Go 语言实现的极简 AI Agent 教育套件，源自 Python "nano" 家族项目的工程化重构。
> 
> **设计哲学**： intentionally small, statically typed, copy-paste runnable.

---

## 1. 项目概述

`go-agent` 是一个基于 Go 语言的教育型 AI Agent 框架，完整移植并整合了以下三个 Python 子项目的核心概念：

- **nanoAgent** → `pkg/agent`：OpenAI Function Calling 的 Go 实现，支持基础 Agent、增强 Agent（记忆+规划）、ClaudeCode 风格 Agent（富工具+规则+技能+MCP）
- **nanoMemory** → `pkg/memory`：6 级渐进式记忆架构（状态机→文件→向量→混合评分→知识图谱→摘要压缩）
- **nanoSkills** → `pkg/skills`：YAML Frontmatter 技能文件系统，支持动态技能激活

与 Python 原版相比，Go 版本在保持代码精简（每文件 < 300 行）的同时，利用静态类型系统和显式错误处理提升可读性和工程化水平。

---

## 2. 快速开始

### 2.1 环境要求

- Go 1.21+
- OpenAI 兼容的 API Key（OpenAI、StepFun、Minimax、OpenRouter 等）

### 2.2 安装

```bash
git clone <repo-url>
cd go-agent
go mod tidy
```

### 2.3 配置

复制 `minmax.json` 并填入你的 API Key：

```json
{
  "providers": {
    "openai": {
      "api_key": "sk-xxxxxxxx",
      "base_url": "https://api.openai.com/v1",
      "model": "gpt-4o-mini",
      "embed_model": "text-embedding-3-small"
    },
    "minimax": {
      "api_key": "your-minimax-key",
      "base_url": "https://api.minimax.chat/v1",
      "model": "abab6.5-chat"
    }
  },
  "default_provider": "openai",
  "agent": {
    "max_iterations": 5,
    "skills_dir": "./skills-real",
    "memory_level": 2,
    "use_plan": false
  }
}
```

或直接使用环境变量（优先级高于配置文件）：

```bash
export OPENAI_API_KEY="sk-xxxxxxxx"
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_MODEL="gpt-4o-mini"
```

### 2.4 运行

**基础 Agent：**
```bash
go run ./cmd/agent "list all go files in current directory"
```

**Agent Plus（带记忆和规划）：**
```bash
go run ./cmd/agent_plus "create a hello.txt with 'Hello World'"
go run ./cmd/agent_plus --plan "find all .go files and count lines"
```

**ClaudeCode 风格 Agent（富工具集）：**
```bash
go run ./cmd/agent_claudecode "read README.md and summarize"
go run ./cmd/agent_claudecode --plan "refactor the main package"
```

**记忆级别演示：**
```bash
go run ./cmd/memory_l1 "I prefer dark mode"
go run ./cmd/memory_l2 "What do you know about me?"
go run ./cmd/memory_l4 "Alice moved to Tokyo"
go run ./cmd/memory_l5 "I've been learning Rust"
```

**Skills Agent：**
```bash
go run ./cmd/skills "review my code"
go run ./cmd/skills --skills-dir ./skills-fake "book a flight to Paris"
```

---

## 3. 架构设计

### 3.1 核心分层

```
┌─────────────────────────────────────────────┐
│  cmd/              # 单文件可运行入口        │
│  (agent, agent_plus, agent_claudecode, ...) │
├─────────────────────────────────────────────┤
│  pkg/agent/        # Agent 核心引擎          │
│  - 消息循环、工具分发、规划、规则加载        │
├─────────────────────────────────────────────┤
│  pkg/memory/       # 渐进式记忆系统          │
│  - L0~L5 六级实现，统一 Memory 接口          │
├─────────────────────────────────────────────┤
│  pkg/skills/       # Skills 技能系统         │
│  - YAML Frontmatter 解析、动态激活           │
├─────────────────────────────────────────────┤
│  pkg/tools/        # 工具实现                │
│  - bash, file, search, plan                  │
├─────────────────────────────────────────────┤
│  pkg/llm/          # LLM 客户端抽象          │
│  - OpenAI 兼容接口、消息类型定义             │
└─────────────────────────────────────────────┘
```

### 3.2 Agent 消息循环（核心机制）

```
用户输入
  ↓
构建 messages：[system, user]
  ↓
调用 LLM（附带 tools schema）
  ↓
┌─────────────────────────────────────┐
│  assistant message 含 tool_calls？  │
│  → 否：返回 content，结束           │
│  → 是：提取 name + arguments        │
│         → ToolRegistry 分发执行     │
│         → 构造 tool result message  │
│         → append 到 messages        │
│         → 再次调用 LLM（循环）      │
└─────────────────────────────────────┘
```

### 3.3 记忆级别对比

| 级别 | 存储 | 检索策略 | 适用场景 |
|------|------|----------|----------|
| L0 | 无 | 无 | 基准对照 |
| L1 | `memory_l1.jsonl` | 关键词交集匹配 | 简单偏好记录 |
| L2 | `memory_l2.jsonl` + embedding | 余弦相似度 | 语义相关回忆 |
| L3 | `memory_l3.jsonl` + reflections | 相似度×时效×重要性 | 长期个性化 |
| L4 | `memory_l4.db` (SQLite) | SPO 三元组图查询 | 结构化知识、时序推理 |
| L5 | `memory_l5.jsonl` | 关键词匹配摘要 | 长对话压缩 |

---

## 4. 目录详解

### 4.1 `pkg/llm/` — LLM 客户端

- `client.go`：`LLMClient` 接口，封装 `go-openai`
- `message.go`：与 OpenAI API 对齐的消息类型（`Message`、`ToolCall`、`ToolSchema`）
- `tools.go`：`Tool` 接口定义、`ToolRegistry` 实现

### 4.2 `pkg/tools/` — 工具实现

| 工具 | 对应 Python | 说明 |
|------|-------------|------|
| `bash.go` | `execute_bash` / `bash` | 执行 shell 命令，30 秒超时 |
| `file.go` | `read_file` / `write_file` / `read` / `write` / `edit` | 文件读写，edit 要求精确匹配 |
| `search.go` | `glob` / `grep` | 文件查找和内容搜索 |
| `plan.go` | `plan` | 调用 LLM 分解任务为步骤 |

### 4.3 `pkg/memory/` — 记忆系统

每级记忆独立文件，均实现统一接口：

```go
type Memory interface {
    Save(ctx context.Context, userInput, aiResponse string) error
    Search(ctx context.Context, query string, topK int) ([]Entry, error)
}
```

### 4.4 `pkg/skills/` — 技能系统

- `skill.go`：`Skill` 结构体 + `ParseSkill()`（正则解析 YAML Frontmatter）
- `store.go`：`SkillStore` 接口，`Discover()` 递归扫描目录

Skill 文件格式示例（`skills-real/git-expert/SKILL.md`）：
```markdown
---
name: git-expert
description: Advanced Git operations and best practices
---

You are a Git expert. Follow these principles:
- Prefer rebase over merge for feature branches
- Write atomic commits
- ...
```

### 4.5 `cmd/` — 单文件入口

每个子目录对应一个可独立运行的教育示例，保持与 Python 原版一致的运行方式。

---

## 5. 开发指南

### 5.1 添加新工具

1. 在 `pkg/tools/` 创建 `your_tool.go`：
```go
type YourTool struct{}

func (t *YourTool) Name() string        { return "your_tool" }
func (t *YourTool) Description() string { return "Does something useful" }
func (t *YourTool) Schema() llm.ToolSchema { /* ... */ }
func (t *YourTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    // 实现逻辑
}
```

2. 在 `pkg/tools/tools.go` 的 `NewRegistry()` 中注册：
```go
r.Register(&YourTool{})
```

### 5.2 添加新记忆级别

1. 在 `pkg/memory/` 创建 `l6_yours.go`
2. 实现 `Memory` 接口
3. 在 `cmd/memory_l6/main.go` 添加运行入口

### 5.3 运行测试

```bash
# 全量测试（无网络依赖，使用 Mock）
go test ./...

# 带覆盖率
go test -cover ./...

# 单包测试
go test ./pkg/memory/...
```

---

## 6. 与 Python 原版的差异

| 方面 | Python 版 | Go 版 |
|------|-----------|-------|
| 类型系统 | 动态类型 | 静态类型，显式结构体 |
| 错误处理 | `try/except` | `if err != nil`，错误作为值 |
| 并发 | `asyncio`（未使用） | 原生 goroutine（可选用于并行 Embedding） |
| 运行时 | 解释器 + pip | 单二进制，无运行时依赖 |
| 全局状态 | 模块级全局变量 | 结构体封装，显式依赖注入 |
| JSONL 解析 | `json.loads` 逐行 | `bufio.Scanner` + `json.Decoder` |
| SQLite | `sqlite3` (stdlib) | `database/sql` + `modernc.org/sqlite` |
| 向量运算 | `numpy` | 手写循环（数据量小，足够高效） |

---

## 7. 许可证

MIT License — 与 Python "nano" 家族保持一致。

---

## 8. 参考与致谢

- [nanoAgent](https://github.com/sanbuphy/nanoAgent) — OpenAI Function Calling Agent
- [nanoMemory](https://github.com/sanbuphy/nanoMemory) — 9-Level Agent Memory Architectures
- [nanoSkills](https://github.com/sanbuphy/nanoSkills) — Progressive Disclosure via YAML Skills
- [go-openai](https://github.com/sashabaranov/go-openai) — OpenAI SDK for Go
