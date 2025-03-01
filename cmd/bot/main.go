package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/magooney-loon/token-2022-refill-bot/internal/bot"
	"github.com/magooney-loon/token-2022-refill-bot/internal/config"
	"github.com/magooney-loon/token-2022-refill-bot/internal/utils"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/token2022"
	"github.com/magooney-loon/token-2022-refill-bot/pkg/wallet"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	configPath = flag.String("config", "config/config.yaml", "Path to configuration file")
)

func displayMenu() int {

	fmt.Println("\n🚀 Token-2022 Bot Menu")
	fmt.Println("1 - Start Bot")
	fmt.Println("2 - Check Wallet")
	fmt.Println("3 - Check Dividends")
	fmt.Println("4 - Analytics")
	fmt.Println("0 - Exit")
	fmt.Print("\nSelect an option: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	choice := 0
	fmt.Sscanf(input, "%d", &choice)
	return choice
}

func main() {
	flag.Parse()

	// Initialize random source
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Play startup animation
	utils.PlayMatrixAnimation()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	// Initialize logger
	if err := utils.InitLogger(
		cfg.Logging.Level,
		cfg.Logging.FilePath,
		cfg.Logging.MaxSizeMB,
		cfg.Logging.MaxBackups,
		cfg.Logging.MaxAgeDays,
		cfg.Logging.Compress,
	); err != nil {
		panic(err)
	}
	defer utils.Close()

	// Create bot instance
	b, err := bot.NewBot(cfg)
	if err != nil {
		utils.Fatal("Failed to create bot", err)
	}

	for {
		choice := displayMenu()

		switch choice {
		case 0:
			utils.Info("Exiting...")
			return
		case 1:
			utils.Info("Starting bot...")

			// Display settings and get confirmation
			fmt.Println("\n📊 Bot Settings:")
			fmt.Printf("Input Token: %s\n", cfg.Token.InputMint)
			fmt.Printf("Output Token: %s\n", cfg.Token.OutputMint)
			if cfg.Token.DividendMint != "" {
				fmt.Printf("Dividend Token: %s\n", cfg.Token.DividendMint)
			}
			fmt.Printf("Swap Amount: %.2f\n", cfg.Token.SwapAmount)
			fmt.Printf("Slippage: %.2f%%\n", float64(cfg.Token.SlippageBPS)/100)
			fmt.Printf("Min SOL Balance: %.2f\n", cfg.Wallet.MinSolBalance)
			fmt.Printf("Reserve Amount: %.2f\n", cfg.Wallet.ReserveAmount)
			fmt.Printf("Check Interval: %d minutes\n", cfg.Monitor.CheckIntervalMinutes)
			fmt.Printf("Direct Routes Only: %v\n", cfg.Jupiter.OnlyDirectRoutes)

			fmt.Print("\nDo you want to start the bot with these settings? (y/n): ")
			reader := bufio.NewReader(os.Stdin)
			confirm, _ := reader.ReadString('\n')
			confirm = strings.TrimSpace(strings.ToLower(confirm))

			if confirm != "y" && confirm != "yes" {
				fmt.Println("Bot start cancelled")
				continue
			}

			// Create context that can be cancelled
			ctx, cancel := context.WithCancel(context.Background())

			// Handle shutdown signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				sig := <-sigChan
				utils.Info("Received shutdown signal", "signal", sig)
				cancel()
			}()

			if err := b.Start(ctx); err != nil {
				utils.Error("Bot error", err)
			}
			cancel()
		case 2:
			utils.Info("Checking wallet...")

			// Create RPC client
			rpcClient := rpc.New(cfg.RPC.Endpoint)

			// Create token client
			tokenClient := token2022.NewClient(cfg, rpcClient)

			// Get public key from private key
			privKey := solana.MustPrivateKeyFromBase58(cfg.Wallet.PrivateKey)
			defaultWallet := privKey.PublicKey().String()

			// Prompt for wallet address
			walletAddr, err := wallet.PromptWalletAddress(defaultWallet)
			if err != nil {
				fmt.Printf("❌ Error: %s\n", err)
				continue
			}

			// Display balances with portfolio info
			if err := wallet.DisplayWalletBalances(context.Background(), cfg, rpcClient, tokenClient, walletAddr); err != nil {
				utils.Error("Failed to display wallet balances", err)
			}
		case 3:
			utils.Info("Checking dividends...")

			if cfg.Token.DividendMint == "" {
				utils.Error("No dividend address configured", fmt.Errorf("please set dividend_mint in config.yaml"))
				continue
			}

			// Create RPC client
			rpcClient := rpc.New(cfg.RPC.Endpoint)

			// Create token client
			tokenClient := token2022.NewClient(cfg, rpcClient)

			// Ask for wallet address
			fmt.Print("\n👛 Enter wallet address to check (or press Enter to use your wallet): ")
			reader := bufio.NewReader(os.Stdin)
			walletAddr, _ := reader.ReadString('\n')
			walletAddr = strings.TrimSpace(walletAddr)

			// If no address provided, use configured wallet
			if walletAddr == "" {
				privKey := solana.MustPrivateKeyFromBase58(cfg.Wallet.PrivateKey)
				walletAddr = privKey.PublicKey().String()
				utils.Info("Using configured wallet address", "address", walletAddr[:8]+"..."+walletAddr[len(walletAddr)-8:])
			}

			// Validate wallet address
			if _, err := solana.PublicKeyFromBase58(walletAddr); err != nil {
				utils.Error("Invalid wallet address", err)
				continue
			}

			// Display dividend info
			if err := wallet.DisplayDividendInfo(context.Background(), cfg, rpcClient, tokenClient, walletAddr); err != nil {
				utils.Error("Failed to display dividend info", err)
			}
		case 4:
			utils.Info("Opening analytics...")
			// TODO: Implement analytics
			fmt.Println("Analytics functionality coming soon!")
		default:
			fmt.Println("Invalid option, please try again")
		}
	}
}
