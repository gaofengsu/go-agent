# go-agent

> A minimal yet comprehensive AI Agent framework in Go, distilled from the educational "nano" family (nanoAgent / nanoMemory / nanoSkills). Designed to demonstrate deep understanding of LLM-based agent architecture, memory hierarchies, and extensible skill systems.
>
> **Educational philosophy**: intentionally small (~50-250 LOC per module), statically typed, and copy-paste runnable — but architected like production code.

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## Table of Contents

1. [Why This Project?](#why-this-project)
2. [Architecture Overview](#architecture-overview)
3. [Core Concepts Deep Dive](#core-concepts-deep-dive)
   - Agent Core: The ReAct Loop
   - Memory Hierarchy: 6 Progressive Levels
   - Skills System: Progressive Disclosure
   - Tool Registry: Plugin Architecture
4. [Project Structure](#project-structure)
5. [Quick Start](#quick-start)
6. [Design Decisions](#design-decisions)
7. [Extending the Framework](#extending-the-framework)
8. [Testing Strategy](#testing-strategy)
9. [References & Further Reading](#references--further-reading)

---

## Why This Project?

Most agent frameworks (LangChain, AutoGPT, etc.) are **black boxes**. You import, configure, and pray. This project takes the opposite approach: **every concept is exposed, typed, and testable**.

Building `go-agent` taught me (and demonstrates) the following:

| Skill Demonstrated | Evidence in Code |
|-------------------|------------------|
| **LLM Function Calling** | `pkg/llm/client.go` — raw OpenAI API mapping, message type system, tool schema construction |
| **ReAct Agent Loop** | `pkg/agent/agent.go` — explicit observation → thought → action iteration with max-iteration guard |
| **Memory Architecture** | `pkg/memory/` — 6 levels from stateless (L0) to temporal knowledge graph (L4), each implementing the same `Memory` interface |
| **Vector Retrieval** | `pkg/memory/l2_vector.go` — hand-rolled cosine similarity, no numpy dependency |
| **Structured Knowledge** | `pkg/memory/l4_graph.go` — SQLite SPO triples with contradiction detection and temporal invalidation |
| **Plugin System** | `pkg/tools/tools.go` — `Tool` interface + `Registry` enabling zero-cost tool addition |
| **Go Engineering** | Interface-driven design, explicit error handling, mockable LLM client, table-driven tests |

> **For hiring managers**: This is not a tutorial copy-paste. Every line was written from first principles, with architectural trade-offs documented below.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           User Input                                 │
└──────────────────────────────────┬──────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Agent (pkg/agent)                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐   │
│  │ Base Agent   │  │ Plus Agent   │  │ ClaudeCode Agent         │   │
│  │ (tool loop)  │  │ (+ memory    │  │ (+ rules + skills + MCP  │   │
│  │              │  │  + planning) │  │  + rich tools)           │   │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘   │
└──────────────────────────────────┬──────────────────────────────────┘
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      LLM Client (pkg/llm)                            │
│         OpenAI-compatible API abstraction layer                      │
│         ChatCompletion + Embeddings unified interface                │
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
│                    Memory System (pkg/memory) — 6 Levels             │
│  L0 Stateless → L1 Keyword → L2 Vector → L3 Hybrid → L4 Graph → L5   │
│  Summary                                                              │
└─────────────────────────────────────────────────────────────────────┘
                                   ▲
┌──────────────────────────────────┴──────────────────────────────────┐
│                   Skills System (pkg/skills)                         │
│         YAML-frontmatter skill files with dynamic activation         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Core Concepts Deep Dive

### 1. Agent Core: The ReAct Loop

The fundamental pattern of all LLM agents is the **ReAct loop** (Reasoning + Acting):

```
User Query → System Prompt → LLM
    ← Assistant Message (may contain tool_calls)
    → Execute Tools → Feed Results back to LLM
    ← Assistant Message (may contain more tool_calls)
    → ... iterate until no more tool_calls
    ← Final Answer
```

**Implementation**: `pkg/agent/agent.go:Run()`

```go
for i := 0; i < a.MaxIterations; i++ {
    resp, err := a.Client.Chat(ctx, messages, toolSchemas)
    // ... append assistant message
    if len(resp.ToolCalls) == 0 {
        return resp.Content  // done
    }
    for _, tc := range resp.ToolCalls {
        result := a.Registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
        // ... append tool result message
    }
}
```

**Key insight**: The `messages` slice is the **entire state** of the agent. No hidden context, no magic session management. This makes the agent trivially mockable and debuggable.

### 2. Memory Hierarchy: 6 Progressive Levels

Memory is the hardest problem in agent engineering. This project implements 6 levels, each building on the previous:

| Level | File | Core Mechanism | When to Use |
|-------|------|---------------|-------------|
| **L0** | `l0_stateless.go` | No memory | Baseline / stateless APIs |
| **L1** | `l1_file.go` | JSONL + keyword matching | Simple preference tracking |
| **L2** | `l2_vector.go` | OpenAI Embedding + cosine similarity | Semantic recall |
| **L3** | `l3_scored.go` | sim × recency × importance + reflection | Long-term personalization |
| **L4** | `l4_graph.go` | SQLite SPO triples + temporal tracking | Structured knowledge, history |
| **L5** | `l5_summary.go` | LLM-compressed summaries | Bounded context windows |

#### L2 Vector Search (No NumPy!)

Python relies on `numpy` for vector ops. Go doesn't need it for small dimensions:

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

At 1536 dimensions and <10k memories, this is **sub-millisecond**. No CGO, no BLAS, no deployment headache.

#### L4 Knowledge Graph: Temporal Reasoning

The graph memory stores facts as `(Subject, Predicate, Object)` triples with `valid_from` and `valid_until` timestamps.

**Contradiction detection**: When "Alice lives in Tokyo" is recorded but "Alice lives in Paris" already exists for the same subject+predicate, the old triple is invalidated:

```sql
UPDATE triples SET valid_until=? WHERE id=?
```

This enables **temporal queries**: "Where did Alice live *before* 2024?"

#### L3 Hybrid Scoring: The Park Framework

Inspired by [Park et al. (2023)](https://arxiv.org/abs/2304.03442) *Generative Agents*:

```
score = α·similarity + β·recency + γ·importance

recency = 0.5^(days / 30)   // Ebbinghaus forgetting curve
importance = LLM-rated 1-10 // computed once at write time
```

This mimics human memory: recent events and important facts stick longer.

### 3. Skills System: Progressive Disclosure

Skills are YAML-frontmatter markdown files that inject expertise into the context **only when needed**:

```markdown
---
name: git-expert
description: Advanced Git operations and best practices expert
---

You are a Git expert. Prefer rebase over merge...
```

The agent exposes an `activate_skill(name)` tool. The LLM decides **when** to activate a skill based on the user's query. This keeps the system prompt short (saving tokens and reducing distraction) while making deep expertise available on demand.

**Contrast with Fine-tuning**:
- Fine-tuning: bake knowledge into model weights (expensive, slow to update)
- Skills: inject knowledge via context (free, instant, version-controlled)

### 4. Tool Registry: Plugin Architecture

Every tool implements the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() llm.ToolSchema
    Execute(ctx context.Context, args map[string]any) (string, error)
}
```

Adding a new tool is one line:

```go
r.Register(&MyCustomTool{})
```

The registry auto-generates JSON schemas for OpenAI's `functions` parameter. No manual schema maintenance.

---

## Project Structure

```
go-agent/
├── main.go                     # Unified CLI: go-agent -plan -claudecode
├── config.json                 # Config: API keys, model params, memory settings
├── go.mod / go.sum
├── README.md / README_CN.md
│
├── pkg/
│   ├── llm/                    # LLM abstraction layer
│   │   ├── message.go          # Message, ToolCall, ToolSchema types
│   │   └── client.go           # OpenAI-compatible client (Chat + Embed)
│   │
│   ├── agent/                  # Three agent variants
│   │   ├── agent.go            # Base ReAct loop (~60 LOC)
│   │   ├── plus.go             # + memory + planning
│   │   └── claudecode.go       # + rules + skills + MCP + rich tools
│   │
│   ├── memory/                 # 6-level progressive memory
│   │   ├── memory.go           # Memory interface
│   │   ├── l0_stateless.go     # No-op baseline
│   │   ├── l1_file.go          # Keyword matching (JSONL)
│   │   ├── l2_vector.go        # Embedding + cosine similarity
│   │   ├── l3_scored.go        # Hybrid scoring + reflection
│   │   ├── l4_graph.go         # SQLite SPO triples
│   │   └── l5_summary.go       # LLM compression
│   │
│   ├── skills/                 # Skill parsing and activation
│   │   ├── skill.go            # YAML frontmatter parser
│   │   └── store.go            # Discovery + activate_skill tool builder
│   │
│   └── tools/                  # Tool implementations
│       ├── tools.go            # Registry + Tool interface
│       ├── bash.go             # Shell execution
│       ├── file.go             # read / write / edit
│       ├── search.go           # glob / grep
│       └── plan.go             # LLM-based task decomposition
│
├── cmd/                        # Standalone educational executables
│   ├── agent/
│   ├── agent_plus/
│   ├── agent_claudecode/
│   ├── memory_l1 ~ memory_l5/
│   └── skills/
│
├── skills-real/                # Real expertise (git, code-review, API design...)
│   ├── git-expert/SKILL.md
│   ├── code-reviewer/SKILL.md
│   ├── api-designer/SKILL.md
│   ├── docs-writer/SKILL.md
│   └── web-best-practices/SKILL.md
│
├── skills-fake/                # Simulated scenarios
│   └── travel-agent/SKILL.md
│
├── internal/testutil/          # Mock LLM client for zero-network testing
│   └── mock.go
│
└── 文档/                        # Local docs (not in repo)
```

---

## Quick Start

```bash
# 1. Clone and enter
cd go-agent

# 2. Install dependencies (only 2 external: go-openai + sqlite)
go mod tidy

# 3. Configure API key
export OPENAI_API_KEY="sk-xxxxxxxx"

# 4. Run

# Base agent
go run . "list all go files"

# With planning
go run . -plan "refactor the main package"

# ClaudeCode-style with rich tools
go run . -claudecode "read README.md and summarize"

# Memory levels
go run ./cmd/memory_l1 "I prefer dark mode"
go run ./cmd/memory_l2 "What do you know about me?"
go run ./cmd/memory_l4 "Alice moved to Tokyo"

# Skills
go run ./cmd/skills --skills-dir ./skills-real "review my code"
```

---

## Design Decisions

### Why Go instead of Python?

| Dimension | Python (original) | Go (this project) |
|-----------|-------------------|-------------------|
| **Type Safety** | Runtime errors from dynamic types | Compile-time guarantees |
| **Error Handling** | `try/except` (easy to swallow) | `if err != nil` (explicit) |
| **Deployment** | Interpreter + pip + venv | Single static binary |
| **Concurrency** | GIL-limited threads | Goroutines + channels |
| **Readability** | Magic methods, metaclasses | Explicit interfaces, structs |

The original Python projects are **educational** (copy-paste runnable). This Go version preserves that spirit while adding **production-ready architecture**.

### Why `modernc.org/sqlite` instead of `mattn/go-sqlite3`?

`mattn/go-sqlite3` requires CGO, which breaks cross-compilation. `modernc.org/sqlite` is a pure Go transpilation of SQLite — same SQL dialect, zero CGO, compiles to a single binary for Linux/macOS/Windows.

### Why hand-rolled cosine similarity instead of a BLAS library?

Embedding dimensions are small (1536 for `text-embedding-3-small`). At this scale, a 20-line loop outperforms the overhead of calling into C BLAS. No dependency, no cross-platform pain.

### Why no `viper` / `cobra` / `urfave/cli`?

The `flag` package from the standard library is sufficient. The goal is **minimalism**. Every external dependency is a liability.

---

## Extending the Framework

### Adding a New Tool

```go
// pkg/tools/weather.go
type WeatherTool struct{}

func (t *WeatherTool) Name() string        { return "get_weather" }
func (t *WeatherTool) Description() string { return "Get current weather" }
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
    // ... fetch weather
    return "25°C, sunny", nil
}
```

Register in `pkg/tools/tools.go`:
```go
r.Register(&WeatherTool{})
```

### Adding a New Memory Level

1. Create `pkg/memory/l6_custom.go`
2. Implement the `Memory` interface
3. Add `cmd/memory_l6/main.go`

---

## Testing Strategy

All core logic is tested with **zero network calls** using `internal/testutil.MockClient`:

```go
// Simulate: LLM requests a tool → tool executes → LLM returns final answer
client := testutil.NewMockToolClient("execute_bash", `{"command":"echo test"}`, "Done")
a := agent.NewAgent(client)
result, _, err := a.Run(ctx, "sys", "run echo test")
// result == "Done"
```

Run tests:
```bash
go test ./...
```

| Package | Coverage Focus |
|---------|---------------|
| `pkg/llm` | Message serialization, type round-trips |
| `pkg/tools` | Registry dispatch, argument parsing, error paths |
| `pkg/skills` | Frontmatter parsing edge cases |
| `pkg/memory` | Retrieval accuracy, temporal logic |
| `pkg/agent` | Loop termination, plan nesting, error propagation |

---

## References & Further Reading

- **ReAct**: Yao et al. (2023) "ReAct: Synergizing Reasoning and Acting in Language Models" — [arXiv:2210.03629](https://arxiv.org/abs/2210.03629)
- **Generative Agents**: Park et al. (2023) "Generative Agents: Interactive Simulacra of Human Behavior" — [arXiv:2304.03442](https://arxiv.org/abs/2304.03442)
- **MemoryBank**: Zhong et al. (2024) "MemoryBank: Enhancing Large Language Models with Long-Term Memory" — [arXiv:2401.10917](https://arxiv.org/abs/2401.10917)
- **LoCoMo**: LoCoMo Benchmark — [arXiv:2402.17753](https://arxiv.org/abs/2402.17753)
- **go-openai**: [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai)

---

## License

MIT — same as the original nano family.

---

> *"If you can't build it from scratch, you don't understand it."*
