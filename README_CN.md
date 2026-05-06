# go-agent

> Go 语言实现的极简 AI Agent 教育套件。
> 源自 [nanoAgent](https://github.com/sanbuphy/nanoAgent)、[nanoMemory](https://github.com/sanbuphy/nanoMemory) 和 [nanoSkills](https://github.com/sanbuphy/nanoSkills) 的工程化重构。

## 快速开始

```bash
# 安装依赖
go mod tidy

# 设置 API Key
export OPENAI_API_KEY="sk-xxxxxxxx"

# 运行基础 Agent
go run . "列出当前目录所有 go 文件"

# 启用任务规划
go run . -plan "重构 main 包"

# ClaudeCode 风格 Agent
go run . -claudecode "阅读 README.md 并总结"
```

## 记忆级别

```bash
go run ./cmd/memory_l1 "我喜欢暗色模式"
go run ./cmd/memory_l2 "你了解我什么？"
go run ./cmd/memory_l3 "提醒我之前讨论的内容"
go run ./cmd/memory_l4 "Alice 搬到了东京"
go run ./cmd/memory_l5 "我一直在学习 Rust"
```

## 技能系统

```bash
go run ./cmd/skills "review my code"
go run ./cmd/skills --skills-dir ./skills-fake "预订去巴黎的机票"
```

## 许可证

MIT
