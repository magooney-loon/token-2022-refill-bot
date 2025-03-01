package jupiter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"golang.org/x/time/rate"
)

type Client struct {
	httpClient   *http.Client
	rateLimiter  *rate.Limiter
	config       *config.JupiterConfig
	tokenClient  *token2022.Client
	refreshCache bool
}

func NewClient(cfg *config.JupiterConfig, tokenClient *token2022.Client, refreshCache bool) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		rateLimiter:  rate.NewLimiter(rate.Limit(1), 1), // 1 request per second
		config:       cfg,
		tokenClient:  tokenClient,
		refreshCache: refreshCache,
	}
}

func (c *Client) GetQuote(ctx context.Context, inputMint, outputMint string, amount uint64, slippageBps int, dexes string) (*Quote, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	// Get token info with cache refresh setting
	_, err := c.tokenClient.GetTokenInfo(ctx, inputMint)
	if err != nil {
		utils.Warn("⚠️ Failed to get input token info", "error", err)
	}

	_, err = c.tokenClient.GetTokenInfo(ctx, outputMint)
	if err != nil {
		utils.Warn("⚠️ Failed to get output token info", "error", err)
	}

	utils.Info("🔍 Fetching Jupiter Quote",
		"input_amount", amount,
		"input_mint", inputMint,
		"output_mint", outputMint,
		"slippage", fmt.Sprintf("%d bps", slippageBps),
		"direct_routes_only", c.config.OnlyDirectRoutes)

	// Build URL with priority fees and compute units
	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&onlyDirectRoutes=%t&prioritizationFeeLamports=auto&computeUnitPriceMicroLamports=auto",
		c.config.QuoteEndpoint, inputMint, outputMint, amount, c.config.OnlyDirectRoutes)

	// Only add slippage if not using dynamic slippage
	if slippageBps > 0 {
		url = fmt.Sprintf("%s&slippageBps=%d", url, slippageBps)
	} else {
		url = fmt.Sprintf("%s&dynamicSlippage=true", url)
	}

	if dexes != "" {
		url = fmt.Sprintf("%s&dexes=%s", url, dexes)
	}

	utils.Debug("📡 API Request Details",
		"url", url,
		"method", "GET")

	resp, err := c.httpClient.Get(url)
	if err != nil {
		utils.Error("❌ Failed to get Jupiter quote", err,
			"input_mint", inputMint,
			"output_mint", outputMint)
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		utils.Error("❌ Jupiter API Error", fmt.Errorf("status: %d", resp.StatusCode),
			"body", string(body))
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var quote Quote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		utils.Error("❌ Failed to decode Jupiter response", err)
		return nil, fmt.Errorf("failed to decode quote response: %w", err)
	}

	// Format route details
	var routeDetails []string
	for _, route := range quote.RoutePlan {
		routeDetails = append(routeDetails, fmt.Sprintf("%s -> %s (%.2f%%)",
			route.SwapInfo.InputMint[:8],
			route.SwapInfo.OutputMint[:8],
			float64(route.Percent)))
	}

	priceImpact, _ := strconv.ParseFloat(quote.PriceImpactPct, 64)

	utils.Info("✅ Quote Received",
		"input_amount", fmt.Sprintf("%s %s", quote.InAmount, quote.InputMint[:8]),
		"output_amount", fmt.Sprintf("%s %s", quote.OutAmount, quote.OutputMint[:8]),
		"price_impact", fmt.Sprintf("%.2f%%", priceImpact),
		"route_count", len(quote.RoutePlan))

	utils.Debug("📊 Route Details",
		"routes", routeDetails,
		"market_impact", fmt.Sprintf("%.4f%%", priceImpact),
		"other_amounts_in", quote.OtherAmountThreshold,
		"swap_mode", quote.SwapMode)

	return &quote, nil
}

func (c *Client) ExecuteSwap(ctx context.Context, wallet *solana.Wallet, quote *Quote, rpcClient *rpc.Client, priorityFee int) (solana.Signature, error) {
	utils.Info("🚀 Initiating Swap Transaction",
		"wallet", fmt.Sprintf("%s...%s", wallet.PublicKey().String()[:8], wallet.PublicKey().String()[len(wallet.PublicKey().String())-8:]),
		"input", fmt.Sprintf("%s %s", quote.InAmount, quote.InputMint[:8]),
		"output", fmt.Sprintf("%s %s", quote.OutAmount, quote.OutputMint[:8]),
		"slippage", fmt.Sprintf("%d bps", quote.SlippageBps))

	// Check if output token is Token-2022
	outputToken, err := c.tokenClient.GetTokenInfo(ctx, quote.OutputMint)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get output token info: %w", err)
	}

	isToken2022 := false
	for _, tag := range outputToken.Tags {
		if tag == "token-2022" {
			isToken2022 = true
			break
		}
	}

	swapRequest := SwapRequest{
		UserPublicKey:             wallet.PublicKey().String(),
		WrapAndUnwrapSol:          true,
		UseSharedAccounts:         true, // Enable shared accounts for better efficiency
		PrioritizationFeeLamports: priorityFee,
		AsLegacyTransaction:       false,
		UseTokenLedger:            false,
		DynamicComputeUnitLimit:   true,
		SkipUserAccountsRpcCalls:  true,
		QuoteResponse:             *quote,
		ComputeUnitLimit:          200000, // Default limit
		ComputeUnitPrice:          0,      // Let Jupiter set this dynamically
	}

	// Increase compute limit for Token-2022 tokens
	if isToken2022 {
		swapRequest.ComputeUnitLimit = 400000
	}

	utils.Debug("📝 Swap Request Details",
		"priority_fee", fmt.Sprintf("%d lamports", priorityFee),
		"compute_limit", swapRequest.ComputeUnitLimit,
		"is_token2022", isToken2022,
		"wrap_sol", true)

	swapResp, err := c.submitSwapRequest(&swapRequest)
	if err != nil {
		utils.Error("❌ Failed to submit swap request", err)
		return solana.Signature{}, err
	}

	utils.Debug("📦 Swap Response Received",
		"tx_size", fmt.Sprintf("%d bytes", len(swapResp.SwapTransaction)))

	return c.processSwapTransaction(ctx, swapResp, wallet, rpcClient)
}

func (c *Client) submitSwapRequest(req *SwapRequest) (*SwapResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal swap request: %w", err)
	}

	utils.Debug("📡 Submitting Swap Request",
		"endpoint", c.config.SwapEndpoint,
		"request_size", fmt.Sprintf("%d bytes", len(jsonData)))

	resp, err := c.httpClient.Post(c.config.SwapEndpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to submit swap request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		utils.Error("❌ Jupiter Swap API Error", fmt.Errorf("status: %d", resp.StatusCode),
			"response", string(body))
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var swapResp SwapResponse
	if err := json.NewDecoder(resp.Body).Decode(&swapResp); err != nil {
		return nil, fmt.Errorf("failed to decode swap response: %w", err)
	}

	return &swapResp, nil
}

func (c *Client) processSwapTransaction(ctx context.Context, swapResp *SwapResponse, wallet *solana.Wallet, rpcClient *rpc.Client) (solana.Signature, error) {
	utils.Info("🔄 Processing Swap Transaction",
		"tx_size", fmt.Sprintf("%d bytes", len(swapResp.SwapTransaction)))

	txBytes, err := base64.StdEncoding.DecodeString(swapResp.SwapTransaction)
	if err != nil {
		utils.Error("❌ Failed to decode transaction", err)
		return solana.Signature{}, fmt.Errorf("failed to decode swap transaction: %w", err)
	}

	tx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(txBytes))
	if err != nil {
		utils.Error("❌ Failed to deserialize transaction", err)
		return solana.Signature{}, fmt.Errorf("failed to deserialize transaction: %w", err)
	}

	utils.Debug("✍️ Signing Transaction",
		"signer", fmt.Sprintf("%s...%s", wallet.PublicKey().String()[:8], wallet.PublicKey().String()[len(wallet.PublicKey().String())-8:]))

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if wallet.PublicKey().String() == key.String() {
			pk := wallet.PrivateKey
			return &pk
		}
		return nil
	})
	if err != nil {
		utils.Error("❌ Failed to sign transaction", err)
		return solana.Signature{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	utils.Info("📡 Sending Transaction to Solana")

	sig, err := rpcClient.SendTransaction(ctx, tx)
	if err != nil {
		utils.Error("❌ Failed to send transaction", err)
		return solana.Signature{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	utils.Info("✅ Transaction Sent Successfully",
		"signature", fmt.Sprintf("https://solscan.io/tx/%s", sig.String()),
		"status", "processing")

	return sig, nil
}
