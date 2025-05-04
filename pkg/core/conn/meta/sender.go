package meta

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const maxDataSize = 65535 // Maximum size for uint16

// WriteData serializes data to JSON and writes it to the provided writer.
// Before sending the JSON data, it sends the length of the data as a uint16.
// The maximum length of data is limited by uint16 (65535 bytes).
func WriteData(w io.Writer, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	if len(jsonData) > maxDataSize {
		return fmt.Errorf("data length exceeds maximum allowed size (65535 bytes)")
	}

	lenBuf := make([]byte, 2) // uint16 is 2 bytes
	binary.BigEndian.PutUint16(lenBuf, uint16(len(jsonData)))

	if _, err := w.Write(lenBuf); err != nil {
		return fmt.Errorf("failed to write data length: %w", err)
	}

	// Write the JSON data
	if _, err := w.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write JSON data: %w", err)
	}

	return nil
}

// ReadData reads JSON data from the provided reader and deserializes it into the provided target.
// It first reads the length of the data as a uint16, then reads that many bytes and deserializes them.
// The target parameter should be a pointer to the type you want to deserialize into.
func ReadData(r io.Reader, target interface{}) error {
	lenBuf := make([]byte, 2) // uint16 is 2 bytes
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return fmt.Errorf("failed to read data length: %w", err)
	}

	dataLen := binary.BigEndian.Uint16(lenBuf)

	jsonData := make([]byte, dataLen)
	if _, err := io.ReadFull(r, jsonData); err != nil {
		return fmt.Errorf("failed to read JSON data: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	return nil
}
