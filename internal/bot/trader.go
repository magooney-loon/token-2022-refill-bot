package bot

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/jupiter"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Trader struct {
	config        *config.Config
	jupiterClient *jupiter.Client
	rpcClient     *rpc.Client
	wallet        *solana.Wallet
	tokenClient   *token2022.Client
}

func NewTrader(cfg *config.Config, jupiterClient *jupiter.Client, rpcClient *rpc.Client, wallet *solana.Wallet) *Trader {
	return &Trader{
		config:        cfg,
		jupiterClient: jupiterClient,
		rpcClient:     rpcClient,
		wallet:        wallet,
		tokenClient:   token2022.NewClient(cfg, rpcClient),
	}
}

func (t *Trader) ExecuteSwap(ctx context.Context, balance float64) error {
	// Use configured swap amount instead of full balance
	amount := t.config.Token.SwapAmount

	// Ensure we have enough balance (including reserve)
	if balance < amount+t.config.Wallet.ReserveAmount {
		return fmt.Errorf("insufficient balance for swap: have %.6f, need %.6f (including reserve)",
			balance, amount+t.config.Wallet.ReserveAmount)
	}

	// Get token info for both tokens
	inputToken, err := t.tokenClient.GetTokenInfo(ctx, t.config.Token.InputMint)
	if err != nil {
		return fmt.Errorf("failed to get input token info: %w", err)
	}

	outputToken, err := t.tokenClient.GetTokenInfo(ctx, t.config.Token.OutputMint)
	if err != nil {
		return fmt.Errorf("failed to get output token info: %w", err)
	}

	// Convert amount to lamports/smallest unit
	rawAmount := t.toRawAmount(amount, inputToken.Decimals)

	utils.Info("Starting swap execution",
		"amount", amount,
		"raw_amount", rawAmount,
		"input_token", inputToken.Symbol,
		"output_token", outputToken.Symbol)

	// Calculate effective slippage with tax buffer
	effectiveSlippage := uint16(t.config.Token.SlippageBPS)
	outputTransferFee := outputToken.GetTransferFeeBps()
	if outputTransferFee > 0 {
		effectiveSlippage += outputTransferFee
		utils.Debug("Added tax buffer to slippage",
			"base_slippage", t.config.Token.SlippageBPS,
			"tax_buffer", outputTransferFee,
			"effective_slippage", effectiveSlippage)
	}

	// Get quote with retry
	quote, err := utils.WithRetry(func() (*jupiter.Quote, error) {
		return t.jupiterClient.GetQuote(
			ctx,
			t.config.Token.InputMint,
			t.config.Token.OutputMint,
			rawAmount,
			int(effectiveSlippage),
			"",
		)
	}, t.config.Monitor.MaxRetries, time.Second*time.Duration(t.config.Monitor.RetryDelaySeconds))

	if err != nil {
		return fmt.Errorf("failed to get quote: %w", err)
	}

	// Check price impact
	priceImpact, err := t.calculatePriceImpact(quote.PriceImpactPct)
	if err != nil {
		return fmt.Errorf("failed to calculate price impact: %w", err)
	}

	// Check if price impact is too high
	maxPriceImpact := float64(effectiveSlippage) / 100.0
	if priceImpact > maxPriceImpact {
		return fmt.Errorf("price impact too high: %.2f%% (max: %.2f%%)", priceImpact*100, maxPriceImpact*100)
	}

	// Execute swap with retry
	sig, err := utils.WithRetry(func() (solana.Signature, error) {
		return t.jupiterClient.ExecuteSwap(
			ctx,
			t.wallet,
			quote,
			t.rpcClient,
			36699, // TODO: Add priority fee config
		)
	}, t.config.Monitor.MaxRetries, time.Second*time.Duration(t.config.Monitor.RetryDelaySeconds))

	if err != nil {
		return fmt.Errorf("failed to execute swap: %w", err)
	}

	// Convert amounts to human readable
	inAmount := t.fromRawAmount(quote.InAmount, inputToken.Decimals)
	outAmount := t.fromRawAmount(quote.OutAmount, outputToken.Decimals)

	utils.Info("Swap executed successfully",
		"signature", sig.String(),
		"input_amount", fmt.Sprintf("%.6f %s", inAmount, inputToken.Symbol),
		"output_amount", fmt.Sprintf("%.6f %s", outAmount, outputToken.Symbol),
		"price_impact", fmt.Sprintf("%.2f%%", priceImpact*100))

	return nil
}

func (t *Trader) calculatePriceImpact(priceImpactStr string) (float64, error) {
	var priceImpact float64
	_, err := fmt.Sscanf(priceImpactStr, "%f", &priceImpact)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price impact: %w", err)
	}
	return math.Abs(priceImpact), nil
}

// toRawAmount converts a human readable amount to raw units (lamports)
func (t *Trader) toRawAmount(amount float64, decimals int) uint64 {
	multiplier := new(big.Float).SetFloat64(math.Pow10(decimals))
	rawAmount := new(big.Float).Mul(new(big.Float).SetFloat64(amount), multiplier)

	result := new(big.Float)
	result.Add(rawAmount, new(big.Float).SetFloat64(0.5)) // Round to nearest

	intAmount, _ := result.Uint64()
	return intAmount
}

// fromRawAmount converts raw units (lamports) to human readable amount
func (t *Trader) fromRawAmount(rawAmount string, decimals int) float64 {
	amount := new(big.Float)
	amount.SetString(rawAmount)

	divisor := new(big.Float).SetFloat64(math.Pow10(decimals))
	result := new(big.Float).Quo(amount, divisor)

	humanAmount, _ := result.Float64()
	return humanAmount
}
