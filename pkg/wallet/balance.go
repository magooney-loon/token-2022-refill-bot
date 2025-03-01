package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type TokenBalance struct {
	Symbol    string
	Name      string
	Mint      string
	Balance   float64
	Decimals  uint8
	UiAmount  string
	IsInput   bool
	IsOutput  bool
	IsToken22 bool
	TokenInfo *token2022.TokenInfo
}

type TokenPrice struct {
	ID    string  `json:"id"`
	Type  string  `json:"type"`
	Price float64 `json:"price,string"`
}

type PriceResponse struct {
	Data      map[string]TokenPrice `json:"data"`
	TimeTaken float64               `json:"timeTaken"`
}

type PortfolioToken struct {
	TokenBalance
	USDValue     float64
	Distribution float64
}

type Portfolio struct {
	Tokens        []PortfolioToken
	TotalUSDValue float64
}

// formatAmount formats a number with k/m/b suffixes
func formatAmount(amount float64) string {
	if amount == 0 {
		return "0"
	}

	abs := math.Abs(amount)
	if abs < 1000 {
		return fmt.Sprintf("%.2f", amount)
	}
	if abs < 1000000 {
		return fmt.Sprintf("%.2fk", amount/1000)
	}
	if abs < 1000000000 {
		return fmt.Sprintf("%.2fM", amount/1000000)
	}
	return fmt.Sprintf("%.2fB", amount/1000000000)
}

// GetWalletBalances returns the SOL and token balances for a wallet
func GetWalletBalances(ctx context.Context, cfg *config.Config, rpcClient *rpc.Client, wallet string, tokenClient *token2022.Client) ([]TokenBalance, error) {
	// Validate wallet address
	if len(wallet) == 0 {
		return nil, fmt.Errorf("wallet address cannot be empty")
	}

	// Try to parse the wallet address - will fail if invalid
	_, err := solana.PublicKeyFromBase58(wallet)
	if err != nil {
		return nil, fmt.Errorf("invalid Solana wallet address: %s", err)
	}

	utils.Debug("Fetching wallet balances", "wallet", wallet[:4]+"..."+wallet[len(wallet)-4:])
	pubKey := solana.MustPublicKeyFromBase58(wallet)

	// Get regular token accounts
	utils.Debug("Getting regular token accounts")
	regularAccounts, err := rpcClient.GetTokenAccountsByOwner(
		ctx,
		pubKey,
		&rpc.GetTokenAccountsConfig{
			ProgramId: &solana.TokenProgramID,
		},
		&rpc.GetTokenAccountsOpts{
			Commitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get regular token accounts: %w", err)
	}

	// Get Token-2022 accounts
	utils.Debug("Getting Token-2022 accounts")
	token2022ProgramID := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	token2022Accounts, err := rpcClient.GetTokenAccountsByOwner(
		ctx,
		pubKey,
		&rpc.GetTokenAccountsConfig{
			ProgramId: &token2022ProgramID,
		},
		&rpc.GetTokenAccountsOpts{
			Commitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		utils.Warn("Failed to get Token-2022 accounts", "error", err)
	}

	// Combine accounts
	allAccounts := regularAccounts.Value
	if token2022Accounts != nil {
		allAccounts = append(allAccounts, token2022Accounts.Value...)
	}
	utils.Debug("Total token accounts found", "regular", len(regularAccounts.Value), "token2022", len(token2022Accounts.Value))

	balances := make([]TokenBalance, 0)

	// Get SOL balance
	utils.Debug("Getting SOL balance")
	solBalance, err := rpcClient.GetBalance(
		ctx,
		pubKey,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get SOL balance: %w", err)
	}

	solAmount := float64(solBalance.Value) / math.Pow10(9)
	balances = append(balances, TokenBalance{
		Symbol:   "SOL",
		Name:     "Solana",
		Mint:     "So11111111111111111111111111111111111111112",
		Balance:  solAmount,
		Decimals: 9,
		UiAmount: formatAmount(solAmount),
		IsInput:  cfg.Token.InputMint == "So11111111111111111111111111111111111111112",
		IsOutput: cfg.Token.OutputMint == "So11111111111111111111111111111111111111112",
	})

	// Process token accounts
	utils.Debug("Processing token accounts", "count", len(allAccounts))
	for _, account := range allAccounts {
		// Get token balance
		tokenBalance, err := rpcClient.GetTokenAccountBalance(
			ctx,
			account.Pubkey,
			rpc.CommitmentFinalized,
		)
		if err != nil {
			utils.Debug("Failed to get token balance", "account", account.Pubkey, "error", err)
			continue
		}

		// Get mint address from account
		data := account.Account.Data.GetBinary()
		if len(data) < 40 {
			utils.Debug("Invalid token account data", "account", account.Pubkey)
			continue
		}

		// The mint address is stored at offset 0 in the token account data
		mintAddr := solana.PublicKey(data[0:32])
		mint := mintAddr.String()

		if tokenBalance.Value.UiAmount == nil {
			utils.Debug("No UI amount for token", "mint", mint[:4]+"..."+mint[len(mint)-4:])
			continue
		}

		// Get token info from Token-2022 client
		var tokenInfo *token2022.TokenInfo
		if tokenClient != nil {
			tokenInfo, err = tokenClient.GetTokenInfo(ctx, mint)
			if err != nil {
				utils.Debug("Failed to get token info", "mint", mint, "error", err)
			}
		}

		symbol := mint[:4] + "..." + mint[len(mint)-4:]
		name := symbol
		if tokenInfo != nil {
			if tokenInfo.Symbol != "" {
				symbol = tokenInfo.Symbol
			}
			if tokenInfo.Name != "" {
				name = tokenInfo.Name
			}
		}

		// Check if token is Token-2022
		isToken2022 := false
		if tokenInfo != nil {
			for _, tag := range tokenInfo.Tags {
				if tag == "token-2022" {
					isToken2022 = true
					break
				}
			}
		}

		balance := TokenBalance{
			Symbol:    symbol,
			Name:      name,
			Mint:      mint,
			Balance:   *tokenBalance.Value.UiAmount,
			Decimals:  uint8(tokenBalance.Value.Decimals),
			UiAmount:  formatAmount(*tokenBalance.Value.UiAmount),
			IsInput:   mint == cfg.Token.InputMint,
			IsOutput:  mint == cfg.Token.OutputMint,
			IsToken22: isToken2022,
			TokenInfo: tokenInfo,
		}

		balances = append(balances, balance)
		utils.Debug("Added token balance",
			"symbol", balance.Symbol,
			"name", balance.Name,
			"amount", balance.UiAmount,
			"is_input", balance.IsInput,
			"is_output", balance.IsOutput,
			"program", "token2022")
	}

	// Sort balances: Input/Output first, then by balance
	sort.Slice(balances, func(i, j int) bool {
		if balances[i].IsInput || balances[i].IsOutput {
			if !(balances[j].IsInput || balances[j].IsOutput) {
				return true
			}
		} else if balances[j].IsInput || balances[j].IsOutput {
			return false
		}
		return balances[i].Balance > balances[j].Balance
	})

	utils.Debug("Finished processing balances", "total_tokens", len(balances))
	return balances, nil
}

// GetTokenPrices fetches token prices from Jupiter API
func GetTokenPrices(ctx context.Context, cfg *config.Config, mints []string) (map[string]float64, error) {
	// Build comma-separated list of token mints
	mintList := strings.Join(mints, ",")
	url := fmt.Sprintf("%s?ids=%s", cfg.Jupiter.TokenPriceEndpoint, mintList)

	utils.Debug("Fetching token prices",
		"url", url,
		"token_count", len(mints))

	// Make request
	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("price API error: %d", resp.StatusCode)
	}

	var priceResp PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return nil, fmt.Errorf("failed to decode price response: %w", err)
	}

	// Convert to simple map
	prices := make(map[string]float64)
	for mint, price := range priceResp.Data {
		prices[mint] = price.Price
	}

	utils.Debug("Prices fetched successfully",
		"token_count", len(prices),
		"time_taken", fmt.Sprintf("%.2fms", priceResp.TimeTaken*1000))

	return prices, nil
}

// CalculatePortfolio calculates portfolio value and distribution
func CalculatePortfolio(balances []TokenBalance, prices map[string]float64) Portfolio {
	portfolio := Portfolio{
		Tokens: make([]PortfolioToken, 0, len(balances)),
	}

	// Calculate total value
	for _, bal := range balances {
		price := prices[bal.Mint]
		usdValue := bal.Balance * price
		portfolio.TotalUSDValue += usdValue

		pToken := PortfolioToken{
			TokenBalance: bal,
			USDValue:     usdValue,
		}
		portfolio.Tokens = append(portfolio.Tokens, pToken)
	}

	// Calculate distribution
	for i := range portfolio.Tokens {
		if portfolio.TotalUSDValue > 0 {
			portfolio.Tokens[i].Distribution = (portfolio.Tokens[i].USDValue / portfolio.TotalUSDValue) * 100
		}
	}

	// Sort by USD value
	sort.Slice(portfolio.Tokens, func(i, j int) bool {
		return portfolio.Tokens[i].USDValue > portfolio.Tokens[j].USDValue
	})

	return portfolio
}
