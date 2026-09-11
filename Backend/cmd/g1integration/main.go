// Run through cmd/testdb: the outer runner owns the temporary PostgreSQL container.
package main

import (
	"aigame/server/backend/infra"
	"aigame/server/backend/usecase"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type logBuffer struct {
	sync.Mutex
	text strings.Builder
}

func (b *logBuffer) Write(p []byte) (int, error) { b.Lock(); defer b.Unlock(); return b.text.Write(p) }
func (b *logBuffer) String() string              { b.Lock(); defer b.Unlock(); return b.text.String() }
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if os.Getenv("TEST_DB_INTEGRATION") != "1" {
		return fmt.Errorf("run: go run ./cmd/testdb go run ./cmd/g1integration")
	}
	dbSchema := os.Getenv("TEST_DB_SCHEMA")
	if dbSchema == "" {
		return fmt.Errorf("TEST_DB_SCHEMA is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "g1-integration-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	bin := filepath.Join(temp, "backend")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build: %w %s", err, out)
	}
	p, err := infra.Open(ctx, os.Getenv("DATABASE_URL"), os.Getenv("TEST_DB_SCHEMA"))
	if err != nil {
		return err
	}
	defer p.Pool.Close()
	for p.Ping(ctx) != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if err = infra.Migrate(ctx, p); err != nil {
		return err
	}
	tcp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := tcp.Addr().(*net.TCPAddr).Port
	tcp.Close()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	roomPort := udp.LocalAddr().(*net.UDPAddr).Port
	udp.Close()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	env := append(os.Environ(), "TEST_DB_SCHEMA="+dbSchema, "ALLOW_LOOPBACK_HTTP=1", "G0_LOOPBACK_TEST=1", "G0_BACKEND_URL="+base, fmt.Sprintf("BACKEND_PORT=%d", port), fmt.Sprintf("ROOM_PORT=%d", roomPort), "ROOM_ID=g0-room", "ROOM_GENERATION=1", "G0_TEST_PASSWORD=g1-local-test-password", "G0_PASSWORD=g1-local-test-password")
	for _, user := range []string{"g1-a", "g1-b"} {
		cmd := exec.CommandContext(ctx, bin, "seed", user)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("seed: %w %s", err, out)
		}
	}
	logs := map[string]*logBuffer{}
	start := func(name string, args []string, extra ...string) (*exec.Cmd, chan error, error) {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = append(append([]string{}, env...), extra...)
		b := &logBuffer{}
		logs[name] = b
		cmd.Stdout = b
		cmd.Stderr = b
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		return cmd, done, nil
	}
	defer func() {
		for _, name := range []string{"backend", "gameserver", "A", "B"} {
			if b := logs[name]; b != nil {
				fmt.Printf("--- %s ---\n%s", name, b.String())
			}
		}
	}()
	backend, bd, err := start("backend", []string{bin})
	if err != nil {
		return err
	}
	defer func() { backend.Process.Kill(); <-bd }()
	client := http.Client{Timeout: time.Second}
	for {
		resp, e := client.Get(base + "/health/ready")
		if e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	gs, gd, err := start("gameserver", []string{"godot", "--headless", "--path", filepath.Join(root, "Server/GameServer"), "--", "--server"})
	if err != nil {
		return err
	}
	defer func() { gs.Process.Kill(); <-gd }()
	waitLog := func(name, marker string) error {
		for !strings.Contains(logs[name].String(), marker) {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting %s %s", name, marker)
			case <-time.After(100 * time.Millisecond):
			}
		}
		return nil
	}
	if err = waitLog("gameserver", "G0_SERVER_LISTENING"); err != nil {
		return err
	}
	var host, content, config, status string
	var protocol, capacity, generation, actualPort int
	if err = p.Pool.QueryRow(ctx, "select host,port,protocol_version,gameplay_content_hash,resolved_config_hash,capacity,generation,status from room where room_id='g0-room'").Scan(&host, &actualPort, &protocol, &content, &config, &capacity, &generation, &status); err != nil {
		return err
	}
	if host != "127.0.0.1" || actualPort != roomPort || protocol != 1 || content != "g0-empty-v1" || config != usecase.ConfigHash() || capacity != 8 || generation != 1 || status != "ready" {
		return fmt.Errorf("registered fields mismatch")
	}
	if os.Getenv("G1_RECONNECT") == "1" {
		a, ad, err := start("A", []string{"godot", "--headless", "--path", filepath.Join(root, "3D_App"), "--script", "res://Tests/Godot/g1_reconnect_driver.gd"}, "G0_USERNAME=g1-a", "G1_CLIENT_ROLE=A", "G3_INTEGRATION=0")
		if err != nil {
			return err
		}
		defer a.Process.Kill()
		if err = waitLog("gameserver", "G1 player_reconnectable"); err != nil {
			return err
		}
		if err = waitLog("gameserver", "G1 player_reconnected"); err != nil {
			return err
		}
		if err = waitLog("A", "G1_RECONNECT_PASS"); err != nil {
			return err
		}
		select {
		case err := <-ad:
			if err != nil {
				return fmt.Errorf("client A: %w", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		fmt.Println("G1_RECONNECT_INTEGRATION_PASS")
		return nil
	}
	// G1 keeps its late-join lease check; G3 starts both clients together to race one pickup.
	a, ad, err := start("A", []string{"godot", "--headless", "--path", filepath.Join(root, "3D_App"), "--", "--smoke-exit"}, "G0_USERNAME=g1-a", "G1_CLIENT_ROLE=A", "G3_INTEGRATION="+os.Getenv("G3_INTEGRATION"), "G3_BARRIER_DIR="+os.Getenv("G3_BARRIER_DIR"))
	if err != nil {
		return err
	}
	defer a.Process.Kill()
	if err = waitLog("A", "self=true"); err != nil {
		return err
	}
	if os.Getenv("G3_INTEGRATION") == "1" {
		b, _, err := start("B", []string{"godot", "--headless", "--path", filepath.Join(root, "3D_App"), "--", "--smoke-exit"}, "G0_USERNAME=g1-b", "G1_CLIENT_ROLE=B", "G3_INTEGRATION=1", "G3_BARRIER_DIR="+os.Getenv("G3_BARRIER_DIR"))
		if err != nil {
			return err
		}
		defer b.Process.Kill()
		if err = waitLog("B", "self=true"); err != nil {
			return err
		}
		deadline := time.Now().Add(20 * time.Second)
		for !barrierExists(os.Getenv("G3_BARRIER_DIR")+"/A") || !barrierExists(os.Getenv("G3_BARRIER_DIR")+"/B") {
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting G3 pickup results")
			}
			time.Sleep(100 * time.Millisecond)
		}
		var statuses []string
		for _, role := range []string{"A", "B"} {
			data, readErr := os.ReadFile(os.Getenv("G3_BARRIER_DIR") + "/" + role)
			if readErr != nil {
				return readErr
			}
			statuses = append(statuses, strings.TrimSpace(string(data)))
		}
		if (statuses[0] == statuses[1]) || (statuses[0] != "succeeded" && statuses[1] != "succeeded") {
			return fmt.Errorf("invalid G3 statuses %v", statuses)
		}
		winner := "A"
		if statuses[0] != "succeeded" {
			winner = "B"
		}
		if err = waitLog(winner, "G3_INVENTORY_SNAPSHOT"); err != nil {
			return err
		}
		var amount int
		winnerUsername := "g1-a"
		if winner == "B" {
			winnerUsername = "g1-b"
		}
		if err = p.Pool.QueryRow(ctx, "select coalesce((inventory->>'coin')::int,0) from player_progress where player_id=(select player_id from player where username=$1)", winnerUsername).Scan(&amount); err != nil || amount != 1 {
			return fmt.Errorf("G3 inventory amount=%d err=%v", amount, err)
		}
		fmt.Printf("G3_INTEGRATION_PASS statuses=%v inventory_coin=%d\n", statuses, amount)
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(17 * time.Second):
	}
	b, bb, err := start("B", []string{"godot", "--headless", "--path", filepath.Join(root, "3D_App"), "--", "--smoke-exit"}, "G0_USERNAME=g1-b", "G1_CLIENT_ROLE=B")
	if err != nil {
		return err
	}
	defer b.Process.Kill()
	for _, entry := range []struct {
		name string
		done chan error
	}{{"A", ad}, {"B", bb}} {
		select {
		case err := <-entry.done:
			if err != nil {
				return fmt.Errorf("client %s: %w", entry.name, err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		for _, marker := range []string{"self=true", "self=false"} {
			if !strings.Contains(logs[entry.name].String(), marker) {
				return fmt.Errorf("client %s missing %s", entry.name, marker)
			}
		}
	}
	if err = waitLog("gameserver", "G1 player_left"); err != nil {
		return err
	}
	var redeemed, players int
	if err = p.Pool.QueryRow(ctx, "select count(*),count(distinct player_id) from tickets where redeemed_at is not null").Scan(&redeemed, &players); err != nil {
		return err
	}
	if redeemed != 2 || players != 2 {
		return fmt.Errorf("expected two independently redeemed tickets")
	}
	for name, log := range logs {
		if strings.Contains(log.String(), "ERROR:") || strings.Contains(log.String(), "HEARTBEAT_FAILED") || strings.Contains(log.String(), "AUTH_REJECTED") {
			return fmt.Errorf("runtime error in %s", name)
		}
	}
	fmt.Println("G1_INTEGRATION_PASS A=0 B=0 self/remote joins, late join, player_left, registration, two tickets")
	return nil
}

func barrierExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
