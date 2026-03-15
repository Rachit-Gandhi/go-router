package httpapi

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rachit-Gandhi/go-router/internal/auth"
	"github.com/Rachit-Gandhi/go-router/internal/store"
	"github.com/pressly/goose/v3"
)

var (
	postgresSetupOnce sync.Once
	postgresSetupErr  error
	postgresDB        *sql.DB
	postgresContainer string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if postgresContainer != "" {
		_ = exec.Command("docker", "rm", "-f", postgresContainer).Run()
	}
	os.Exit(code)
}

func newTestHandler(t *testing.T) *testControlHandler {
	t.Helper()
	db := ensurePostgres(t)
	resetPostgresData(t, db)

	codec, err := auth.NewSessionCodec("test-control-session-secret")
	if err != nil {
		t.Fatalf("new session codec: %v", err)
	}
	return &testControlHandler{
		handler: NewHandlerWithDB(db, codec, time.Now),
	}
}

type testControlHandler struct {
	handler http.Handler
}

func ensurePostgres(t *testing.T) *sql.DB {
	t.Helper()

	postgresSetupOnce.Do(func() {
		postgresSetupErr = startPostgresForTests()
	})
	if postgresSetupErr != nil {
		t.Skipf("skipping postgres-backed control tests: %v", postgresSetupErr)
	}
	return postgresDB
}

func startPostgresForTests() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found: %w", err)
	}

	port, err := reserveTCPPort()
	if err != nil {
		return fmt.Errorf("reserve tcp port: %w", err)
	}

	run := exec.Command(
		"docker", "run", "-d", "--rm",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_DB=gorouter_test",
		"-p", fmt.Sprintf("%d:5432", port),
		"postgres:16-alpine",
	)
	out, err := run.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start postgres container: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	postgresContainer = strings.TrimSpace(string(out))

	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/gorouter_test?sslmode=disable", port)
	var db *sql.DB
	for i := 0; i < 30; i++ {
		db, err = store.OpenPostgres(dsn)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("wait for postgres readiness: %w", err)
	}
	postgresDB = db

	repoRoot, err := repoRoot()
	if err != nil {
		return err
	}
	migrationsDir := filepath.Join(repoRoot, "db", "migrations")
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

func resetPostgresData(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`TRUNCATE TABLE orgs, users RESTART IDENTITY CASCADE;`)
	if err != nil {
		t.Fatalf("truncate test data: %v", err)
	}
}

func reserveTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("not tcp addr")
	}
	return addr.Port, nil
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to resolve runtime caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../..")), nil
}
