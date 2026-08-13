//go:build !windows

package launcher

import "testing"

func TestShellJoin(t *testing.T) {
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"npm", "run", "dev"}, "npm run dev"},
		{[]string{"jigyll", "serve", "-P", "{port}"}, "jigyll serve -P {port}"},
		{[]string{"echo", "hello world"}, "echo 'hello world'"},
		{[]string{"sh", "-c", "a && b"}, `sh -c 'a && b'`},
		{[]string{"say", "it's"}, `say 'it'\''s'`},
	}
	for _, test := range cases {
		if got := ShellJoin(test.argv); got != test.want {
			t.Errorf("ShellJoin(%q):\n got %q\nwant %q", test.argv, got, test.want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("/a b/it's"); got != `'/a b/it'\''s'` {
		t.Errorf("ShellQuote: got %q", got)
	}
}
