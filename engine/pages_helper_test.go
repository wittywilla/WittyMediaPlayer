// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package main

import (
	"encoding/json"
	"testing"

	"github.com/wittywilla/wittymediaserver/pkg/sdui"
)

// The window asks the helper for a page with what it keeps, and gets a document the server's own
// decoder accepts, with the disc modules the helper itself knows.
func TestHelperMakesPages(t *testing.T) {
	c := startHelper(t)
	var raw json.RawMessage
	params := map[string]any{"page": "modules", "width": 900, "height": 700, "state": map[string]any{"version": "0.2.0", "offers": map[string]string{"wittydvdplayer": "9.9.9"}}}
	if _, err := c.Call("pages", params, &raw); err != nil {
		t.Fatal(err)
	}
	doc, err := sdui.Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var names, installs int
	var walk func(n sdui.Node)
	walk = func(n sdui.Node) {
		if n.Type == "ListItem" && (n.ID == "mod-wittydvdplayer" || n.ID == "mod-wittyblurayplayer") {
			names++
		}
		if n.Action != nil && n.Action.Op == "app.modules.install" && n.Action.Params["version"] == "9.9.9" {
			installs++
		}
		for _, k := range n.Children {
			walk(k)
		}
	}
	walk(doc.Root)
	if names != 2 || installs != 1 {
		t.Errorf("modules listed %d, install offers %d", names, installs)
	}
	if _, err := c.Call("pages", map[string]any{"page": "nope"}, &raw); err == nil {
		t.Error("a page that doesn't exist must be refused")
	}
}
