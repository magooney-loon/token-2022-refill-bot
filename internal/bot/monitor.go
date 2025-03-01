package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Monitor handles balance checking and triggers swaps
type Monitor struct {
	rpcClient     *rpc.Client
	wallet        *solana.Wallet
	minBalance    float64
	checkInterval time.Duration
	stopChan      chan struct{}
	resultChan    chan BalanceCheck
}

// NewMonitor creates a new balance monitor
func NewMonitor(
	rpcClient *rpc.Client,
	wallet *solana.Wallet,
	minBalance float64,
	checkInterval time.Duration,
) *Monitor {
	return &Monitor{
		rpcClient:     rpcClient,
		wallet:        wallet,
		minBalance:    minBalance,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
		resultChan:    make(chan BalanceCheck, 1),
	}
}

// Start begins the balance monitoring process
func (m *Monitor) Start(ctx context.Context) error {
	utils.Info("Starting balance monitor",
		"wallet", m.wallet.PublicKey().String(),
		"minBalance", m.minBalance,
		"interval", m.checkInterval)

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Do initial check
	if err := m.checkBalance(ctx); err != nil {
		utils.Error("Initial balance check failed", err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			utils.Info("Monitor stopped by context")
			return ctx.Err()
		case <-m.stopChan:
			utils.Info("Monitor stopped by request")
			return nil
		case <-ticker.C:
			if err := m.checkBalance(ctx); err != nil {
				utils.Error("Balance check failed", err)
				// Don't return, keep trying
			}
		}
	}
}

// Stop halts the monitoring process
func (m *Monitor) Stop() {
	close(m.stopChan)
}

// GetResultChannel returns the channel for balance check results
func (m *Monitor) GetResultChannel() <-chan BalanceCheck {
	return m.resultChan
}

// checkBalance performs a single balance check
func (m *Monitor) checkBalance(ctx context.Context) error {
	utils.Debug("Checking SOL balance", "wallet", m.wallet.PublicKey().String())

	balance, err := m.rpcClient.GetBalance(
		ctx,
		m.wallet.PublicKey(),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// Convert lamports to SOL
	solBalance := float64(balance.Value) / float64(solana.LAMPORTS_PER_SOL)

	result := BalanceCheck{
		Balance:   solBalance,
		Timestamp: time.Now(),
		MetTarget: solBalance >= m.minBalance,
		Error:     nil,
	}

	utils.Debug("Balance check complete",
		"balance", solBalance,
		"minBalance", m.minBalance,
		"metTarget", result.MetTarget)

	// Send result on channel (non-blocking)
	select {
	case m.resultChan <- result:
	default:
		utils.Warn("Result channel full, skipping update")
	}

	return nil
}

// UpdateMinBalance updates the minimum balance threshold
func (m *Monitor) UpdateMinBalance(newMin float64) {
	m.minBalance = newMin
	utils.Info("Updated minimum balance threshold", "newMin", newMin)
}

// UpdateCheckInterval updates the check interval
func (m *Monitor) UpdateCheckInterval(newInterval time.Duration) {
	m.checkInterval = newInterval
	utils.Info("Updated check interval", "newInterval", newInterval)
}
