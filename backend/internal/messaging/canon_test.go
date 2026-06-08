package messaging

import "testing"

func TestCanonPhone(t *testing.T) {
	cases := map[string]string{
		"554691393338":      "5546991393338",
		"5546991393338":     "5546991393338",
		"+55 46 99139-3338": "5546991393338",
		"551133334444":      "551133334444",
	}
	for in, want := range cases {
		if got := CanonPhone(in); got != want {
			t.Errorf("CanonPhone(%q) = %q, quer %q", in, got, want)
		}
	}
}
