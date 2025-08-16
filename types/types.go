package types

import (
	"fmt"
	"os"
)

const (
	// ExitCodeSuccess 成功，会将stdout的内容显示给用户
	ExitCodeSuccess = 0
	// ExitCodeBlockingError 阻塞性错误，stderr的内容会显示给claude，以供处理
	// 行为:
	// - PreToolUse 阻止工具调用，向claude显示错误
	// - PostToolUse 像claude显示错误（Tool已经运行完毕)
	// - Notification 不适用
	// - Stop 阻止停止，向claude显示错误
	ExitCodeBlockingError = 2
	// ExitCodeError 其他错误，stderr显示给用户，不影响claude
	ExitCodeError = 1
)

type BaseHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	HookEventName  string `json:"hook_event_name"`
}

type ToolInput struct {
	BaseHookInput
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

func (t *ToolInput) GetFilePath() string {
	filePath, ok := t.ToolInput["file_path"].(string)
	if !ok {
		return ""
	}
	return filePath
}

type PostToolUseInput struct {
	ToolInput
	ToolResponse map[string]any `json:"tool_response"`
}

type NotificationInput struct {
	BaseHookInput
	Message string `json:"message"`
}

type StopInput struct {
	BaseHookInput
	StopHookActive bool `json:"stop_hook_active"`
}

type SubagentStopInput struct {
	BaseHookInput
	StopHookActive bool `json:"stop_hook_active"`
}

type BaseHookOutput struct {
	// Hook执行后，claude是否继续（默认为true)
	// 当continue为false，claude在hooks运行后停止处理
	// - PreToolUse, 这与decision: block不同，block只是阻止本次工具调用，continue:false是停止claude工作
	// - PostToolUse, 这与decision: block不同，block只是给claude提供错误，continue:false是停止claude工作
	// - 其他所有情况下，continue:false优先于任何block的输出
	Continue *bool `json:"continue,omitempty"`
	// 当continue为false时，显示给用户的消息（不显示给claude)
	StopReason string `json:"stopReason,omitempty"`
	// 从transcript模式中隐藏stdout (默认为false)
	SuppressOutput bool `json:"suppressOutput,omitempty"`
}

func (o *BaseHookOutput) Stop(userMessage string) {
	if o.Continue == nil {
		o.Continue = ptr(true)
	}
	o.StopReason = userMessage
}

func (o *BaseHookOutput) IgnoreStdout() {
	o.SuppressOutput = true
}

type DecisionOutput struct {
	BaseHookOutput
	Decision *string `json:"decision,omitzero"` // approve 或 block 或者没有，没有将进入现有决策中
	Reason   *string `json:"reason,omitzero"`
}

func ptr[T any](input T) *T {
	return &input
}

// PreToolUseOutput
//
//	approve: reason显示给用户✅
//	block: reason显示给claude
//	其他： 现有的决策流程
type PreToolUseOutput DecisionOutput

// PostToolUseOutput
// 默认值: 不执行任何操作，reason被忽略✅
// Block(): 用reason提示claude
type PostToolUseOutput DecisionOutput

// StopOutput
// 默认值: 允许claude停止，reason被忽略✅
// NotAllowed()：不允许claude停止，并提供信息给claude以供继续
type StopOutput DecisionOutput

func (o *PreToolUseOutput) Default() {
	if o.Continue == nil {
		o.Continue = ptr(true)
	}
	o.Approve(true, "")
}

// Approve
//
//	approve: reason显示给用户
//	block: reason显示给claude
//	其他： 进入现有的决策流程
func (o *PreToolUseOutput) Approve(approved bool, msg ...string) *PreToolUseOutput {
	if approved {
		o.Decision = ptr("approve")
		if len(msg) > 0 {
			o.Reason = ptr(msg[0])
		}
	} else {
		o.Decision = ptr("block")
		if len(msg) > 0 {
			o.Reason = ptr(msg[0])
		} else {
			o.Reason = ptr("rejected")
		}
	}
	return o
}

func (o *PostToolUseOutput) Default() {
	if o.Continue == nil {
		o.Continue = ptr(true)
	}
}
func (o *PostToolUseOutput) Block(msg string) *PostToolUseOutput {
	o.Decision = ptr("block")
	o.Reason = ptr(msg)
	return o
}

func (o *StopOutput) Default() {
	if o.Continue == nil {
		o.Continue = ptr(true)
	}
}

// NotAllowed
// 默认值: 允许claude停止，reason被忽略
// 否则：不允许claude停止，并提供信息给claude以供继续
func (o *StopOutput) NotAllowed(msg string) {
	// 不允许claude停止，必须提供消息
	o.Decision = ptr("block")
	o.Reason = ptr(msg)
}

type Result struct {
	Code  int
	Error string
	Data  string
}

func NewSuccess(data string) Result {
	return Result{
		Code: ExitCodeSuccess,
		Data: data,
	}
}

func NewError(msg string) Result {
	return Result{
		Code:  ExitCodeError,
		Error: msg,
	}
}

func (r Result) IsSuccess() bool {
	return r.Code == ExitCodeSuccess
}

func (r Result) ExitWithMessage() {
	if r.IsSuccess() {
		if len(r.Data) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, r.Data)
		}
	} else {
		if len(r.Error) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, r.Error)
		}
	}
	os.Exit(r.Code)
}
