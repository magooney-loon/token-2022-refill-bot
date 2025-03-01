package token2022

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// TokenInfo combines Jupiter API token info with Token-2022 metadata
type TokenInfo struct {
	// Jupiter Info
	Address     string   `json:"address"`
	Name        string   `json:"name"`
	Symbol      string   `json:"symbol"`
	Decimals    int      `json:"decimals"`
	LogoURI     string   `json:"logoURI"`
	Tags        []string `json:"tags"`
	DailyVolume float64  `json:"daily_volume"`
	CreatedAt   string   `json:"created_at"`
	MintedAt    string   `json:"minted_at"`

	// Token-2022 Extensions
	TransferFee       *TransferFee  `json:"transfer_fee"`
	InterestRate      *InterestRate `json:"interest_rate"`
	PermanentDelegate *string       `json:"permanent_delegate"`
	FreezeAuthority   *string       `json:"freeze_authority"`
	MintAuthority     *string       `json:"mint_authority"`

	// Additional metadata
	Extensions map[string]interface{} `json:"extensions"`
}

// TransferFee represents token transfer fee configuration
type TransferFee struct {
	BasisPoints     uint16           `json:"basis_points"`
	MaximumFee      uint64           `json:"maximum_fee"`
	CollectorWallet solana.PublicKey `json:"collector_wallet"`
}

// InterestRate represents token interest/dividend configuration
type InterestRate struct {
	CurrentRate    uint16  `json:"current_rate"`
	APY            float64 `json:"apy"`
	LastUpdateSlot uint64  `json:"last_update_slot"`
}

// TokenCache provides thread-safe caching of token information
type TokenCache struct {
	cache    *sync.Map
	cacheTTL time.Duration
}

type cacheEntry struct {
	info      *TokenInfo
	expiresAt time.Time
}

// Client handles Token-2022 operations and Jupiter token info
type Client struct {
	rpcClient  *rpc.Client
	httpClient *http.Client
	config     *config.Config
	cache      *TokenCache
}

func NewClient(cfg *config.Config, rpcClient *rpc.Client) *Client {
	cache := &TokenCache{
		cache:    &sync.Map{},
		cacheTTL: time.Duration(cfg.Token.CacheTTLMinutes) * time.Minute,
	}

	utils.Info("🔧 Token Info Cache Initialized",
		"ttl", fmt.Sprintf("%d minutes", cfg.Token.CacheTTLMinutes),
		"type", "thread-safe")

	return &Client{
		rpcClient:  rpcClient,
		httpClient: &http.Client{Timeout: time.Second * 30},
		config:     cfg,
		cache:      cache,
	}
}

// GetTokenInfo fetches combined token information
func (c *Client) GetTokenInfo(ctx context.Context, address string) (*TokenInfo, error) {
	// Check cache first
	if !c.config.Token.RefreshCache {
		if info, ok := c.cache.Get(address); ok {
			utils.Debug("📦 Token Info Cache Hit",
				"address", fmt.Sprintf("%s...%s", address[:8], address[len(address)-8:]),
				"symbol", info.Symbol,
				"decimals", info.Decimals)
			return info, nil
		}
	}

	utils.Info("🔍 Fetching Token Info",
		"address", fmt.Sprintf("%s...%s", address[:8], address[len(address)-8:]),
		"endpoint", c.config.Jupiter.TokenAPIEndpoint)

	// Fetch from Jupiter API
	url := fmt.Sprintf("%s/%s", c.config.Jupiter.TokenAPIEndpoint, address)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		utils.Error("❌ Failed to fetch token info", err,
			"address", address)
		return nil, fmt.Errorf("failed to fetch token info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		utils.Error("❌ Jupiter API Error", fmt.Errorf("status: %d", resp.StatusCode),
			"address", address)
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var info TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		utils.Error("❌ Failed to decode token info", err)
		return nil, fmt.Errorf("failed to decode token info: %w", err)
	}

	// Fetch Token-2022 extensions if available
	if err := c.enrichWithToken2022Data(ctx, &info); err != nil {
		utils.Warn("⚠️ Failed to fetch Token-2022 data",
			"error", err,
			"address", address)
	}

	utils.Info("✅ Token Info Retrieved",
		"symbol", info.Symbol,
		"name", info.Name,
		"decimals", info.Decimals,
		"tags", info.Tags)

	// Cache the result
	c.cache.Set(address, &info)
	utils.Debug("📦 Token Info Cached",
		"address", fmt.Sprintf("%s...%s", address[:8], address[len(address)-8:]),
		"ttl", fmt.Sprintf("%d minutes", c.config.Token.CacheTTLMinutes))

	return &info, nil
}

// GetTransferFeeBps returns the transfer fee in basis points
func (t *TokenInfo) GetTransferFeeBps() uint16 {
	if t.TransferFee != nil {
		return t.TransferFee.BasisPoints
	}
	return 0
}

// enrichWithToken2022Data fetches and adds Token-2022 specific data
func (c *Client) enrichWithToken2022Data(ctx context.Context, info *TokenInfo) error {
	pubkey, err := solana.PublicKeyFromBase58(info.Address)
	if err != nil {
		return fmt.Errorf("invalid token address: %w", err)
	}

	// Fetch account info
	account, err := c.rpcClient.GetAccountInfo(ctx, pubkey)
	if err != nil {
		return fmt.Errorf("failed to fetch account info: %w", err)
	}

	if account == nil || account.Value == nil {
		return fmt.Errorf("account not found")
	}

	// Parse Token-2022 extensions
	info.Extensions = make(map[string]interface{})

	// Check for transfer fee extension
	if transferFee, err := c.parseTransferFee(account.Value.Data.GetBinary()); err == nil {
		info.TransferFee = transferFee
		info.Extensions["transfer_fee"] = true
		utils.Debug("📊 Found Transfer Fee Extension",
			"token", info.Symbol,
			"bps", transferFee.BasisPoints,
			"max_fee", transferFee.MaximumFee)
	}

	// Check for interest rate extension
	if interestRate, err := c.parseInterestRate(account.Value.Data.GetBinary()); err == nil {
		info.InterestRate = interestRate
		info.Extensions["interest_rate"] = true
		utils.Debug("📈 Found Interest Rate Extension",
			"token", info.Symbol,
			"rate", interestRate.CurrentRate,
			"apy", interestRate.APY)
	}

	// Check for permanent delegate
	if delegate, err := c.parsePermanentDelegate(account.Value.Data.GetBinary()); err == nil {
		info.PermanentDelegate = &delegate
		info.Extensions["permanent_delegate"] = true
		utils.Debug("👥 Found Permanent Delegate",
			"token", info.Symbol,
			"delegate", delegate)
	}

	// Check for authorities
	if auth, err := c.parseAuthorities(account.Value.Data.GetBinary()); err == nil {
		if auth.FreezeAuthority != "" {
			info.FreezeAuthority = &auth.FreezeAuthority
			info.Extensions["freeze_authority"] = true
		}
		if auth.MintAuthority != "" {
			info.MintAuthority = &auth.MintAuthority
			info.Extensions["mint_authority"] = true
		}
		utils.Debug("🔑 Found Authority Extensions",
			"token", info.Symbol,
			"freeze_auth", auth.FreezeAuthority != "",
			"mint_auth", auth.MintAuthority != "")
	}

	return nil
}

// Cache methods
func (c *TokenCache) Get(address string) (*TokenInfo, bool) {
	if value, ok := c.cache.Load(address); ok {
		entry := value.(*cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.info, true
		}
		c.cache.Delete(address)
	}
	return nil, false
}

func (c *TokenCache) Set(address string, info *TokenInfo) {
	entry := &cacheEntry{
		info:      info,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
	c.cache.Store(address, entry)
}

func (c *TokenCache) Clear() {
	oldCache := c.cache
	c.cache = &sync.Map{}
	count := 0
	oldCache.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	utils.Info("🧹 Cache Cleared",
		"entries_removed", count)
}

func (c *TokenCache) Count() int {
	count := 0
	c.cache.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
