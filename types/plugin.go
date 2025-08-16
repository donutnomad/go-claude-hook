package types

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
)

type PluginMetadata struct {
	Description string
	Matcher     struct {
		PreToolUse  string
		PostToolUse string
	}
}

type PluginInfo struct {
	Name        string
	Path        string
	Description string
}

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

type UnimplementedPlugin struct{}

func (u UnimplementedPlugin) Initialize() error {
	return nil
}

func (u UnimplementedPlugin) Cleanup() error {
	return nil
}

func (u UnimplementedPlugin) PreToolUse(arg ToolInput) (*PreToolUseOutput, error) {
	panic("implement me")
}

func (u UnimplementedPlugin) PostToolUse(arg PostToolUseInput) (*PostToolUseOutput, error) {
	panic("implement me")
}

func (u UnimplementedPlugin) Notification(arg NotificationInput) (*BaseHookOutput, error) {
	panic("implement me")
}

func (u UnimplementedPlugin) Stop(arg StopInput) (*StopOutput, error) {
	panic("implement me")
}

func (u UnimplementedPlugin) SubagentStop(arg SubagentStopInput) (*DecisionOutput, error) {
	panic("implement me")
}

// PluginManager 插件管理器
type PluginManager struct {
	plugins     map[string]IPlugin
	pluginPaths map[string]string // 存储插件名称到路径的映射
	pluginDir   string
	mu          sync.RWMutex
}

// NewPluginManager 创建新的插件管理器
func NewPluginManager(pluginDir string) *PluginManager {
	return &PluginManager{
		plugins:     make(map[string]IPlugin),
		pluginPaths: make(map[string]string),
		pluginDir:   pluginDir,
	}
}

// LoadPlugin 加载单个插件
func (pm *PluginManager) LoadPlugin(pluginPath string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin file does not exist: %s", pluginPath)
	}

	// 加载动态库
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin %s: %v", pluginPath, err)
	}

	// 查找New函数
	newFunc, err := p.Lookup("New")
	if err != nil {
		return fmt.Errorf("plugin %s does not export New function: %v", pluginPath, err)
	}

	// 类型断言
	creator, ok := newFunc.(func() IPlugin)
	if !ok {
		return fmt.Errorf("plugin %s New function has wrong signature", pluginPath)
	}

	// 创建插件实例
	pluginInstance := creator()
	if pluginInstance == nil {
		return fmt.Errorf("plugin %s New function returned nil", pluginPath)
	}

	// 初始化插件
	if err := pluginInstance.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %v", pluginPath, err)
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(pluginPath)
	if err != nil {
		absPath = pluginPath // 如果无法获取绝对路径，使用原路径
	}
	
	// 注册插件
	pluginName := filepath.Base(pluginPath)
	pm.plugins[pluginName] = pluginInstance
	pm.pluginPaths[pluginName] = absPath

	return nil
}

// LoadAllPlugins 加载目录中的所有插件
func (pm *PluginManager) LoadAllPlugins() error {
	if pm.pluginDir == "" {
		return fmt.Errorf("plugin directory not set")
	}

	// 检查目录是否存在
	if _, err := os.Stat(pm.pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin directory does not exist: %s", pm.pluginDir)
	}

	// 扫描.so文件
	files, err := filepath.Glob(filepath.Join(pm.pluginDir, "*.so"))
	if err != nil {
		return fmt.Errorf("failed to scan plugin directory: %v", err)
	}

	var loadErrors []string
	for _, file := range files {
		if err := pm.LoadPlugin(file); err != nil {
			loadErrors = append(loadErrors, err.Error())
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load some plugins: %s", strings.Join(loadErrors, "; "))
	}

	return nil
}

// UnloadPlugin 卸载插件
func (pm *PluginManager) UnloadPlugin(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pluginInstance, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// 清理插件资源
	if err := pluginInstance.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup plugin %s: %v", name, err)
	}

	// 从管理器中移除
	delete(pm.plugins, name)
	delete(pm.pluginPaths, name)

	return nil
}

// GetPlugin 获取插件实例
func (pm *PluginManager) GetPlugin(name string) (IPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pluginInstance, exists := pm.plugins[name]
	return pluginInstance, exists
}

func (pm *PluginManager) Plugins() []IPlugin {
	var ret = make([]IPlugin, 0)
	for _, p := range pm.plugins {
		ret = append(ret, p)
	}
	return ret
}

// ListPlugins 列出所有已加载的插件
func (pm *PluginManager) ListPlugins() []PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var plugins []PluginInfo
	for name, pluginInstance := range pm.plugins {
		metadata := pluginInstance.GetMetadata()
		plugins = append(plugins, PluginInfo{
			Name:        name,
			Path:        pm.pluginPaths[name],
			Description: metadata.Description,
		})
	}
	return plugins
}

// Shutdown 关闭插件管理器
func (pm *PluginManager) Shutdown() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var errors []string
	for name, pluginInstance := range pm.plugins {
		if err := pluginInstance.Cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to cleanup plugin %s: %v", name, err))
		}
	}

	// 清空插件映射
	pm.plugins = make(map[string]IPlugin)
	pm.pluginPaths = make(map[string]string)

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errors, "; "))
	}

	return nil
}
