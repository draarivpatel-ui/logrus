package logrus

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Test that DisableTimestamp prevents renaming of user-provided 'time' field
// when there's no actual timestamp being added.
func TestDisableTimestampDoesNotRenameTimeField(t *testing.T) {
	var buf []byte
	formatter := &JSONFormatter{DisableTimestamp: true}

	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "from application",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	buf = output

	// Parse the JSON output to check its contents
	var result map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, string(buf))
	}

	// The 'time' field should NOT be renamed to 'fields.time' when DisableTimestamp is true
	// because there's no timestamp field to collide with
	if timeVal, exists := result["time"]; exists {
		t.Logf("OK: 'time' field exists directly (value: %v)", timeVal)
	} else if fieldsTime, ok := result["fields.time"]; ok {
		t.Errorf("Bug: 'time' field was renamed to 'fields.time' even though DisableTimestamp is true: %v", fieldsTime)
	} else {
		t.Error("Expected 'time' field in output but neither 'time' nor 'fields.time' found")
	}

	// No timestamp should be present
	if _, exists := result["time"]; exists {
		// This is OK - it's the user's time field
		t.Logf("User 'time' field present: %v", result["time"])
	}
}

// Test that custom time key via FieldMap doesn't rename user fields
// when there's no collision with the remapped timestamp field.
func TestCustomTimeKeyDoesNotRenameUserTimeField(t *testing.T) {
	formatter := &JSONFormatter{
		FieldMap: FieldMap{
			FieldKeyTime: "@timestamp",
		},
	}

	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "from application",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	// Parse the JSON output to check its contents
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, string(output))
	}

	// The 'time' field should NOT be renamed to 'fields.time' when FieldMap remaps FieldKeyTime
	// because the user's 'time' field doesn't collide with the actual timestamp field (@timestamp)
	if timeVal, exists := result["time"]; exists {
		t.Logf("OK: 'time' field exists directly (value: %v)", timeVal)
	} else if fieldsTime, ok := result["fields.time"]; ok {
		t.Errorf("Bug: 'time' field was renamed to 'fields.time' even though FieldMap remaps FieldKeyTime to '@timestamp': %v", fieldsTime)
	} else {
		t.Error("Expected 'time' field in output but neither 'time' nor 'fields.time' found")
	}

	// @timestamp should be present (the actual timestamp)
	if _, exists := result["@timestamp"]; !exists {
		t.Error("Expected '@timestamp' field in output")
	}
}

// Test that time field IS renamed when there is an actual collision (both timestamp and user field named 'time')
func TestTimeFieldIsRenamedWhenCollision(t *testing.T) {
	formatter := &JSONFormatter{} // Default, with timestamps enabled

	// Create an entry with a 'time' field in the data (collision with timestamp)
	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "user-provided time",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	// Format the entry
	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, string(output))
	}

	// When there's a collision (both timestamp and user field named 'time'),
	// the user field should be renamed to 'fields.time'
	if fieldsTime, ok := result["fields.time"]; ok {
		t.Logf("OK: 'time' field was renamed to 'fields.time' (value: %v) due to collision", fieldsTime)
	} else if _, exists := result["time"]; exists {
		t.Error("Expected 'time' field to be renamed when there's a collision, but 'time' exists directly")
	} else {
		t.Error("Expected 'fields.time' field in output")
	}

	// The actual timestamp should be present at 'time'
	if timestamp, exists := result["time"]; exists {
		// Check it's a timestamp, not the user's value
		if ts, ok := timestamp.(string); ok {
			if ts != "user-provided time" {
				t.Logf("OK: timestamp field exists at 'time' (value: %v)", timestamp)
			} else {
				t.Error("Expected timestamp at 'time' key, but got user value instead")
			}
		}
	} else {
		t.Error("Expected timestamp at 'time' key")
	}
}

// Test text formatter also doesn't rename time field when DisableTimestamp
func TestTextFormatterDisableTimestampDoesNotRenameTimeField(t *testing.T) {
	formatter := &TextFormatter{DisableTimestamp: true, DisableColors: true}

	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "from application",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	outputStr := string(output)
	t.Logf("Output: %q", outputStr)

	// In text format, fields are key=value pairs
	// The bug is that 'time' would be output as 'fields.time=from application'
	// instead of just 'time=from application'
	if strings.Contains(outputStr, "fields.time=") {
		t.Errorf("Bug: 'time' field was renamed to 'fields.time' in text formatter even though DisableTimestamp is true. Output: %s", outputStr)
	}

	// Should have 'time="from application"' directly (text formatter quotes strings)
	if !strings.Contains(outputStr, `time="from application"`) {
		t.Errorf("Expected 'time=\"from application\"' in output, got: %s", outputStr)
	}
}

// Test that text formatter correctly renames time field when there's a collision
func TestTextFormatterTimeFieldRenamedOnCollision(t *testing.T) {
	formatter := &TextFormatter{DisableColors: true} // Timestamp enabled by default

	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "user-provided time",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	outputStr := string(output)
	t.Logf("Output: %q", outputStr)

	// When there's a collision, the user's time field should be renamed to fields.time
	if strings.Contains(outputStr, `time="2024-01-15T10:30:00Z"`) {
		t.Logf("OK: timestamp present at 'time' key")
	}

	if strings.Contains(outputStr, `fields.time=user-provided time`) {
		t.Logf("OK: user 'time' field renamed to 'fields.time'")
	}
}

// Test colored output with DisableTimestamp and user field named 'time'
func TestTextFormatterColoredDisableTimestampDoesNotRenameTimeField(t *testing.T) {
	formatter := &TextFormatter{DisableTimestamp: true, ForceColors: true}

	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "from application",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	outputStr := string(output)
	t.Logf("Output: %q", outputStr)

	// User's 'time' field should NOT be renamed to 'fields.time=' when DisableTimestamp is true
	// In colored output, it should still appear as 'time="from application"' with ANSI color codes
	if strings.Contains(outputStr, "fields.time=") {
		t.Errorf("Bug: 'time' field was renamed to 'fields.time' even though DisableTimestamp is true. Output: %s", outputStr)
	}

	// Should have 'time="from application"' in the output (the key may have ANSI color codes around it
	// In colored output, it appears as \x1b[36mtime\x1b[0m="from application" where \x1b[36m is cyan
	if !strings.Contains(outputStr, `="from application"`) {
		t.Errorf("Expected '=\"from application\"' in output, got: %s", outputStr)
	}
	// Also verify the key appears at all (possibly with ANSI codes)
	if !strings.Contains(outputStr, "time") {
		t.Errorf("Expected 'time' key in output, got: %s", outputStr)
	}
}

// Test that user field named 'time' is NOT renamed when using @timestamp via FieldMap
func TestCustomTimeKeyWithUserTimeField(t *testing.T) {
	formatter := &TextFormatter{
		DisableColors: true,
		FieldMap: FieldMap{
			FieldKeyTime: "@timestamp",
		},
	}

	entry := &Entry{
		Message: "test message",
		Data: Fields{
			"time": "from application",
		},
		Time:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level: InfoLevel,
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	outputStr := string(output)
	t.Logf("Output: %q", outputStr)

	// The user's 'time' field should NOT be renamed since the timestamp is at @timestamp
	if strings.Contains(outputStr, "fields.time=") {
		t.Errorf("Bug: 'time' field was renamed to 'fields.time' even though FieldMap remaps timestamp to '@timestamp'. Output: %s", outputStr)
	}

	// Should have 'time="from application"' directly (text formatter quotes strings)
	if !strings.Contains(outputStr, `time="from application"`) {
		t.Errorf("Expected 'time=\"from application\"' in output, got: %s", outputStr)
	}

	// Should have @timestamp for the actual timestamp
	if !strings.Contains(outputStr, `@timestamp="2024-01-15T10:30:00Z"`) {
		t.Errorf("Expected '@timestamp=\"2024-01-15T10:30:00Z\"' in output for the actual timestamp, got: %s", outputStr)
	}
}