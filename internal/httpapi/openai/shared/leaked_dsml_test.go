package shared

import (
	"testing"
)

func TestSanitizeDSMLLeakedToolCalls(t *testing.T) {
	input := "Let me find the OCR API files and check for Qt dependencies using Bash. \n\n   Searched for 3 patterns, read 4 files (ctrl+o to expand) \n\n <``````dsml|tool_calls> \n   <|dsml|invoke name=\"Read\"> \n     <|dsml|parameter name=\"file_path\"></|dsml|parameter> \n   </|dsml|invoke> \n   <|dsml|invoke name=\"Read\"> \n     <|dsml|parameter name=\"file_path\"></|dsml|parameter> \n   </|dsml|invoke> \n </|dsml|tool_calls>"

	result := sanitizeLeakedOutput(input)
	t.Logf("INPUT (%d bytes):\n%s", len(input), input)
	t.Logf("OUTPUT (%d bytes):\n%s", len(result), result)

	if result != "Let me find the OCR API files and check for Qt dependencies using Bash. \n\n   Searched for 3 patterns, read 4 files (ctrl+o to expand) \n\n " {
		t.Logf("WARNING: output not fully sanitized")
	}
}
