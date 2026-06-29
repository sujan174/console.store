package updater

import (
	"strconv"
	"strings"
)

// Newer reports whether release want is strictly newer than have.
// Format: vMAJOR.MINOR.PATCH[-(alpha|beta).N]. "dev" sorts below all releases.
func Newer(have, want string) bool { return cmpVer(have, want) < 0 }

// cmpVer returns -1/0/1 for a<b / a==b / a>b.
func cmpVer(a, b string) int {
	if a == b {
		return 0
	}
	if a == "dev" {
		return -1
	}
	if b == "dev" {
		return 1
	}
	an, ap := splitPre(a)
	bn, bp := splitPre(b)
	for i := 0; i < 3; i++ {
		if d := an[i] - bn[i]; d != 0 {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	return cmpPre(ap, bp)
}

// splitPre parses "v1.2.3-beta.4" into ([1 2 3], "beta.4"). Missing prerelease → "".
func splitPre(v string) ([3]int, string) {
	v = strings.TrimPrefix(v, "v")
	core, pre, _ := strings.Cut(v, "-")
	var out [3]int
	for i, p := range strings.SplitN(core, ".", 3) {
		if i < 3 {
			out[i], _ = strconv.Atoi(p)
		}
	}
	return out, pre
}

// preRank: "" (final release) outranks beta outranks alpha.
func preRank(pre string) int {
	switch {
	case pre == "":
		return 3
	case strings.HasPrefix(pre, "beta"):
		return 2
	case strings.HasPrefix(pre, "alpha"):
		return 1
	default:
		return 0
	}
}

func preNum(pre string) int {
	_, n, found := strings.Cut(pre, ".")
	if !found {
		return 0
	}
	x, _ := strconv.Atoi(n)
	return x
}

func cmpPre(a, b string) int {
	if ra, rb := preRank(a), preRank(b); ra != rb {
		if ra < rb {
			return -1
		}
		return 1
	}
	if na, nb := preNum(a), preNum(b); na != nb {
		if na < nb {
			return -1
		}
		return 1
	}
	return 0
}
