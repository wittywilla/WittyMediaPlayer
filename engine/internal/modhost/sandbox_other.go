// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

//go:build !windows

package modhost

import "os/exec"

// The limits on other systems come with their releases.
func prepareSandbox(*exec.Cmd) {}

func lockDown(*exec.Cmd) func() { return func() {} }
