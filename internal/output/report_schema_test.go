package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cmd-mint/internal/model"
)

func TestReportJSONConformsToPublishedSchema(t *testing.T) {
	reports := map[string]model.Report{
		"sample report": reportWithSchemaEdgeFields(),
		"minimal report": func() model.Report {
			report := model.NewReport(time.Date(2026, 6, 1, 14, 30, 22, 0, time.UTC))
			report.Summary = model.AggregateSummary{}
			report.Exclusions = model.ExclusionSummary{}
			return report
		}(),
	}

	schema := readReportSchema(t)
	for name, report := range reports {
		t.Run(name, func(t *testing.T) {
			data, err := RenderReportJSON(report)
			if err != nil {
				t.Fatalf("RenderReportJSON() error = %v", err)
			}
			value := decodeJSONValue(t, data)
			if err := validateJSONValue(schema, schema, value, "$"); err != nil {
				t.Fatalf("report JSON does not match docs/report.schema.json: %v\n%s", err, data)
			}
		})
	}
}

func TestReportJSONSchemaVersionMatchesModelConstant(t *testing.T) {
	schema := readReportSchema(t)
	properties := objectAt(t, schema, "properties")
	schemaVersion := objectAt(t, properties, "schema_version")

	got, ok := schemaVersion["const"].(string)
	if !ok {
		t.Fatalf("schema_version const missing or non-string: %#v", schemaVersion["const"])
	}
	if got != model.ReportSchemaVersion {
		t.Fatalf("schema_version const = %q, want %q", got, model.ReportSchemaVersion)
	}
}

func reportWithSchemaEdgeFields() model.Report {
	report := sampleReport()
	report.AliasSuggestions[0].AliasFileEligible = true
	report.AliasSuggestions[0].ExclusionReasons = []model.ExclusionReason{model.ExclusionLowSavings}
	report.Exclusions.MalformedCommandCount = 1
	report.Warnings = []model.Warning{
		{
			Code:       "source_parse_warning",
			Message:    "history source had malformed entries",
			SourceFile: "/home/alex/.zsh_history",
		},
	}
	return report
}

func readReportSchema(t *testing.T) map[string]any {
	t.Helper()

	path := filepath.Join("..", "..", "docs", "report.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	schema := decodeJSONValue(t, data)
	object, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("schema root is %T, want object", schema)
	}
	return object
}

func decodeJSONValue(t *testing.T, data []byte) any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("Decode JSON error = %v\n%s", err, data)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("Decode trailing JSON = %v, want EOF\n%s", err, data)
	}
	return value
}

func objectAt(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := object[key]
	if !ok {
		t.Fatalf("missing object key %q in %#v", key, object)
	}
	child, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("object key %q is %T, want object", key, value)
	}
	return child
}

func validateJSONValue(root map[string]any, schema map[string]any, value any, path string) error {
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := resolveLocalRef(root, ref)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return validateJSONValue(root, resolved, value, path)
	}

	if want, ok := schema["const"]; ok && !jsonValuesEqual(value, want) {
		return fmt.Errorf("%s: got %#v, want const %#v", path, value, want)
	}
	if values, ok := schema["enum"].([]any); ok && !valueInEnum(value, values) {
		return fmt.Errorf("%s: got %#v, want one of %#v", path, value, values)
	}

	if typeName, ok := schema["type"].(string); ok {
		if err := validateJSONType(typeName, value, path); err != nil {
			return err
		}
	}
	if format, ok := schema["format"].(string); ok {
		if err := validateJSONFormat(format, value, path); err != nil {
			return err
		}
	}
	if minimum, ok := schema["minimum"]; ok {
		if err := validateMinimum(minimum, value, path); err != nil {
			return err
		}
	}

	switch typed := value.(type) {
	case map[string]any:
		if err := validateJSONObject(root, schema, typed, path); err != nil {
			return err
		}
	case []any:
		if err := validateJSONArray(root, schema, typed, path); err != nil {
			return err
		}
	}

	return nil
}

func resolveLocalRef(root map[string]any, ref string) (map[string]any, error) {
	const prefix = "#/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported ref %q", ref)
	}

	var current any = root
	for _, part := range strings.Split(strings.TrimPrefix(ref, prefix), "/") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ref %q crosses non-object at %q", ref, part)
		}
		next, ok := object[part]
		if !ok {
			return nil, fmt.Errorf("ref %q missing part %q", ref, part)
		}
		current = next
	}

	object, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("ref %q resolves to %T, want object", ref, current)
	}
	return object, nil
}

func validateJSONType(typeName string, value any, path string) error {
	switch typeName {
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s: got %T, want object", path, value)
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%s: got %T, want array", path, value)
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: got %T, want string", path, value)
		}
	case "integer":
		if !isJSONInteger(value) {
			return fmt.Errorf("%s: got %#v, want integer", path, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: got %T, want boolean", path, value)
		}
	default:
		return fmt.Errorf("%s: unsupported schema type %q", path, typeName)
	}
	return nil
}

func validateJSONFormat(format string, value any, path string) error {
	switch format {
	case "date-time":
		text, ok := value.(string)
		if !ok {
			return nil
		}
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			return fmt.Errorf("%s: got %q, want RFC3339 date-time", path, text)
		}
	default:
		return fmt.Errorf("%s: unsupported schema format %q", path, format)
	}
	return nil
}

func validateMinimum(minimum any, value any, path string) error {
	limit, ok := jsonInteger(minimum)
	if !ok {
		return fmt.Errorf("%s: schema minimum is %#v, want integer", path, minimum)
	}
	got, ok := jsonInteger(value)
	if !ok {
		return nil
	}
	if got < limit {
		return fmt.Errorf("%s: got %d, want >= %d", path, got, limit)
	}
	return nil
}

func validateJSONObject(root map[string]any, schema map[string]any, value map[string]any, path string) error {
	properties, _ := schema["properties"].(map[string]any)
	required, err := stringSlice(schema["required"])
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, key := range required {
		if _, ok := value[key]; !ok {
			return fmt.Errorf("%s: missing required property %q", path, key)
		}
	}

	propertyNames, hasPropertyNames := schema["propertyNames"].(map[string]any)
	additionalProperties, hasAdditionalProperties := schema["additionalProperties"]
	for key, childValue := range value {
		if hasPropertyNames {
			if err := validateJSONValue(root, propertyNames, key, path+"."+key+" property name"); err != nil {
				return err
			}
		}
		if childSchemaAny, ok := properties[key]; ok {
			childSchema, ok := childSchemaAny.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.%s: property schema is %T, want object", path, key, childSchemaAny)
			}
			if err := validateJSONValue(root, childSchema, childValue, path+"."+key); err != nil {
				return err
			}
			continue
		}
		if !hasAdditionalProperties {
			continue
		}
		switch additional := additionalProperties.(type) {
		case bool:
			if !additional {
				return fmt.Errorf("%s: unexpected property %q", path, key)
			}
		case map[string]any:
			if err := validateJSONValue(root, additional, childValue, path+"."+key); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: additionalProperties is %T, want bool or object", path, additionalProperties)
		}
	}
	return nil
}

func validateJSONArray(root map[string]any, schema map[string]any, value []any, path string) error {
	items, ok := schema["items"].(map[string]any)
	if !ok {
		return nil
	}
	for index, item := range value {
		if err := validateJSONValue(root, items, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
			return err
		}
	}
	return nil
}

func stringSlice(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("required is %T, want array", value)
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("required item is %T, want string", item)
		}
		result = append(result, text)
	}
	return result, nil
}

func valueInEnum(value any, values []any) bool {
	for _, candidate := range values {
		if jsonValuesEqual(value, candidate) {
			return true
		}
	}
	return false
}

func jsonValuesEqual(left any, right any) bool {
	leftInt, leftOK := jsonInteger(left)
	rightInt, rightOK := jsonInteger(right)
	if leftOK && rightOK {
		return leftInt == rightInt
	}
	return left == right
}

func isJSONInteger(value any) bool {
	_, ok := jsonInteger(value)
	return ok
}

func jsonInteger(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		integer, err := strconv.ParseInt(typed.String(), 10, 64)
		return integer, err == nil
	case float64:
		integer := int64(typed)
		return integer, float64(integer) == typed
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	default:
		return 0, false
	}
}
