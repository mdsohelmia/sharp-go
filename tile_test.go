package sharp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
)

func TestToTilesDZ(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	dir := t.TempDir()
	prefix := filepath.Join(dir, "pyramid")

	info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 800}).
		ToTiles(context.Background(), prefix, sharp.TileOptions{
			Layout: sharp.TileLayoutDZ,
			Size:   256,
		})
	if err != nil {
		t.Fatalf("ToTiles: %v", err)
	}
	if info.Width != 800 {
		t.Errorf("info.Width = %d, want 800", info.Width)
	}

	// DZ layout writes <prefix>.dzi + <prefix>_files/.
	if _, err := os.Stat(prefix + ".dzi"); err != nil {
		t.Errorf("expected %s.dzi: %v", prefix, err)
	}
	if _, err := os.Stat(prefix + "_files"); err != nil {
		t.Errorf("expected %s_files dir: %v", prefix, err)
	}
}

func TestToTilesZIP(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	dir := t.TempDir()
	prefix := filepath.Join(dir, "out")

	_, err := sharp.FromBytes(in).
		ToTiles(context.Background(), prefix, sharp.TileOptions{
			Layout:    sharp.TileLayoutDZ,
			Container: sharp.TileContainerZIP,
			Size:      128,
		})
	if err != nil {
		t.Fatalf("ToTiles ZIP: %v", err)
	}
	// libvips may write the ZIP either as <prefix> or <prefix>.zip depending
	// on version; accept either.
	if _, errPrefix := os.Stat(prefix); errPrefix != nil {
		if _, errZip := os.Stat(prefix + ".zip"); errZip != nil {
			t.Errorf("expected %s or %s.zip: %v / %v", prefix, prefix, errPrefix, errZip)
		}
	}
}
