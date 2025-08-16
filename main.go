package main

import (
	"claude-hooks/types"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func printHelp() {
	fmt.Println("Claude Hooks Plugin Manager")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  claude-plugin [OPTIONS] <plugins...> <command>")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  list         列出已加载的插件信息")
	fmt.Println("  execute      执行插件（从stdin读取JSON输入）")
	fmt.Println("  configure    根据指定插件自动配置hooks到settings.local.json")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --dir <path>  指定插件目录路径")
	fmt.Println("  --help, -h    显示此帮助信息")
	fmt.Println()
	fmt.Println("PLUGIN SPECIFICATION:")
	fmt.Println("  - 可以直接指定.so文件的完整路径")
	fmt.Println("  - 可以只指定插件名称，会从以下位置查找：")
	fmt.Println("    1. 使用--dir指定的目录")
	fmt.Println("    2. ~/.claude/hooks/（默认目录）")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # 直接指定插件文件路径")
	fmt.Println("  claude-plugin ./plugins/env.so ./plugins/announce.so list")
	fmt.Println()
	fmt.Println("  # 从指定目录加载插件")
	fmt.Println("  claude-plugin --dir ./plugins env announce list")
	fmt.Println()
	fmt.Println("  # 从默认路径加载插件")
	fmt.Println("  claude-plugin env announce execute")
	fmt.Println()
	fmt.Println("  # 配置插件到settings.local.json")
	fmt.Println("  claude-plugin gofmt env configure")
	fmt.Println()
	fmt.Println("  # 混合使用")
	fmt.Println("  claude-plugin --dir ./plugins env announce list")
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || hasHelpFlag(args) {
		printHelp()
		if len(args) == 0 {
			os.Exit(types.ExitCodeError)
		}
		return
	}

	if err := run(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		printHelp()
		os.Exit(types.ExitCodeError)
	}
}

func run(args []string) error {
	config, err := parseArgs(args)
	if err != nil {
		return err
	}

	pm := types.NewPluginManager("")
	if err := loadPlugins(pm, config.pluginPaths); err != nil {
		return err
	}

	return executeCommand(pm, config.command)
}

type config struct {
	pluginPaths []string
	command     string
}

func parseArgs(args []string) (*config, error) {
	cfg := &config{
		pluginPaths: make([]string, 0),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--help" || arg == "-h":
			// 帮助标志在main函数中已经处理，这里跳过
			continue

		case arg == "--dir":
			if i+1 >= len(args) {
				return nil, errors.New("--dir requires a directory path")
			}
			i++
			dir := args[i]
			i = parsePluginsFromDir(args, i, dir, cfg)

		case isCommand(arg):
			cfg.command = arg

		case strings.HasSuffix(arg, ".so"):
			// 直接指定的 .so 文件路径
			cfg.pluginPaths = append(cfg.pluginPaths, arg)

		default:
			// 如果不是以上情况，可能是插件名称，从默认路径查找
			if !strings.HasPrefix(arg, "-") {
				// 先尝试从默认路径查找
				if pluginPath := findPluginInDefaultPath(arg); pluginPath != "" {
					cfg.pluginPaths = append(cfg.pluginPaths, pluginPath)
				} else {
					return nil, fmt.Errorf("plugin not found: %q\n\nUse --help for usage information", arg)
				}
			} else {
				return nil, fmt.Errorf("unknown option: %q\n\nUse --help for usage information", arg)
			}
		}
	}

	if cfg.command == "" {
		return nil, errors.New("no command specified (list or execute)\n\nUse --help for usage information")
	}

	return cfg, nil
}

func parsePluginsFromDir(args []string, startIdx int, dir string, cfg *config) int {
	i := startIdx
	for i+1 < len(args) {
		i++
		nextArg := args[i]

		if isCommand(nextArg) {
			cfg.command = nextArg
			return i
		}

		// 先尝试从指定目录查找
		pluginPath := filepath.Join(dir, nextArg+".so")
		if _, err := os.Stat(pluginPath); err == nil {
			cfg.pluginPaths = append(cfg.pluginPaths, pluginPath)
		} else {
			// 如果指定目录没有，再从默认路径查找
			if defaultPath := findPluginInDefaultPath(nextArg); defaultPath != "" {
				cfg.pluginPaths = append(cfg.pluginPaths, defaultPath)
			} else {
				// 如果都没找到，还是使用指定目录的路径（可能用户想创建新插件）
				cfg.pluginPaths = append(cfg.pluginPaths, pluginPath)
			}
		}
	}
	return i
}

func findPluginInDefaultPath(pluginName string) string {
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// 默认插件路径
	defaultPath := filepath.Join(homeDir, ".claude", "hooks")

	// 构建可能的插件文件名
	possibleNames := []string{
		pluginName + ".so", // 如果输入的是纯名称
		pluginName,         // 如果输入的已经包含 .so
	}

	for _, name := range possibleNames {
		pluginPath := filepath.Join(defaultPath, name)
		if _, err := os.Stat(pluginPath); err == nil {
			// 确保返回的路径以 .so 结尾
			if strings.HasSuffix(pluginPath, ".so") {
				return pluginPath
			}
		}
	}

	return ""
}

func isCommand(arg string) bool {
	return arg == "list" || arg == "execute" || arg == "configure"
}

func loadPlugins(pm *types.PluginManager, paths []string) error {
	for _, path := range paths {
		if err := pm.LoadPlugin(path); err != nil {
			return fmt.Errorf("failed to load plugin %s: %w", path, err)
		}
	}
	return nil
}

func executeCommand(pm *types.PluginManager, command string) error {
	switch command {
	case "list":
		return handleListCommand(pm)
	case "execute":
		return handleExecuteCommand(pm)
	case "configure":
		return handleConfigureCommand(pm)
	default:
		return fmt.Errorf("unknown command: %q", command)
	}
}

func handleListCommand(pm *types.PluginManager) error {
	result := listPlugins(pm)
	result.ExitWithMessage()
	return nil
}

func handleExecuteCommand(pm *types.PluginManager) error {
	plugins := pm.Plugins()
	if len(plugins) == 0 {
		return errors.New("no plugins loaded")
	}

	input, err := readAndParseInput()
	if err != nil {
		return err
	}

	hookType, err := extractHookType(input)
	if err != nil {
		return err
	}

	inputData, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	var lastResult *types.Result
	for _, plugin := range plugins {
		result := executePlugin(hookType, string(inputData), plugin)
		if !result.IsSuccess() {
			result.ExitWithMessage()
			return nil
		}
		lastResult = &result
	}

	if lastResult != nil {
		lastResult.ExitWithMessage()
	}
	return nil
}

func readAndParseInput() (map[string]any, error) {
	data, err := readStdin()
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(data), &input); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}

	return input, nil
}

func extractHookType(input map[string]any) (string, error) {
	hookType, ok := input["hook_event_name"].(string)
	if !ok {
		return "", errors.New("missing or invalid hook_event_name")
	}
	return hookType, nil
}

func listPlugins(pm *types.PluginManager) types.Result {
	plugins := pm.ListPlugins()
	if len(plugins) == 0 {
		return types.NewSuccess("No plugins loaded.\n")
	}

	var sb strings.Builder
	sb.WriteString("Loaded plugins:\n")

	for _, info := range plugins {
		plugin, exists := pm.GetPlugin(info.Name)
		if !exists {
			continue
		}

		writePluginInfo(&sb, info, plugin.GetMetadata())
	}

	return types.NewSuccess(sb.String())
}

func writePluginInfo(sb *strings.Builder, info types.PluginInfo, metadata types.PluginMetadata) {
	// 将用户主目录路径替换为 ~
	displayPath := info.Path
	if homeDir, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(displayPath, homeDir) {
			displayPath = "~" + strings.TrimPrefix(displayPath, homeDir)
		}
	}

	fmt.Fprintf(sb, "\n• Plugin: %s (%s)\n", info.Name, displayPath)
	fmt.Fprintf(sb, "  Description: %s\n", info.Description)
	sb.WriteString("  Matchers:\n")

	matchers := []struct {
		name  string
		value string
	}{
		{"PreToolUse", metadata.Matcher.PreToolUse},
		{"PostToolUse", metadata.Matcher.PostToolUse},
	}

	hasMatchers := false
	for _, m := range matchers {
		if m.value != "" {
			fmt.Fprintf(sb, "    %s: %s\n", m.name, m.value)
			hasMatchers = true
		}
	}

	if !hasMatchers {
		sb.WriteString("    No matchers configured\n")
	}
}

func executePlugin(hookType string, inputData string, plugin types.IPlugin) types.Result {
	handler, ok := hookHandlers[hookType]
	if !ok {
		return types.NewError(fmt.Sprintf("unknown hook type: %s", hookType))
	}
	return handler(inputData, plugin)
}

var hookHandlers = map[string]func(string, types.IPlugin) types.Result{
	"PreToolUse":   handlePreToolUse,
	"PostToolUse":  handlePostToolUse,
	"Notification": handleNotification,
	"Stop":         handleStop,
	"SubagentStop": handleSubagentStop,
}

func handlePreToolUse(inputData string, plugin types.IPlugin) types.Result {
	var input types.ToolInput
	if err := json.Unmarshal([]byte(inputData), &input); err != nil {
		return types.NewError(fmt.Sprintf("invalid PreToolUse input: %v", err))
	}

	result, err := plugin.PreToolUse(input)
	return processPluginResult(withDefault(result), err)
}

func handlePostToolUse(inputData string, plugin types.IPlugin) types.Result {
	var input types.PostToolUseInput
	if err := json.Unmarshal([]byte(inputData), &input); err != nil {
		return types.NewError(fmt.Sprintf("invalid PostToolUse input: %v", err))
	}

	result, err := plugin.PostToolUse(input)
	return processPluginResult(withDefault(result), err)
}

func handleNotification(inputData string, plugin types.IPlugin) types.Result {
	var input types.NotificationInput
	if err := json.Unmarshal([]byte(inputData), &input); err != nil {
		return types.NewError(fmt.Sprintf("invalid Notification input: %v", err))
	}

	result, err := plugin.Notification(input)
	return processPluginResult(result, err)
}

func handleStop(inputData string, plugin types.IPlugin) types.Result {
	var input types.StopInput
	if err := json.Unmarshal([]byte(inputData), &input); err != nil {
		return types.NewError(fmt.Sprintf("invalid Stop input: %v", err))
	}

	result, err := plugin.Stop(input)
	return processPluginResult(withDefault(result), err)
}

func handleSubagentStop(inputData string, plugin types.IPlugin) types.Result {
	var input types.SubagentStopInput
	if err := json.Unmarshal([]byte(inputData), &input); err != nil {
		return types.NewError(fmt.Sprintf("invalid SubagentStop input: %v", err))
	}

	result, err := plugin.SubagentStop(input)
	return processPluginResult(result, err)
}

func processPluginResult(result any, err error) types.Result {
	if err != nil {
		return types.NewError(err.Error())
	}

	if result == nil {
		return types.NewSuccess("")
	}

	data, err := json.Marshal(result)
	if err != nil {
		return types.NewError(fmt.Sprintf("failed to marshal result: %v", err))
	}

	if blockResult := checkBlockDecision(data); blockResult != nil {
		return *blockResult
	}

	return types.NewSuccess(string(data))
}

func checkBlockDecision(data []byte) *types.Result {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	decision, ok := m["decision"].(string)
	if !ok || decision != "block" {
		return nil
	}

	reason, _ := m["reason"].(string)
	return &types.Result{
		Code:  types.ExitCodeBlockingError,
		Error: fmt.Sprintf("%s\n", reason),
	}
}

func withDefault[T any, U interface {
	*T
	Default()
}](input U) U {
	if input != nil {
		return input
	}
	var result U = new(T)
	result.Default()
	return result
}

func readStdin() (string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}
	return string(data), nil
}

// 配置管理相关结构体
type ClaudeSettings struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Deny  []string `json:"deny"`
		Ask   []string `json:"ask"`
	} `json:"permissions"`
	Hooks struct {
		PreToolUse  []HookConfig `json:"PreToolUse"`
		PostToolUse []HookConfig `json:"PostToolUse"`
	} `json:"hooks"`
}

type HookConfig struct {
	Matcher string      `json:"matcher"`
	Hooks   []HookEntry `json:"hooks"`
}

type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

func handleConfigureCommand(pm *types.PluginManager) error {
	plugins := pm.ListPlugins()
	if len(plugins) == 0 {
		return errors.New("no plugins loaded")
	}

	fmt.Printf("配置 %d 个插件到 settings.local.json...\n", len(plugins))

	if err := updateSettingsFile(pm); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	fmt.Println("✓ 配置更新完成")
	return nil
}

func updateSettingsFile(pm *types.PluginManager) error {
	settingsPath := "./.claude/settings.local.json"

	// 读取现有配置
	settings, err := loadSettings(settingsPath)
	if err != nil {
		return err
	}

	// 获取所有插件信息
	plugins := pm.ListPlugins()

	// 按hook类型组织插件
	preToolUsePlugins := make(map[string][]string)  // matcher -> plugin names
	postToolUsePlugins := make(map[string][]string) // matcher -> plugin names

	for _, info := range plugins {
		plugin, exists := pm.GetPlugin(info.Name)
		if !exists {
			continue
		}

		metadata := plugin.GetMetadata()

		// 处理PreToolUse匹配器
		if metadata.Matcher.PreToolUse != "" {
			matcher := metadata.Matcher.PreToolUse
			preToolUsePlugins[matcher] = append(preToolUsePlugins[matcher], info.Name)
		}

		// 处理PostToolUse匹配器
		if metadata.Matcher.PostToolUse != "" {
			matcher := metadata.Matcher.PostToolUse
			postToolUsePlugins[matcher] = append(postToolUsePlugins[matcher], info.Name)
		}
	}

	// 更新PreToolUse配置
	settings.Hooks.PreToolUse = updateHookConfigs(settings.Hooks.PreToolUse, preToolUsePlugins)

	// 更新PostToolUse配置
	settings.Hooks.PostToolUse = updateHookConfigs(settings.Hooks.PostToolUse, postToolUsePlugins)

	// 保存配置
	return saveSettings(settingsPath, settings)
}

func loadSettings(path string) (*ClaudeSettings, error) {
	settings := &ClaudeSettings{}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 文件不存在，创建默认配置
		settings.Permissions.Allow = []string{}
		settings.Permissions.Deny = []string{}
		settings.Permissions.Ask = []string{}
		settings.Hooks.PreToolUse = []HookConfig{}
		settings.Hooks.PostToolUse = []HookConfig{}
		return settings, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	if err := json.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings file: %w", err)
	}

	return settings, nil
}

func updateHookConfigs(existing []HookConfig, newPlugins map[string][]string) []HookConfig {
	result := make([]HookConfig, 0)
	processedMatchers := make(map[string]bool)

	// 更新现有配置
	for _, config := range existing {
		if pluginNames, exists := newPlugins[config.Matcher]; exists {
			// 更新现有匹配器的插件列表
			config.Hooks = []HookEntry{}
			for _, pluginName := range pluginNames {
				// 移除.so后缀
				cleanName := strings.TrimSuffix(pluginName, ".so")
				config.Hooks = append(config.Hooks, HookEntry{
					Type:    "command",
					Command: fmt.Sprintf("claude-plugin %s execute", cleanName),
				})
			}
			result = append(result, config)
			processedMatchers[config.Matcher] = true
		} else {
			// 保留不相关的现有配置
			result = append(result, config)
		}
	}

	// 添加新的匹配器配置
	for matcher, pluginNames := range newPlugins {
		if !processedMatchers[matcher] {
			config := HookConfig{
				Matcher: matcher,
				Hooks:   []HookEntry{},
			}
			for _, pluginName := range pluginNames {
				// 移除.so后缀
				cleanName := strings.TrimSuffix(pluginName, ".so")
				config.Hooks = append(config.Hooks, HookEntry{
					Type:    "command",
					Command: fmt.Sprintf("claude-plugin %s execute", cleanName),
				})
			}
			result = append(result, config)
		}
	}

	return result
}

func saveSettings(path string, settings *ClaudeSettings) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}
