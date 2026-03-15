package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
	"github.com/Rachit-Gandhi/go-router/internal/httputil"
	"github.com/Rachit-Gandhi/go-router/internal/store"
	"github.com/pressly/goose/v3"
)

var (
	postgresSetupOnce sync.Once
	postgresSetupErr  error
	postgresDB        *sql.DB
	postgresContainer string

	errDockerUnavailable = errors.New("docker not found")
)

func TestMain(m *testing.M) {
	code := m.Run()
	if postgresContainer != "" {
		_ = exec.Command("docker", "rm", "-f", postgresContainer).Run()
	}
	os.Exit(code)
}

type testRouterHandler struct {
	db      *sql.DB
	handler http.Handler
	router  *routerHandler
}

func newTestRouterHandler(t *testing.T) *testRouterHandler {
	t.Helper()
	return newTestRouterHandlerWithAdapters(t, nil)
}

func newTestRouterHandlerWithAdapters(t *testing.T, adapters map[string]completionAdapter) *testRouterHandler {
	t.Helper()
	db := ensurePostgres(t)
	resetPostgresData(t, db)
	if adapters == nil {
		adapters = defaultAdapters()
	}

	router := &routerHandler{
		queries:  dbquery.New(db),
		adapters: adapters,
		now:      time.Now,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/router/healthz", httputil.HealthHandler())
	mux.HandleFunc("POST /v1/router/chat/completions", router.handleChatCompletions)

	return &testRouterHandler{
		db:      db,
		handler: mux,
		router:  router,
	}
}

func ensurePostgres(t *testing.T) *sql.DB {
	t.Helper()

	postgresSetupOnce.Do(func() {
		postgresSetupErr = startPostgresForTests()
	})
	if postgresSetupErr != nil {
		if errors.Is(postgresSetupErr, errDockerUnavailable) {
			t.Skipf("skipping postgres-backed router tests: %v", postgresSetupErr)
		}
		t.Fatalf("postgres-backed router test setup failed: %v", postgresSetupErr)
	}
	return postgresDB
}

func startPostgresForTests() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("%w: %v", errDockerUnavailable, err)
	}

	run := exec.Command(
		"docker", "run", "-d", "--rm",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_DB=gorouter_test",
		"-p", "127.0.0.1::5432",
		"postgres:16-alpine",
	)
	out, err := run.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start postgres container: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	postgresContainer = strings.TrimSpace(string(out))

	port, err := resolvePublishedPostgresPort(postgresContainer)
	if err != nil {
		return err
	}

	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/gorouter_test?sslmode=disable", port)
	var db *sql.DB
	time.Sleep(1 * time.Second)
	for i := 0; i < 30; i++ {
		db, err = store.OpenPostgres(context.Background(), dsn)
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

func resolvePublishedPostgresPort(containerID string) (int, error) {
	out, err := exec.Command("docker", "port", containerID, "5432/tcp").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("inspect postgres mapped port: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, portText, err := net.SplitHostPort(line)
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("unable to parse postgres mapped port from %q", strings.TrimSpace(string(out)))
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to resolve runtime caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../..")), nil
}
