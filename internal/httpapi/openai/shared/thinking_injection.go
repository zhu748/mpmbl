package shared

import "ds2api/internal/promptcompat"

func ApplyThinkingInjection(store ConfigReader, stdReq promptcompat.StandardRequest) promptcompat.StandardRequest {
	if !stdReq.Thinking {
		return stdReq
	}
	enabled := stdReq.ForceThinkingInjection
	prompt := ""
	if store != nil {
		enabled = enabled || store.ThinkingInjectionEnabled()
		prompt = store.ThinkingInjectionPrompt()
	}
	if !enabled {
		return stdReq
	}
	messages, changed := promptcompat.AppendThinkingInjectionPromptToLatestUser(stdReq.Messages, prompt)
	if !changed {
		return stdReq
	}
	finalPrompt, toolNames := promptcompat.BuildOpenAIPrompt(messages, stdReq.ToolsRaw, "", stdReq.ToolChoice, stdReq.Thinking)
	if len(toolNames) == 0 && len(stdReq.ToolNames) > 0 {
		toolNames = stdReq.ToolNames
	}
	stdReq.Messages = messages
	stdReq.FinalPrompt = finalPrompt
	stdReq.ToolNames = toolNames
	return stdReq
}
