package commands

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newStaticCmd serves a directory in the foreground without loading Servd
// configuration. It is intended for explicit site commands and standalone use.
func newStaticCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "static",
		Short: "Serve static files from a directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := staticServerConfig(cmd)
			if err != nil {
				return err
			}

			listener, err := net.Listen("tcp", config.addr)
			if err != nil {
				return err
			}
			server := &http.Server{
				Handler:      staticHandler{root: config.root},
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}
			return server.Serve(listener)
		},
	}
	command.Flags().String("host", "127.0.0.1", "bind host")
	command.Flags().String("port", "8080", "listen port")
	command.Flags().String("dir", ".", "directory to serve")
	return command
}

type staticConfig struct {
	addr string
	root string
}

func staticServerConfig(command *cobra.Command) (staticConfig, error) {
	host, err := command.Flags().GetString("host")
	if err != nil {
		return staticConfig{}, err
	}
	if !command.Flags().Changed("host") {
		if value, ok := os.LookupEnv("HOST"); ok {
			host = value
		}
	}
	if strings.TrimSpace(host) == "" {
		return staticConfig{}, fmt.Errorf("static host must not be empty")
	}

	port, err := command.Flags().GetString("port")
	if err != nil {
		return staticConfig{}, err
	}
	if !command.Flags().Changed("port") {
		if value, ok := os.LookupEnv("PORT"); ok {
			port = value
		}
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return staticConfig{}, fmt.Errorf("static port must be an integer from 1 through 65535")
	}

	dir, err := command.Flags().GetString("dir")
	if err != nil {
		return staticConfig{}, err
	}
	root, err := canonicalStaticRoot(dir)
	if err != nil {
		return staticConfig{}, err
	}
	return staticConfig{addr: net.JoinHostPort(host, strconv.Itoa(portNumber)), root: root}, nil
}

func canonicalStaticRoot(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("static root must not be empty")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve static root %q: %w", dir, err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve static root %q: %w", dir, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("read static root %q: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("static root %q is not a directory", dir)
	}

	file, err := os.Open(root)
	if err != nil {
		return "", fmt.Errorf("read static root %q: %w", dir, err)
	}
	defer file.Close()
	if _, err := file.Readdirnames(1); err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read static root %q: %w", dir, err)
	}
	return root, nil
}

type staticHandler struct{ root string }

func (server staticHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	requested := strings.ReplaceAll(request.URL.Path, "\\", "/")
	if containsDotSegment(requested) {
		http.Error(writer, "Forbidden", http.StatusForbidden)
		return
	}

	relative := strings.TrimPrefix(path.Clean("/"+requested), "/")
	target, info, status := server.resolve(filepath.Join(server.root, filepath.FromSlash(relative)))
	if status != 0 {
		staticError(writer, status)
		return
	}
	if info.IsDir() {
		target, info, status = server.resolve(filepath.Join(target, "index.html"))
		if status != 0 {
			staticError(writer, status)
			return
		}
		if !info.Mode().IsRegular() {
			staticError(writer, http.StatusNotFound)
			return
		}
	}
	if !info.Mode().IsRegular() {
		staticError(writer, http.StatusNotFound)
		return
	}

	file, err := os.Open(target)
	if err != nil {
		staticError(writer, http.StatusNotFound)
		return
	}
	defer file.Close()
	http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
}

func (server staticHandler) resolve(candidate string) (string, fs.FileInfo, int) {
	target, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", nil, http.StatusNotFound
	}
	relative, err := filepath.Rel(server.root, target)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", nil, http.StatusForbidden
	}
	if relative != "." && containsDotSegment(relative) {
		return "", nil, http.StatusForbidden
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", nil, http.StatusNotFound
	}
	return target, info, 0
}

func staticError(writer http.ResponseWriter, status int) {
	if status == http.StatusForbidden {
		http.Error(writer, "Forbidden", status)
		return
	}
	http.NotFound(writer, nil)
}

// containsDotSegment reports whether a slash- or backslash-separated path has
// a segment that starts with a dot.
func containsDotSegment(name string) bool {
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
