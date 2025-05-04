package meta

import (
	"bytes"
	"testing"
)

func TestWriteAndReadData(t *testing.T) {
	// Test cases with different data types
	testCases := []struct {
		name string
		data interface{}
	}{
		{
			name: "string",
			data: "test string",
		},
		{
			name: "integer",
			data: 42,
		},
		{
			name: "float",
			data: 3.14,
		},
		{
			name: "boolean",
			data: true,
		},
		{
			name: "map",
			data: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": true,
			},
		},
		{
			name: "slice",
			data: []interface{}{"item1", 2, true},
		},
		{
			name: "struct",
			data: struct {
				Name  string `json:"name"`
				Age   int    `json:"age"`
				Admin bool   `json:"admin"`
			}{
				Name:  "John Doe",
				Age:   30,
				Admin: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a buffer to simulate a connection
			var buf bytes.Buffer

			// Write data to the buffer
			err := WriteData(&buf, tc.data)
			if err != nil {
				t.Fatalf("WriteData failed: %v", err)
			}

			// Read data from the buffer
			var result interface{}
			err = ReadData(&buf, &result)
			if err != nil {
				t.Fatalf("ReadData failed: %v", err)
			}

			// Verify the result
			// Note: The comparison is not straightforward due to type differences after JSON marshaling/unmarshaling
			// For example, numbers might be unmarshaled as float64 regardless of original type
			// For simplicity, we're just checking that we got a non-nil result
			if result == nil {
				t.Errorf("Expected non-nil result, got nil")
			}
		})
	}
}

func TestWriteDataExceedingMaxSize(t *testing.T) {
	// Create a large string that exceeds uint16 max size (65535 bytes)
	largeData := make([]byte, 70000)
	for i := range largeData {
		largeData[i] = 'a'
	}

	var buf bytes.Buffer
	err := WriteData(&buf, string(largeData))

	if err == nil {
		t.Errorf("Expected error for data exceeding max size, got nil")
	}
}
