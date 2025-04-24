package jsonrpc2_test

import (
	"context"
	"testing"

	"github.com/shawn-hurley/jsonrpc2"
)

type testCancelHandler struct {
}

func (t *testCancelHandler) Handle(ctx context.Context, params jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
	return nil, nil
}

func testCancelHandlerFunc(ctx context.Context, params jsonrpc2.Parameters) (any, *jsonrpc2.JsonRpcError) {
	return nil, nil
}

func TestRouterCallHandler(t *testing.T) {

	testCases := []struct {
		Name            string
		handler         jsonrpc2.CallHandler
		path            string
		shouldErrRoute  bool
		testPath        string
		shouldErrHandle bool
	}{
		{
			Name:     "full_path",
			handler:  &testCallHandler{},
			path:     "test_call",
			testPath: "test_call",
		},
		{
			Name:           "method_not_found",
			handler:        &testCallHandler{},
			path:           "test_call",
			testPath:       "not_found",
			shouldErrRoute: true,
		},
		{
			Name:     "handle_partial",
			handler:  &testCallHandler{},
			path:     "notification/*",
			testPath: "notification/change_test",
		},
		{
			Name:    "handle_partial_not_found",
			handler: &testCallHandler{},
			// Not path is notifications and we have notification
			path:           "notifications/*",
			testPath:       "notification/change_test",
			shouldErrRoute: true,
		},
		{
			Name:     "base_star_always_match",
			handler:  &testCallHandler{},
			path:     "*",
			testPath: "notification/change_test",
		},
		{
			Name:            "invalid_partial",
			handler:         &testCallHandler{},
			path:            "*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_middle",
			handler:         &testCallHandler{},
			path:            "test/*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_multiple",
			handler:         &testCallHandler{},
			path:            "test/*/change_test*testing",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r := jsonrpc2.NewRouter()
			err := r.HandleCall(tc.path, tc.handler, &jsonrpc2.TypedParameters[smallArgs]{})
			if err != nil && !tc.shouldErrHandle {
				t.Log("error trying to handle call")
				t.Fail()
				return
			}
			if err == nil && tc.shouldErrHandle {
				t.Log("should error trying to handle call")
				t.Fail()
				return
			}
			if err != nil {
				return
			}
			_, err = r.RouteCall(tc.testPath)
			if err != nil && !tc.shouldErrRoute {
				t.Log("error not routing", "err", err)
				t.Fail()
			}
			if err == nil && tc.shouldErrRoute {
				t.Log("did not error and we should have")
				t.Fail()
			}
		})
	}

}

func TestRouterCallHandlerFunc(t *testing.T) {

	testCases := []struct {
		Name            string
		handler         jsonrpc2.CallHandlerFunc
		path            string
		shouldErrRoute  bool
		testPath        string
		shouldErrHandle bool
	}{
		{
			Name:     "full_path",
			handler:  testCallHandlerFunc,
			path:     "test_call",
			testPath: "test_call",
		},
		{
			Name:           "method_not_found",
			handler:        testCallHandlerFunc,
			path:           "test_call",
			testPath:       "not_found",
			shouldErrRoute: true,
		},
		{
			Name:     "handle_partial",
			handler:  testCallHandlerFunc,
			path:     "notification/*",
			testPath: "notification/change_test",
		},
		{
			Name:    "handle_partial_not_found",
			handler: testCallHandlerFunc,
			// Not path is notifications and we have notification
			path:           "notifications/*",
			testPath:       "notification/change_test",
			shouldErrRoute: true,
		},
		{
			Name:     "base_star_always_match",
			handler:  testCallHandlerFunc,
			path:     "*",
			testPath: "notification/change_test",
		},
		{
			Name:            "invalid_partial",
			handler:         testCallHandlerFunc,
			path:            "*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_middle",
			handler:         testCallHandlerFunc,
			path:            "test/*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_multiple",
			handler:         testCallHandlerFunc,
			path:            "test/*/change_test*testing",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r := jsonrpc2.NewRouter()
			err := r.HandleCallFunc(tc.path, tc.handler, &jsonrpc2.TypedParameters[smallArgs]{})
			if err != nil && !tc.shouldErrHandle {
				t.Log("error trying to handle call")
				t.Fail()
				return
			}
			if err == nil && tc.shouldErrHandle {
				t.Log("should error trying to handle call")
				t.Fail()
				return
			}
			if err != nil {
				return
			}
			_, err = r.RouteCall(tc.testPath)
			if err != nil && !tc.shouldErrRoute {
				t.Log("error not routing", "err", err)
				t.Fail()
			}
			if err == nil && tc.shouldErrRoute {
				t.Log("did not error and we should have")
				t.Fail()
			}
		})
	}

}

func TestRouterNotifactionHandler(t *testing.T) {

	testCases := []struct {
		Name            string
		handler         jsonrpc2.NotificationHandler
		path            string
		shouldErrRoute  bool
		testPath        string
		shouldErrHandle bool
	}{
		{
			Name:     "full_path",
			handler:  &testNotificationHandler{},
			path:     "test_call",
			testPath: "test_call",
		},
		{
			Name:           "method_not_found",
			handler:        &testNotificationHandler{},
			path:           "test_call",
			testPath:       "not_found",
			shouldErrRoute: true,
		},
		{
			Name:     "handle_partial",
			handler:  &testNotificationHandler{},
			path:     "notification/*",
			testPath: "notification/change_test",
		},
		{
			Name:    "handle_partial_not_found",
			handler: &testNotificationHandler{},
			// Not path is notifications and we have notification
			path:           "notifications/*",
			testPath:       "notification/change_test",
			shouldErrRoute: true,
		},
		{
			Name:     "base_star_always_match",
			handler:  &testNotificationHandler{},
			path:     "*",
			testPath: "notification/change_test",
		},
		{
			Name:            "invalid_partial",
			handler:         &testNotificationHandler{},
			path:            "*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_middle",
			handler:         &testNotificationHandler{},
			path:            "test/*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_multiple",
			handler:         &testNotificationHandler{},
			path:            "test/*/change_test*testing",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r := jsonrpc2.NewRouter()
			err := r.HandleNotification(tc.path, tc.handler, &jsonrpc2.TypedParameters[smallArgs]{})
			if err != nil && !tc.shouldErrHandle {
				t.Log("error trying to handle call")
				t.Fail()
				return
			}
			if err == nil && tc.shouldErrHandle {
				t.Log("should error trying to handle call")
				t.Fail()
				return
			}
			if err != nil {
				return
			}
			_, err = r.RouteNotifaction(tc.testPath)
			if err != nil && !tc.shouldErrRoute {
				t.Log("error not routing", "err", err)
				t.Fail()
			}
			if err == nil && tc.shouldErrRoute {
				t.Log("did not error and we should have")
				t.Fail()
			}
		})
	}

}
func TestRouterNotifactionHandlerFunc(t *testing.T) {

	testCases := []struct {
		Name            string
		handler         jsonrpc2.NotificationHandlerFunc
		path            string
		shouldErrRoute  bool
		testPath        string
		shouldErrHandle bool
	}{
		{
			Name:     "full_path",
			handler:  NotifyFuncTest,
			path:     "test_call",
			testPath: "test_call",
		},
		{
			Name:           "method_not_found",
			handler:        NotifyFuncTest,
			path:           "test_call",
			testPath:       "not_found",
			shouldErrRoute: true,
		},
		{
			Name:     "handle_partial",
			handler:  NotifyFuncTest,
			path:     "notification/*",
			testPath: "notification/change_test",
		},
		{
			Name:    "handle_partial_not_found",
			handler: NotifyFuncTest,
			// Not path is notifications and we have notification
			path:           "notifications/*",
			testPath:       "notification/change_test",
			shouldErrRoute: true,
		},
		{
			Name:     "base_star_always_match",
			handler:  NotifyFuncTest,
			path:     "*",
			testPath: "notification/change_test",
		},
		{
			Name:            "invalid_partial",
			handler:         NotifyFuncTest,
			path:            "*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_middle",
			handler:         NotifyFuncTest,
			path:            "test/*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_multiple",
			handler:         NotifyFuncTest,
			path:            "test/*/change_test*testing",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r := jsonrpc2.NewRouter()
			err := r.HandleNotificationFunc(tc.path, tc.handler, &jsonrpc2.TypedParameters[smallArgs]{})
			if err != nil && !tc.shouldErrHandle {
				t.Log("error trying to handle call")
				t.Fail()
				return
			}
			if err == nil && tc.shouldErrHandle {
				t.Log("should error trying to handle call")
				t.Fail()
				return
			}
			if err != nil {
				return
			}
			_, err = r.RouteNotifaction(tc.testPath)
			if err != nil && !tc.shouldErrRoute {
				t.Log("error not routing", "err", err)
				t.Fail()
			}
			if err == nil && tc.shouldErrRoute {
				t.Log("did not error and we should have")
				t.Fail()
			}
		})
	}
}
func TestRouterCancelHandler(t *testing.T) {

	testCases := []struct {
		Name            string
		handler         jsonrpc2.CancelHandler
		path            string
		shouldRoute     bool
		testPath        string
		shouldErrHandle bool
	}{
		{
			Name:     "full_path",
			handler:  &testCancelHandler{},
			path:     "test_call",
			testPath: "test_call",
		},
		{
			Name:        "method_not_found",
			handler:     &testCancelHandler{},
			path:        "test_call",
			testPath:    "not_found",
			shouldRoute: true,
		},
		{
			Name:     "handle_partial",
			handler:  &testCancelHandler{},
			path:     "notification/*",
			testPath: "notification/change_test",
		},
		{
			Name:    "handle_partial_not_found",
			handler: &testCancelHandler{},
			// Not path is notifications and we have notification
			path:        "notifications/*",
			testPath:    "notification/change_test",
			shouldRoute: true,
		},
		{
			Name:     "base_star_always_match",
			handler:  &testCancelHandler{},
			path:     "*",
			testPath: "notification/change_test",
		},
		{
			Name:            "invalid_partial",
			handler:         &testCancelHandler{},
			path:            "*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_middle",
			handler:         &testCancelHandler{},
			path:            "test/*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_multiple",
			handler:         &testCancelHandler{},
			path:            "test/*/change_test*testing",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r := jsonrpc2.NewRouter()
			err := r.HandleCancel(tc.path, tc.handler,
				func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[smallArgs]{})
			if err != nil && !tc.shouldErrHandle {
				t.Log("error trying to handle call")
				t.Fail()
				return
			}
			if err == nil && tc.shouldErrHandle {
				t.Log("should error trying to handle call")
				t.Fail()
				return
			}
			if err != nil {
				return
			}
			_, ok := r.IsCancelRequest(tc.testPath)
			if !ok && !tc.shouldRoute {
				t.Log("error not routing", "err", err)
				t.Fail()
			}
			if ok && tc.shouldRoute {
				t.Log("did not error and we should have")
				t.Fail()
			}
		})
	}
}

func TestRouterCancelHandlerFunc(t *testing.T) {

	testCases := []struct {
		Name            string
		handler         jsonrpc2.CancelHandlerFunc
		path            string
		shouldRoute     bool
		testPath        string
		shouldErrHandle bool
	}{
		{
			Name:     "full_path",
			handler:  testCancelHandlerFunc,
			path:     "test_call",
			testPath: "test_call",
		},
		{
			Name:        "method_not_found",
			handler:     testCancelHandlerFunc,
			path:        "test_call",
			testPath:    "not_found",
			shouldRoute: true,
		},
		{
			Name:     "handle_partial",
			handler:  testCancelHandlerFunc,
			path:     "notification/*",
			testPath: "notification/change_test",
		},
		{
			Name:    "handle_partial_not_found",
			handler: testCancelHandlerFunc,
			// Not path is notifications and we have notification
			path:        "notifications/*",
			testPath:    "notification/change_test",
			shouldRoute: true,
		},
		{
			Name:     "base_star_always_match",
			handler:  testCancelHandlerFunc,
			path:     "*",
			testPath: "notification/change_test",
		},
		{
			Name:            "invalid_partial",
			handler:         testCancelHandlerFunc,
			path:            "*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_middle",
			handler:         testCancelHandlerFunc,
			path:            "test/*/change_test",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
		{
			Name:            "invalid_partial_multiple",
			handler:         testCancelHandlerFunc,
			path:            "test/*/change_test*testing",
			testPath:        "notification/change_test",
			shouldErrHandle: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			r := jsonrpc2.NewRouter()
			err := r.HandleCancelFunc(tc.path, tc.handler,
				func(params jsonrpc2.Parameters) (string, error) { return "", nil }, &jsonrpc2.TypedParameters[smallArgs]{})
			if err != nil && !tc.shouldErrHandle {
				t.Log("error trying to handle call")
				t.Fail()
				return
			}
			if err == nil && tc.shouldErrHandle {
				t.Log("should error trying to handle call")
				t.Fail()
				return
			}
			if err != nil {
				return
			}
			_, ok := r.IsCancelRequest(tc.testPath)
			if !ok && !tc.shouldRoute {
				t.Log("error not routing", "err", err)
				t.Fail()
			}
			if ok && tc.shouldRoute {
				t.Log("did not error and we should have")
				t.Fail()
			}
		})
	}
}
