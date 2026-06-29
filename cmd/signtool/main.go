// Command signtool is a dev/CI-only tool (never shipped to users). It generates
// the ed25519 signing keypair and signs the release manifest envelope that the
// store binary verifies with its embedded public key.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"console.store/internal/updater"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: signtool keygen | sign --version V --channel C --dir D --out F")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "keygen":
		keygen()
	case "sign":
		if err := signCmd(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "signtool:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "signtool: unknown subcommand", os.Args[1])
		os.Exit(2)
	}
}

func keygen() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "signtool:", err)
		os.Exit(1)
	}
	fmt.Println("PUBLIC=" + base64.StdEncoding.EncodeToString(pub))
	fmt.Println("PRIVATE=" + base64.StdEncoding.EncodeToString(priv))
}

func signCmd(args []string) error {
	flags := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		flags[strings.TrimPrefix(args[i], "--")] = args[i+1]
	}
	for _, k := range []string{"version", "channel", "dir", "out"} {
		if flags[k] == "" {
			return fmt.Errorf("sign: missing required --%s", k)
		}
	}
	keyB64 := os.Getenv("CONSOLE_SIGN_KEY")
	if keyB64 == "" {
		return fmt.Errorf("CONSOLE_SIGN_KEY not set")
	}
	keyB, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return fmt.Errorf("CONSOLE_SIGN_KEY: invalid base64")
	}
	if len(keyB) != ed25519.PrivateKeySize {
		return fmt.Errorf("CONSOLE_SIGN_KEY: wrong length (want %d bytes)", ed25519.PrivateKeySize)
	}
	priv := ed25519.PrivateKey(keyB)

	assets, err := readAssets(flags["dir"])
	if err != nil {
		return err
	}
	env, err := buildEnvelope(priv, flags["version"], flags["channel"], assets)
	if err != nil {
		return err
	}
	return os.WriteFile(flags["out"], env, 0o644)
}

// readAssets reads dist/SHA256SUMS and returns the asset map from it.
// This is robust to goreleaser's per-build subdirectory layout (store_linux_amd64_v1/store)
// because the flat published names and their hashes live in SHA256SUMS, not as top-level files.
func readAssets(dir string) (map[string]string, error) {
	path := filepath.Join(dir, "SHA256SUMS")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return assetsFromChecksums(data)
}

// assetsFromChecksums parses SHA256SUMS file bytes and returns a map of
// asset key → hex sha256. Asset keys are derived by stripping the "store_"
// prefix and ".exe" suffix from each asset name. Lines not starting with
// "store_" are skipped.
func assetsFromChecksums(data []byte) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash, name := fields[0], fields[1]
		if !strings.HasPrefix(name, "store_") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, "store_"), ".exe")
		out[key] = hash
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no store_* entries in SHA256SUMS")
	}
	return out, nil
}

func buildEnvelope(priv ed25519.PrivateKey, ver, channel string, assets map[string]string) ([]byte, error) {
	pl := updater.Payload{Version: ver, Channel: channel, Assets: assets}
	raw, err := json.Marshal(pl)
	if err != nil {
		return nil, err
	}
	env := updater.Envelope{
		Payload: base64.StdEncoding.EncodeToString(raw),
		Sig:     base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw)),
	}
	return json.Marshal(env)
}
