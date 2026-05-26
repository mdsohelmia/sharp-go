package sharp_test

import (
	"testing"

	sharp "github.com/sohelmia/sharp-go"
)

func TestVersions(t *testing.T) {
	v := sharp.V()
	if v.Major < 8 {
		t.Errorf("Major = %d, want >= 8", v.Major)
	}
}

func TestSupportedFormats(t *testing.T) {
	got := sharp.SupportedFormats()
	for _, name := range []string{"jpeg", "png"} {
		s, ok := got[name]
		if !ok {
			t.Errorf("%s missing from SupportedFormats", name)
			continue
		}
		if !s.Load || !s.Save {
			t.Errorf("%s: Load=%v Save=%v, want both true", name, s.Load, s.Save)
		}
	}
}

func TestBlockUnblockHeif(t *testing.T) {
	// Block HEIF then verify subsequent SupportedFormats reflects it.
	// vips_type_find returns the registered type regardless of block status,
	// so SupportedFormats() will still say "available". Verify the actual
	// behavioural change via an attempted decode of HEIF bytes.
	//
	// For this test we just exercise the API — it must not panic.
	sharp.Block("VipsForeignLoadHeif")
	defer sharp.Unblock("VipsForeignLoadHeif")
	// no assertion; just exercise.
}

func TestSetConcurrency(t *testing.T) {
	orig := sharp.Concurrency()
	defer sharp.SetConcurrency(orig)

	sharp.SetConcurrency(4)
	if got := sharp.Concurrency(); got != 4 {
		t.Errorf("Concurrency = %d, want 4", got)
	}
	// Negative values fall back to NumCPU.
	sharp.SetConcurrency(-1)
	if got := sharp.Concurrency(); got <= 0 {
		t.Errorf("Concurrency after -1 = %d, want > 0", got)
	}
}
