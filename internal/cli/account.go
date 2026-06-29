package cli

import "fmt"

// runLogout disconnects the Swiggy account (purges the keyring token). A no-op
// when not signed in.
func runLogout(d Deps) int {
	st := newStyle(d.Color)
	if !d.SignedIn {
		fmt.Fprintf(d.Out, "%s\n", st.dim("not signed in — nothing to disconnect."))
		return 0
	}
	if err := d.Backend.Logout(); err != nil {
		fmt.Fprintf(d.Out, "store: logout failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(d.Out, "%s\n%s\n", st.ok("✓ disconnected from Swiggy."), st.dim("run `store` to reconnect."))
	return 0
}

// runWhoami reports the connection state and the account's saved addresses.
func runWhoami(d Deps) int {
	st := newStyle(d.Color)
	if !d.SignedIn {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn("not signed in."), st.dim("run `store` to connect your Swiggy account."))
		return 0
	}
	fmt.Fprintf(d.Out, "%s\n", st.ok("✓ connected to Swiggy."))
	addrs, err := d.Backend.Addresses()
	if err != nil {
		fmt.Fprintf(d.Out, "store: %v\n", err)
		return 1
	}
	if len(addrs) > 0 {
		fmt.Fprintf(d.Out, "%s\n", st.dim("saved addresses:"))
		for _, a := range addrs {
			label := a.Label
			if label == "" {
				label = shortAddr(a.Line)
			}
			fmt.Fprintf(d.Out, "  %s %s  %s\n", st.num("·"), st.head(label), st.dim(shortAddr(a.Line)))
		}
	}
	return 0
}
