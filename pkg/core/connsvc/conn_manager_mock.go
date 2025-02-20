// Code generated by mockery v2.50.1. DO NOT EDIT.

//go:build !compile

package connsvc

import (
	context "context"
	net "net"

	mock "github.com/stretchr/testify/mock"

	proto "github.com/ksysoev/revdial/proto"

	uuid "github.com/google/uuid"
)

// MockConnManager is an autogenerated mock type for the ConnManager type
type MockConnManager struct {
	mock.Mock
}

type MockConnManager_Expecter struct {
	mock *mock.Mock
}

func (_m *MockConnManager) EXPECT() *MockConnManager_Expecter {
	return &MockConnManager_Expecter{mock: &_m.Mock}
}

// AddConnection provides a mock function with given fields: user, conn
func (_m *MockConnManager) AddConnection(user string, conn *proto.Server) {
	_m.Called(user, conn)
}

// MockConnManager_AddConnection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AddConnection'
type MockConnManager_AddConnection_Call struct {
	*mock.Call
}

// AddConnection is a helper method to define mock.On call
//   - user string
//   - conn *proto.Server
func (_e *MockConnManager_Expecter) AddConnection(user interface{}, conn interface{}) *MockConnManager_AddConnection_Call {
	return &MockConnManager_AddConnection_Call{Call: _e.mock.On("AddConnection", user, conn)}
}

func (_c *MockConnManager_AddConnection_Call) Run(run func(user string, conn *proto.Server)) *MockConnManager_AddConnection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(*proto.Server))
	})
	return _c
}

func (_c *MockConnManager_AddConnection_Call) Return() *MockConnManager_AddConnection_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockConnManager_AddConnection_Call) RunAndReturn(run func(string, *proto.Server)) *MockConnManager_AddConnection_Call {
	_c.Run(run)
	return _c
}

// RequestConnection provides a mock function with given fields: ctx, userID
func (_m *MockConnManager) RequestConnection(ctx context.Context, userID string) (chan net.Conn, error) {
	ret := _m.Called(ctx, userID)

	if len(ret) == 0 {
		panic("no return value specified for RequestConnection")
	}

	var r0 chan net.Conn
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (chan net.Conn, error)); ok {
		return rf(ctx, userID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) chan net.Conn); ok {
		r0 = rf(ctx, userID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan net.Conn)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, userID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockConnManager_RequestConnection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RequestConnection'
type MockConnManager_RequestConnection_Call struct {
	*mock.Call
}

// RequestConnection is a helper method to define mock.On call
//   - ctx context.Context
//   - userID string
func (_e *MockConnManager_Expecter) RequestConnection(ctx interface{}, userID interface{}) *MockConnManager_RequestConnection_Call {
	return &MockConnManager_RequestConnection_Call{Call: _e.mock.On("RequestConnection", ctx, userID)}
}

func (_c *MockConnManager_RequestConnection_Call) Run(run func(ctx context.Context, userID string)) *MockConnManager_RequestConnection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockConnManager_RequestConnection_Call) Return(_a0 chan net.Conn, _a1 error) *MockConnManager_RequestConnection_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockConnManager_RequestConnection_Call) RunAndReturn(run func(context.Context, string) (chan net.Conn, error)) *MockConnManager_RequestConnection_Call {
	_c.Call.Return(run)
	return _c
}

// ResolveRequest provides a mock function with given fields: id, conn
func (_m *MockConnManager) ResolveRequest(id uuid.UUID, conn net.Conn) {
	_m.Called(id, conn)
}

// MockConnManager_ResolveRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ResolveRequest'
type MockConnManager_ResolveRequest_Call struct {
	*mock.Call
}

// ResolveRequest is a helper method to define mock.On call
//   - id uuid.UUID
//   - conn net.Conn
func (_e *MockConnManager_Expecter) ResolveRequest(id interface{}, conn interface{}) *MockConnManager_ResolveRequest_Call {
	return &MockConnManager_ResolveRequest_Call{Call: _e.mock.On("ResolveRequest", id, conn)}
}

func (_c *MockConnManager_ResolveRequest_Call) Run(run func(id uuid.UUID, conn net.Conn)) *MockConnManager_ResolveRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(uuid.UUID), args[1].(net.Conn))
	})
	return _c
}

func (_c *MockConnManager_ResolveRequest_Call) Return() *MockConnManager_ResolveRequest_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockConnManager_ResolveRequest_Call) RunAndReturn(run func(uuid.UUID, net.Conn)) *MockConnManager_ResolveRequest_Call {
	_c.Run(run)
	return _c
}

// NewMockConnManager creates a new instance of MockConnManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockConnManager(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockConnManager {
	mock := &MockConnManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
