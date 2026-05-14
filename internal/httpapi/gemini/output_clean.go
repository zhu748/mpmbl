package gemini

import openaishared "ds2api/internal/httpapi/openai/shared"

//nolint:unused // retained for native Gemini output post-processing path.
func cleanVisibleOutput(text string, stripReferenceMarkers bool) string {
	return openaishared.CleanVisibleOutput(text, stripReferenceMarkers)
}
