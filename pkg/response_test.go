package pkg

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNewResponse(t *testing.T) {
	r := NewResponse(201, map[string]string{"ok": "y"}, "created")
	if r.Code != 201 || r.Message != "created" {
		t.Fatalf("mismatch: %+v", r)
	}
	m := r.Data.(map[string]string)
	if m["ok"] != "y" {
		t.Fatalf("data mismatch: %+v", r.Data)
	}
}

func TestNewResponse_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
	}{
		{"OK", http.StatusOK, "success"},
		{"Created", http.StatusCreated, "resource created"},
		{"BadRequest", http.StatusBadRequest, "bad request"},
		{"NotFound", http.StatusNotFound, "not found"},
		{"InternalError", http.StatusInternalServerError, "internal error"},
		{"Custom", 299, "custom status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResponse(tt.code, nil, tt.message)
			if r.Code != tt.code {
				t.Fatalf("expected code %d, got %d", tt.code, r.Code)
			}
			if r.Message != tt.message {
				t.Fatalf("expected message %s, got %s", tt.message, r.Message)
			}
		})
	}
}

func TestNewResponse_DifferentDataTypes(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{"Nil", nil},
		{"String", "test string"},
		{"Int", 42},
		{"Float", 3.14},
		{"Bool", true},
		{"Slice", []string{"a", "b", "c"}},
		{"Map", map[string]interface{}{"key": "value", "count": 10}},
		{"Struct", struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{Name: "John", Age: 30}},
		{"EmptySlice", []string{}},
		{"EmptyMap", map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResponse(200, tt.data, "test")
			// For slices and maps, we need special comparison
			switch data := tt.data.(type) {
			case []string:
				if responseData, ok := r.Data.([]string); ok {
					if len(responseData) != len(data) {
						t.Fatalf("expected slice length %d, got %d", len(data), len(responseData))
					}
					for i, v := range data {
						if responseData[i] != v {
							t.Fatalf("expected slice[%d] = %s, got %s", i, v, responseData[i])
						}
					}
				} else {
					t.Fatalf("expected data to be []string")
				}
			case map[string]interface{}:
				if responseData, ok := r.Data.(map[string]interface{}); ok {
					if len(responseData) != len(data) {
						t.Fatalf("expected map length %d, got %d", len(data), len(responseData))
					}
					for k, v := range data {
						if responseData[k] != v {
							t.Fatalf("expected map[%s] = %v, got %v", k, v, responseData[k])
						}
					}
				} else {
					t.Fatalf("expected data to be map[string]interface{}")
				}
			case map[string]string:
				if responseData, ok := r.Data.(map[string]string); ok {
					if len(responseData) != len(data) {
						t.Fatalf("expected map length %d, got %d", len(data), len(responseData))
					}
					for k, v := range data {
						if responseData[k] != v {
							t.Fatalf("expected map[%s] = %s, got %s", k, v, responseData[k])
						}
					}
				} else {
					t.Fatalf("expected data to be map[string]string")
				}
			default:
				// For basic types, direct comparison is fine
				if r.Data != tt.data {
					t.Fatalf("expected data %v, got %v", tt.data, r.Data)
				}
			}
		})
	}
}

func TestNewResponse_EmptyMessage(t *testing.T) {
	r := NewResponse(200, "data", "")
	if r.Message != "" {
		t.Fatalf("expected empty message, got %s", r.Message)
	}
}

func TestNewResponse_ZeroCode(t *testing.T) {
	r := NewResponse(0, "data", "test")
	if r.Code != 0 {
		t.Fatalf("expected code 0, got %d", r.Code)
	}
}

func TestNewResponse_NegativeCode(t *testing.T) {
	r := NewResponse(-1, "data", "test")
	if r.Code != -1 {
		t.Fatalf("expected code -1, got %d", r.Code)
	}
}

func TestResponse_JSONSerialization(t *testing.T) {
	data := map[string]interface{}{
		"name":   "test",
		"count":  42,
		"active": true,
	}
	r := NewResponse(200, data, "success")

	jsonBytes, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled Response
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if unmarshaled.Code != r.Code {
		t.Fatalf("expected code %d, got %d", r.Code, unmarshaled.Code)
	}
	if unmarshaled.Message != r.Message {
		t.Fatalf("expected message %s, got %s", r.Message, unmarshaled.Message)
	}

	// Data will be unmarshaled as map[string]interface{}
	unmarshaledData := unmarshaled.Data.(map[string]interface{})
	originalData := data
	for key, expectedValue := range originalData {
		actualValue, ok := unmarshaledData[key]
		if !ok {
			t.Fatalf("key %s not found in unmarshaled data", key)
		}
		// JSON unmarshals numbers as float64, so we need special handling
		switch expectedValue := expectedValue.(type) {
		case int:
			if actualFloat, ok := actualValue.(float64); ok {
				if actualFloat != float64(expectedValue) {
					t.Fatalf("expected data[%s] = %v, got %v", key, expectedValue, actualValue)
				}
			} else {
				t.Fatalf("expected data[%s] to be float64 after JSON unmarshal, got %T", key, actualValue)
			}
		default:
			if actualValue != expectedValue {
				t.Fatalf("expected data[%s] = %v, got %v", key, expectedValue, actualValue)
			}
		}
	}
}

func TestResponse_JSONSerializationWithNilData(t *testing.T) {
	r := NewResponse(404, nil, "not found")

	jsonBytes, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled Response
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if unmarshaled.Code != 404 {
		t.Fatalf("expected code 404, got %d", unmarshaled.Code)
	}
	if unmarshaled.Message != "not found" {
		t.Fatalf("expected message 'not found', got %s", unmarshaled.Message)
	}
	if unmarshaled.Data != nil {
		t.Fatalf("expected nil data, got %v", unmarshaled.Data)
	}
}

func TestResponse_StructFields(t *testing.T) {
	r := NewResponse(201, "test data", "created successfully")

	// Test that all fields are accessible
	if r.Code == 0 {
		t.Fatalf("Code field should be accessible")
	}
	if r.Data == nil {
		t.Fatalf("Data field should be accessible")
	}
	if r.Message == "" {
		t.Fatalf("Message field should be accessible")
	}
}

func TestResponse_StructFieldTypes(t *testing.T) {
	r := NewResponse(500, []int{1, 2, 3}, "internal error")

	// Test field types
	var _ int = r.Code
	var _ string = r.Message
	var _ interface{} = r.Data

	// Test data type assertion
	if slice, ok := r.Data.([]int); ok {
		if len(slice) != 3 {
			t.Fatalf("expected slice length 3, got %d", len(slice))
		}
	} else {
		t.Fatalf("expected data to be []int")
	}
}

func TestResponse_ConcurrentAccess(t *testing.T) {
	r := NewResponse(200, map[string]string{"key": "value"}, "success")

	// Test concurrent read access (should be safe since we're not modifying)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			// Read fields concurrently
			_ = r.Code
			_ = r.Message
			_ = r.Data
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestResponse_LargeDataSimple(t *testing.T) {
	// Create simple large data for testing
	largeSlice := make([]int, 1000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	r := NewResponse(200, largeSlice, "large array")
	if r.Data == nil {
		t.Fatalf("expected large data to be stored")
	}

	storedSlice := r.Data.([]int)
	if len(storedSlice) != 1000 {
		t.Fatalf("expected 1000 elements, got %d", len(storedSlice))
	}
	if storedSlice[0] != 0 {
		t.Fatalf("expected first element 0, got %d", storedSlice[0])
	}
	if storedSlice[999] != 999 {
		t.Fatalf("expected last element 999, got %d", storedSlice[999])
	}
}
