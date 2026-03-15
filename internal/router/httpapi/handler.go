package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
	"github.com/Rachit-Gandhi/go-router/internal/httputil"
	"github.com/Rachit-Gandhi/go-router/internal/store"
)

const (
	effectiveModelPageSize int32 = 128

	policyDeniedErrorMessage = "model not allowed by org/team policy"
)

type completionAdapter interface {
	Complete(ctx context.Context, req adapterCompletionRequest) (adapterCompletionResult, error)
}

type adapterCompletionRequest struct {
	Model    string
	Messages []chatMessage
}

type adapterCompletionResult struct {
	Content          string
	CompletionTokens int
}

type stubAdapter struct{}

func (a stubAdapter) Complete(_ context.Context, req adapterCompletionRequest) (adapterCompletionResult, error) {
	last := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			last = strings.TrimSpace(req.Messages[i].Content)
			break
		}
	}
	if last == "" {
		last = "ok"
	}
	content := fmt.Sprintf("stub reply for %s", req.Model)
	completionTokens := estimateTokens(content)
	if completionTokens < 8 {
		completionTokens = 8
	}
	return adapterCompletionResult{
		Content:          content,
		CompletionTokens: completionTokens,
	}, nil
}

type routerHandler struct {
	queries  *dbquery.Queries
	adapters map[string]completionAdapter
	now      func() time.Time
}

// NewHandler builds the router-plane HTTP router using Postgres config from env.
func NewHandler() http.Handler {
	h, _, err := NewHandlerWithPostgresFromEnv(time.Now)
	if err != nil {
		panic(err)
	}
	return h
}

// NewHandlerWithPostgresFromEnv creates a postgres-backed handler and returns the opened DB.
func NewHandlerWithPostgresFromEnv(now func() time.Time) (http.Handler, *sql.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("ROUTER_DB_DSN"))
	if dsn == "" {
		return nil, nil, errors.New("ROUTER_DB_DSN is required")
	}

	db, err := store.OpenPostgres(context.Background(), dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}

	return NewHandlerWithDB(db, now), db, nil
}

// NewHandlerWithDB builds the router-plane router with explicit DB dependencies.
func NewHandlerWithDB(db *sql.DB, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	if db == nil {
		panic("db is required")
	}

	h := &routerHandler{
		queries:  dbquery.New(db),
		adapters: defaultAdapters(),
		now:      now,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/router/healthz", httputil.HealthHandler())
	mux.HandleFunc("POST /v1/router/chat/completions", h.handleChatCompletions)

	return mux
}

// NewHandlerWithoutDB builds a lightweight router useful for route-level tests.
func NewHandlerWithoutDB(now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	h := &routerHandler{
		adapters: defaultAdapters(),
		now:      now,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/router/healthz", httputil.HealthHandler())
	mux.HandleFunc("POST /v1/router/chat/completions", h.handleChatCompletions)
	return mux
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatCompletionsResponse struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []chatCompletionsPick `json:"choices"`
	Usage   usageSummary          `json:"usage"`
}

type chatCompletionsPick struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type usageSummary struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type allowedModel struct {
	Provider string
	Model    string
}

func (h *routerHandler) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if h.queries == nil {
		writeError(w, http.StatusServiceUnavailable, "router backend is not configured")
		return
	}

	apiKey, ok := parseBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
		return
	}

	identity, err := h.queries.ResolveIdentityByAPIKeyHash(r.Context(), hashValue(apiKey))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to authenticate api key")
		return
	}

	var req chatCompletionsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages are required")
		return
	}
	for _, m := range req.Messages {
		if strings.TrimSpace(m.Role) == "" || strings.TrimSpace(m.Content) == "" {
			writeError(w, http.StatusBadRequest, "each message requires non-empty role and content")
			return
		}
	}

	allowedRows, err := h.listAllEffectiveAllowedModels(r.Context(), identity.OrgID, identity.TeamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve model policy")
		return
	}

	allowed := make([]allowedModel, 0, len(allowedRows))
	for _, row := range allowedRows {
		allowed = append(allowed, allowedModel{Provider: row.Provider, Model: row.Model})
	}

	selectableModels := allowed
	if isAutoModelSelection(req.Model) {
		selectableModels = filterModelsByConfiguredAdapters(allowed, h.adapters)
		if len(selectableModels) == 0 && len(allowed) > 0 {
			writeError(w, http.StatusBadGateway, "provider adapter not configured")
			return
		}
	}

	selected, err := selectModel(req, selectableModels)
	if err != nil {
		writeError(w, http.StatusForbidden, policyDeniedErrorMessage)
		return
	}

	adapter, ok := h.adapters[selected.Provider]
	if !ok {
		writeError(w, http.StatusBadGateway, "provider adapter not configured")
		return
	}

	completion, err := adapter.Complete(r.Context(), adapterCompletionRequest{Model: selected.Model, Messages: req.Messages})
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream completion failed")
		return
	}

	promptTokens := estimatePromptTokens(req.Messages)
	completionTokens := completion.CompletionTokens
	if completionTokens < 1 {
		completionTokens = 1
	}

	responseID, err := randomID("chatcmpl")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create completion id")
		return
	}

	writeJSON(w, http.StatusOK, chatCompletionsResponse{
		ID:      responseID,
		Object:  "chat.completion",
		Created: h.now().Unix(),
		Model:   selected.Model,
		Choices: []chatCompletionsPick{{
			Index: 0,
			Message: chatMessage{
				Role:    "assistant",
				Content: completion.Content,
			},
			FinishReason: "stop",
		}},
		Usage: usageSummary{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	})
}

func (h *routerHandler) listAllEffectiveAllowedModels(ctx context.Context, orgID, teamID string) ([]dbquery.ListEffectiveAllowedModelsRow, error) {
	offset := int32(0)
	all := make([]dbquery.ListEffectiveAllowedModelsRow, 0, effectiveModelPageSize)

	for {
		rows, err := h.queries.ListEffectiveAllowedModels(ctx, dbquery.ListEffectiveAllowedModelsParams{
			OrgID:      orgID,
			TeamID:     teamID,
			OffsetRows: sql.NullInt32{Int32: offset, Valid: true},
			LimitRows:  effectiveModelPageSize,
		})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}

		all = append(all, rows...)
		if len(rows) < int(effectiveModelPageSize) {
			break
		}
		offset += int32(len(rows))
	}

	return all, nil
}

var errPolicyDenied = errors.New("policy denied")

func selectModel(req chatCompletionsRequest, allowed []allowedModel) (allowedModel, error) {
	var zero allowedModel
	if len(allowed) == 0 {
		return zero, errPolicyDenied
	}

	requested := strings.TrimSpace(req.Model)
	if !isAutoModelSelection(req.Model) {
		for _, candidate := range allowed {
			if candidate.Model == requested {
				return candidate, nil
			}
		}
		return zero, errPolicyDenied
	}

	promptChars := 0
	for _, msg := range req.Messages {
		promptChars += len(msg.Content)
	}

	if promptChars <= 600 {
		for _, candidate := range allowed {
			if isFastTierModel(candidate.Model) {
				return candidate, nil
			}
		}
		return allowed[0], nil
	}

	for _, candidate := range allowed {
		if !isFastTierModel(candidate.Model) {
			return candidate, nil
		}
	}
	return allowed[len(allowed)-1], nil
}

func isAutoModelSelection(model string) bool {
	trimmed := strings.TrimSpace(model)
	return trimmed == "" || strings.EqualFold(trimmed, "auto")
}

func filterModelsByConfiguredAdapters(allowed []allowedModel, adapters map[string]completionAdapter) []allowedModel {
	filtered := make([]allowedModel, 0, len(allowed))
	for _, candidate := range allowed {
		if _, ok := adapters[candidate.Provider]; ok {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func isFastTierModel(model string) bool {
	m := strings.ToLower(model)
	fastHints := []string{"mini", "flash", "haiku", "nano", "small", "lite"}
	for _, hint := range fastHints {
		if strings.Contains(m, hint) {
			return true
		}
	}
	return false
}

func parseBearerToken(headerValue string) (string, bool) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return "", false
	}

	parts := strings.SplitN(headerValue, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func estimatePromptTokens(messages []chatMessage) int {
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Content)
	}
	tokens := (totalChars + 3) / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func estimateTokens(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return 1
	}
	return (len(trimmed) + 3) / 4
}

func decodeJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("unexpected trailing JSON content")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomID(prefix string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buf)), nil
}

func defaultAdapters() map[string]completionAdapter {
	return map[string]completionAdapter{
		"openai":    stubAdapter{},
		"claude":    stubAdapter{},
		"anthropic": stubAdapter{},
		"gemini":    stubAdapter{},
		"codex":     stubAdapter{},
	}
}
