package haproxy

import "testing"

var haLabels = []struct {
	name   string
	haname string
}{
	{"mytest", "mytest"},
	{"mytest/backslash", "mytest_backslash"},
	{"mytest.point", "mytest_point"},
}

// convertToValidName replace non valid characters in config names
func TestConvertToValidName(t *testing.T) {

	for _, tc := range haLabels {
		r := convertToValidName(tc.name)
		if r != tc.haname {
			t.Errorf("expected '%s', got '%s'", tc.haname, r)
		}
	}
}
