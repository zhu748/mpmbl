package sse

import (
	"fmt"
	"testing"
)

func TestDebugParserTypeTracking(t *testing.T) {
	lines := []string{
		`data: {"p":"response/fragments","o":"APPEND","v":[{"type":"THINK","content":"我们"}]}`,
		`data: {"p":"response/fragments/-1/content","v":"被"}`,
		`data: {"v":"要求"}`,
		`data: {"p":"response/fragments","o":"APPEND","v":[{"type":"RESPONSE","content":"答"}]}`,
		`data: {"p":"response/fragments/-1/content","v":"案"}`,
	}

	currentType := "text"
	for i, line := range lines {
		res := ParseDeepSeekContentLine([]byte(line), false, currentType)
		fmt.Printf("Line %d: Parts=%v NextType=%q\n", i+1, res.Parts, res.NextType)
		currentType = res.NextType
	}
}
