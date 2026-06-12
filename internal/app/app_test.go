package app

import (
	"testing"
	"time"
)

func TestFmtDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m30s"},
		{3*time.Hour + 5*time.Minute, "3h5m"},
	}
	for _, c := range cases {
		if got := FmtDuration(c.d); got != c.want {
			t.Errorf("FmtDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestDash(t *testing.T) {
	if Dash("") != "-" || Dash("x") != "x" {
		t.Error("Dash should substitute '-' only for empty strings")
	}
}
