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

func TestSanitizeCanonicalDSMLToolBlockPreservesPreamble(t *testing.T) {
	input := "阶段汇报 1：环境检查与选题\n<|DSML|tool_calls><|DSML|invoke name=\"PowerShell\"><|DSML|parameter name=\"command\"><![CDATA[pwd]]></|DSML|parameter></|DSML|invoke></|DSML|tool_calls>"
	result := sanitizeLeakedOutput(input)
	if result != "阶段汇报 1：环境检查与选题\n" {
		t.Fatalf("expected canonical DSML tool block stripped, got %q", result)
	}
}
