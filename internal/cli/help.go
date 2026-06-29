package cli

import (
	"fmt"
	"io"
)

func printUsage(out io.Writer) {
	fmt.Fprint(out, `consolestore — terminal food ordering

usage:
  store                       open the interactive app (TUI)
  store status                show your live order status (or none)
  store order <name>          order a saved preset (lists them if several share the name)
  store order <name> <n>      order the nth same-named preset directly
  store alias list            list your saved presets
  store alias rm <name> [n]   remove preset <name> (the nth, if several share it)
  store whoami                show connection + saved addresses
  store logout                disconnect your Swiggy account
  store help                  show this help

presets are created inside the app: build a cart, press : and run
  alias set <name>

presets are bound to the restaurant + delivery address they were saved with.
`)
}
