package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Wallet  WalletConfig  `yaml:"wallet"`
	RPC     RPCConfig     `yaml:"rpc"`
	Token   TokenConfig   `yaml:"token"`
	Monitor MonitorConfig `yaml:"monitor"`
	Jupiter JupiterConfig `yaml:"jupiter"`
	Logging LoggingConfig `yaml:"logging"`
}

type WalletConfig struct {
	PrivateKey    string  `yaml:"private_key"`
	MinSolBalance float64 `yaml:"min_sol_balance"`
	ReserveAmount float64 `yaml:"reserve_amount"`
}

type RPCConfig struct {
	Endpoint       string `yaml:"endpoint"`
	RetryAttempts  int    `yaml:"retry_attempts"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type TokenConfig struct {
	InputMint       string  `yaml:"input_mint" validate:"required"`
	OutputMint      string  `yaml:"output_mint" validate:"required"`
	DividendMint    string  `yaml:"dividend_mint"`
	SwapAmount      float64 `yaml:"swap_amount" validate:"required,gt=0"`
	SlippageBPS     uint64  `yaml:"slippage_bps" validate:"required,gt=0"`
	ProgramID       string  `yaml:"program_id" validate:"required"`
	RefreshCache    bool    `yaml:"refresh_cache"`
	CacheTTLMinutes int     `yaml:"cache_ttl_minutes" validate:"required,gt=0"`
}

type MonitorConfig struct {
	CheckIntervalMinutes int `yaml:"check_interval_minutes"`
	MaxRetries           int `yaml:"max_retries"`
	RetryDelaySeconds    int `yaml:"retry_delay_seconds"`
}

type JupiterConfig struct {
	QuoteEndpoint      string `yaml:"quote_endpoint"`
	SwapEndpoint       string `yaml:"swap_endpoint"`
	TokenAPIEndpoint   string `yaml:"token_api_endpoint"`
	TokenPriceEndpoint string `yaml:"token_price_endpoint"`
	OnlyDirectRoutes   bool   `yaml:"only_direct_routes"`
	StrictTokenList    bool   `yaml:"strict_token_list"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	FilePath   string `yaml:"file_path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

// LoadConfig loads the configuration from the specified YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse YAML
	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Validate config
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	// Create logs directory if it doesn't exist
	logsDir := filepath.Dir(config.Logging.FilePath)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating logs directory: %w", err)
	}

	return config, nil
}

// validateConfig performs basic validation of the configuration
func validateConfig(config *Config) error {
	if config.Wallet.PrivateKey == "" {
		return fmt.Errorf("wallet private key is required")
	}

	if config.RPC.Endpoint == "" {
		return fmt.Errorf("RPC endpoint is required")
	}

	if config.Token.InputMint == "" && config.Token.OutputMint == "" {
		return fmt.Errorf("at least one token mint address must be provided")
	}

	if config.Wallet.MinSolBalance <= 0 {
		return fmt.Errorf("minimum SOL balance must be greater than 0")
	}

	if config.Token.SwapAmount <= 0 {
		return fmt.Errorf("swap amount must be greater than 0")
	}

	if config.Monitor.CheckIntervalMinutes <= 0 {
		return fmt.Errorf("check interval must be greater than 0")
	}

	return nil
}
