package screens

import "math/rand"

// splashPhrases are the Minecraft-style one-liners shown by the wordmark on the
// landing screen — geeky, dev-flavoured, with a wink at the coffee/snacks/SSH
// nature of the shop. Keep them short (they sit to the top-right of CONSOLE).
// Add freely: the more there are, the rarer a repeat.
var splashPhrases = []string{
	"It works on my machine!",
	"git push --force",
	"sudo make me coffee",
	"404: hunger not found",
	"while(coffee){ code(); }",
	":wq to checkout",
	"No mouse required",
	"Powered by caffeine",
	"rm -rf /problems",
	"It's not a bug!",
	"Tabs > spaces",
	"Ship it!",
	"Stack Overflow approved",
	"Have you tried rebooting?",
	"Cold brew driven dev",
	"Feed your daemon",
	"npm install coffee",
	"Hangry-driven development",
	"Now Turing complete",
	"Out of memory, into snacks",
	"Latency: 12.4ms",
	"cat /dev/snacks",
	"Segfault before coffee",
	"Also try Swiggy!",
	"Order from the shell",
	"O(1) checkout",
	"curl -X ORDER",
	"Vim users welcome",
	"Real devs eat in the terminal",
	"Commit message: \"fix\"",
}

// RandomPhrase returns a splash phrase at random, avoiding prev so the same
// line never shows twice in a row. The global rand source is auto-seeded
// (Go 1.20+), so each process picks a fresh sequence.
func RandomPhrase(prev string) string {
	switch len(splashPhrases) {
	case 0:
		return ""
	case 1:
		return splashPhrases[0]
	}
	for {
		p := splashPhrases[rand.Intn(len(splashPhrases))]
		if p != prev {
			return p
		}
	}
}
