package sharp

import "strings"

// isOutOfOrderErr reports whether err is a libvips "out of order read" — the
// signature of a sequential/streaming decode being asked for pixels it can no
// longer serve in order (an interlaced PNG, or a stricter/older libvips
// rejecting a forward skip). Such a pipeline succeeds when re-run with a full
// random-access decode, so it is the trigger for the recovery path in execute.
func isOutOfOrderErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "out of order")
}

// canRetryRandom reports whether im's input can be decoded a second time.
// Buffer, file, raw and synth inputs are re-readable; a streaming reader is
// consumed on first decode (and loadFromReader already does a full
// random-access copy), so there is nothing left to retry.
func (im *Image) canRetryRandom() bool {
	return im.in.reader == nil
}
