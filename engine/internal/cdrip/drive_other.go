// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

//go:build !windows

package cdrip

import "errors"

// Reading a physical audio CD comes with these systems' releases. Images work anywhere.
func openDrive(string) (Source, error) {
	return nil, errors.New("reading audio CDs from a drive isn't supported on this system yet")
}
