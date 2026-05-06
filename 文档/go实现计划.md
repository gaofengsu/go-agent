# Go 实现计划

## 1. 技术选型

### 1.1 核心依赖

| 功能 | 选型 | 版本 | 理由 |
|------|------|------|------|
| OpenAI API | `github.com/sashabaranov/go-openai` | v1.24+ | 社区最成熟，支持 Tools、Streaming、Embedding |
| SQLite (纯 Go) | `modernc.org/sqlite` | v1.30+ | 零 CGO，跨平台编译友好 |
| 配置解析 | `encoding/json` (stdlib) | - | 足够简单，无需 viper |
| YAML 解析 | `gopkg.in/yaml.v3` | v3 | 解析 Skill Frontmatter |
| 命令行 | `flag` (stdlib) | - | 保持极简，教育目的优先 |
| 测试 | `testing` (stdlib) | - | 标准库足够 |

### 1.2 Go 版本要求

- **Go 1.21+**：利用 `slog`（可选）、`slices`/`maps` 包（如需要）
- **模块路径**：`github.com/sanbuphy/go-agent`

---

## 2. 目录结构

```
go-agent/
├── main.go                    # CLI 入口，flag 解析，模式分发
├── minmax.json                # 全局配置文件（API keys、模型参数）
├── go.mod / go.sum
│
├── pkg/                       # 可复用包（核心库）
│   ├── llm/
│   │   ├── client.go          # LLMClient 接口 + OpenAI 实现
│   │   ├── message.go         # Message、ToolCall 等类型定义
│   │   └── tools.go           # Tool 接口、ToolRegistry、Schema 构建
│   │
│   ├── agent/
│   │   ├── agent.go           # 基础 Agent 实现（对应 nanoAgent agent.py）
│   │   ├── plus.go            # Agent Plus（记忆 + 规划）
│   │   └── claudecode.go      # ClaudeCode 风格 Agent
│   │
│   ├── memory/
│   │   ├── memory.go          # Memory 接口
│   │   ├── l0_stateless.go    # L0：无记忆（透传）
│   │   ├── l1_file.go         # L1：JSONL + 关键词匹配
│   │   ├── l2_vector.go       # L2：Embedding + 余弦相似度
│   │   ├── l3_scored.go       # L3：混合评分 + 反思
│   │   ├── l4_graph.go        # L4：SQLite SPO 三元组
│   │   └── l5_summary.go      # L5：摘要压缩
│   │
│   ├── skills/
│   │   ├── skill.go           # Skill 结构体 + 解析逻辑
│   │   └── store.go           # SkillStore 接口 + 文件系统实现
│   │
│   └── tools/
│       ├── bash.go            # execute_bash / bash
│       ├── file.go            # read_file / write_file / read / write / edit
│       ├── search.go          # glob / grep
│       └── plan.go            # plan 工具（内含 LLM 调用）
│
├── cmd/                       # 可执行子命令（教育性单文件运行）
│   ├── agent/
│   │   └── main.go            # go run ./cmd/agent "list all go files"
│   ├── agent_plus/
│   │   └── main.go            # go run ./cmd/agent_plus --plan "..."
│   ├── agent_claudecode/
│   │   └── main.go            # go run ./cmd/agent_claudecode "..."
│   ├── memory_l1/
│   │   └── main.go            # 各记忆级别独立运行
│   ├── memory_l2/
│   │   └── main.go
│   ├── memory_l3/
│   │   └── main.go
│   ├── memory_l4/
│   │   └── main.go
│   ├── memory_l5/
│   │   └── main.go
│   └── skills/
│       └── main.go            # go run ./cmd/skills --skills-dir ./skills-real "..."
│
├── skills-real/               # 真实技能（从 nanoSkills 移植）
│   ├── git-expert/SKILL.md
│   ├── code-reviewer/SKILL.md
│   ├── docs-writer/SKILL.md
│   ├── api-designer/SKILL.md
│   └── web-best-practices/SKILL.md
│
├── skills-fake/               # 模拟技能（从 nanoSkills 移植）
│   └── travel-agent/SKILL.md
│
├── .agent/                    # agent-claudecode 运行时配置
│   ├── rules/                 # 规则文件（.md）
│   └── skills/                # 动态技能（.json）
│
├── internal/
│   └── testutil/
│       └── mock.go            # Mock LLM Client，用于单元测试
│
└── 文档/
    ├── 需求分析.md
    ├── 当前python代码分析.md
    ├── go实现计划.md
    └── go-agent项目说明.md
```

---

## 3. 模块实现顺序

### Phase 1：基础设施（第 1-2 天）

| 优先级 | 包 | 文件 | 说明 |
|--------|-----|------|------|
| P0 | `pkg/llm` | `message.go` | 定义 `Message`、`Role`、`ToolCall`、`Tool` 等核心类型 |
| P0 | `pkg/llm` | `client.go` | `LLMClient` 接口；`OpenAIClient` 包装 `go-openai` |
| P0 | `internal/testutil` | `mock.go` | Mock LLM Client，预设响应序列 |
| P1 | `pkg/tools` | `bash.go`, `file.go` | 基础工具实现 |
| P1 | `pkg/tools` | `tools.go` | `ToolRegistry`：`Register`、`Execute`、`BuildSchemas` |

**验收标准：**
- `go test ./pkg/llm/...` 通过
- Mock Client 能模拟一次 tool_call 循环

### Phase 2：Agent 核心（第 3-4 天）

| 优先级 | 包 | 文件 | 说明 |
|--------|-----|------|------|
| P0 | `pkg/agent` | `agent.go` | 基础 Agent：消息循环、工具分发 |
| P1 | `pkg/agent` | `plus.go` | Agent Plus：`MemoryLoader`、`Planner` 嵌入 |
| P1 | `pkg/agent` | `claudecode.go` | ClaudeCode Agent：富工具、Rules、Skills、MCP 加载 |
| P1 | `cmd/*` | 各 main.go | 单文件可运行入口 |

**验收标准：**
- `go run ./cmd/agent "list all go files"` 可执行（需 API Key）
- `go test ./pkg/agent/...` 覆盖基础循环、规划模式

### Phase 3：记忆系统（第 5-7 天）

| 优先级 | 包 | 文件 | 说明 |
|--------|-----|------|------|
| P0 | `pkg/memory` | `memory.go` | `Memory` 接口定义 |
| P0 | `pkg/memory` | `l1_file.go` | JSONL 读写、关键词匹配 |
| P1 | `pkg/memory` | `l2_vector.go` | Embedding 调用、余弦相似度（手写） |
| P1 | `pkg/memory` | `l3_scored.go` | 三因素评分、反思触发 |
| P2 | `pkg/memory` | `l4_graph.go` | SQLite 初始化、SPO CRUD、矛盾检测 |
| P2 | `pkg/memory` | `l5_summary.go` | 摘要提取、压缩合并 |

**关键实现细节：**

**L2 余弦相似度（无 numpy）：**
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

**L4 SQLite 模式：**
```go
const schema = `
CREATE TABLE IF NOT EXISTS triples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject TEXT NOT NULL,
    predicate TEXT NOT NULL,
    object TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    source TEXT DEFAULT '',
    valid_from TEXT NOT NULL,
    valid_until TEXT,
    embedding TEXT
);
CREATE INDEX IF NOT EXISTS idx_subject ON triples(subject);
CREATE INDEX IF NOT EXISTS idx_predicate ON triples(predicate);
CREATE INDEX IF NOT EXISTS idx_active ON triples(valid_until);
`
```

**验收标准：**
- 各 `cmd/memory_l*` 独立运行
- `go test ./pkg/memory/...` 使用临时目录/SQLite 通过

### Phase 4：Skills 系统（第 8 天）

| 优先级 | 包 | 文件 | 说明 |
|--------|-----|------|------|
| P1 | `pkg/skills` | `skill.go` | `Skill` 结构体、`ParseSkill`（正则提取 YAML Frontmatter） |
| P1 | `pkg/skills` | `store.go` | `Discover`、`Activate`、`BuildActivateTool` |
| P1 | `skills-real/`, `skills-fake/` | 移植 SKILL.md | 从 nanoSkills 复制并适配 |

**YAML Frontmatter 解析策略：**
不引入完整 YAML 解析器（避免依赖膨胀），使用正则 + 简单行解析：
```go
var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
// 然后对 group(1) 逐行解析 "key: value"
```

**验收标准：**
- `go run ./cmd/skills --skills-dir ./skills-real "review my code"` 可运行
- `go test ./pkg/skills/...` 通过

### Phase 5：集成与 CLI（第 9-10 天）

| 优先级 | 文件 | 说明 |
|--------|------|------|
| P0 | `main.go` | 统一 CLI：`go-agent run`、`go-agent memory --level` 等 |
| P1 | `minmax.json` | 配置文件设计与加载 |
| P1 | README.md / README_CN.md | 双语文档 |
| P2 | `go test ./...` | 全量测试通过 |

---

## 4. 类型系统设计

### 4.1 消息类型（兼容 OpenAI API）

```go
package llm

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

type Message struct {
    Role         Role       `json:"role"`
    Content      string     `json:"content"`
    ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID   string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"`     // "function"
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"` // JSON string
    } `json:"function"`
}

type ToolSchema struct {
    Type        string          `json:"type"`
    Function    FunctionSchema  `json:"function"`
}

type FunctionSchema struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
}
```

### 4.2 Agent 结构体

```go
package agent

type Config struct {
    Model          string
    MaxIterations  int
    SkillsDir      string
    MemoryLevel    int      // 0-5
    UsePlan        bool
    RulesDir       string
    MCPConfig      string
}

type Agent struct {
    client    llm.LLMClient
    registry  *tools.Registry
    memory    memory.Memory
    skills    *skills.Store
    config    Config
    messages  []llm.Message
    
    // claudecode 状态
    planMode     bool
    currentPlan  []string
}
```

---

## 5. 关键算法 Go 实现

### 5.1 L1 关键词匹配

```go
func keywordScore(query, text string) int {
    qWords := strings.Fields(strings.ToLower(query))
    tWords := make(map[string]struct{})
    for _, w := range strings.Fields(strings.ToLower(text)) {
        tWords[w] = struct{}{}
    }
    score := 0
    for _, w := range qWords {
        if _, ok := tWords[w]; ok {
            score++
        }
    }
    return score
}
```

### 5.2 L3 三因素评分

```go
func hybridScore(similarity float32, timestamp time.Time, importance int) float32 {
    const (
        alpha      = 0.5
        beta       = 0.3
        gamma      = 0.2
        halfLife   = 30.0 // days
    )
    days := time.Since(timestamp).Hours() / 24.0
    recency := float32(math.Pow(0.5, days/halfLife))
    imp := float32(importance) / 10.0
    return alpha*similarity + beta*recency + gamma*imp
}
```

### 5.3 L4 矛盾检测

```go
func (g *GraphMemory) detectContradiction(subject, predicate, object string) ([]int64, error) {
    rows, err := g.db.Query(
        "SELECT id, object FROM triples WHERE subject=? AND predicate=? AND valid_until IS NULL",
        normalize(subject), strings.ToLower(predicate),
    )
    // ... 收集 id，当 oldObject != normalize(object) 时加入结果
}
```

---

## 6. 测试策略

### 6.1 测试金字塔

```
        /\
       /  \     E2E（手动）：运行各 cmd/ 入口，观察实际 LLM 调用
      /____\    
     /      \   集成测试：pkg/agent + MockLLM + 临时文件/SQLite
    /________\  
   /          \ 单元测试：pkg/llm, pkg/tools, pkg/skills, pkg/memory
  /____________\
```

### 6.2 Mock LLM Client

```go
package testutil

type MockClient struct {
    Responses []llm.ChatResponse // 预设响应队列
    Index     int
}

func (m *MockClient) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolSchema) (*llm.ChatResponse, error) {
    if m.Index >= len(m.Responses) {
        return nil, errors.New("no more mock responses")
    }
    resp := &m.Responses[m.Index]
    m.Index++
    return resp, nil
}
```

### 6.3 测试覆盖目标

| 包 | 目标覆盖率 | 重点测试 |
|-----|-----------|----------|
| `pkg/llm` | 90% | 消息序列化、类型转换 |
| `pkg/tools` | 85% | 各工具执行、参数解析错误 |
| `pkg/skills` | 90% | Frontmatter 解析边界情况 |
| `pkg/memory` | 80% | L1-L5 检索准确性、时序逻辑 |
| `pkg/agent` | 75% | 循环终止、plan 嵌套、错误传播 |

---

## 7. 风险与回退方案

| 风险 | 回退方案 |
|------|----------|
| `go-openai` 不支持某新特性 | 直接自研 HTTP 客户端（OpenAI API 为简单 REST） |
| `modernc.org/sqlite` 编译问题 | 回退到 CGO `github.com/mattn/go-sqlite3`（限制 Linux/macOS） |
| Embedding 维度变化 | 将维度作为配置项，运行时动态处理 |
| 测试不稳定（依赖 LLM） | 全部使用 MockClient，核心逻辑零网络依赖 |
