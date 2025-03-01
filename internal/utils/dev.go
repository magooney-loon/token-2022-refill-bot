package utils

import (
	"fmt"
	"math/rand"
	"time"
)

const (
	matrixChars = "ｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ1234567890"
	botArt      = `
  ████████╗ █████╗ ██╗  ██╗    ████████╗ ██████╗ ██╗  ██╗███████╗███╗   ██╗    ██████╗  ██████╗ ████████╗
  ╚══██╔══╝██╔══██╗╚██╗██╔╝    ╚══██╔══╝██╔═══██╗██║ ██╔╝██╔════╝████╗  ██║    ██╔══██╗██╔═══██╗╚══██╔══╝
     ██║   ███████║ ╚███╔╝        ██║   ██║   ██║█████╔╝ █████╗  ██╔██╗ ██║    ██████╔╝██║   ██║   ██║   
     ██║   ██╔══██║ ██╔██╗        ██║   ██║   ██║██╔═██╗ ██╔══╝  ██║╚██╗██║    ██╔══██╗██║   ██║   ██║   
     ██║   ██║  ██║██╔╝ ██╗       ██║   ╚██████╔╝██║  ██╗███████╗██║ ╚████║    ██████╔╝╚██████╔╝   ██║   
     ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝       ╚═╝    ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═══╝    ╚═════╝  ╚═════╝    ╚═╝   

	 POWERED BY: https://bonerbot.tech

╭──────────────────────────────── Support the Dev ────────────────────────────────╮
│  If You want to support the dev's local AI machine build:                       │
│  Framework Desktop DIY Edition (AMD Ryzen™ AI Max 300 Series)                   │
│                                                                                 │
│  Donate to: 8XaW537nayBrvcEUPozfkkAS4KtPycniuwKGeC9UJsqA                        │
│                                                                                 │
│  Thank You and enjoy the bot! <3                                                │
╰─────────────────────────────────────────────────────────────────────────────────╯`
)

// ClearScreen clears the terminal screen
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// PlayMatrixAnimation plays a Matrix-style animation with the Token2022Bot logo
func PlayMatrixAnimation() {
	ClearScreen()

	// Get terminal size
	width := 80
	height := 20

	// Create matrix columns
	columns := make([]int, width)
	for i := range columns {
		columns[i] = rand.Intn(height)
	}

	// Create color map for gradient effect
	colors := []string{
		"\033[38;5;22m", // dark green
		"\033[38;5;28m", // medium green
		"\033[38;5;34m", // bright green
		"\033[38;5;40m", // very bright green
		"\033[38;5;46m", // neon green
	}

	// Play animation
	for frame := 0; frame < 50; frame++ {
		fmt.Print("\033[H") // Move cursor to top

		// Draw matrix effect
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				if y < columns[x] {
					// Calculate color based on position
					colorIndex := (y + frame) % len(colors)
					fmt.Print(colors[colorIndex], string(matrixChars[rand.Intn(len(matrixChars))]))
				} else {
					fmt.Print(" ")
				}
			}
			fmt.Println()
		}

		// Update columns
		for i := range columns {
			if columns[i] > 0 && rand.Float32() < 0.1 {
				columns[i]--
			}
		}

		// Add new drops
		if frame%2 == 0 {
			columns[rand.Intn(width)] = height
		}

		time.Sleep(50 * time.Millisecond)
	}

	ClearScreen()

	// Display bot art with donation message in green
	fmt.Print("\033[38;5;46m") // Set bright green color
	fmt.Print(botArt)
	fmt.Print("\033[0m")               // Reset color
	fmt.Print("\n\n")                  // Add some spacing
	time.Sleep(100 * time.Millisecond) // Small delay to ensure visibility
}
