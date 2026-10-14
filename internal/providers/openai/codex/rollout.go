package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zzzblxx/glance/internal/model"
)

var ErrActivityUnavailable = errors.New("codex task activity unavailable")

type LifecycleEvent struct {
	ExternalID string
	State      model.ActivityState
	At         time.Time
	SeenAt     time.Time
}

type RolloutObserver struct {
	CodexHome string
	Now       func() time.Time
	cache     map[string]*rolloutCache
}

type rolloutCache struct {
	info         fs.FileInfo
	offset       int64
	identity     string
	latest       LifecycleEvent
	incompatible bool
}

func (observer *RolloutObserver) ReadEvents() ([]LifecycleEvent, error) {
	if observer.cache == nil {
		observer.cache = make(map[string]*rolloutCache)
	}
	now := time.Now()
	if observer.Now != nil {
		now = observer.Now()
	}
	cutoff := now.Add(-model.EndedTaskRetention - time.Hour)
	var paths []string
	for _, folder := range []string{"sessions", "archived_sessions"} {
		root := filepath.Join(observer.CodexHome, folder)
		if folder == "archived_sessions" {
			if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
			if walkError != nil {
				return walkError
			}
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.ModTime().Before(cutoff) {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, ErrActivityUnavailable
		}
	}
	sort.Strings(paths)
	var events []LifecycleEvent
	incompatible := false
	seenPaths := make(map[string]bool)
	for _, path := range paths {
		seenPaths[path] = true
		entry := observer.cache[path]
		if entry == nil {
			entry = &rolloutCache{}
			observer.cache[path] = entry
		}
		err := entry.read(path)
		if err != nil {
			return nil, ErrActivityUnavailable
		}
		if entry.latest.ExternalID != "" {
			events = append(events, entry.latest)
		}
		incompatible = incompatible || entry.incompatible
	}
	for path := range observer.cache {
		if !seenPaths[path] {
			delete(observer.cache, path)
		}
	}
	if incompatible {
		return nil, ErrActivityUnavailable
	}
	return events, nil
}

type rolloutEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetadata struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
}

type lifecyclePayload struct {
	Type        string          `json:"type"`
	StartedAt   json.RawMessage `json:"started_at"`
	CompletedAt json.RawMessage `json:"completed_at"`
	Reason      string          `json:"reason"`
	Error       json.RawMessage `json:"error"`
}

func parseRollout(path string) ([]LifecycleEvent, bool, error) {
	c := &rolloutCache{}
	err := c.read(path)
	if c.latest.ExternalID == "" {
		return nil, c.incompatible, err
	}
	return []LifecycleEvent{c.latest}, c.incompatible, err
}

func (c *rolloutCache) read(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if c.info != nil && (!os.SameFile(c.info, info) || info.Size() < c.offset) {
		*c = rolloutCache{}
	}
	if c.info != nil && info.Size() == c.offset {
		return nil
	}
	if _, err = file.Seek(c.offset, io.SeekStart); err != nil {
		return err
	}
	c.info = info
	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		// A partially appended JSON line is retried on the next observation.
		line, consumed, err := boundedLine(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		c.offset += consumed
		if line == nil {
			continue
		}
		var envelope rolloutEnvelope
		if json.Unmarshal(line, &envelope) != nil {
			c.incompatible = true
			continue
		}
		switch envelope.Type {
		case "session_meta":
			var metadata sessionMetadata
			if json.Unmarshal(envelope.Payload, &metadata) != nil {
				continue
			}
			c.identity = metadata.ID
		case "event_msg":
			var payload lifecyclePayload
			if json.Unmarshal(envelope.Payload, &payload) != nil {
				continue
			}
			state, timestampRaw, recognized := lifecycleState(payload)
			if !recognized {
				// Fresh, recognized execution evidence extends running freshness only.
				switch payload.Type {
				case "token_count", "agent_message", "agent_reasoning", "exec_command_end", "mcp_tool_call_end", "patch_apply_end":
					if c.latest.State == model.ActivityRunning && envelope.Timestamp.After(c.latest.SeenAt) {
						c.latest.SeenAt = envelope.Timestamp
					}
				}
				continue
			}
			at, ok := parseEpoch(timestampRaw)
			if !ok || c.identity == "" {
				c.incompatible = true
				continue
			}
			if !at.Before(c.latest.At) {
				c.latest = LifecycleEvent{ExternalID: c.identity, State: state, At: at, SeenAt: at}
				c.incompatible = false
			}
		}
	}
	return nil
}

// Large conversation records are skipped without retaining their payload.
// Lifecycle envelopes are small. An unfinished line never advances the offset.
func boundedLine(r *bufio.Reader) ([]byte, int64, error) {
	var result []byte
	var n int64
	tooLarge := false
	for {
		fragment, err := r.ReadSlice('\n')
		n += int64(len(fragment))
		if n > 1024*1024 {
			tooLarge = true
			result = nil
		}
		if !tooLarge {
			result = append(result, fragment...)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return result, n, err
	}
}

func lifecycleState(payload lifecyclePayload) (model.ActivityState, json.RawMessage, bool) {
	switch payload.Type {
	case "task_started":
		return model.ActivityRunning, payload.StartedAt, true
	case "task_complete":
		if len(payload.Error) > 0 && string(payload.Error) != "null" {
			return model.ActivityFailed, payload.CompletedAt, true
		}
		return model.ActivityCompleted, payload.CompletedAt, true
	case "turn_aborted":
		if payload.Reason == "interrupted" || payload.Reason == "cancelled" {
			return model.ActivityInterrupted, payload.CompletedAt, true
		}
	}
	return "", nil, false
}

func parseEpoch(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, false
	}
	var number json.Number
	if raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return time.Time{}, false
		}
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed, true
		}
		number = json.Number(text)
	} else {
		number = json.Number(string(raw))
	}
	seconds, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

// CorrelateTasks joins official titles with lifecycle facts and folds subagent
// activity into its top-level task. It never derives titles from rollout data.
func CorrelateTasks(threads []ThreadMetadata, events []LifecycleEvent) []model.Task {
	threadByID := make(map[string]ThreadMetadata, len(threads))
	for _, thread := range threads {
		threadByID[thread.ID] = thread
	}
	latestEvent := make(map[string]LifecycleEvent)
	for _, event := range events {
		current, exists := latestEvent[event.ExternalID]
		if !exists || !event.At.Before(current.At) {
			latestEvent[event.ExternalID] = event
		}
	}
	tasks := make(map[string]model.Task)
	for _, thread := range threads {
		event, found := newestEvent(latestEvent, thread.ID)
		if !found {
			continue
		}
		top := thread
		visited := map[string]struct{}{thread.ID: {}}
		for top.ParentThreadID != "" {
			parent, exists := threadByID[top.ParentThreadID]
			if !exists {
				break
			}
			if _, loop := visited[parent.ID]; loop {
				break
			}
			visited[parent.ID] = struct{}{}
			top = parent
		}
		// Child terminal events never finish a parent. A running child can only
		// refresh an already-running parent's evidence for the same execution.
		if top.ID != thread.ID {
			parentEvent, ok := latestEvent[top.ID]
			if !ok || parentEvent.State != model.ActivityRunning || event.State != model.ActivityRunning || event.At.Before(parentEvent.At) {
				continue
			}
			event.At = parentEvent.At
		}
		if thread.ParentThreadID != "" && top.ID == thread.ID {
			continue
		}
		candidate := model.Task{ID: top.ID, Title: top.Title, State: event.State, ActivityAt: event.At, SeenAt: event.SeenAt}
		current, exists := tasks[top.ID]
		if !exists || candidate.ActivityAt.After(current.ActivityAt) || (candidate.ActivityAt.Equal(current.ActivityAt) && candidate.SeenAt.After(current.SeenAt)) {
			tasks[top.ID] = candidate
		}
	}
	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, task)
	}
	return result
}

func newestEvent(events map[string]LifecycleEvent, ids ...string) (LifecycleEvent, bool) {
	var newest LifecycleEvent
	found := false
	for _, id := range ids {
		event, exists := events[id]
		if exists && (!found || event.At.After(newest.At)) {
			newest, found = event, true
		}
	}
	return newest, found
}
