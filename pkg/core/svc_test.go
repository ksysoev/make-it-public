package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_SetEndpointGenerator(t *testing.T) {
	svc := New(nil, nil, nil)
	expectedEndpoint := "generated-endpoint"
	generator := func(_ string) (string, error) {
		return expectedEndpoint, nil
	}

	svc.SetEndpointGenerator(generator)

	result, err := svc.endpointGenerator("input")

	require.NoError(t, err)
	assert.Equal(t, expectedEndpoint, result)
}

func TestService_CheckHealth(t *testing.T) {
	repo := NewMockAuthRepo(t)
	repo.EXPECT().CheckHealth(mock.Anything).Return(assert.AnError)

	svc := New(nil, nil, repo)

	err := svc.CheckHealth(t.Context())

	assert.ErrorIs(t, err, assert.AnError)
}
