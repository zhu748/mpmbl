package toolstream

import "regexp"

// --- XML tool call support for the streaming sieve ---

//nolint:unused // kept as explicit tag inventory for future XML sieve refinements.
var xmlToolCallClosingTags = buildXMLToolCallClosingTags()

// xmlToolCallBlockPattern matches a complete canonical XML tool call block.
//
//nolint:unused // reserved for future fast-path XML block detection.
var xmlToolCallBlockPattern = regexp.MustCompile(`(?is)((?:<tool_calls\b|<\|dsml\|tool_calls\b|<\|dmsl\|tool_calls\b)[^>]*>\s*(?:.*?)\s*(?:</tool_calls>|</\|dsml\|tool_calls>|</\|dmsl\|tool_calls>))`)

// xmlToolTagsToDetect is the set of XML tag prefixes used by findToolSegmentStart.
var xmlToolTagsToDetect = buildXMLToolTagsToDetect()

func buildXMLToolCallClosingTags() []string {
	tags := []string{"</tool_calls>", "</｜tool_calls>", "</|tool_calls>"}
	for _, prefix := range []string{"dsml", "dmsl"} {
		tags = append(tags,
			"</|"+prefix+"|tool_calls>",
			"</|"+prefix+"tool_calls>",
			"</|"+prefix+" tool_calls>",
			"</"+prefix+"|tool_calls>",
			"</"+prefix+"tool_calls>",
			"</"+prefix+" tool_calls>",
		)
	}
	return tags
}

func buildXMLToolTagsToDetect() []string {
	var tags []string
	addTerminated := func(prefix string, terminators ...string) {
		for _, term := range terminators {
			tags = append(tags, prefix+term)
		}
	}
	addWrapper := func(prefix string) {
		addTerminated(prefix+"tool_calls", ">", "\n", " ")
	}
	addInvoke := func(prefix string) {
		addTerminated(prefix+"invoke", " ", "\n", "\t", "\r", ">")
	}
	for _, marker := range []string{"dsml", "dmsl"} {
		for _, prefix := range []string{
			"<|" + marker + "|",
			"<｜" + marker + "|",
			"<|" + marker,
			"<｜" + marker,
			"<|" + marker + " ",
			"<｜" + marker + " ",
			"<" + marker + "|",
			"<" + marker,
			"<" + marker + " ",
		} {
			addWrapper(prefix)
			addInvoke(prefix)
		}
	}
	for _, prefix := range []string{"<｜", "<|", "<"} {
		addWrapper(prefix)
		addInvoke(prefix)
	}
	return tags
}
