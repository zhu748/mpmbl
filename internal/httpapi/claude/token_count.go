package claude

import (
	"strings"

	"ds2api/internal/promptcompat"
	"ds2api/internal/util"
)

func countClaudeInputTokens(stdReq promptcompat.StandardRequest) int {
	promptText := stdReq.FinalPrompt
	if strings.TrimSpace(promptText) == "" {
		promptText = stdReq.FinalPrompt
	}
	return countClaudeInputTokensFromText(promptText, stdReq.ResolvedModel)
}

func countClaudeInputTokensFromText(promptText, model string) int {
	return util.CountPromptTokens(promptText, model)
}
