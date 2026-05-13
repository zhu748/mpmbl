package toolcall

import (
	"fmt"
	"testing"
)

func TestDSMLPipeVariant(t *testing.T) {
	input := `<DSML|tool_calls>
  <DSML|invoke name="Grep">
    <DSML|parameter name="head_limit" value="3"/>
    <DSML|parameter name="pattern" value="test"/>
  </DSML|invoke>
</DSML|tool_calls>`

	// Check canonicalization
	canon := canonicalizeToolCallCandidateSpans(input)
	fmt.Printf("After canonicalize: %q\n\n", canon)

	// Check what the scan-tag loop in rewrite produces
	for i := 0; i < len(input); {
		tag, ok := scanToolMarkupTagAt(input, i)
		if !ok {
			i++
			continue
		}
		fmt.Printf("Tag at %d: name=%q start=%d end=%d nameStart=%d nameEnd=%d dsmlLike=%v\n",
			i, tag.Name, tag.Start, tag.End, tag.NameStart, tag.NameEnd, tag.DSMLLike)
		fmt.Printf("  Tag text: %q\n", input[tag.Start:tag.End+1])
		fmt.Printf("  NameEnd..End: %q\n", input[tag.NameEnd:tag.End+1])
		i = tag.End + 1
	}
}
