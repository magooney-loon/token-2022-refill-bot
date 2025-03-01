package jupiter

// Quote represents a Jupiter quote response
type Quote struct {
	InputMint            string  `json:"inputMint"`
	OutputMint           string  `json:"outputMint"`
	InAmount             string  `json:"inAmount"`
	OutAmount            string  `json:"outAmount"`
	OtherAmountThreshold string  `json:"otherAmountThreshold"`
	SwapMode             string  `json:"swapMode"`
	SlippageBps          int     `json:"slippageBps"`
	PlatformFee          *Fee    `json:"platformFee,omitempty"`
	PriceImpactPct       string  `json:"priceImpactPct"`
	RoutePlan            []Route `json:"routePlan"`
	ContextSlot          int64   `json:"contextSlot"`
	TimeTaken            float64 `json:"timeTaken"`
}

type Fee struct {
	Amount string `json:"amount"`
	FeeBps int    `json:"feeBps"`
}

type Route struct {
	SwapInfo SwapInfo `json:"swapInfo"`
	Percent  int      `json:"percent"`
}

type SwapInfo struct {
	AmmKey     string `json:"ammKey"`
	Label      string `json:"label"`
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	InAmount   string `json:"inAmount"`
	OutAmount  string `json:"outAmount"`
	FeeAmount  string `json:"feeAmount"`
	FeeMint    string `json:"feeMint"`
}

type SwapRequest struct {
	UserPublicKey             string `json:"userPublicKey"`
	WrapAndUnwrapSol          bool   `json:"wrapAndUnwrapSol"`
	UseSharedAccounts         bool   `json:"useSharedAccounts"`
	PrioritizationFeeLamports int    `json:"prioritizationFeeLamports"`
	AsLegacyTransaction       bool   `json:"asLegacyTransaction"`
	UseTokenLedger            bool   `json:"useTokenLedger"`
	DynamicComputeUnitLimit   bool   `json:"dynamicComputeUnitLimit"`
	SkipUserAccountsRpcCalls  bool   `json:"skipUserAccountsRpcCalls"`
	QuoteResponse             Quote  `json:"quoteResponse"`
	ComputeUnitLimit          int    `json:"computeUnitLimit"`
	ComputeUnitPrice          int    `json:"computeUnitPrice"`
}

type SwapResponse struct {
	SwapTransaction string `json:"swapTransaction"`
}
