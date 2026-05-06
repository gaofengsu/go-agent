# go-agent

> 基于 Go 语言的极简但完整的 AI Agent 框架，源自教育型 "nano" 家族（nanoAgent / nanoMemory / nanoSkills）。旨在深度展示对 LLM Agent 架构、记忆层级和可扩展技能系统的理解。
>
> **设计哲学**：有意精简（每模块 ~50-250 行）、静态类型、复制粘贴即可运行——但架构上像生产代码一样严谨。

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## 目录

1. [为什么做这个项目？](#为什么做这个项目)
2. [架构总览](#架构总览)
3. [核心概念深度解析](#核心概念深度解析)
   - Agent 核心：ReAct 循环
   - 记忆层级：6 级渐进架构
   - Skills 系统：渐进式披露
   - 工具注册表：插件架构
4. [项目结构](#项目结构)
5. [快速开始](#快速开始)
6. [设计决策](#设计决策)
7. [扩展框架](#扩展框架)
8. [测试策略](#测试策略)
9. [参考文献](#参考文献)

---

## 为什么做这个项目？

大多数 Agent 框架（LangChain、AutoGPT 等）是**黑盒**。你导入、配置、祈祷。这个项目采取相反的方法：**每个概念都是暴露的、带类型的、可测试的**。

构建 `go-agent` 让我（并展示了）以下能力：

| 展示的能力 | 代码中的证据 |
|-------------------|------------------|
| **LLM Function Calling** | `pkg/llm/client.go` — 原生 OpenAI API 映射、消息类型系统、工具 Schema 构建 |
| **ReAct Agent 循环** | `pkg/agent/agent.go` — 显式的观察 → 思考 → 行动迭代，带最大迭代保护 |
| **记忆架构** | `pkg/memory/` — 从状态机（L0）到时序知识图谱（L4）的 6 个级别，均实现统一 `Memory` 接口 |
| **向量检索** | `pkg/memory/l2_vector.go` — 手写的余弦相似度，零 numpy 依赖 |
| **结构化知识** | `pkg/memory/l4_graph.go` — SQLite SPO 三元组，含矛盾检测与时序失效 |
| **插件系统** | `pkg/tools/tools.go` — `Tool` 接口 + `Registry`，零成本添加工具 |
| **Go 工程能力** | 接口驱动设计、显式错误处理、可 Mock 的 LLM 客户端、表驱动测试 |

> **给面试官看**：这不是教程复制粘贴。每一行都是从第一性原理写出的，架构权衡在下方有详细记录。

---

## 架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                           用户输入                                   │
└──────────────────────────────────┬──────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Agent (pkg/agent)                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐   │
│  │ Base Agent   │  │ Plus Agent   │  │ ClaudeCode Agent         │   │
│  │ (工具循环)    │  │ (+ 记忆      │  │ (+ 规则 + 技能 + MCP      │   │
│  │              │  │  + 规划)     │  │  + 富工具)               │   │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘   │
└──────────────────────────────────┬──────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      LLM Client (pkg/llm)                            │
│              OpenAI 兼容 API 抽象层                                  │
│         ChatCompletion + Embeddings 统一接口                         │
└──────────────────────────────────┬──────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Tool Registry (pkg/tools)                        │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ │
│  │ bash   │ │ read   │ │ write  │ │ edit   │ │ glob   │ │ grep   │ │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                                   ▲
┌──────────────────────────────────┴──────────────────────────────────┐
│                    Memory System (pkg/memory) — 6 级                 │
│  L0 状态机 → L1 关键词 → L2 向量 → L3 混合评分 → L4 图谱 → L5 摘要   │
└─────────────────────────────────────────────────────────────────────┘
                                   ▲
┌──────────────────────────────────┴──────────────────────────────────┐
│                   Skills System (pkg/skills)                         │
│              YAML Frontmatter 技能文件，动态激活                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 核心概念深度解析

### 1. Agent 核心：ReAct 循环

所有 LLM Agent 的根本模式是 **ReAct 循环**（推理 + 行动）：

```
用户查询 → 系统提示 → LLM
    ← 助手消息（可能包含 tool_calls）
    → 执行工具 → 将结果反馈给 LLM
    ← 助手消息（可能包含更多 tool_calls）
    → ... 迭代直到没有 tool_calls
    ← 最终答案
```

**实现**：`pkg/agent/agent.go:Run()`

```go
for i := 0; i < a.MaxIterations; i++ {
    resp, err := a.Client.Chat(ctx, messages, toolSchemas)
    // ... 追加助手消息
    if len(resp.ToolCalls) == 0 {
        return resp.Content  // 完成
    }
    for _, tc := range resp.ToolCalls {
        result := a.Registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
        // ... 追加工具结果消息
    }
}
```

**关键洞察**：`messages` 切片就是 Agent 的**全部状态**。没有隐藏上下文，没有魔法会话管理。这让 Agent 极易 Mock 和调试。

### 2. 记忆层级：6 级渐进架构

记忆是 Agent 工程中最难的问题。本项目实现了 6 个级别，每一级在前一级基础上增强：

| 级别 | 文件 | 核心机制 | 适用场景 |
|-------|------|---------------|-------------|
| **L0** | `l0_stateless.go` | 无记忆 | 基线 / 无状态 API |
| **L1** | `l1_file.go` | JSONL + 关键词匹配 | 简单偏好追踪 |
| **L2** | `l2_vector.go` | OpenAI Embedding + 余弦相似度 | 语义回忆 |
| **L3** | `l3_scored.go` | 相似度 × 时效 × 重要性 + 反思 | 长期个性化 |
| **L4** | `l4_graph.go` | SQLite SPO 三元组 + 时序追踪 | 结构化知识、历史 |
| **L5** | `l5_summary.go` | LLM 压缩摘要 | 有限上下文窗口 |

#### L2 向量检索（不用 NumPy！）

Python 依赖 `numpy` 做向量运算。Go 不需要：

```go
func cosineSimilarity(a, b []float32) float32 {
    var dot, normA, normB float32
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    return dot / (float32(math.Sqrt(float64(normA*normB))) + 1e-8)
}
```

在 1536 维度和 <10k 记忆量下，这是**亚毫秒级**的。无 CGO，无 BLAS，无部署麻烦。

#### L4 知识图谱：时序推理

图谱记忆将事实存储为 `(Subject, Predicate, Object)` 三元组，带 `valid_from` 和 `valid_until` 时间戳。

**矛盾检测**：当记录 "Alice 住在东京" 时，如果同一 subject+predicate 已存在 "Alice 住在巴黎"，旧三元组被失效：

```sql
UPDATE triples SET valid_until=? WHERE id=?
```

这支持**时序查询**："Alice 在 2024 年之前住在哪里？"

#### L3 混合评分：Park 框架

灵感来自 [Park et al. (2023)](https://arxiv.org/abs/2304.03442) *Generative Agents*：

```
score = α·similarity + β·recency + γ·importance

recency = 0.5^(days / 30)   // 艾宾浩斯遗忘曲线
importance = LLM 评分 1-10  // 写入时计算一次
```

这模拟了人类记忆：近期事件和重要事实留存更久。

### 3. Skills 系统：渐进式披露

Skills 是 YAML-frontmatter Markdown 文件，**仅在需要时**将专业知识注入上下文：

```markdown
---
name: git-expert
description: Git 高级操作和最佳实践专家
---

你是 Git 专家。优先使用 rebase 而非 merge...
```

Agent 暴露 `activate_skill(name)` 工具。LLM 根据用户查询**自主决定**何时激活技能。这让系统提示保持简短（节省 token、减少干扰），同时让深度专业知识按需可用。

**与微调对比**：
- 微调：将知识烘焙进模型权重（昂贵、更新慢）
- Skills：通过上下文注入知识（免费、即时、版本可控）

### 4. 工具注册表：插件架构

每个工具实现 `Tool` 接口：

```go
type Tool interface {
    Name() string
    Description() string
    Schema() llm.ToolSchema
    Execute(ctx context.Context, args map[string]any) (string, error)
}
```

添加新工具只需一行：

```go
r.Register(&MyCustomTool{})
```

注册表自动为 OpenAI 的 `functions` 参数生成 JSON Schema。无需手动维护 Schema。

---

## 项目结构

```
go-agent/
├── main.go                     # 统一 CLI: go-agent -plan -claudecode
├── minmax.json                 # 配置: API keys、模型参数、记忆设置
├── go.mod / go.sum
├── README.md / README_CN.md
│
├── pkg/
│   ├── llm/                    # LLM 抽象层
│   │   ├── message.go          # Message、ToolCall、ToolSchema 类型
│   │   └── client.go           # OpenAI 兼容客户端 (Chat + Embed)
│   │
│   ├── agent/                  # 三种 Agent 变体
│   │   ├── agent.go            # 基础 ReAct 循环 (~60 行)
│   │   ├── plus.go             # + 记忆 + 规划
│   │   └── claudecode.go       # + 规则 + 技能 + MCP + 富工具
│   │
│   ├── memory/                 # 6 级渐进记忆
│   │   ├── memory.go           # Memory 接口
│   │   ├── l0_stateless.go     # 无操作基线
│   │   ├── l1_file.go          # 关键词匹配 (JSONL)
│   │   ├── l2_vector.go        # Embedding + 余弦相似度
│   │   ├── l3_scored.go        # 混合评分 + 反思
│   │   ├── l4_graph.go         # SQLite SPO 三元组
│   │   └── l5_summary.go       # LLM 压缩
│   │
│   ├── skills/                 # 技能解析与激活
│   │   ├── skill.go            # YAML Frontmatter 解析器
│   │   └── store.go            # 发现 + activate_skill 工具构建
│   │
│   └── tools/                  # 工具实现
│       ├── tools.go            # 注册表 + Tool 接口
│       ├── bash.go             # Shell 执行
│       ├── file.go             # read / write / edit
│       ├── search.go           # glob / grep
│       └── plan.go             # 基于 LLM 的任务分解
│
├── cmd/                        # 独立可执行的教育示例
│   ├── agent/
│   ├── agent_plus/
│   ├── agent_claudecode/
│   ├── memory_l1 ~ memory_l5/
│   └── skills/
│
├── skills-real/                # 真实专业能力 (git、代码审查、API 设计...)
│   ├── git-expert/SKILL.md
│   ├── code-reviewer/SKILL.md
│   ├── api-designer/SKILL.md
│   ├── docs-writer/SKILL.md
│   └── web-best-practices/SKILL.md
│
├── skills-fake/                # 模拟场景
│   └── travel-agent/SKILL.md
│
├── internal/testutil/          # Mock LLM 客户端，零网络测试
│   └── mock.go
│
└── 文档/                        # 本地文档 (不在仓库中)
```

---

## 快速开始

```bash
# 1. 克隆并进入
cd go-agent

# 2. 安装依赖（仅 2 个外部依赖: go-openai + sqlite)
go mod tidy

# 3. 配置 API key
export OPENAI_API_KEY="sk-xxxxxxxx"

# 4. 运行

# 基础 Agent
go run . "list all go files"

# 带规划
go run . -plan "refactor the main package"

# ClaudeCode 风格，富工具集
go run . -claudecode "read README.md and summarize"

# 记忆级别
go run ./cmd/memory_l1 "I prefer dark mode"
go run ./cmd/memory_l2 "What do you know about me?"
go run ./cmd/memory_l4 "Alice moved to Tokyo"

# Skills
go run ./cmd/skills --skills-dir ./skills-real "review my code"
```

---

## 设计决策

### 为什么用 Go 而不是 Python？

| 维度 | Python（原版） | Go（本项目） |
|-----------|-------------------|-------------------|
| **类型安全** | 动态类型导致运行时错误 | 编译时保证 |
| **错误处理** | `try/except`（容易吞掉） | `if err != nil`（显式） |
| **部署** | 解释器 + pip + venv | 单静态二进制文件 |
| **并发** | GIL 限制的线程 | Goroutine + Channel |
| **可读性** | 魔术方法、元类 | 显式接口、结构体 |

原版 Python 项目是**教育性**的（复制粘贴即可运行）。这个 Go 版本在保持这种精神的同时，增加了**生产级架构**。

### 为什么用 `modernc.org/sqlite` 而不是 `mattn/go-sqlite3`？

`mattn/go-sqlite3` 需要 CGO，会破坏交叉编译。`modernc.org/sqlite` 是 SQLite 的纯 Go 转译——相同的 SQL 方言，零 CGO，可编译为 Linux/macOS/Windows 的单一二进制文件。

### 为什么手写余弦相似度而不是用 BLAS 库？

Embedding 维度很小（`text-embedding-3-small` 为 1536）。在这个规模下，20 行的循环调用开销低于调用 C BLAS。无依赖，无跨平台痛苦。

### 为什么不用 `viper` / `cobra` / `urfave/cli`？

标准库的 `flag` 包已足够。目标是**极简主义**。每个外部依赖都是一种负担。

---

## 扩展框架

### 添加新工具

```go
// pkg/tools/weather.go
type WeatherTool struct{}

func (t *WeatherTool) Name() string        { return "get_weather" }
func (t *WeatherTool) Description() string { return "获取当前天气" }
func (t *WeatherTool) Schema() llm.ToolSchema {
    return llm.ToolSchema{
        Type: "function",
        Function: llm.FunctionSchema{
            Name:        t.Name(),
            Description: t.Description(),
            Parameters:  []byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
        },
    }
}
func (t *WeatherTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    city := args["city"].(string)
    // ... 获取天气
    return "25°C, 晴", nil
}
```

在 `pkg/tools/tools.go` 中注册：
```go
r.Register(&WeatherTool{})
```

### 添加新记忆级别

1. 创建 `pkg/memory/l6_custom.go`
2. 实现 `Memory` 接口
3. 添加 `cmd/memory_l6/main.go`

---

## 测试策略

所有核心逻辑都使用 `internal/testutil.MockClient` 进行**零网络调用**测试：

```go
// 模拟: LLM 请求工具 → 工具执行 → LLM 返回最终答案
client := testutil.NewMockToolClient("execute_bash", `{"command":"echo test"}`, "Done")
a := agent.NewAgent(client)
result, _, err := a.Run(ctx, "sys", "run echo test")
// result == "Done"
```

运行测试：
```bash
go test ./...
```

| 包 | 测试重点 |
|---------|---------------|
| `pkg/llm` | 消息序列化、类型往返 |
| `pkg/tools` | 注册表分发、参数解析、错误路径 |
| `pkg/skills` | Frontmatter 解析边界情况 |
| `pkg/memory` | 检索准确性、时序逻辑 |
| `pkg/agent` | 循环终止、规划嵌套、错误传播 |

---

## 参考文献

- **ReAct**: Yao et al. (2023) "ReAct: Synergizing Reasoning and Acting in Language Models" — [arXiv:2210.03629](https://arxiv.org/abs/2210.03629)
- **Generative Agents**: Park et al. (2023) "Generative Agents: Interactive Simulacra of Human Behavior" — [arXiv:2304.03442](https://arxiv.org/abs/2304.03442)
- **MemoryBank**: Zhong et al. (2024) "MemoryBank: Enhancing Large Language Models with Long-Term Memory" — [arXiv:2401.10917](https://arxiv.org/abs/2401.10917)
- **LoCoMo**: LoCoMo Benchmark — [arXiv:2402.17753](https://arxiv.org/abs/2402.17753)
- **go-openai**: [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai)

---

## License

MIT — 与原版 nano 家族一致。

---

> *"如果你不能从零构建它，你就不理解它。"*
