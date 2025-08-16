package main

import (
	"claude-hooks/types"
	"fmt"
	"regexp"
)

var (
	// Patterns for example env files (allowed)
	exampleFilePattern1 = regexp.MustCompile(`(?i)\.env\.(example|sample|template|dist)$`)
	exampleFilePattern2 = regexp.MustCompile(`(?i)\.env\..*\.(example|sample|template|dist)$`)

	// Patterns for actual env files (blocked)
	envFilePattern1 = regexp.MustCompile(`(?i)\.env$`)        // .env
	envFilePattern2 = regexp.MustCompile(`(?i)\.env\.[^.]+$`) // .env.local, .env.production, etc.
)

type EnvPlugin struct {
	types.UnimplementedPlugin
}

func New() types.IPlugin {
	return &EnvPlugin{}
}

func (e *EnvPlugin) GetMetadata() types.PluginMetadata {
	return types.PluginMetadata{
		Description: "阻止读取.env文件",
		Matcher: struct {
			PreToolUse  string
			PostToolUse string
		}{
			"Read|Write|Edit|MultiEdit",
			"",
		},
	}
}

func (e *EnvPlugin) PreToolUse(arg types.ToolInput) (*types.PreToolUseOutput, error) {
	var ret types.PreToolUseOutput
	filePath := arg.GetFilePath()
	if filePath == "" {
		return nil, nil
	}

	// Check if this is an example env file (allowed)
	isExampleFile := exampleFilePattern1.MatchString(filePath) ||
		exampleFilePattern2.MatchString(filePath)

	if isExampleFile {
		return nil, nil // Allow example env files
	}

	// Check if this is a .env file or variant (blocked)
	isEnvFile := envFilePattern1.MatchString(filePath) ||
		envFilePattern2.MatchString(filePath)

	if isEnvFile {
		msg := fmt.Sprintf("Access to .env files is not allowed. File: %s", filePath)
		return ret.Approve(false, msg), nil
	}

	return nil, nil
}
