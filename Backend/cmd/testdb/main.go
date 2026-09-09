package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if _, e := exec.LookPath("docker"); e != nil {
		return errors.New("docker is required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		return e
	}
	suffix := hex.EncodeToString(b)
	name := "g0-go-test-" + suffix
	password := hex.EncodeToString(b) + "-P9!"
	schema := "test_" + suffix
	cleanup := func() { _, _ = exec.Command("docker", "rm", "-f", name).CombinedOutput() }
	defer cleanup()
	cmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", name, "-e", "POSTGRES_USER=g0", "-e", "POSTGRES_DB=g0", "-e", "POSTGRES_PASSWORD="+password, "-p", "127.0.0.1::5432", "postgres:17.6")
	out, e := cmd.Output()
	if e != nil {
		return fmt.Errorf("docker start failed: %w", e)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return errors.New("docker returned no container id")
	}
	var port string
	for i := 0; i < 60; i++ {
		out, e = exec.CommandContext(ctx, "docker", "port", name, "5432/tcp").Output()
		if e == nil && strings.TrimSpace(string(out)) != "" {
			parts := strings.Split(strings.TrimSpace(string(out)), ":")
			port = parts[len(parts)-1]
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if port == "" {
		return errors.New("postgres container did not expose a port")
	}
	url := "postgres://g0:" + password + "@127.0.0.1:" + port + "/g0?sslmode=disable"
	time.Sleep(500 * time.Millisecond)
	wd, e := os.Getwd()
	if e != nil {
		return e
	}
	env := append(os.Environ(), "DATABASE_URL="+url, "ROOM_SERVICE_TOKEN=runner-service-token-012345678901234567890123", "TEST_DB_SCHEMA="+schema, "TEST_DB_INTEGRATION=1", "MIGRATIONS_DIR="+filepath.Join(wd, "migrations"))
	// The child owns migration/test execution; all credentials are inherited, never printed.
	if len(args) == 0 {
		args = []string{"go", "test", "-race", "./..."}
	}
	child := exec.CommandContext(ctx, args[0], args[1:]...)
	child.Env = env
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if e = child.Run(); e != nil {
		return e
	}
	return nil
}

var _ = strconv.IntSize
