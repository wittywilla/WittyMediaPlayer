// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package disc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

// Fingerprint names a disc for the window to remember things by, such as the title a
// viewer chose to play as its film. It is worked out from what the disc's index says
// (its kind, and each title's number, length to the second and chapters), so the same
// disc gets the same name from a drive, a folder or an image, whichever module read
// it. The label is left out, because a folder's is its name and an image's is the
// disc's, and so is which title the engine picked as the film, so a newer engine's
// better pick doesn't make a viewer's own choice forgotten. Two discs can only share
// one if every title matches. It is "" for a disc with no titles.
func Fingerprint(info *Info) string {
	if info == nil || len(info.Titles) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "wmp-disc-1\x00%s\x00", info.Kind)
	for _, t := range info.Titles {
		d := t.Duration
		if math.IsNaN(d) || math.IsInf(d, 0) || d < 0 {
			d = 0
		}
		fmt.Fprintf(&b, "%d:%d:%d\x00", t.N, int64(math.Round(d)), t.Chapters)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}
