package sharp_test

import (
	"context"
	"sync"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestLogSinkInstallable(t *testing.T) {
	// We can't reliably provoke a libvips warning in a fixture test, so this
	// test asserts the sink installs + uninstalls cleanly and a regular
	// successful pipeline still works while the sink is installed.
	var mu sync.Mutex
	captured := []string{}
	sharp.SetLogSink(func(domain string, level sharp.LogLevel, message string) {
		mu.Lock()
		captured = append(captured, message)
		mu.Unlock()
	})
	defer sharp.SetLogSink(nil)

	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	// captured may be empty if libvips emitted no warnings; assertion is just
	// that nothing crashed.
}
