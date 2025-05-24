package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_SetEndpointGenerator(t *testing.T) {
	svc := New(nil, nil)
	expectedEndpoint := "generated-endpoint"
	generator := func(s string) (string, error) {
		return expectedEndpoint, nil
	}

	svc.SetEndpointGenerator(generator)

	result, err := svc.endpointGenerator("input")

	require.NoError(t, err)
	assert.Equal(t, expectedEndpoint, result)
}
