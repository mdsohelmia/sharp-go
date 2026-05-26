package sharp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

// seedFixture loads a fixture from disk; used to seed fuzz corpora.
func seedFixture(name string) []byte {
	b, err := os.ReadFile(filepath.Join("test", "fixtures", name))
	if err != nil {
		return nil
	}
	return b
}

// FuzzMetadataDecode feeds arbitrary bytes to Metadata and asserts no panic.
// Decode failures are expected and must return as errors, not crashes.
func FuzzMetadataDecode(f *testing.F) {
	for _, name := range []string{"320x240.jpg", "Flag_of_the_Netherlands.png"} {
		if b := seedFixture(name); b != nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = sharp.FromBytes(data).Metadata(context.Background())
	})
}

// FuzzJPEGEncode feeds arbitrary bytes through the full decode-encode loop.
func FuzzJPEGEncode(f *testing.F) {
	for _, name := range []string{"320x240.jpg", "Flag_of_the_Netherlands.png"} {
		if b := seedFixture(name); b != nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = sharp.FromBytes(data).
			Resize(sharp.ResizeOptions{Width: 64, Height: 64}).
			JPEG(format.JPEGOptions{Quality: 50}).
			ToBytes(context.Background())
	})
}
