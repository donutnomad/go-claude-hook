# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

这是一个用Go语言编写的Claude Code插件管理器。它允许加载和执行Go动态库(.so文件)插件来处理Claude Code的各种hook事件。

## 开发命令

```bash
# 构建主程序
make build

# 构建所有插件
make build-plugin

# 运行测试
go test

# 格式化代码
goimports -w .

# 构建特定插件
go build -buildmode=plugin -o ./.claude/hooks/plugin_name.so ./plugins/plugin_name/plugin_name.go

# 运行插件管理器
./claude-plugin --help
```

## 核心架构

### 主要组件

- **main.go**: CLI入口点，处理命令行参数解析和插件执行流程
- **types/**: 包含所有类型定义和接口
  - `types.go`: Hook输入/输出结构体和退出码定义
  - `plugin.go`: 插件接口和插件管理器实现
- **plugins/**: 具体插件实现目录
  - `env/`: 阻止访问.env文件的安全插件
  - `gofmt/`: Go代码格式化插件

### 插件架构

插件系统基于Go的plugin包实现动态库加载：

1. **插件接口**: `IPlugin`接口定义了所有插件必须实现的方法
2. **Hook类型**: 支持PreToolUse、PostToolUse、Notification、Stop、SubagentStop五种hook事件
3. **插件管理器**: `PluginManager`负责加载、管理和执行插件
4. **动态库**: 插件编译为.so文件，通过plugin包动态加载

### Hook处理流程

1. CLI从stdin接收JSON格式的hook输入
2. 解析hook类型和数据
3. 按顺序执行所有已加载的插件
4. 根据插件返回结果决定是否继续或阻止操作
5. 通过退出码和输出与Claude Code通信

### 退出码系统

- `ExitCodeSuccess (0)`: 成功，stdout内容显示给用户
- `ExitCodeBlockingError (2)`: 阻塞性错误，stderr显示给Claude处理
- `ExitCodeError (1)`: 一般错误，stderr显示给用户

### 插件开发规范

新插件必须：
1. 实现`IPlugin`接口
2. 导出`New() IPlugin`函数
3. 嵌入`UnimplementedPlugin`并override需要的方法
4. 在`GetMetadata()`中定义描述和匹配器

### 目录结构

- 插件源码存放在`plugins/`目录
- 编译后的.so文件存放在`.claude/hooks/`
- 最终安装到`~/.claude/hooks/`供Claude Code使用

## 测试说明

项目包含基本的测试框架，当前测试主要验证代码格式和语法检查工具的集成。可以通过`go test`运行所有测试。

## 插件示例

- **env插件**: 安全插件，阻止访问实际的.env文件，但允许访问示例文件
- **gofmt插件**: 代码质量插件，在编辑Go文件后自动运行goimports格式化