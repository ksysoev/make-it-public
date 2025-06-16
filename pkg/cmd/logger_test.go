package cmd

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name    string
		arg     args
		wantErr bool
	}{
		{
			name: "Valid log level with text format",
			arg: args{
				LogLevel:   "info",
				TextFormat: true,
				Version:    "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "Valid log level with JSON format",
			arg: args{
				LogLevel:   "debug",
				TextFormat: false,
				Version:    "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "Invalid log level",
			arg: args{
				LogLevel:   "invalid",
				TextFormat: true,
				Version:    "1.0.0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.arg
			err := initLogger(&args)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateReplacer(t *testing.T) {
	tests := []struct {
		name        string
		arg         args
		inputGroup  []string
		inputAttr   slog.Attr
		expected    slog.Attr
		expectedNil bool
	}{
		{
			name: "Non-interactive mode",
			arg: args{
				Interactive: false,
			},
			expectedNil: true,
		},
		{
			name: "Interactive mode - key 'time'",
			arg: args{
				Interactive: true,
			},
			inputAttr: slog.Attr{
				Key: "time",
			},
			expected: slog.Attr{},
		},
		{
			name: "Interactive mode - key 'app'",
			arg: args{
				Interactive: true,
			},
			inputAttr: slog.Attr{
				Key: "app",
			},
			expected: slog.Attr{},
		},
		{
			name: "Interactive mode - key 'level'",
			arg: args{
				Interactive: true,
			},
			inputAttr: slog.Attr{
				Key: "level",
			},
			expected: slog.Attr{},
		},
		{
			name: "Interactive mode - other key",
			arg: args{
				Interactive: true,
			},
			inputAttr: slog.String("customKey", "customValue"),
			expected:  slog.String("customKey", "customValue"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replacer := createReplacer(&tt.arg)
			if tt.expectedNil {
				assert.Nil(t, replacer)
				return
			}
			assert.NotNil(t, replacer)
			output := replacer(tt.inputGroup, tt.inputAttr)
			assert.Equal(t, tt.expected, output)
		})
	}
}
