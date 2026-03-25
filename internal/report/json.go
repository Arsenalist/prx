package report

import (
	"encoding/json"

	"github.com/Arsenalist/prx/internal/metrics"
)

// FormatJSON returns the analysis result as pretty-printed JSON.
func FormatJSON(result *metrics.AnalysisResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
