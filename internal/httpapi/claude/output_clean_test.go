package claude

import "testing"

func TestCleanVisibleOutputStripsLeakedDSMLToolBlock(t *testing.T) {
	raw := "阶段汇报 1：环境检查与选题\n<|DSML|tool_calls><|DSML|invoke name=\"PowerShell\"><|DSML|parameter name=\"command\"><![CDATA[pwd]]></|DSML|parameter></|DSML|invoke></|DSML|tool_calls>"
	got := cleanVisibleOutput(raw, false)
	want := "阶段汇报 1：环境检查与选题\n"
	if got != want {
		t.Fatalf("expected leaked DSML block stripped, got %q want %q", got, want)
	}
}
