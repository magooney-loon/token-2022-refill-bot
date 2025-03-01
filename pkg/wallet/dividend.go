package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type DividendInfo struct {
	TotalAmount   float64
	LastReceived  time.Time
	TransferCount int
	TokenInfo     *token2022.TokenInfo
	USDValue      float64
	Last24hAmount float64
	Last7dAmount  float64
	Last30dAmount float64
}

type DividendCache struct {
	LastUpdate    time.Time                    `json:"last_update"`
	LastSignature string                       `json:"last_signature"`
	Transactions  map[string]*DividendTransfer `json:"transactions"`
}

type DividendTransfer struct {
	Signature   string    `json:"signature"`
	Amount      float64   `json:"amount"`
	Timestamp   time.Time `json:"timestamp"`
	PreBalance  float64   `json:"pre_balance"`
	PostBalance float64   `json:"post_balance"`
}

func loadDividendCache(wallet string) (*DividendCache, error) {
	cacheDir := "cache"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("dividend_cache_%s.json", wallet[:8]))
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &DividendCache{
				Transactions: make(map[string]*DividendTransfer),
			}, nil
		}
		return nil, err
	}

	var cache DividendCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	if cache.Transactions == nil {
		cache.Transactions = make(map[string]*DividendTransfer)
	}
	return &cache, nil
}

func saveDividendCache(wallet string, cache *DividendCache) error {
	cacheDir := "cache"
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("dividend_cache_%s.json", wallet[:8]))

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// GetDividendHistory fetches all transfers from dividend address to user wallet
func GetDividendHistory(ctx context.Context, cfg *config.Config, rpcClient *rpc.Client, tokenClient *token2022.Client, wallet string) (*DividendInfo, error) {
	start := time.Now()

	// Validate wallet address
	if _, err := solana.PublicKeyFromBase58(wallet); err != nil {
		return nil, fmt.Errorf("invalid wallet address: %w", err)
	}

	if cfg.Token.DividendMint == "" {
		return nil, fmt.Errorf("dividend address not configured")
	}

	// Validate dividend address
	if _, err := solana.PublicKeyFromBase58(cfg.Token.DividendMint); err != nil {
		return nil, fmt.Errorf("invalid dividend address: %w", err)
	}

	utils.Info("🔍 Fetching SOL dividend history",
		"wallet", wallet[:8]+"..."+wallet[len(wallet)-8:],
		"dividend_address", cfg.Token.DividendMint[:8]+"..."+cfg.Token.DividendMint[len(cfg.Token.DividendMint)-8:])

	// Load cache
	cache, err := loadDividendCache(wallet)
	if err != nil {
		utils.Error("❌ Failed to load dividend cache", err)
		return nil, fmt.Errorf("failed to load dividend cache: %w", err)
	}

	// Get new transfers since last update
	pubKey := solana.MustPublicKeyFromBase58(wallet)
	var signatures []solana.Signature

	if cache.LastSignature != "" {
		// Get only new transactions since last cached signature
		utils.Debug("📦 Using cached transactions, fetching new ones since",
			"last_signature", cache.LastSignature[:8]+"...",
			"elapsed", utils.Timer(start))
		sigs, err := rpcClient.GetSignaturesForAddress(ctx, pubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get signatures: %w", err)
		}

		// Filter signatures until we hit our cached one
		for _, sig := range sigs {
			if sig.Signature.String() == cache.LastSignature {
				break
			}
			signatures = append(signatures, sig.Signature)
		}
	} else {
		utils.Debug("📦 No cache found, fetching all transactions",
			"elapsed", utils.Timer(start))
		sigs, err := rpcClient.GetSignaturesForAddress(ctx, pubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get signatures: %w", err)
		}

		for _, sig := range sigs {
			signatures = append(signatures, sig.Signature)
		}
	}

	utils.Info("📝 Processing transactions",
		"total", len(signatures),
		"elapsed", utils.Timer(start))

	// Process new transactions
	var processed, found int
	for _, sig := range signatures {
		processed++
		if processed%50 == 0 {
			utils.Info("⏳ Processing progress",
				"processed", processed,
				"total", len(signatures),
				"found", found,
				"elapsed", utils.Timer(start))
		}

		if _, exists := cache.Transactions[sig.String()]; exists {
			continue
		}

		legacyVersion := uint64(0)
		tx, err := rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
			Commitment:                     rpc.CommitmentFinalized,
			MaxSupportedTransactionVersion: &legacyVersion,
		})
		if err != nil {
			utils.Debug("⚠️ Failed to get transaction",
				"signature", sig.String()[:8]+"...",
				"error", err)
			continue
		}

		// Skip if not a SOL transfer
		if tx == nil || tx.Meta == nil || len(tx.Meta.PreBalances) == 0 || len(tx.Meta.PostBalances) == 0 {
			continue
		}

		// Find wallet index and check dividend
		amount := getTransferAmount(tx, cfg.Token.DividendMint, wallet)
		if amount <= 0 {
			continue
		}

		found++
		// Add to cache
		cache.Transactions[sig.String()] = &DividendTransfer{
			Signature:   sig.String(),
			Amount:      amount,
			Timestamp:   time.Unix(int64(*tx.BlockTime), 0),
			PreBalance:  float64(tx.Meta.PreBalances[0]) / 1e9,
			PostBalance: float64(tx.Meta.PostBalances[0]) / 1e9,
		}

		if cache.LastSignature == "" || sig.String() > cache.LastSignature {
			cache.LastSignature = sig.String()
		}
	}

	// Update cache
	cache.LastUpdate = time.Now()
	if err := saveDividendCache(wallet, cache); err != nil {
		utils.Error("❌ Failed to save dividend cache", err)
	}

	// Calculate totals from cache
	info := &DividendInfo{}
	now := time.Now()
	day := time.Hour * 24

	for _, tx := range cache.Transactions {
		info.TotalAmount += tx.Amount
		info.TransferCount++

		age := now.Sub(tx.Timestamp)
		if age <= day {
			info.Last24hAmount += tx.Amount
		}
		if age <= 7*day {
			info.Last7dAmount += tx.Amount
		}
		if age <= 30*day {
			info.Last30dAmount += tx.Amount
		}

		if tx.Timestamp.After(info.LastReceived) {
			info.LastReceived = tx.Timestamp
		}
	}

	utils.Info("✅ SOL dividend history processed",
		"total_txs", len(cache.Transactions),
		"new_txs", len(signatures),
		"processed", processed,
		"found", found,
		"total_amount", formatAmount(info.TotalAmount),
		"last_24h", formatAmount(info.Last24hAmount),
		"last_7d", formatAmount(info.Last7dAmount),
		"last_30d", formatAmount(info.Last30dAmount),
		"elapsed", utils.Timer(start))

	return info, nil
}

// Helper to get transfer amount from transaction
func getTransferAmount(tx *rpc.GetTransactionResult, dividendAddress, wallet string) float64 {
	if tx == nil || tx.Meta == nil {
		return 0
	}

	// Look for SOL transfers from dividend address to wallet
	if tx.Meta.PreBalances != nil && tx.Meta.PostBalances != nil {
		solTx, err := tx.Transaction.GetTransaction()
		if err != nil {
			return 0
		}
		for i, account := range solTx.Message.AccountKeys {
			if account.String() == wallet {
				// Found our wallet, check if there's a transfer from dividend address
				for _, sender := range solTx.Message.AccountKeys {
					if sender.String() == dividendAddress {
						// Calculate SOL transfer amount
						preBalance := float64(tx.Meta.PreBalances[i]) / 1e9 // Convert lamports to SOL
						postBalance := float64(tx.Meta.PostBalances[i]) / 1e9
						if postBalance > preBalance {
							utils.Debug("💎 Found SOL transfer",
								"from", dividendAddress[:8]+"...",
								"amount", formatAmount(postBalance-preBalance),
								"pre_balance", formatAmount(preBalance),
								"post_balance", formatAmount(postBalance))
							return postBalance - preBalance
						}
					}
				}
			}
		}
	}
	return 0
}

// Helper to check if transaction is a token transfer
func isTokenTransfer(tx *rpc.GetTransactionResult) bool {
	if tx == nil || tx.Meta == nil {
		return false
	}

	// Check for System Program transfers
	for _, prog := range tx.Meta.LogMessages {
		if prog == "Program 11111111111111111111111111111111 invoke [1]" {
			return true
		}
	}
	return false
}
