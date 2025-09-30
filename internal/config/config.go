package config

import (
	"fmt"
	"time"
	
	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Broker     BrokerConfig     `mapstructure:"broker"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Tracing    TracingConfig    `mapstructure:"tracing"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
}

type ServerConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	MaxRequestSize    int64         `mapstructure:"max_request_size"`
	EnableWebSocket   bool          `mapstructure:"enable_websocket"`
	TLS               TLSConfig     `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

type StorageConfig struct {
	Type            string        `mapstructure:"type"` // memory, redis, postgres
	MaxMessageSize  int64         `mapstructure:"max_message_size"`
	MaxQueueSize    int           `mapstructure:"max_queue_size"`
	RetentionTime   time.Duration `mapstructure:"retention_time"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

type RedisConfig struct {
	Addresses          []string      `mapstructure:"addresses"`
	Password           string        `mapstructure:"password"`
	DB                 int           `mapstructure:"db"`
	MaxRetries         int           `mapstructure:"max_retries"`
	PoolSize           int           `mapstructure:"pool_size"`
	MinIdleConns       int           `mapstructure:"min_idle_conns"`
	DialTimeout        time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	PoolTimeout        time.Duration `mapstructure:"pool_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	IdleCheckFrequency time.Duration `mapstructure:"idle_check_frequency"`
	EnableCluster      bool          `mapstructure:"enable_cluster"`
}

type BrokerConfig struct {
	DefaultPartitions     int32         `mapstructure:"default_partitions"`
	DefaultReplication    int32         `mapstructure:"default_replication"`
	MaxMessageBatchSize   int           `mapstructure:"max_message_batch_size"`
	BatchFlushInterval    time.Duration `mapstructure:"batch_flush_interval"`
	CompressionType       string        `mapstructure:"compression_type"` // none, snappy, gzip, lz4
	CompressionLevel      int           `mapstructure:"compression_level"`
	EnableDeduplication   bool          `mapstructure:"enable_deduplication"`
	DeduplicationWindow   time.Duration `mapstructure:"deduplication_window"`
	DefaultAckDeadline    time.Duration `mapstructure:"default_ack_deadline"`
	MaxRetries            int           `mapstructure:"max_retries"`
	RetryBackoff          time.Duration `mapstructure:"retry_backoff"`
	EnableDLQ             bool          `mapstructure:"enable_dlq"`
	DLQPrefix             string        `mapstructure:"dlq_prefix"`
}

type MetricsConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	Port              int           `mapstructure:"port"`
	Path              string        `mapstructure:"path"`
	CollectInterval   time.Duration `mapstructure:"collect_interval"`
	Namespace         string        `mapstructure:"namespace"`
}

type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	ServiceName  string  `mapstructure:"service_name"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	SamplingRate float64 `mapstructure:"sampling_rate"`
}

type RateLimitConfig struct {
	Enabled          bool  `mapstructure:"enabled"`
	RequestsPerSecond int  `mapstructure:"requests_per_second"`
	BurstSize        int   `mapstructure:"burst_size"`
	PerTopic         bool  `mapstructure:"per_topic"`
	PerConsumer      bool  `mapstructure:"per_consumer"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	
	// Set defaults
	setDefaults()
	
	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("PUBSUB")
	
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}
	
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	return &cfg, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.shutdown_timeout", "30s")
	viper.SetDefault("server.max_request_size", 8<<20) // 8MB
	viper.SetDefault("server.enable_websocket", true)
	
	// Storage defaults
	viper.SetDefault("storage.type", "memory")
	viper.SetDefault("storage.max_message_size", 1<<20) // 1MB
	viper.SetDefault("storage.max_queue_size", 10000)
	viper.SetDefault("storage.retention_time", "24h")
	viper.SetDefault("storage.cleanup_interval", "1h")
	
	// Redis defaults
	viper.SetDefault("redis.addresses", []string{"localhost:6379"})
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.pool_timeout", "4s")
	viper.SetDefault("redis.idle_timeout", "5m")
	viper.SetDefault("redis.idle_check_frequency", "1m")
	
	// Broker defaults
	viper.SetDefault("broker.default_partitions", 1)
	viper.SetDefault("broker.default_replication", 1)
	viper.SetDefault("broker.max_message_batch_size", 1000)
	viper.SetDefault("broker.batch_flush_interval", "1s")
	viper.SetDefault("broker.compression_type", "none")
	viper.SetDefault("broker.compression_level", 6)
	viper.SetDefault("broker.default_ack_deadline", "30s")
	viper.SetDefault("broker.max_retries", 3)
	viper.SetDefault("broker.retry_backoff", "1s")
	viper.SetDefault("broker.enable_dlq", true)
	viper.SetDefault("broker.dlq_prefix", "dlq-")
	
	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 9091)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.collect_interval", "10s")
	viper.SetDefault("metrics.namespace", "pubsubgo")
	
	// Tracing defaults
	viper.SetDefault("tracing.enabled", false)
	viper.SetDefault("tracing.service_name", "pubsubgo")
	viper.SetDefault("tracing.sampling_rate", 0.1)
	
	// Rate limit defaults
	viper.SetDefault("rate_limit.enabled", false)
	viper.SetDefault("rate_limit.requests_per_second", 1000)
	viper.SetDefault("rate_limit.burst_size", 100)
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	
	if c.Storage.Type != "memory" && c.Storage.Type != "redis" && c.Storage.Type != "postgres" {
		return fmt.Errorf("unsupported storage type: %s", c.Storage.Type)
	}
	
	if c.Storage.MaxMessageSize <= 0 {
		return fmt.Errorf("max message size must be positive")
	}
	
	if c.Broker.DefaultPartitions <= 0 {
		return fmt.Errorf("default partitions must be positive")
	}
	
	return nil
}