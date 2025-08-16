# Claude Code Plugin Manager

A Go-based plugin management system for Claude Code hooks, allowing dynamic loading and execution of Go shared library (.so) plugins to handle various Claude Code hook events.

## Features

- **Dynamic Plugin Loading**: Load and execute Go plugins compiled as shared libraries (.so files)
- **Hook Support**: Handle PreToolUse, PostToolUse, Notification, Stop, and SubagentStop events
- **CLI Interface**: Comprehensive command-line tool for plugin management
- **Auto Configuration**: Automatically configure hooks in Claude Code settings
- **Built-in Plugins**: Includes security and code quality plugins

## Quick Start

### Prerequisites

- Go 1.21 or later
- Claude Code installed and configured

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd claudeplugin
```

2. Build and install:
```bash
make install
```

This will:
- Build the main `claude-plugin` binary and install it to `~/.local/bin/`
- Compile all plugins and install them to `~/.claude/hooks/`

### Basic Usage

1. **List loaded plugins**:
```bash
claude-plugin env gofmt list
```

2. **Execute plugins** (typically called by Claude Code):
```bash
echo '{"hook_event_name":"PreToolUse","tool":"Read","file_path":".env"}' | claude-plugin env execute
```

3. **Auto-configure hooks**:
```bash
claude-plugin env gofmt configure
```

## Plugin Architecture

### Plugin Interface

All plugins must implement the `IPlugin` interface:

```go
type IPlugin interface {
    Initialize() error
    Cleanup() error
    GetMetadata() PluginMetadata
    PreToolUse(arg ToolInput) (*PreToolUseOutput, error)
    PostToolUse(arg PostToolUseInput) (*PostToolUseOutput, error)
    Notification(arg NotificationInput) (*BaseHookOutput, error)
    Stop(arg StopInput) (*StopOutput, error)
    SubagentStop(arg SubagentStopInput) (*DecisionOutput, error)
}
```

### Creating a Plugin

1. Create a new directory under `plugins/`:
```bash
mkdir plugins/myplugin
```

2. Implement the plugin:
```go
package main

import "claude-hooks/types"

type MyPlugin struct {
    types.UnimplementedPlugin
}

func New() types.IPlugin {
    return &MyPlugin{}
}

func (p *MyPlugin) GetMetadata() types.PluginMetadata {
    return types.PluginMetadata{
        Description: "My custom plugin",
        Matcher: struct {
            PreToolUse  string
            PostToolUse string
        }{
            PreToolUse: "Read|Write",
            PostToolUse: "",
        },
    }
}

func (p *MyPlugin) PreToolUse(arg types.ToolInput) (*types.PreToolUseOutput, error) {
    var ret types.PreToolUseOutput
    // Your logic here
    return nil, nil
}
```

3. Build the plugin:
```bash
make build-plugin
```

## Built-in Plugins

### env Plugin
- **Purpose**: Security plugin that blocks access to `.env` files
- **Behavior**: Allows access to example files (`.env.example`, `.env.sample`) but blocks actual environment files
- **Hook**: PreToolUse
- **Matcher**: `Read|Write|Edit|MultiEdit`

### gofmt Plugin
- **Purpose**: Code quality plugin for automatic Go code formatting
- **Behavior**: Runs `goimports -w` on Go files after editing
- **Hook**: PostToolUse
- **Matcher**: `Write|Edit|MultiEdit`

### gocheck Plugin
- **Purpose**: Go syntax checking plugin
- **Behavior**: Runs `gopls check` on Go files after editing
- **Hook**: PostToolUse
- **Matcher**: `Write|Edit|MultiEdit`

## CLI Commands

### claude-plugin [OPTIONS] <plugins...> <command>

**Commands:**
- `list` - List loaded plugin information
- `execute` - Execute plugins (reads JSON input from stdin)
- `configure` - Auto-configure hooks in settings.local.json

**Options:**
- `--dir <path>` - Specify plugin directory path
- `--help, -h` - Show help information

**Plugin Specification:**
- Direct .so file paths: `./plugins/env.so`
- Plugin names (searches in `~/.claude/hooks/`): `env gofmt`
- Custom directory with `--dir`: `--dir ./plugins env gofmt`

### Examples

```bash
# Load plugins from default directory and list them
claude-plugin env gofmt list

# Load plugins from custom directory
claude-plugin --dir ./plugins env gofmt list

# Configure hooks automatically
claude-plugin env gofmt configure

# Execute plugins (used by Claude Code)
echo '{"hook_event_name":"PreToolUse",...}' | claude-plugin env execute
```

## Hook Processing Flow

1. CLI receives JSON hook input from stdin
2. Parses hook type and data
3. Executes all loaded plugins in sequence
4. Returns result based on plugin responses:
   - Exit code 0: Success, stdout shown to user
   - Exit code 1: General error, stderr shown to user
   - Exit code 2: Blocking error, stderr handled by Claude

## Development

### Build Commands

```bash
# Build main binary
make build

# Build all plugins
make build-plugin

# Clean build artifacts
make clean

# List available plugins
make list-plugins

# Full install (build + install)
make install
```

### Testing

```bash
go test ./...
```

### Code Formatting

```bash
goimports -w .
```

## Project Structure

```
.
├── main.go              # CLI entry point
├── types/
│   ├── types.go         # Hook input/output structures
│   └── plugin.go        # Plugin interfaces and manager
├── plugins/
│   ├── env/             # Environment file security plugin
│   ├── gofmt/           # Go formatting plugin
│   └── gocheck/         # Go syntax checking plugin
├── .claude/
│   └── hooks/           # Compiled plugin files (.so)
├── Makefile             # Build automation
└── CLAUDE.md            # Project guidance for Claude Code
```

## Integration with Claude Code

After running `claude-plugin <plugins> configure`, the tool automatically updates `.claude/settings.local.json` with appropriate hook configurations. Claude Code will then call the plugin manager for matching tool operations.

Example generated configuration:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Read|Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "claude-plugin env execute"
          }
        ]
      }
    ]
  }
}
```

## License

This project is open source. See LICENSE file for details.