package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zzzblxx/glance/internal/model"
)

const (
	defaultRequestTimeout = 8 * time.Second
	quotaRequestTimeout   = 20 * time.Second
)

var ErrProtocol = errors.New("codex app-server protocol unavailable")

type ThreadMetadata struct {
	ID             string
	SessionID      string
	Title          string
	ParentThreadID string
}

type Client struct {
	program string
	home    string
	mu      sync.Mutex
	session *session
}

func NewClient(program string, home ...string) *Client {
	c := &Client{program: program}
	if len(home) > 0 {
		c.home = home[0]
	}
	return c
}

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.session == nil {
		return nil
	}
	err := client.session.close()
	client.session = nil
	return err
}

func (client *Client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.session == nil {
		session, err := startSession(ctx, client.program, client.home)
		if err != nil {
			return nil, err
		}
		client.session = session
	}
	result, err := client.session.request(ctx, method, params)
	if err != nil {
		_ = client.session.close()
		client.session = nil
		return nil, err
	}
	return result, nil
}

func (client *Client) ReadQuota(ctx context.Context) (model.Quota, error) {
	requestContext, cancel := context.WithTimeout(ctx, quotaRequestTimeout)
	defer cancel()
	raw, err := client.request(requestContext, "account/rateLimits/read", nil)
	if err != nil {
		return model.Quota{}, err
	}
	return ParseQuota(raw)
}

func (client *Client) ReadThreads(ctx context.Context) ([]ThreadMetadata, error) {
	return client.readThreads(ctx, false)
}

func (client *Client) ReadArchivedThreads(ctx context.Context) ([]ThreadMetadata, error) {
	return client.readThreads(ctx, true)
}

func (client *Client) readThreads(ctx context.Context, archived bool) ([]ThreadMetadata, error) {
	var all []ThreadMetadata
	var cursor any
	seen := make(map[string]struct{})
	for pages := 0; pages < 1000; pages++ {
		requestContext, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
		raw, err := client.request(requestContext, "thread/list", map[string]any{
			"cursor":        cursor,
			"limit":         100,
			"sortKey":       "updated_at",
			"sortDirection": "desc",
			"archived":      archived,
			"sourceKinds": []string{
				"cli", "vscode", "exec", "appServer", "subAgent", "subAgentReview",
				"subAgentCompact", "subAgentThreadSpawn", "subAgentOther", "unknown",
			},
			"useStateDbOnly": true,
		})
		cancel()
		if err != nil {
			return nil, err
		}
		page, next, err := parseThreadPage(raw)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			return all, nil
		}
		if _, duplicate := seen[next]; duplicate {
			return nil, ErrProtocol
		}
		seen[next] = struct{}{}
		cursor = next
	}
	return nil, ErrProtocol
}

type session struct {
	command  *exec.Cmd
	stdin    io.WriteCloser
	messages chan rpcMessage
	nextID   int64
	closed   bool
	done     chan struct{}
}

type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func startSession(ctx context.Context, program, home string) (*session, error) {
	command := exec.Command(program, "app-server", "--stdio")
	if home != "" {
		command.Env = append(os.Environ(), "CODEX_HOME="+home)
		command.Dir = home
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("start codex observer: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("start codex observer: %w", err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start codex observer: %w", err)
	}
	s := &session{
		command:  command,
		stdin:    stdin,
		messages: make(chan rpcMessage, 16),
		nextID:   2,
		done:     make(chan struct{}),
	}
	go s.readLoop(stdout)
	initContext, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()
	if err := s.write(map[string]any{
		"id":     1,
		"method": "initialize",
		"params": map[string]any{"clientInfo": map[string]string{
			"name": "glance", "version": "1.0.0",
		}},
	}); err != nil {
		_ = s.close()
		return nil, err
	}
	if _, err := s.waitFor(initContext, 1); err != nil {
		_ = s.close()
		return nil, err
	}
	if err := s.write(map[string]any{"method": "initialized"}); err != nil {
		_ = s.close()
		return nil, err
	}
	return s, nil
}

func (s *session) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := s.nextID
	s.nextID++
	if err := s.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	return s.waitFor(ctx, id)
}

func (s *session) write(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ErrProtocol
	}
	encoded = append(encoded, '\n')
	if _, err := s.stdin.Write(encoded); err != nil {
		return fmt.Errorf("codex observer transport: %w", err)
	}
	return nil
}

func (s *session) waitFor(ctx context.Context, id int64) (json.RawMessage, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("codex observer timeout: %w", ctx.Err())
		case message, ok := <-s.messages:
			if !ok {
				return nil, fmt.Errorf("codex observer transport: %w", io.EOF)
			}
			var responseID int64
			if err := json.Unmarshal(message.ID, &responseID); err != nil || responseID != id {
				continue
			}
			if len(message.Error) > 0 && string(message.Error) != "null" {
				return nil, ErrProtocol
			}
			if len(message.Result) == 0 || string(message.Result) == "null" {
				return nil, ErrProtocol
			}
			return message.Result, nil
		}
	}
}

func (s *session) readLoop(reader io.Reader) {
	defer close(s.messages)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		if len(message.ID) == 0 {
			continue
		}
		select {
		case s.messages <- message:
		case <-s.done:
			return
		}
	}
}

func (s *session) close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	_ = s.stdin.Close()
	if s.command.Process == nil {
		return nil
	}
	_ = syscall.Kill(-s.command.Process.Pid, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- s.command.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		_ = syscall.Kill(-s.command.Process.Pid, syscall.SIGKILL)
		<-done
		return nil
	}
}

type rawQuotaResponse struct {
	RateLimits json.RawMessage            `json:"rateLimits"`
	Buckets    map[string]json.RawMessage `json:"rateLimitsByLimitId"`
	Credits    *struct {
		AvailableCount *int `json:"availableCount"`
		Credits        []struct {
			ID        string          `json:"id"`
			Status    string          `json:"status"`
			ResetType string          `json:"resetType"`
			ExpiresAt json.RawMessage `json:"expiresAt"`
		} `json:"credits"`
	} `json:"rateLimitResetCredits"`
}

type rawQuotaWindow struct {
	UsedPercent        json.Number `json:"usedPercent"`
	WindowDurationMins json.Number `json:"windowDurationMins"`
	ResetsAt           json.Number `json:"resetsAt"`
}

func ParseQuota(raw []byte) (model.Quota, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var response rawQuotaResponse
	if err := decoder.Decode(&response); err != nil {
		return model.Quota{}, ErrProtocol
	}
	if len(response.RateLimits) == 0 && len(response.Buckets) == 0 {
		return model.Quota{}, ErrProtocol
	}
	buckets := response.Buckets
	if len(buckets) == 0 {
		buckets = map[string]json.RawMessage{"": response.RateLimits}
	}
	bucketKeys := make([]string, 0, len(buckets))
	for k := range buckets {
		bucketKeys = append(bucketKeys, k)
	}
	sort.Strings(bucketKeys)
	quota := model.Quota{}
	for _, bucket := range bucketKeys {
		var windows map[string]json.RawMessage
		if json.Unmarshal(buckets[bucket], &windows) != nil {
			return model.Quota{}, ErrProtocol
		}
		keys := make([]string, 0, len(windows))
		for key, rawWindow := range windows {
			if string(rawWindow) == "null" {
				continue
			}
			var shape map[string]json.RawMessage
			if json.Unmarshal(rawWindow, &shape) == nil && shape["usedPercent"] != nil {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(i, j int) bool {
			order := func(key string) int {
				switch key {
				case "primary":
					return 0
				case "secondary":
					return 1
				default:
					return 2
				}
			}
			if order(keys[i]) != order(keys[j]) {
				return order(keys[i]) < order(keys[j])
			}
			return keys[i] < keys[j]
		})
		for _, key := range keys {
			var window rawQuotaWindow
			if json.Unmarshal(windows[key], &window) != nil {
				return model.Quota{}, ErrProtocol
			}
			used, err := window.UsedPercent.Float64()
			if err != nil || math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used > 100 {
				return model.Quota{}, ErrProtocol
			}
			duration, err := numberInt(window.WindowDurationMins)
			if window.WindowDurationMins != "" && (err != nil || duration <= 0) {
				return model.Quota{}, ErrProtocol
			}
			resetsAt, err := numberInt64(window.ResetsAt)
			if window.ResetsAt != "" && (err != nil || resetsAt <= 0) {
				return model.Quota{}, ErrProtocol
			}
			name := key
			if duration > 0 {
				name = durationLabel(duration)
			}
			reset := time.Time{}
			if resetsAt > 0 {
				reset = time.Unix(resetsAt, 0)
			}
			quota.Windows = append(quota.Windows, model.QuotaWindow{
				DurationMinutes: duration,
				Bucket:          bucket,
				Name:            name,
				UsedPercent:     used,
				ResetsAt:        reset,
			})
		}
	}
	if len(quota.Windows) == 0 {
		return model.Quota{}, ErrProtocol
	}
	if response.Credits != nil && response.Credits.AvailableCount != nil {
		if *response.Credits.AvailableCount < 0 {
			return model.Quota{}, ErrProtocol
		}
		quota.ResetCredits = *response.Credits.AvailableCount
		quota.ResetCreditsKnown = true
		// The count is authoritative; detail rows may be absent or capped.
		// Keep only expiry metadata, never opaque credit IDs or descriptions.
		seen := map[string]bool{}
		known, never := 0, 0
		for _, credit := range response.Credits.Credits {
			if quota.ResetCredits == 0 || credit.Status != "available" || credit.ResetType != "codexRateLimits" || credit.ID == "" || seen[credit.ID] {
				continue
			}
			seen[credit.ID] = true
			if string(credit.ExpiresAt) == "null" {
				known++
				never++
				continue
			}
			var unix int64
			if json.Unmarshal(credit.ExpiresAt, &unix) != nil || unix <= 0 || unix > 253402300799 {
				continue
			}
			known++
			expires := time.Unix(unix, 0)
			if quota.ResetCreditsExpireAt.IsZero() || expires.Before(quota.ResetCreditsExpireAt) {
				quota.ResetCreditsExpireAt = expires
			}
		}
		quota.ResetCreditsExpiryPartial = known != quota.ResetCredits
		quota.ResetCreditsNeverExpire = quota.ResetCredits > 0 && never == quota.ResetCredits && known == never
	}
	return quota, nil
}

func numberInt(number json.Number) (int, error) {
	value, err := strconv.ParseInt(string(number), 10, 32)
	return int(value), err
}

func numberInt64(number json.Number) (int64, error) {
	return strconv.ParseInt(string(number), 10, 64)
}

func durationLabel(minutes int) string {
	switch {
	case minutes%(24*60) == 0:
		return fmt.Sprintf("%d DAY", minutes/(24*60))
	case minutes%60 == 0:
		return fmt.Sprintf("%d HOUR", minutes/60)
	default:
		return fmt.Sprintf("%d MIN", minutes)
	}
}

func parseThreadPage(raw []byte) ([]ThreadMetadata, string, error) {
	var response struct {
		Data       []json.RawMessage `json:"data"`
		NextCursor string            `json:"nextCursor"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, "", ErrProtocol
	}
	if response.Data == nil {
		return nil, "", ErrProtocol
	}
	threads := make([]ThreadMetadata, 0, len(response.Data))
	for _, item := range response.Data {
		var thread struct {
			ID             string `json:"id"`
			SessionID      string `json:"sessionId"`
			Name           string `json:"name"`
			ParentThreadID string `json:"parentThreadId"`
		}
		if err := json.Unmarshal(item, &thread); err != nil {
			continue
		}
		if thread.ID == "" {
			continue
		}
		if strings.TrimSpace(thread.Name) == "" {
			thread.Name = "未命名任务"
		}
		threads = append(threads, ThreadMetadata{
			ID: thread.ID, SessionID: thread.SessionID, Title: strings.TrimSpace(thread.Name),
			ParentThreadID: thread.ParentThreadID,
		})
	}
	if len(response.Data) > 0 && len(threads) == 0 {
		return nil, "", ErrProtocol
	}
	return threads, response.NextCursor, nil
}
