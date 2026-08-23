package proxy

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func workerArgs(port int, lan bool) []string {
	args := []string{"__proxy-worker", "--port", strconv.Itoa(port)}
	if lan {
		args = append(args, "--lan")
	}
	return args
}

func waitForWorkerReady(ready *os.File) error {
	if err := ready.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	line, err := bufio.NewReader(ready).ReadString('\n')
	if err != nil {
		return fmt.Errorf("proxy worker did not become ready: %w", err)
	}
	if line != "ready\n" {
		return fmt.Errorf("proxy worker failed to start: %s", strings.TrimSpace(line))
	}
	return nil
}
