package proxy

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestSlugFromHost(t *testing.T) {
	s := &Server{settings: config.Settings{DomainSuffix: "127.0.0.1.nip.io"}}
	cases := []struct{ host, want string }{
		{"foo.127.0.0.1.nip.io:8080", "foo"},
		{"foo.127.0.0.1.nip.io", "foo"},
		{"FOO.127.0.0.1.NIP.IO:8080", "foo"},
		{"127.0.0.1.nip.io:8080", ""},
		{"127.0.0.1:8080", ""},
		{"evil.com", ""},
		{"a.b.127.0.0.1.nip.io:8080", "a"},
	}
	for _, c := range cases {
		if got := s.slugFromHost(c.host); got != c.want {
			t.Errorf("slugFromHost(%q) = %q, want %q", c.host, got, c.want)
		}
	}
}

func TestLandingEscapesHTML(t *testing.T) {
	s := &Server{
		settings: config.Settings{DomainSuffix: "127.0.0.1.nip.io", ProxyPort: 8080},
		sites: []config.Site{
			{Slug: "ok", Port: 4001, Launcher: `<script>alert(1)</script>`},
		},
	}
	rec := httptest.NewRecorder()
	s.landing(rec, httptest.NewRequest("GET", "http://127.0.0.1.nip.io:8080/", nil))
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("launcher field not escaped in landing page")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected escaped launcher text in landing page")
	}
	if !strings.Contains(body, "http://ok.127.0.0.1.nip.io:8080/") {
		t.Error("expected site link in landing page")
	}
}
