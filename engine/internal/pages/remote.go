// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package pages

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wittywilla/wittymediaserver/pkg/sdui"
)

// RemoteState says what is playing, so the remote knows which keys stay dimmed.
type RemoteState struct {
	// Kind is dvd, bluray, video or cd.
	Kind string `json:"kind"`
	// Angles is how many camera angles the disc has; one angle dims ANGLE on a Blu-ray.
	Angles int `json:"angles"`
	// Page is 0 (menus, options, pad, transport) or 1 (digits, colours); a window too small for both uses it.
	Page int `json:"page"`
}

// remoteTable is the catalogue's `remote` table: WittyRemote's keys and which stay dimmed.
type remoteTable struct {
	Keys []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Group string `json:"group"`
	} `json:"keys"`
	Groups []string   `json:"groups"`
	Pages  [][]string `json:"pages"`
	Kinds  map[string]struct {
		Label      string              `json:"label"`
		Dimmed     []string            `json:"dimmed"`
		DimmedWhen map[string][]string `json:"dimmedWhen"`
	} `json:"kinds"`
	Rule string `json:"rule"`
}

func loadRemote() (*remoteTable, error) {
	var t remoteTable
	if err := json.Unmarshal(sdui.Catalog().Remote, &t); err != nil {
		return nil, fmt.Errorf("pages: the catalogue's remote table: %w", err)
	}
	return &t, nil
}

// Dimmed lists the keys a kind of disc doesn't use, from the catalogue.
func (t *remoteTable) dimmed(kind string, angles int) map[string]bool {
	out := map[string]bool{}
	k, ok := t.Kinds[kind]
	if !ok {
		return out
	}
	for _, id := range k.Dimmed {
		out[id] = true
	}
	if angles <= 1 {
		for _, id := range k.DimmedWhen["oneAngle"] {
			out[id] = true
		}
	}
	return out
}

// remote is WittyRemote: every key stays in place, a key the disc doesn't use dimmed and with
// no action, so it can't be pressed and is drawn with a dashed outline.
func (b *builder) remote() error {
	t, err := loadRemote()
	if err != nil {
		return err
	}
	b.page("Remote", "devices")
	kind := b.st.Remote.Kind
	if kind == "" {
		kind = "video"
	}
	dim := t.dimmed(kind, b.st.Remote.Angles)
	label := kind
	if k, ok := t.Kinds[kind]; ok && k.Label != "" {
		label = k.Label
	}
	if strings.HasPrefix(label, "A ") {
		label = "a " + label[2:] // "A video file" in the middle of a sentence
	}
	b.add(caption("remote-for", "For "+label+". Keys it doesn't use stay where they are, dimmed."))
	pages := t.Pages
	if len(pages) == 0 {
		pages = [][]string{t.Groups}
	}
	want := b.st.Remote.Page
	// A window wide enough shows both pages at once; a narrow one, the page asked for.
	both := b.layout.Breakpoint != "compact" && b.layout.Breakpoint != "medium"
	for pi, groups := range pages {
		if !both && pi != want {
			continue
		}
		for _, g := range groups {
			var keys []nodeT
			for _, k := range t.Keys {
				if k.Group != g {
					continue
				}
				n := node("Button", "key-"+k.ID, props{"label": k.Label, "key": k.ID, "group": g, "variant": "secondary"})
				switch k.ID {
				case "enter":
					n.Props["variant"] = "primary"
				case "eject":
					n.Props["tone"] = "danger"
				}
				if dim[k.ID] {
					n.Props["dimmed"] = true
				} else {
					n.Action = command("app.remote.key", k.ID, nil)
				}
				keys = append(keys, n)
			}
			if len(keys) > 0 {
				b.add(node("Stack", "keys-"+g, props{"direction": "row", "gap": "xs", "role": "remote", "group": g}, keys...))
			}
		}
	}
	if !both && len(pages) > 1 {
		other := 1 - want
		b.add(button("more-keys", map[int]string{0: "Keys", 1: "Digits and colours"}[other], "ghost",
			command("app.remote.page", "", props{"page": other})))
	}
	b.add(dock("dock", []string{"select", "back"}))
	return nil
}
