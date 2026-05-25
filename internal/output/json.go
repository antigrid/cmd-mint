package output

import (
	"bytes"
	"encoding/json"

	"cmd-mint/internal/model"
)

func RenderReportJSON(report model.Report) ([]byte, error) {
	safe := safeReport(report)
	if safe.SchemaVersion == "" {
		safe.SchemaVersion = model.ReportSchemaVersion
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(safe); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
