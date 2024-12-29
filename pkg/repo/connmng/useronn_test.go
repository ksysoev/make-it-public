package connmng

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewUserConnections(t *testing.T) {
	// Act
	userConns := NewUserConnections()

	// Assert
	assert.NotNil(t, userConns, "NewUserConnections should not return nil")
	assert.Equal(t, 0, len(userConns.connections), "Initial connection list should be empty")
}

func TestUserConnections_AddConnection(t *testing.T) {
	sameID := uuid.New()

	tests := []struct {
		name         string
		initialConns []uuid.UUID
		newConn      uuid.UUID
		expectedLen  int
		replace      bool
		closeError   bool
	}{
		{
			name:         "Add new connection successfully",
			initialConns: []uuid.UUID{uuid.New()},
			newConn:      uuid.New(),
			expectedLen:  2,
		},
		{
			name:         "Replace existing connection with same ID",
			initialConns: []uuid.UUID{sameID},
			newConn:      sameID,
			expectedLen:  1,
			replace:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewUserConnections()

			for _, id := range tt.initialConns {
				mockConn := NewMockServerConn(t)
				mockConn.EXPECT().ID().Return(id)
				mockConn.EXPECT().Close().Return(nil).Maybe()

				uc.connections = append(uc.connections, mockConn)
			}

			// Act
			mockConn := NewMockServerConn(t)
			mockConn.EXPECT().ID().Return(tt.newConn).Maybe()

			uc.AddConnection(mockConn)

			// Assert
			assert.Equal(t, tt.expectedLen, len(uc.connections), "Connection count mismatch")
		})
	}
}

func TestUserConnections_RemoveConnection(t *testing.T) {
	sameID := uuid.New()

	tests := []struct {
		name           string
		initialConns   []uuid.UUID
		removeConnID   uuid.UUID
		expectedLen    int
		expectToRemove bool
	}{
		{
			name:           "Remove existing connection successfully",
			initialConns:   []uuid.UUID{sameID},
			removeConnID:   sameID,
			expectedLen:    0,
			expectToRemove: true,
		},
		{
			name:           "Attempt to remove connection not in list",
			initialConns:   []uuid.UUID{uuid.New()},
			removeConnID:   uuid.New(),
			expectedLen:    1,
			expectToRemove: false,
		},
		{
			name:           "Remove from multiple connections successfully",
			initialConns:   []uuid.UUID{uuid.New(), sameID},
			removeConnID:   sameID,
			expectedLen:    1,
			expectToRemove: true,
		},
		{
			name:           "Remove connection from empty list",
			initialConns:   []uuid.UUID{},
			removeConnID:   uuid.New(),
			expectedLen:    0,
			expectToRemove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc := NewUserConnections()

			for _, id := range tt.initialConns {
				mockConn := NewMockServerConn(t)
				mockConn.EXPECT().ID().Return(id).Maybe()
				mockConn.EXPECT().Close().Return(nil).Maybe()
				uc.connections = append(uc.connections, mockConn)
			}

			mockConn := NewMockServerConn(t)
			mockConn.EXPECT().ID().Return(tt.removeConnID).Maybe()
			mockConn.EXPECT().Close().Return(nil).Maybe()

			// Act
			uc.RemoveConnection(tt.removeConnID)

			// Assert
			assert.Equal(t, tt.expectedLen, len(uc.connections), "Connection count mismatch")

			for _, conn := range uc.connections {
				assert.NotEqual(t, tt.removeConnID, conn.ID(), "Removed connection ID should not exist")
			}
		})
	}
}

func TestUserConnections_GetConn(t *testing.T) {
	sameID := uuid.New()
	sameID2 := uuid.New()

	tests := []struct {
		name           string
		initialConns   []uuid.UUID
		rotation       []uuid.UUID
		expectedConnID uuid.UUID
	}{
		{
			name:           "GetConn from empty connections",
			initialConns:   []uuid.UUID{},
			expectedConnID: uuid.Nil,
			rotation:       nil,
		},
		{
			name:           "GetConn with one connection",
			initialConns:   []uuid.UUID{sameID},
			expectedConnID: sameID,              // Replace with the same UUID used in initialConns
			rotation:       []uuid.UUID{sameID}, // Same for subsequent calls
		},
		{
			name:           "GetConn rotates through multiple connections",
			initialConns:   []uuid.UUID{sameID, sameID2}, // Replace with specific UUIDs
			expectedConnID: sameID,                       // First connection ID
			rotation:       []uuid.UUID{sameID2, sameID}, // Each ID in rotation order
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc := NewUserConnections()

			for _, id := range tt.initialConns {
				mockConn := NewMockServerConn(t)
				mockConn.EXPECT().ID().Return(id).Maybe()
				uc.AddConnection(mockConn)
			}

			// Act
			conn := uc.GetConn()

			// Assert
			if tt.expectedConnID == uuid.Nil {
				assert.Nil(t, conn, "Expected no connection but got one")
			} else {
				assert.NotNil(t, conn, "Expected connection but got none")
				assert.Equal(t, tt.expectedConnID, conn.ID(), "Unexpected connection ID")
			}

			// Check subsequent rotations, if applicable
			for i, expectedID := range tt.rotation {
				conn = uc.GetConn()
				assert.NotNil(t, conn, "Expected connection but got none during rotation")
				assert.Equal(t, expectedID, conn.ID(), "Unexpected connection ID in rotation at index %d", i)
			}
		})
	}
}

func TestUserConnections_Close(t *testing.T) {
	tests := []struct {
		name         string
		initialConns []uuid.UUID
	}{
		{
			name:         "Close empty connections",
			initialConns: []uuid.UUID{},
		},
		{
			name:         "Close multiple connections",
			initialConns: []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			uc := NewUserConnections()

			for _, id := range tt.initialConns {
				mockConn := NewMockServerConn(t)
				mockConn.EXPECT().ID().Return(id).Maybe()
				mockConn.EXPECT().Close().Return(nil)
				uc.AddConnection(mockConn)
			}

			// Act
			uc.Close()

			// Assert
			assert.Equal(t, 0, len(uc.connections), "Connections should be empty after closing")
		})
	}
}
