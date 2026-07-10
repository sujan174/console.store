package updater

import (
	"io"
	"time"
)

// UIDemo renders the update block with a fake download — dev-only visual check.
func UIDemo(out io.Writer, tick time.Duration) {
	u := newUI(out)
	u.header("v0.4.0-beta.3", "v0.4.0-beta.4", "beta")
	total := int64(9 << 20)
	for done := int64(0); done <= total; done += total / 20 {
		u.progress(done, total)
		time.Sleep(tick)
	}
	u.progressDone()
	u.success()
}
