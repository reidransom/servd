package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProcfile(t *testing.T) {
	in := strings.Join([]string{
		"",
		"# a comment",
		"web: npm run dev",
		"worker: node worker.js",
		"not-an-entry",
		": missing name",
		"missingcmd:",
		"  spaced :  docker run -p 80:80 img  ",
	}, "\n")
	got := parseProcfile(strings.NewReader(in))
	want := []procEntry{
		{Name: "web", Cmd: "npm run dev"},
		{Name: "worker", Cmd: "node worker.js"},
		{Name: "spaced", Cmd: "docker run -p 80:80 img"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestWebProcess(t *testing.T) {
	if _, ok := webProcess(nil); ok {
		t.Error("empty entries should not yield a process")
	}
	e, ok := webProcess([]procEntry{{Name: "worker", Cmd: "a"}, {Name: "cron", Cmd: "b"}})
	if !ok || e.Name != "worker" {
		t.Errorf("no web entry: got %+v, want first entry", e)
	}
	e, ok = webProcess([]procEntry{{Name: "worker", Cmd: "a"}, {Name: "web", Cmd: "b"}})
	if !ok || e.Name != "web" {
		t.Errorf("web entry present: got %+v, want web", e)
	}
}

func TestReadProcfile(t *testing.T) {
	dir := t.TempDir()
	if got := readProcfile(dir); got != nil {
		t.Errorf("no Procfile: got %v, want nil", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "Procfile"), []byte("web: plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readProcfile(dir); len(got) != 1 || got[0].Cmd != "plain" {
		t.Errorf("Procfile only: got %v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "Procfile.dev"), []byte("web: dev"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readProcfile(dir); len(got) != 1 || got[0].Cmd != "dev" {
		t.Errorf("Procfile.dev should win: got %v", got)
	}
}
