package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config contains the runtime contract for the JD collector service.
type Config struct {
	ServiceName       string        `yaml:"service_name"`
	HTTPPort          string        `yaml:"http_port"`
	LogLevel          string        `yaml:"log_level"`
	PostgresDSN       string        `yaml:"postgres_dsn"`
	RedisAddr         string        `yaml:"redis_addr"`
	JDCollectorMode   string        `yaml:"jd_collector_mode"`
	ScheduleEnabled   bool          `yaml:"schedule_enabled"`
	ScheduleTime      string        `yaml:"schedule_time"`
	RunOnStartup      bool          `yaml:"run_on_startup"`
	RequestInterval   time.Duration `yaml:"-"`
	DefaultQueryLimit int           `yaml:"default_query_limit"`
	ReadTimeout       time.Duration `yaml:"-"`
	WriteTimeout      time.Duration `yaml:"-"`
	IdleTimeout       time.Duration `yaml:"-"`
}

type fileConfig struct {
	ServiceName       string `yaml:"service_name"`
	HTTPPort          string `yaml:"http_port"`
	LogLevel          string `yaml:"log_level"`
	PostgresDSN       string `yaml:"postgres_dsn"`
	RedisAddr         string `yaml:"redis_addr"`
	JDCollectorMode   string `yaml:"jd_collector_mode"`
	ScheduleEnabled   *bool  `yaml:"schedule_enabled"`
	ScheduleTime      string `yaml:"schedule_time"`
	RunOnStartup      bool   `yaml:"run_on_startup"`
	RequestInterval   string `yaml:"request_interval"`
	DefaultQueryLimit int    `yaml:"default_query_limit"`
	ReadTimeout       string `yaml:"read_timeout"`
	WriteTimeout      string `yaml:"write_timeout"`
	IdleTimeout       string `yaml:"idle_timeout"`
}

func DefaultPath() string {
	return filepath.Join("configs", "config.yaml")
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %s: %w", path, err)
	}

	var raw fileConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	readTimeout, err := parseDuration(raw.ReadTimeout, 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := parseDuration(raw.WriteTimeout, 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := parseDuration(raw.IdleTimeout, 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	requestInterval, err := parseDuration(raw.RequestInterval, 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ServiceName:       blankFallback(raw.ServiceName, "rigel-jd-collector"),
		HTTPPort:          blankFallback(raw.HTTPPort, "18081"),
		LogLevel:          blankFallback(raw.LogLevel, "info"),
		PostgresDSN:       blankFallback(raw.PostgresDSN, "postgres://rigel:rigel@postgres:5432/rigel?sslmode=disable"),
		RedisAddr:         blankFallback(raw.RedisAddr, "redis:6379"),
		JDCollectorMode:   blankFallback(raw.JDCollectorMode, "union"),
		ScheduleEnabled:   boolFallback(raw.ScheduleEnabled, true),
		ScheduleTime:      blankFallback(raw.ScheduleTime, "03:00"),
		RunOnStartup:      raw.RunOnStartup,
		RequestInterval:   requestInterval,
		DefaultQueryLimit: intFallback(raw.DefaultQueryLimit, 3),
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	if cfg.HTTPPort == "" {
		return Config{}, fmt.Errorf("http_port must not be empty")
	}
	if cfg.PostgresDSN == "" {
		return Config{}, fmt.Errorf("postgres_dsn must not be empty")
	}
	if err := validateScheduleTime(cfg.ScheduleTime); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseDuration(value string, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", value, err)
	}
	return parsed, nil
}

func validateScheduleTime(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("schedule_time must not be empty")
	}
	if _, err := time.Parse("15:04", value); err != nil {
		return fmt.Errorf("parse schedule_time %q: %w", value, err)
	}
	return nil
}

func blankFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func intFallback(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func boolFallback(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
