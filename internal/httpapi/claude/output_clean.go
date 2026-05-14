package claude

import openaishared "ds2api/internal/httpapi/openai/shared"

func cleanVisibleOutput(text string, stripReferenceMarkers bool) string {
	return openaishared.CleanVisibleOutput(text, stripReferenceMarkers)
}
