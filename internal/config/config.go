package config

import (
	"time"

	"github.com/spf13/viper"
)

// SuricataCfg holds Suricata input settings.
type SuricataCfg struct {
	EveJSONPath       string `mapstructure:"eve_json_path"`
	FilestoreDir      string `mapstructure:"filestore_dir"`
	PathStrategy      string `mapstructure:"path_strategy"`      // "absolute" | "file_id"
	FileNamingPattern string `mapstructure:"file_naming_pattern"`
	UseDateSubdirs    bool   `mapstructure:"use_date_subdirs"`
	DateLayout        string `mapstructure:"date_layout"`
}

// UploaderCfg controls worker/backoff behavior.
type UploaderCfg struct {
	Workers    int    `mapstructure:"workers"`
	Prefix     string `mapstructure:"prefix"`
	MaxRetries int    `mapstructure:"max_retries"`
	BackoffMS  int    `mapstructure:"backoff_ms"`
}

// S3Cfg config.
type S3Cfg struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

// LoggingCfg controls output formatting and level.
type LoggingCfg struct {
	Level  string `mapstructure:"level"`  // debug|info|warn|error
	Format string `mapstructure:"format"` // json|console
}

// DedupeCfg controls persistent dedupe.
type DedupeCfg struct {
	Enabled       bool   `mapstructure:"enabled"`
	SQLitePath    string `mapstructure:"sqlite_path"`
	RetentionDays int    `mapstructure:"retention_days"`
}

// Config is the root configuration.
type Config struct {
	Suricata SuricataCfg `mapstructure:"suricata"`
	Uploader UploaderCfg `mapstructure:"uploader"`
	S3       S3Cfg       `mapstructure:"s3"`
	Logging  LoggingCfg  `mapstructure:"logging"`
	Dedupe   DedupeCfg   `mapstructure:"dedupe"`
}

// Load reads config from a file.
func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("SURISINK")
	v.AutomaticEnv()

	v.SetDefault("uploader.workers", 4)
	v.SetDefault("uploader.prefix", "suricata")
	v.SetDefault("uploader.max_retries", 5)
	v.SetDefault("uploader.backoff_ms", 500)

	v.SetDefault("suricata.path_strategy", "file_id")
	v.SetDefault("suricata.file_naming_pattern", "file.%d")
	v.SetDefault("suricata.use_date_subdirs", false)
	v.SetDefault("suricata.date_layout", "2006/01/02")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	v.SetDefault("dedupe.enabled", false)
	v.SetDefault("dedupe.sqlite_path", "./data/surisink.db")
	v.SetDefault("dedupe.retention_days", 0)

	var c Config
	if err := v.ReadInConfig(); err != nil { return c, err }
	if err := v.Unmarshal(&c); err != nil { return c, err }
	return c, nil
}

// BackoffDuration computes a linear backoff.
func BackoffDuration(ms int, attempt int) time.Duration {
	if ms <= 0 { ms = 250 }
	if attempt < 1 { attempt = 1 }
	return time.Duration(ms*attempt) * time.Millisecond
}
