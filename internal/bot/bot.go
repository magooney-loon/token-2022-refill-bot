package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/jupiter"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// NewBot creates a new bot instance
func NewBot(cfg *config.Config, options ...BotOption) (*Bot, error) {
	// Create wallet from private key
	privateKey, err := solana.PrivateKeyFromBase58(cfg.Wallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}
	wallet := &solana.Wallet{PrivateKey: privateKey}

	bot := &Bot{
		config:    cfg,
		wallet:    wallet,
		isRunning: false,
		stopChan:  make(chan struct{}),
		errorChan: make(chan error, 10),
		stateChan: make(chan State, 1),
	}

	// Apply options
	for _, opt := range options {
		opt(bot)
	}

	return bot, nil
}

// Start begins the bot's operation
func (b *Bot) Start(ctx context.Context) error {
	if b.isRunning {
		return fmt.Errorf("bot is already running")
	}

	utils.Info("Starting bot",
		"wallet", b.wallet.PublicKey().String(),
		"token", b.config.Token.InputMint)

	// Create RPC client
	rpcClient := rpc.New(b.config.RPC.Endpoint)

	// Create token2022 client
	tokenClient := token2022.NewClient(b.config, rpcClient)

	// Create Jupiter client
	jupiterClient := jupiter.NewClient(&b.config.Jupiter, tokenClient, b.config.Token.RefreshCache)

	// Create trader
	b.trader = NewTrader(b.config, jupiterClient, rpcClient, b.wallet)

	// Create monitor
	monitor := NewMonitor(
		rpcClient,
		b.wallet,
		b.config.Wallet.MinSolBalance,
		time.Duration(b.config.Monitor.CheckIntervalMinutes)*time.Minute,
	)

	// Start monitor in background
	monitorCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := monitor.Start(monitorCtx); err != nil {
			b.errorChan <- fmt.Errorf("monitor error: %w", err)
		}
	}()

	// Initialize state
	b.updateState(State{
		Status: StatusIdle,
	})

	b.isRunning = true

	// Main loop
	for {
		select {
		case <-ctx.Done():
			utils.Info("Bot stopped by context")
			b.isRunning = false
			return ctx.Err()

		case <-b.stopChan:
			utils.Info("Bot stopped by request")
			b.isRunning = false
			return nil

		case result := <-monitor.GetResultChannel():
			if err := b.handleBalanceCheck(ctx, result); err != nil {
				utils.Error("Failed to handle balance check", err)
				b.errorChan <- err
			}

		case err := <-b.errorChan:
			utils.Error("Bot error", err)
			b.updateState(State{
				Status: StatusError,
				Errors: b.getState().Errors + 1,
			})
		}
	}
}

// Stop halts the bot's operation
func (b *Bot) Stop() {
	if b.isRunning {
		close(b.stopChan)
	}
}

// GetState returns the current bot state
func (b *Bot) GetState() State {
	return b.getState()
}

// handleBalanceCheck processes a balance check result
func (b *Bot) handleBalanceCheck(ctx context.Context, check BalanceCheck) error {
	if check.Error != nil {
		return fmt.Errorf("balance check error: %w", check.Error)
	}

	state := b.getState()
	state.CurrentBalance = check.Balance
	state.Status = StatusChecking
	state.LastSwapTime = check.Timestamp
	b.updateState(state)

	if check.MetTarget {
		utils.Info("Balance threshold met, initiating swap",
			"balance", check.Balance,
			"threshold", b.config.Wallet.MinSolBalance,
			"swap_amount", b.config.Token.SwapAmount)

		state.Status = StatusSwapping
		b.updateState(state)

		// Execute swap using trader
		if err := b.trader.ExecuteSwap(ctx, check.Balance); err != nil {
			state.Status = StatusError
			state.Errors++
			b.updateState(state)
			return fmt.Errorf("swap failed: %w", err)
		}

		state.Status = StatusIdle
		state.TotalSwaps++
		state.LastSwapAmount = b.config.Token.SwapAmount
		state.LastSwapTime = time.Now()
		b.updateState(state)
	}

	return nil
}

// getState safely retrieves the current state
func (b *Bot) getState() State {
	select {
	case state := <-b.stateChan:
		b.stateChan <- state
		return state
	default:
		return State{Status: StatusIdle}
	}
}

// updateState safely updates the bot state
func (b *Bot) updateState(state State) {
	// Clear channel
	select {
	case <-b.stateChan:
	default:
	}
	// Update state
	b.stateChan <- state
}

// GetStats returns the current bot statistics
func (b *Bot) GetStats() BotStats {
	state := b.getState()
	return BotStats{
		StartTime:       time.Now(), // TODO: Track actual start time
		TotalSwaps:      state.TotalSwaps,
		SuccessfulSwaps: state.TotalSwaps - state.Errors,
		FailedSwaps:     state.Errors,
		// Other stats to be implemented
	}
}
