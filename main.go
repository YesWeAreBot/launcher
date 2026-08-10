// Command yesimbot-cli is the YesImBot launcher CLI: it initializes Koishi
// Apps and manages the Koishi/YesImBot child process.
package main

import (
	"os"

	"launcher/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
