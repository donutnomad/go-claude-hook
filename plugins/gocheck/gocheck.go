package main

import (
	"bytes"
	"claude-hooks/types"
	"fmt"
	"os/exec"
	"strings"
)

type Plugin struct {
	types.UnimplementedPlugin
}

func New() types.IPlugin {
	return &Plugin{}
}

func (p *Plugin) GetMetadata() types.PluginMetadata {
	return types.PluginMetadata{
		Description: "在编辑完go文件后自动进行语法检查",
		Matcher: struct {
			PreToolUse  string
			PostToolUse string
		}{
			PostToolUse: "Write|Edit|MultiEdit",
		},
	}
}

func (p *Plugin) PostToolUse(arg types.PostToolUseInput) (*types.PostToolUseOutput, error) {
	var ret types.PostToolUseOutput
	filePath := arg.ToolInput.GetFilePath()
	if filePath == "" {
		return nil, nil
	}
	// 只处理Go文件
	if !strings.HasSuffix(filePath, ".go") {
		return nil, nil
	}
	msg, err := execCommand("gopls", "check", filePath)
	if err != nil {
		return nil, err
	}
	if len(msg) > 0 {
		return ret.Block(msg), nil
	}
	return nil, nil
}

func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if stdout.Len() > 0 {
		return stdout.String(), nil
	}
	if stderr.Len() > 0 {
		return stderr.String(), nil
	}
	if err != nil {
		return "", fmt.Errorf("exec command %s(%s) failed, %w", name, strings.Join(args, " "), err)
	}
	return "", nil
}
