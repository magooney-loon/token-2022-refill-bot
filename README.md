# 🤖 Solana Token-2022 Auto-Buy Bot

A smart bot that monitors your SOL balance and automatically buys Token-2022 tokens when conditions are met for auto-compounding yield gains.
https://www.youtube.com/watch?v=GzZFrpFMNQQ

## 🎯 What Does This Bot Do?

1. 🔍 Monitors your SOL balance every 10 minutes
2. ⚡ When SOL balance exceeds your set threshold
3. 🛒 Automatically buys the Token-2022 token of your choice
4. 📊 Shows detailed transaction info and balance updates
5. 💰 Tracks portfolio value and token distribution
6. 🔄 Keeps running until stopped

## ⚙️ Features

- ⏰ Configurable check interval (default: 10 minutes)
- 💰 Customizable SOL balance threshold
- 🎯 Support for any Token-2022 token
- 🛡️ Handles tax/dividend tokens correctly
- 📈 Uses Jupiter for best swap rates
- 💹 Real-time portfolio value tracking
- 📊 Token distribution analysis
- 💸 Dividend tracking and analytics
- 🔄 Automatic error recovery and retries
- 💻 Clean terminal UI with real-time updates

## ⚡ Quick Start

1. Get a Solana wallet (Phantom/Solflare)
2. Get your wallet's private key
3. Get an RPC endpoint (QuickNode recommended)
4. Set your desired token and thresholds
5. Run the bot

Stay safe and never share your private keys!

## 🔍 Monitoring

The bot creates a `bot.log` file with detailed operation history.
You can also monitor real-time activity in the terminal.

## 🔧 Setup

1. Install latest Go from [go.dev](https://go.dev/dl/)

2. Clone this repository:
```bash
git clone https://github.com/magooney-loon/token-2022-refill-bot
cd token-2022-refill-bot
```

3. Edit the `config/config.yaml` file with your settings:
   - `wallet.private_key`: Your Solana wallet's private key
   - `wallet.min_sol_balance`: Minimum SOL balance to trigger buy
   - `wallet.reserve_amount`: Amount of SOL to keep for fees
   - `rpc.endpoint`: Your Solana RPC endpoint
   - `token.input_mint`: Token you want to swap from (SOL by default)
   - `token.output_mint`: Token you want to buy (SOLMAX by default)
   - `token.dividend_mint`: For tax tokens, the fee mint address
   - `token.swap_amount`: Amount of input token to swap
   - `token.slippage_bps`: Slippage tolerance (default: 100 = 1%)
   - `monitor.check_interval_minutes`: How often to check balance (default: 10)

4. Install dependencies:
```bash
go mod tidy
```

## 🚀 Running the Bot

Start the bot:
```bash
go run cmd/bot/main.go
```
Build a .exe: (Optional)
```bash
go build cmd/bot/main.go
```

The bot menu provides these options:
1. Start Bot - Begin automated trading
2. Check Wallet - View portfolio value and balances
3. Check Dividends - Track dividend earnings
4. Analytics - View Bot performance (Coming soon)
0. Exit - Close the bot

The bot will:
1. Connect to your wallet
2. Start monitoring SOL balance
3. When threshold is met:
   - Calculate optimal swap route
   - Execute the token purchase
   - Show transaction details
4. Continue monitoring

To gracefully shutdown the bot, press `CTRL+C`. This ensures all operations complete cleanly. Avoid closing the window directly with X as this may leave operations in an inconsistent state.

## 📝 Implementation Progress

See [CURRENT.md](CURRENT.md) for detailed implementation status and features.

## ⚠️ Important Notes

- Never share your private key
- Test with small amounts first
- Keep enough SOL for fees
- The bot uses Jupiter for best rates
- Works with any Token-2022 token
- Handles tax/dividend tokens automatically

## 🤝 Support

For issues or questions, please contact on https://getsession.org/

ID: 0547f12efeb11b95f87bfbfb316fa05b8c6bd70bb9ba2b9cf83aa741e5d8eb175b

## 📜 License

GNU General Public License v3.0
 