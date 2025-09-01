package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/luhtaf/surisink/internal/config"
	"github.com/luhtaf/surisink/internal/dedupe"
	"github.com/luhtaf/surisink/internal/eve"
	"github.com/luhtaf/surisink/internal/log"
	"github.com/luhtaf/surisink/internal/meta"
	"github.com/luhtaf/surisink/internal/uploader"
)

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", os.Getenv("CONFIG_PATH"), "Path to YAML config file")
	flag.Parse()
	if cfgPath == "" { log.L.Fatal("CONFIG_PATH env or --config must be set") }

	cfg, err := config.Load(cfgPath)
	if err != nil { panic(err) }
	if err := log.InitWithConfig(cfg.Logging.Level, cfg.Logging.Format); err != nil { panic(err) }
	defer log.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	r := eve.NewReader(cfg.Suricata)
	if err := r.Start(ctx); err != nil { log.L.Fatalw("eve reader", "err", err) }

	
	sqlDeduper := (*dedupe.SQLite)(nil)
	if cfg.Dedupe.Enabled {
		var err error
		sqlDeduper, err = dedupe.OpenSQLite(cfg.Dedupe.SQLitePath)
		if err != nil { log.L.Fatalw("dedupe open sqlite", "err", err, "path", cfg.Dedupe.SQLitePath) }
		defer sqlDeduper.Close()
	}

	u, err := uploader.New(cfg.S3.Endpoint, cfg.S3.AccessKey, cfg.S3.SecretKey, cfg.S3.Bucket, cfg.Uploader.Prefix, cfg.S3.UseSSL)
	if err != nil { log.L.Fatalw("uploader", "err", err) }
	if err := u.EnsureBucket(ctx); err != nil { log.L.Fatalw("ensure bucket", "err", err) }

		dedBackend := interface{}(nil)
	if cfg.Dedupe.Enabled {
		if sqlDed, err := dedupe.NewSQLite(cfg.Dedupe.SQLitePath); err == nil {
			dedBackend = sqlDed
			log.L.Infow("dedupe_sqlite_enabled", "path", cfg.Dedupe.SQLitePath)
		} else {
			log.L.Warnw("dedupe_sqlite_error", "err", err)
		}
	} else {
		dedBackend = dedupe.NewInMemory()
	}


	workers := cfg.Uploader.Workers
	if workers < 1 { workers = 1 }
	log.L.Infow("starting workers", "n", workers)

	jobs := make(chan eve.FileEvent, 1024)
	// fan-in
	go func() { for ev := range r.Events() { log.L.Debugw("eve_received", "event", "eve_received", "flow_id", ev.FlowID, "file_id", ev.FileID, "filename", ev.Filename); jobs <- ev } }()

	for i := 0; i < workers; i++ {
		go func() {
			for fe := range jobs {
				process(ctx, cfg, u, ded, fe)
			}
		}()
	}

	<-ctx.Done()
	log.L.Info("shutting down")
}

func process(ctx context.Context, cfg config.Config, u *uploader.Uploader, dedBackend interface{}, fe eve.FileEvent) {
	// If path doesn't exist, skip with warning (adjust config strategy)
	if fe.Path == "" { log.L.Warnw("no path resolved", "file_id", fe.FileID, "filename", fe.Filename); return }
	if _, err := os.Stat(fe.Path); err != nil {
		log.L.Warnw("file not found", "path", fe.Path, "err", err)
		return
	}
	hash, size, err := meta.HashSHA256(fe.Path)
	if err != nil { log.L.Warnw("hash", "path", fe.Path, "err", err); return }
	switch d := dedBackend.(type) {
	case *dedupe.InMemory:
		if d.Seen(hash) { log.L.Infow("skip_duplicate", "sha256", hash); return }
	case *dedupe.SQLite:
		if ok, _ := d.Seen(hash); ok { log.L.Infow("skip_duplicate", "sha256", hash); return }
	}
	fm := meta.FileMeta{
		Path: fe.Path,
		OrigName: filepath.Base(fe.Filename),
		SHA256: hash,
		MIME: meta.GuessMIME(fe.Filename),
		FlowID: fe.FlowID,
		SrcIP: fe.SrcIP,
		DstIP: fe.DstIP,
		TS: fe.When,
		Size: size,
	}
	// simple retry
	var lastErr error
	for attempt := 1; attempt <= cfg.Uploader.MaxRetries; attempt++ {
		if key, err := u.UploadFile(ctx, fm); err == nil {
			log.L.Infow("upload_success",
		"event", "upload_success",
		"component", "surisink",
		"key", key,
		"bucket", cfg.S3.Bucket,
		"sha256", fm.SHA256,
		"size", fm.Size,
		"mime", fm.MIME,
		"src", fm.SrcIP,
		"dst", fm.DstIP,
		"flow_id", fm.FlowID,
		"ts_event", fm.TS.UTC().Format(time.RFC3339),
		"attempt", attempt,
		)
			switch d := dedBackend.(type) {
	case *dedupe.InMemory:
		d.Mark(fm.SHA256)
	case *dedupe.SQLite:
		_ = d.Mark(fm.SHA256, key, fm.MIME, fm.Size)
	}
			return
		} else {
			lastErr = err
			d := config.BackoffDuration(cfg.Uploader.BackoffMS, attempt)
			log.L.Warnw("upload_retry",
		"event", "upload_retry",
		"component", "surisink",
		"sha256", fm.SHA256,
		"attempt", attempt,
		"delay", d.String(),
		"err", err,
		)
			time.Sleep(d)
		}
	}
	log.L.Errorw("upload_failed",
		"event", "upload_failed",
		"component", "surisink",
		"sha256", fm.SHA256,
		"err", lastErr,
		"path", fe.Path,
		)
}
