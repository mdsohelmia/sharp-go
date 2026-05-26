package sharp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

// fixturePath resolves a fixture from the vendored test/fixtures dir at the
// module root (go test runs with CWD = the package's dir = module root).
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("test", "fixtures", name))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	return abs
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

// readFixtureBytes reads a fixture from disk without requiring a *testing.T.
// Returns an error if the file is missing; benchmarks call Skip on err.
func readFixtureBytes(name string) ([]byte, error) {
	abs, err := filepath.Abs(filepath.Join("test", "fixtures", name))
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

func TestJPEGRoundtrip(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	out, info, err := sharp.FromBytes(in).JPEG(format.JPEGOptions{Quality: 80}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", info.Format)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", info.Width, info.Height)
	}
	if info.Size != len(out) {
		t.Errorf("Size = %d, len(out) = %d", info.Size, len(out))
	}
	if len(out) < 1024 {
		t.Errorf("output suspiciously small: %d bytes", len(out))
	}
	// JPEG SOI marker
	if out[0] != 0xFF || out[1] != 0xD8 {
		t.Errorf("output is not JPEG (no SOI marker)")
	}
}

func TestPNGRoundtrip(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	out, info, err := sharp.FromBytes(in).PNG(format.PNGOptions{}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Format != "png" {
		t.Errorf("Format = %q, want png", info.Format)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Errorf("dimensions = %dx%d, want positive", info.Width, info.Height)
	}
	// PNG signature
	want := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	for i, b := range want {
		if out[i] != b {
			t.Errorf("output is not PNG (signature mismatch at %d)", i)
			break
		}
	}
}

func TestFormatInferenceFromExt(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")

	info, err := sharp.FromBytes(in).ToFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ToFile: %v", err)
	}
	if info.Format != "png" {
		t.Errorf("Format = %q, want png", info.Format)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Size() == 0 {
		t.Errorf("output file is empty")
	}
}

func TestFromFile(t *testing.T) {
	path := fixturePath(t, "2569067123_aca715a2ee_o.jpg")
	out, info, err := sharp.FromFile(path).JPEG(format.JPEGOptions{Quality: 70}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", info.Format)
	}
	if len(out) == 0 {
		t.Errorf("empty output")
	}
}
