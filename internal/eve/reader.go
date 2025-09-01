package eve

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luhtaf/surisink/internal/config"
	"github.com/luhtaf/surisink/internal/log"
)

// FileEvent represents a single stored file event from Suricata.
type FileEvent struct {
	When     time.Time
	FileID   int64
	Filename string // original filename if present
	Stored   bool
	SrcIP    string
	DstIP    string
	FlowID   string
	Path     string // resolved local path to stored file
}

type rawEvent struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"event_type"`
	SrcIP     string `json:"src_ip"`
	DstIP     string `json:"dst_ip"`
	FlowID    uint64 `json:"flow_id"`
	Filename  string `json:"filename"`
	SHA256    string `json:"sha256"`
	FileID    int64  `json:"file_id"`
	Fileinfo  struct {
		Filename string `json:"filename"`
		Stored   bool   `json:"stored"`
		FileID   int64  `json:"file_id"`
		SHA256   string `json:"sha256"`
		Size     int64  `json:"size"`
	} `json:"fileinfo"`
}

// Reader tails eve.json and emits FileEvent for stored files.
type Reader struct {
	cfg config.SuricataCfg
	out chan FileEvent
}

// NewReader constructs a Reader.
func NewReader(cfg config.SuricataCfg) *Reader {
	return &Reader{cfg: cfg, out: make(chan FileEvent, 1024)}
}

// Events returns the consumer channel.
func (r *Reader) Events() <-chan FileEvent { return r.out }

// Start begins tailing eve.json from EOF.
func (r *Reader) Start(ctx context.Context) error {
	f, err := os.Open(r.cfg.EveJSONPath)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, os.SEEK_END); err != nil {
		return err
	}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	go func() {
		defer f.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if s.Scan() {
					line := strings.TrimSpace(s.Text())
					if line == "" {
						continue
					}
					if fe, ok := r.parseLine(line); ok {
						select {
						case r.out <- fe:
						default:
							log.L.Warn("eve events channel full; dropping")
						}
					}
					continue
				}
				if err := s.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
					log.L.Warnw("eve scanner", "err", err)
				}
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()
	return nil
}

func (r *Reader) parseLine(line string) (FileEvent, bool) {
	var ev rawEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return FileEvent{}, false
	}
	if ev.EventType != "fileinfo" {
		return FileEvent{}, false
	}
	stored := ev.Fileinfo.Stored
	when := time.Now()
	if ts := ev.Timestamp; ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			when = t
		}
	}
	fid := ev.Fileinfo.FileID
	if fid == 0 {
		fid = ev.FileID
	}
	orig := ev.Fileinfo.Filename
	if orig == "" {
		orig = ev.Filename
	}

	path := r.resolvePath(when, fid, orig)
	return FileEvent{
		When:     when,
		FileID:   fid,
		Filename: orig,
		Stored:   stored,
		SrcIP:    ev.SrcIP,
		DstIP:    ev.DstIP,
		FlowID:   fmt.Sprintf("%d", ev.FlowID),
		Path:     path,
	}, stored
}

func (r *Reader) resolvePath(t time.Time, fileID int64, orig string) string {
	switch r.cfg.PathStrategy {
	case "absolute":
		if filepath.IsAbs(orig) {
			return orig
		}
		return ""
	default: // file_id
		name := fmt.Sprintf(r.cfg.FileNamingPattern, fileID)
		if r.cfg.UseDateSubdirs {
			sub := t.Format(r.cfg.DateLayout)
			return filepath.Join(r.cfg.FilestoreDir, sub, name)
		}
		return filepath.Join(r.cfg.FilestoreDir, name)
	}
}
