package flock

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestWithLockSerializesProcesses(t *testing.T) {
	path := t.TempDir() + string(os.PathSeparator) + "state.json"
	first, firstOutput := lockHelper(t, path, "400ms")
	defer func() { _ = first.Process.Kill() }()
	waitForLine(t, firstOutput)

	second, secondOutput := lockHelper(t, path, "0s")
	defer func() { _ = second.Process.Kill() }()
	secondLine := make(chan error, 1)
	go func() {
		_, err := secondOutput.ReadString('\n')
		secondLine <- err
	}()

	select {
	case err := <-secondLine:
		t.Fatalf("second process acquired lock before first exited: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("first lock helper: %v", err)
	}
	select {
	case err := <-secondLine:
		if err != nil {
			t.Fatalf("second lock helper output: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second process did not acquire released lock")
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second lock helper: %v", err)
	}
}

func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("SERVD_LOCK_HELPER") != "1" {
		return
	}
	path := os.Getenv("SERVD_LOCK_PATH")
	hold, err := time.ParseDuration(os.Getenv("SERVD_LOCK_HOLD"))
	if err != nil {
		t.Fatal(err)
	}
	if err := WithLock(path, func() error {
		fmt.Println("locked")
		time.Sleep(hold)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func lockHelper(t *testing.T, path, hold string) (*exec.Cmd, *bufio.Reader) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	command.Env = append(os.Environ(),
		"SERVD_LOCK_HELPER=1",
		"SERVD_LOCK_PATH="+path,
		"SERVD_LOCK_HOLD="+hold,
	)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	return command, bufio.NewReader(stdout)
}

func waitForLine(t *testing.T, output *bufio.Reader) {
	t.Helper()
	line, err := output.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "locked\n" {
		t.Fatalf("lock helper output = %q", line)
	}
}
