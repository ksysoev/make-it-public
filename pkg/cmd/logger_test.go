package cmd

import (
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
				logLevel:   "info",
				textFormat: true,
				version:    "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "Valid log level with JSON format",
			arg: args{
				logLevel:   "debug",
				textFormat: false,
				version:    "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "Invalid log level",
			arg: args{
				logLevel:   "invalid",
				textFormat: true,
				version:    "1.0.0",
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
