//go:build !windows

package proxy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

const (
	inheritedListenerFD = 3
	readyPipeFD         = 4
)

type workerProcess struct {
	PID      int
	PGID     int
	Identity uint64
}

// BindRequest is the complete, resolved handoff contract for the root helper.
// It contains no settings or registry data; the helper only binds and starts
// the regular-user worker.
type BindRequest struct {
	BindHost   string
	Worker     string
	Port       int
	LAN        bool
	UID        uint32
	GID        uint32
	Groups     []uint32
	Home       string
	ConfigHome string
	StateHome  string
}

// RunPrivilegedBind binds the requested port, starts the worker as the
// requesting user, waits for the worker's ready signal, and returns its PID.
func RunPrivilegedBind(request BindRequest) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(request.BindHost, strconv.Itoa(request.Port)))
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	listenerFile, err := listenerFile(listener)
	if err != nil {
		return 0, err
	}
	defer listenerFile.Close()

	process, err := spawnWorker(workerStartRequest{
		Worker:     request.Worker,
		Port:       request.Port,
		LAN:        request.LAN,
		Listener:   listenerFile,
		Home:       request.Home,
		ConfigHome: request.ConfigHome,
		StateHome:  request.StateHome,
		Credential: &syscall.Credential{Uid: request.UID, Gid: request.GID, Groups: request.Groups},
	})
	if err != nil {
		return 0, err
	}
	return process.PID, nil
}

func startElevatedWorker(settings config.Settings) (workerProcess, error) {
	worker, err := os.Executable()
	if err != nil {
		return workerProcess{}, err
	}
	groups, err := os.Getgroups()
	if err != nil {
		return workerProcess{}, err
	}
	pid, err := runSudoBind(sudoBindArgs(worker, settings, groups))
	if err != nil {
		return workerProcess{}, err
	}
	return workerProcessForPID(pid)
}

func sudoBindArgs(worker string, settings config.Settings, groups []int) []string {
	args := []string{
		worker, "__proxy-bind",
		"--worker", worker,
		"--port", strconv.Itoa(settings.Hostnames.HTTPPort),
		"--uid", strconv.Itoa(os.Getuid()),
		"--gid", strconv.Itoa(os.Getgid()),
		"--groups", groupList(groups),
		"--home", os.Getenv("HOME"),
		"--bind-host", settings.BindHost,
		"--config-home", os.Getenv("XDG_CONFIG_HOME"),
		"--state-home", os.Getenv("XDG_STATE_HOME"),
	}
	if settings.Hostnames.LAN {
		args = append(args, "--lan")
	}
	if !interactiveTerminal() {
		args = append([]string{"-n"}, args...)
	}
	return args
}

func runSudoBind(args []string) (int, error) {
	command := exec.Command("sudo", args...)
	command.Stdin = os.Stdin
	command.Stderr = os.Stderr
	output, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("elevation was denied or canceled: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || pid < 1 {
		return 0, fmt.Errorf("invalid privileged worker response %q", strings.TrimSpace(string(output)))
	}
	return pid, nil
}

func workerProcessForPID(pid int) (workerProcess, error) {
	identity, err := state.ProcessIdentity(pid)
	if err != nil {
		return workerProcess{}, fmt.Errorf("identify proxy worker %d: %w", pid, err)
	}
	return workerProcess{PID: pid, PGID: processGroupID(pid), Identity: identity}, nil
}
func groupList(groups []int) string {
	values := make([]string, len(groups))
	for index, group := range groups {
		values[index] = strconv.Itoa(group)
	}
	return strings.Join(values, ",")
}

func ParseGroups(raw string) ([]uint32, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	groups := make([]uint32, len(parts))
	for index, part := range parts {
		group, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid group %q: %w", part, err)
		}
		groups[index] = uint32(group)
	}
	return groups, nil
}

func interactiveTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func processGroupID(pid int) int {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return pid
	}
	return pgid
}

func terminateWorker(process workerProcess) error {
	if process.PGID > 0 {
		if err := syscall.Kill(-process.PGID, syscall.SIGTERM); err == nil {
			return nil
		}
	}
	target, err := os.FindProcess(process.PID)
	if err != nil {
		return err
	}
	return target.Kill()
}

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}
