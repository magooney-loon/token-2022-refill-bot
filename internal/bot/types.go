package bot

import (
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"

	"github.com/gagliardetto/solana-go"
)

// Bot represents the main bot instance
type Bot struct {
	config    *config.Config
	wallet    *solana.Wallet
	isRunning bool
	stopChan  chan struct{}
	errorChan chan error
	stateChan chan State
	trader    *Trader
}

// State represents the current bot state
type State struct {
	CurrentBalance float64
	LastSwapAmount float64
	LastSwapTime   time.Time
	TotalSwaps     int64
	Errors         int64
	Status         Status
}

// Status represents the bot's operational status
type Status string

const (
	StatusIdle     Status = "IDLE"
	StatusChecking Status = "CHECKING"
	StatusSwapping Status = "SWAPPING"
	StatusError    Status = "ERROR"
	StatusStopped  Status = "STOPPED"
)

// SwapResult contains information about a completed swap
type SwapResult struct {
	InputAmount  float64
	OutputAmount float64
	Fee          float64
	Timestamp    time.Time
	TxSignature  string
	Route        string
	PriceImpact  float64
}

// BalanceCheck contains information about a balance check
type BalanceCheck struct {
	Balance   float64
	Timestamp time.Time
	MetTarget bool
	Error     error
}

// BotOption represents a function that can configure a Bot
type BotOption func(*Bot)

// WithInitialState sets the initial state for the bot
func WithInitialState(state State) BotOption {
	return func(b *Bot) {
		b.stateChan <- state
	}
}

// WithErrorBuffer sets the error channel buffer size
func WithErrorBuffer(size int) BotOption {
	return func(b *Bot) {
		b.errorChan = make(chan error, size)
	}
}

// BotStats contains statistics about the bot's operation
type BotStats struct {
	StartTime        time.Time
	TotalSwaps       int64
	SuccessfulSwaps  int64
	FailedSwaps      int64
	TotalVolume      float64
	AverageSlippage  float64
	HighestSlippage  float64
	AverageFee       float64
	TotalFees        float64
	UptimePercentage float64
}
