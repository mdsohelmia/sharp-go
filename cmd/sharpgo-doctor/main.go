// sharpgo-doctor prints the detected libvips environment.
// Use it to verify a new install can find and link against libvips.
package main

/*
#cgo pkg-config: vips
#include <stdlib.h>
#include <vips/vips.h>
#include <vips/vector.h>
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/internal/vips"
)

// Formats sharp exposes via toFormat. Probed against the running libvips.
var formats = []struct {
	name      string
	loader    string
	saver     string
}{
	{"jpeg", "jpegload", "jpegsave"},
	{"png", "pngload", "pngsave"},
	{"webp", "webpload", "webpsave"},
	{"avif", "heifload", "heifsave"}, // AVIF goes through heif* with compression=av1
	{"gif", "gifload", "gifsave"},
	{"tiff", "tiffload", "tiffsave"},
	{"heif", "heifload", "heifsave"},
	{"jxl", "jxlload", "jxlsave"},
	{"jp2", "jp2kload", "jp2ksave"},
	{"svg", "svgload", ""},
	{"pdf", "pdfload", ""},
	{"raw", "rawload", "rawsave"},
}

func main() {
	if err := vips.InitError(); err != nil {
		fmt.Fprintln(os.Stderr, "libvips init failed:", err)
		os.Exit(1)
	}

	major, minor, micro := vips.Version()
	fmt.Println("sharp-go doctor")
	fmt.Println("---------------")
	fmt.Printf("libvips version : %d.%d.%d\n", major, minor, micro)
	fmt.Printf("concurrency     : %d\n", sharp.Concurrency())
	fmt.Printf("SIMD enabled    : %v\n", C.vips_vector_isenabled() != 0)
	fmt.Println()

	fmt.Println("Format    Load   Save")
	fmt.Println("-------   ----   ----")
	for _, f := range formats {
		fmt.Printf("%-9s %-6s %-6s\n", f.name, ok(f.loader), ok(f.saver))
	}
}

func ok(op string) string {
	if op == "" {
		return "-"
	}
	if hasOp(op) {
		return "yes"
	}
	return "no"
}

func hasOp(name string) bool {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	base := C.CString("VipsOperation")
	defer C.free(unsafe.Pointer(base))
	return C.vips_type_find(base, cs) != 0
}
