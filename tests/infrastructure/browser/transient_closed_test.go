package browser_test

import (
	"errors"
	"testing"

	"lonceng_unman_be/internal/infrastructure/browser"
)

func TestIsTransientBrowserError_ClosedNetwork(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"closed network connection", errors.New("write tcp 127.0.0.1:36283->127.0.0.1:9222: use of closed network connection"), true},
		{"closed substring", errors.New("closed"), true},
		{"closed network explicit", errors.New("closed network"), true},
		{"eof still transient", errors.New("EOF"), true},
		{"timeout still transient", errors.New("i/o timeout"), true},
		{"context canceled not transient", errors.New("context canceled"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := browser.IsTransientBrowserError(tc.err)
			if got != tc.want {
				t.Errorf("IsTransientBrowserError(%q)=%v want %v", tc.err.Error(), got, tc.want)
			}
		})
	}
}
