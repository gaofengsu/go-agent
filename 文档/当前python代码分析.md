# 当前 Python 代码分析

> 分析范围：`nanoAgent/`、`nanoMemory/`（L0-L5）、`nanoSkills/`
> 分析目的：识别核心逻辑、数据流、Python 特性依赖，为 Go 迁移提供映射依据。

---

## 1. nanoAgent 分析

### 1.1 agent.py — 最小 Agent（103 行）

**核心结构：**
```python
client = OpenAI(api_key=..., base_url=...)  # 模块级全局客户端
tools = [{...}]  # 硬编码工具 JSON Schema
functions = {"execute_bash": execute_bash, ...}  # 函数字典映射

def run_agent(user_message, max_iterations=5):
    messages = [system, user]
    for _ in range(max_iterations):
        response = client.chat.completions.create(model=..., messages=messages, tools=tools)
        message = response.choices[0].message
        messages.append(message)
        if not message.tool_calls:
            return message.content
        for tool_call in message.tool_calls:
            name = tool_call.function.name
            args = json.loads(tool_call.function.arguments)
            result = functions[name](**args)  # 动态函数调用
            messages.append({"role": "tool", "tool_call_id": tool_call.id, "content": result})
```

**关键 Python 特性：**
- `**args` 关键字参数解包：Go 无此特性，需显式构造参数结构体或使用 `map[string]interface{}`
- 函数作为一等公民存入字典：Go 使用 `map[string]func(...)` 或接口方法表
- `subprocess.run(command, shell=True)`：Go 对应 `exec.Command("sh", "-c", command)`
- 全局变量 `client`：Go 中应显式传入或封装为结构体字段
- OpenAI SDK 返回的 `message` 对象直接 `append` 到 `messages`：Go 中需区分 ChatCompletionMessage / ToolMessage，手动构建消息切片

**数据流：**
```
用户输入 → system prompt + user message → LLM(tools=enabled)
  ← assistant message (可能含 tool_calls)
  → 提取 name + arguments → 本地函数执行 → tool result message
  → 追加到 messages → 再次 LLM 调用 → 循环直到无 tool_calls
```

### 1.2 agent-plus.py — 增强 Agent（206 行）

**新增能力：**

| 能力 | 实现方式 | 数据持久化 |
|------|----------|------------|
| 记忆加载 | `load_memory()` 读取 `agent_memory.md` 最后 50 行 | Markdown 文件追加 |
| 记忆保存 | `save_memory(task, result)` 追加时间戳 + 任务 + 结果 | 同上 |
| 任务规划 | `create_plan(task)` 单独 LLM 调用，要求返回 JSON 数组 | 无持久化 |
| 步骤执行 | `run_agent_step(task, messages)` 复用核心循环 | 内存 messages |

**规划模式数据流：**
```
用户输入 → create_plan() → LLM 返回 steps[]
  → 对每一步调用 run_agent_step()
     → 每步共享 messages 上下文（累积）
  → 所有结果拼接 → save_memory()
```

**Python 特性依赖：**
- `datetime.now().strftime("%Y-%m-%d %H:%M:%S")`：Go `time.Now().Format("2006-01-02 15:04:05")`
- `lines[-50:]` 切片：Go 切片 `lines[max(0, len(lines)-50):]`
- `response_format={"type": "json_object"}`：Go SDK 同等支持
- `getattr(tool_call, "function", None)` 防御式属性访问：Go 结构体字段直接访问，零值安全

### 1.3 agent-claudecode.py — ClaudeCode 风格 Agent（282 行）

**工具集扩展：**
- `read(path, offset, limit)`：带行号读取，支持分页
- `edit(path, old_string, new_string)`：精确字符串替换，要求 `count(old_string) == 1`
- `glob(pattern)`：基于 `glob_module.glob`，按修改时间排序
- `grep(pattern, path)`：封装 `grep -r` 系统命令
- `plan(task)`：动态规划，支持嵌套执行（plan 内禁用 plan）

**扩展子系统：**

```
┌─────────────────────────────────────────────────────────────┐
│                    run_agent_claudecode                      │
├─────────────┬─────────────┬─────────────┬───────────────────┤
│  load_memory│  load_rules │ load_skills │   load_mcp_tools  │
│  (Markdown) │  (.md files)│ (.json files)│  (mcp.json)      │
└─────────────┴─────────────┴─────────────┴───────────────────┘
                        ↓
              合并为 system prompt
                        ↓
              run_agent_step (tools循环)
```

**全局状态风险：**
```python
current_plan = []   # 全局，plan 工具修改
plan_mode = False   # 全局，控制 plan 嵌套检测
```
> **Go 迁移注意**：应将这些状态封装到 `Agent` 结构体中，避免全局可变状态。

**嵌套规划执行逻辑：**
当 tool_call 为 `plan` 时，agent-claudecode 会：
1. 设置 `plan_mode = True`
2. 调用 `plan()` 生成步骤列表
3. 对每个步骤，递归调用 `run_agent_step()`，但**禁用 plan 工具**（从 tools 中过滤）
4. 执行完后重置 `plan_mode = False`

> 这是一个典型的状态机模式，Go 中可用显式结构体 + 方法实现。

---

## 2. nanoMemory 分析

### 2.1 L0 — 无记忆（agent.py）

作为基准，无跨会话状态，每次调用独立。

### 2.2 L1 — 文件记忆（memory_file.py，127 行）

**核心设计：**
- 存储：`memory_facts.jsonl`，每行一个 JSON 对象 `{text, source, timestamp}`
- 检索：关键词匹配 `set(query.lower().split())` 与记忆文本的交集大小排序
- 提取：LLM 从对话中提取事实，返回 JSON 数组
- 保留策略：仅保留最近 200 条

**Python 特性映射：**
| Python | Go 对应 |
|--------|---------|
| `json.loads(line)` | `json.Unmarshal([]byte(line), &entry)` |
| `set(a) & set(b)` | 手写循环或用 `map[string]struct{}` 实现集合 |
| `lines[-200:]` | 切片操作 + `copy` |
| `datetime.now().isoformat()` | `time.Now().Format(time.RFC3339)` |

### 2.3 L2 — 向量记忆（memory_vector.py，136 行）

**核心设计：**
- 存储：`memory_vector.jsonl`，每行追加 `{text, embedding[], timestamp, metadata}`
- 检索：OpenAI Embedding API 获取查询向量 → 与所有记忆向量计算余弦相似度 → 取 top_k
- 向量运算：使用 `numpy` 的 `np.dot`、`np.linalg.norm`

**数学公式：**
```
cosine_sim(a, b) = dot(a, b) / (norm(a) * norm(b) + 1e-8)
```

**Go 迁移要点：**
- 无需 `numpy`，手写向量运算即可（维度 1536，计算量极小）
- `client.embeddings.create()` → Go OpenAI SDK 的 `CreateEmbeddings`
- JSONL 追加写入 → Go `bufio.Writer` + `json.Encoder`

### 2.4 L3 — 混合评分记忆（memory_scored.py，211 行）

**核心设计：**
- 三因素评分：`score = α·sim + β·recency + γ·importance`
  - `α=0.5, β=0.3, γ=0.2`
  - 时效性：`0.5^(days / half_life)`，half_life = 30 天
  - 重要性：LLM 打分 1-10，写入时计算一次
- 访问计数：`access_count` 每次检索递增
- 反思：当记忆数量达到阈值，LLM 生成高层洞察，写入 `memory_reflections.jsonl`

**时间衰减计算（Python → Go）：**
```python
# Python
days = (datetime.now() - ts).total_seconds() / 86400
score = 0.5 ** (days / 30.0)

# Go
days := time.Since(ts).Hours() / 24
score := math.Pow(0.5, days/30.0)
```

### 2.5 L4 — 知识图谱记忆（memory_graph.py，214 行）

**核心设计：**
- 存储：SQLite，单表 `triples(id, subject, predicate, object, confidence, source, valid_from, valid_until, embedding)`
- 索引：`idx_subject`, `idx_predicate`, `idx_active`
- 实体归一化：`normalize_entity()` 小写+去空格
- 矛盾检测：同一 subject+predicate 的活跃三元组，若 object 不同则使旧记录失效
- 查询：支持当前事实查询（`valid_until IS NULL`）和历史查询

**SQL 模式映射：**
```python
# Python sqlite3 (stdlib)
conn = sqlite3.connect(DB_PATH)
conn.execute("...", params)

# Go
import "database/sql"
import _ "modernc.org/sqlite"  // 纯 Go SQLite
db, _ := sql.Open("sqlite", DB_PATH)
db.Query("...", params...)
```

### 2.6 L5 — 摘要压缩记忆（memory_summary.py，159 行）

**核心设计：**
- 存储：`memory_summaries.jsonl`，每行 `{summary, source_turns, timestamp, type}`
- 提取：每轮对话后，LLM 将对话压缩为单句摘要（或 `NONE`）
- 压缩：当累计对话数达到 `COMPRESS_EVERY`（默认 5），将所有摘要合并为一条压缩摘要
- 检索：关键词匹配（与 L1 相同策略，但作用于摘要文本）

---

## 3. nanoSkills 分析

### 3.1 agent.py（140 行）

**核心设计：**
- `parse_skill(path)`：正则解析 YAML Frontmatter
  ```python
  re.match(r"^---\n(.*?)\n---\n(.*)$", content, re.DOTALL)
  ```
- `discover_skills(directory)`：递归查找 `**/SKILL.md`
- `build_activate_tool(skills)`：动态构建 JSON Schema，`enum` 值为所有 skill 名称
- `activate_skill(name, skills)`：查找并返回 XML 包装的技能正文

**Skills 注入数据流：**
```
扫描 skills-dir → parse_skill() → []Skill
  → build_activate_tool() → 添加到 tools 列表
  → 技能列表文本 → 注入 system prompt
  
Agent 对话中 → LLM 调用 activate_skill(name)
  → activate_skill() 返回 XML body
  → 作为 tool result 返回，注入上下文
```

### 3.2 test_skills.py（42 行）

**已知问题：**
```python
from skills import parse_skill, discover_skills, activate_skill
```
但 `skills.py` 不存在，实际函数在 `agent.py` 中。

> **Go 迁移注意**：模块分离应在一开始设计好，提取 `pkg/skills` 包。

---

## 4. 跨项目共性抽象

### 4.1 共同模式

| 模式 | nanoAgent | nanoMemory | nanoSkills |
|------|-----------|------------|------------|
| **LLM 客户端** | 全局 `OpenAI()` | 全局 `OpenAI()` | 全局 `OpenAI()` |
| **消息循环** | tool_calls 迭代 | 纯对话（无工具） | tool_calls 迭代 |
| **配置读取** | `os.environ` | `os.environ` | `os.environ` |
| **文件操作** | `open()`/`subprocess` | `jsonl`/`sqlite3` | `glob`/`re` |
| **错误处理** | try/except 返回字符串 | try/except 忽略 | 简单判断 |

### 4.2 建议的 Go 核心接口

基于以上分析，Go 版应抽象出以下接口：

```go
// LLM 客户端
type LLMClient interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// 工具执行器
type Tool interface {
    Name() string
    Description() string
    Schema() ToolSchema
    Execute(ctx context.Context, args map[string]any) (string, error)
}

// 记忆系统
type Memory interface {
    Save(ctx context.Context, userInput, aiResponse string) error
    Search(ctx context.Context, query string, topK int) ([]MemoryEntry, error)
}

// Skill 系统
type SkillStore interface {
    Discover(dir string) ([]Skill, error)
    Activate(name string) (string, error)
}
```

### 4.3 Python → Go 关键迁移对照表

| Python 特性/库 | Go 替代方案 |
|----------------|-------------|
| `openai` SDK | `github.com/sashabaranov/go-openai` |
| `json` (stdlib) | `encoding/json` |
| `os.environ` | `os.Getenv` |
| `subprocess.run(shell=True)` | `os/exec.Command("sh", "-c", ...)` |
| `glob.glob` | `filepath.Glob` / `filepath.Walk` |
| `re.match` | `regexp` |
| `datetime` | `time` |
| `sqlite3` (stdlib) | `database/sql` + `modernc.org/sqlite` |
| `numpy` 向量运算 | 手写循环（维度小，无需 BLAS） |
| `**kwargs` | `map[string]any` + 类型断言 |
| 全局变量 | 结构体字段 + 构造函数 |
| 列表推导式 | 显式 `for` 循环 |
| `with open(...)` | `defer f.Close()` |
| `try/except` | `if err != nil` |
| `jsonl` 读写 | `bufio.Scanner` + `json.Encoder` |

---

## 5. 复杂度评估

| 模块 | 迁移复杂度 | 主要挑战 |
|------|-----------|----------|
| nanoAgent core | 低 | 消息类型转换、工具注册表 |
| nanoAgent plus | 低 | 规划步骤状态管理 |
| nanoAgent claudecode | 中 | 富工具集、嵌套 plan、多子系统加载 |
| nanoMemory L1 | 低 | JSONL 解析、关键词匹配 |
| nanoMemory L2 | 低 | 向量运算自实现 |
| nanoMemory L3 | 中 | 三因素评分、反思触发条件 |
| nanoMemory L4 | 中 | SQLite 模式、时序 SQL 查询 |
| nanoMemory L5 | 低 | 摘要压缩策略复用 |
| nanoSkills | 低 | YAML Frontmatter 解析 |
| **整体集成** | **中** | 统一 CLI、配置、接口对齐 |
