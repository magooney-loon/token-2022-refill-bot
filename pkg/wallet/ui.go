package wallet

import (
	"context"
	"fmt"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// DisplayWalletBalances shows the wallet balances in a formatted UI
func DisplayWalletBalances(ctx context.Context, cfg *config.Config, rpcClient *rpc.Client, tokenClient *token2022.Client, walletAddr string) error {
	utils.Info("Checking wallet balances", "wallet", walletAddr[:8]+"..."+walletAddr[len(walletAddr)-8:])

	// Get balances
	balances, err := GetWalletBalances(
		ctx,
		cfg,
		rpcClient,
		walletAddr,
		tokenClient,
	)
	if err != nil {
		return fmt.Errorf("failed to get wallet balances: %w", err)
	}

	// Get token prices
	mints := make([]string, 0, len(balances))
	for _, bal := range balances {
		mints = append(mints, bal.Mint)
	}
	prices, err := GetTokenPrices(ctx, cfg, mints)
	if err != nil {
		utils.Warn("Failed to fetch token prices", "error", err)
	}

	// Calculate portfolio
	portfolio := CalculatePortfolio(balances, prices)

	// Display portfolio summary
	fmt.Printf("\n💰 Portfolio Value: $%.2f\n", portfolio.TotalUSDValue)
	fmt.Println("-------------------")

	// Display balances with portfolio info
	fmt.Println("🎯 Input/Output Tokens:")
	for _, token := range portfolio.Tokens {
		if token.IsInput || token.IsOutput {
			displayTokenInfo(token, true)
		}
	}

	fmt.Println("\n📊 Other Tokens:")
	for _, token := range portfolio.Tokens {
		if !token.IsInput && !token.IsOutput && token.Balance > 0 {
			displayTokenInfo(token, false)
		}
	}

	fmt.Println()
	return nil
}

func displayTokenInfo(token PortfolioToken, showRole bool) {
	role := ""
	if showRole {
		if token.IsInput {
			role = "(Input)"
		}
		if token.IsOutput {
			role = "(Output)"
		}
	}

	// Add token info
	tokenInfo := ""
	programType := "SPL"
	if token.TokenInfo != nil {
		if token.TokenInfo.TransferFee != nil {
			tokenInfo += fmt.Sprintf(" | Fee: %.2f%%", float64(token.TokenInfo.TransferFee.BasisPoints)/100)
		}
		if token.TokenInfo.InterestRate != nil {
			tokenInfo += fmt.Sprintf(" | APY: %.2f%%", token.TokenInfo.InterestRate.APY)
		}
	}
	if token.IsToken22 {
		programType = "Token-2022"
	}

	// Add portfolio info
	portfolioInfo := ""
	if token.USDValue > 0 {
		portfolioInfo = fmt.Sprintf(" | $%.2f (%.2f%%)", token.USDValue, token.Distribution)
	}

	fmt.Printf("  • %s %s: %s (%s) [%s]%s%s\n",
		token.Symbol,
		role,
		token.UiAmount,
		token.Name,
		programType,
		tokenInfo,
		portfolioInfo)
}

// DisplayDividendInfo shows the dividend info in a formatted UI
func DisplayDividendInfo(ctx context.Context, cfg *config.Config, rpcClient *rpc.Client, tokenClient *token2022.Client, walletAddr string) error {
	// Get dividend info
	info, err := GetDividendHistory(ctx, cfg, rpcClient, tokenClient, walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get dividend history: %w", err)
	}

	// Get SOL price
	prices, err := GetTokenPrices(ctx, cfg, []string{"So11111111111111111111111111111111111111112"})
	if err != nil {
		utils.Warn("Failed to fetch SOL price", "error", err)
	}

	// Calculate USD values
	solPrice := prices["So11111111111111111111111111111111111111112"]
	totalUSD := info.TotalAmount * solPrice
	last24hUSD := info.Last24hAmount * solPrice
	last7dUSD := info.Last7dAmount * solPrice
	last30dUSD := info.Last30dAmount * solPrice

	// Display dividend info
	fmt.Printf("\n💸 Dividend Summary for %s\n", walletAddr[:8]+"..."+walletAddr[len(walletAddr)-8:])
	fmt.Printf("-------------------\n")
	fmt.Printf("• Total Received: %.6f SOL ($%.2f)\n", info.TotalAmount, totalUSD)
	fmt.Printf("• Last 24h: %.6f SOL ($%.2f)\n", info.Last24hAmount, last24hUSD)
	fmt.Printf("• Last 7d: %.6f SOL ($%.2f)\n", info.Last7dAmount, last7dUSD)
	fmt.Printf("• Last 30d: %.6f SOL ($%.2f)\n", info.Last30dAmount, last30dUSD)
	fmt.Printf("• Total Transfers: %d\n", info.TransferCount)
	if !info.LastReceived.IsZero() {
		fmt.Printf("• Last Received: %s\n", info.LastReceived.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	return nil
}

// PromptWalletAddress prompts the user for a wallet address with validation
func PromptWalletAddress(defaultWallet string) (string, error) {
	fmt.Printf("\n💳 Enter wallet address [default: %s]: ", defaultWallet[:8]+"..."+defaultWallet[len(defaultWallet)-8:])
	var input string
	fmt.Scanln(&input)

	if input == "" {
		return defaultWallet, nil
	}

	// Validate input wallet
	_, err := solana.PublicKeyFromBase58(input)
	if err != nil {
		return "", fmt.Errorf("invalid wallet address: %s", err)
	}

	return input, nil
}
