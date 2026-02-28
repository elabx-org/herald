package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Timestamp   time.Time `json:"ts"`
	Action      string    `json:"action"`
	Stack       string    `json:"stack"`
	Secret      string    `json:"secret"`
	Provider    string    `json:"provider"`
	Delivery    []string  `json:"delivery,omitempty"`
	Policy      string    `json:"policy"`
	CacheHit    bool      `json:"cache_hit"`
	DurationMs  int64     `json:"duration_ms"`
	TriggeredBy string    `json:"triggered_by,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type QueryOptions struct {
	Stack  string
	Secret string
	Hours  int
}

type Logger struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

func New(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f, path: path}, nil
}

func (l *Logger) Close() error { return l.f.Close() }

func (l *Logger) Log(e Entry) {
	e.Timestamp = time.Now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	data, _ := json.Marshal(e)
	l.f.Write(append(data, '\n'))
}

func (l *Logger) Query(opts QueryOptions) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cutoff := time.Now().Add(-time.Duration(opts.Hours) * time.Hour)
	if opts.Hours == 0 {
		cutoff = time.Time{}
	}

	var results []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if opts.Stack != "" && !strings.EqualFold(e.Stack, opts.Stack) {
			continue
		}
		if opts.Secret != "" && !strings.EqualFold(e.Secret, opts.Secret) {
			continue
		}
		if !cutoff.IsZero() && e.Timestamp.Before(cutoff) {
			continue
		}
		results = append(results, e)
	}
	return results, scanner.Err()
}
