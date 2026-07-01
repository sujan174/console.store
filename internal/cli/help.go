package cli

import (
	"fmt"
	"io"
)

func printUsage(out io.Writer) {
	fmt.Fprint(out, `consolestore — terminal food ordering

usage:
  store                       open the interactive app (TUI)
  console status                show your live order status (or none)
  console order <name>          order a saved preset (lists them if several share the name)
  console order <name> <n>      order the nth same-named preset directly
  console alias list            list your saved presets
  console alias rm <name> [n]   remove preset <name> (the nth, if several share it)
  store whoami                show connection + saved addresses
  store logout                disconnect your Swiggy account
  console version               print version + channel
  console update [--channel stable|beta|alpha [--code X]]
                              switch channel, or check for updates now
  console agents [install|list|remove]
                              wire console into your AI agents (MCP + skills)
  console help                  show this help

presets are created inside the app: build a cart, press : and run
  alias set <name>

presets are bound to the restaurant + delivery address they were saved with.
`)
}
