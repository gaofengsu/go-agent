# go-agent

> Go implementation of the minimal AI Agent educational suite.
> Inspired by [nanoAgent](https://github.com/sanbuphy/nanoAgent), [nanoMemory](https://github.com/sanbuphy/nanoMemory), and [nanoSkills](https://github.com/sanbuphy/nanoSkills).

## Quick Start

```bash
# Install dependencies
go mod tidy

# Set your API key
export OPENAI_API_KEY="sk-xxxxxxxx"

# Run base agent
go run . "list all go files"

# Run with planning
go run . -plan "refactor the main package"

# Run ClaudeCode-style agent
go run . -claudecode "read README.md and summarize"
```

## Memory Levels

```bash
go run ./cmd/memory_l1 "I prefer dark mode"
go run ./cmd/memory_l2 "What do you know about me?"
go run ./cmd/memory_l3 "Remind me what we discussed"
go run ./cmd/memory_l4 "Alice moved to Tokyo"
go run ./cmd/memory_l5 "I've been learning Rust"
```

## Skills

```bash
go run ./cmd/skills "review my code"
go run ./cmd/skills --skills-dir ./skills-fake "book a flight"
```

## License

MIT
