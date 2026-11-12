// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package pages

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// VLCCredit is the credit the VLC option carries wherever the libraries are named.
const VLCCredit = "VLC and libVLC are made by VideoLAN and the VLC contributors (videolan.org), and are free software under the LGPL 2.1 or later. Thank you to them. WittyMediaPlayer isn't part of VLC or made by VideoLAN."

// nav is the row of buttons that moves between the app's pages.
func (b *builder) nav(current string) {
	type entry struct{ id, label, icon string }
	entries := []entry{
		{PageStart, "Start", "home"},
		{PageSettings, "Playback", "settings"},
		{PageModules, "Disc modules", "plug"},
		{PageLook, "Look", "palette"},
		{PageCredits, "About", "info"},
	}
	var kids = make([]nodeT, 0, len(entries))
	for _, e := range entries {
		v := "ghost"
		if e.id == current {
			v = "primary"
		}
		to := "/page/" + e.id
		if e.id == PageStart {
			to = "/"
		}
		n := button("nav-"+e.id, e.label, v, navigate(to))
		n.Props["icon"] = e.icon
		kids = append(kids, n)
	}
	b.add(row("nav", kids...))
}

func (b *builder) pageNotice() {
	if b.st.Notice != "" {
		b.add(notice("note", "info", "", b.st.Notice))
	}
}

func baseName(p string) string {
	n := filepath.Base(strings.ReplaceAll(p, "\\", "/"))
	if n == "." || n == "/" || n == "" {
		return p
	}
	return n
}

func dirName(p string) string {
	d := filepath.Dir(strings.ReplaceAll(p, "\\", "/"))
	if d == "." {
		return ""
	}
	return d
}

func driveWhat(d Drive) string {
	switch {
	case !d.Media:
		return "no disc"
	case d.Audio:
		return "audio CD"
	case d.Label != "":
		return d.Label
	}
	return "disc"
}

// start is the page with nothing open: open something, drives, recent items.
func (b *builder) start() {
	b.page("WittyMediaPlayer", "play")
	b.add(
		text("brand", "WittyMediaPlayer", "display"),
		caption("tagline", "Video, audio, DVDs and Blu-rays, on this computer."),
	)
	b.pageNotice()
	open := button("open-file", "Open a file", "primary", command("app.openFile", "", nil))
	open.Props["icon"] = "folder"
	folder := button("open-folder", "Open a disc folder", "secondary", command("app.openFolder", "", nil))
	folder.Props["icon"] = "disc"
	b.add(row("open", open, folder), caption("drop", "Or drop a file, a disc image or a disc folder on this window."))

	if len(b.req.Drives) == 0 {
		b.add(list("drives", "Disc drives", item("no-drive", "No disc drive found", "", "", "disc", nil)))
	} else {
		var rows []nodeT
		for i, d := range b.req.Drives {
			id := fmt.Sprintf("drive-%d", i)
			verb := "Play"
			if d.Audio {
				verb = "Open CD"
			}
			play := button(id+"-play", verb, "secondary", command("app.open", d.Path, props{"path": d.Path}))
			play.Props["a11y"] = fmt.Sprintf("%s the disc in %s", verb, d.Name)
			var kids []nodeT
			it := item(id, d.Name, driveWhat(d), "", "disc", nil)
			kids = append(kids, it, play)
			if d.Media {
				if !d.Audio {
					img := button(id+"-image", "Make image", "ghost", command("app.makeImage", d.Path, props{"path": d.Path}))
					img.Props["a11y"] = fmt.Sprintf("Make a disc image from %s", d.Name)
					kids = append(kids, img)
				}
			} else {
				play.Action = nil // nothing to open: a button with no action is drawn disabled
			}
			rows = append(rows, row(id+"-row", kids...))
		}
		b.add(list("drives", "Disc drives", rows...))
	}

	if len(b.st.Recent) == 0 {
		b.add(list("recent", "Recent", item("recent-none", "Nothing opened yet", "", "", "", nil)))
	} else {
		var rows []nodeT
		for i, r := range b.st.Recent {
			sub := dirName(r.Path)
			trailing := ""
			var a = command("app.open", r.Path, props{"path": r.Path})
			if r.Gone {
				trailing, a = "not found", nil
			}
			rows = append(rows, item(fmt.Sprintf("recent-%d", i), baseName(r.Path), sub, trailing, "video", a))
		}
		clear := button("recent-clear", "Clear this list", "ghost", command("app.clearRecent", "", nil))
		rows = append(rows, clear)
		b.add(list("recent", "Recent", rows...))
	}
	if b.st.UseVLC {
		b.add(notice("vlc-credit", "info", "Playing through VLC's libraries", VLCCredit))
	}
	b.add(dock("dock", []string{"select", "back", "options"},
		button("dock-settings", "Settings", "ghost", navigate("/page/"+PageSettings)),
		button("dock-look", "Look", "ghost", navigate("/page/"+PageLook)),
	))
}

// settings is File, Settings: what plays the media.
func (b *builder) settings() {
	b.page("Settings", "settings")
	b.nav(PageSettings)
	b.pageNotice()
	sel := func(on bool) string {
		if on {
			return "Selected"
		}
		return ""
	}
	b.add(list("engine", "What plays your media",
		item("engine-builtin", "Built in (FFmpeg, built for this player)",
			"The lightest choice, and the one tested most. Subtitles and menus are at their best here.", sel(!b.st.UseVLC), "play",
			command("app.vlc.set", "", props{"on": false})),
		item("engine-vlc", "VLC libraries, from a VLC on this computer",
			"Useful for a format the built-in choice can't play, and it plays surround sound. Disc menus still come from WittyDVD and WittyBluray, and copy-protected discs are still refused: VLC's own disc modules aren't used.",
			sel(b.st.UseVLC), "play", command("app.vlc.set", "", props{"on": true})),
	))
	if b.st.UseVLC {
		folder := b.st.VLCFolder
		if folder == "" {
			folder = "Not chosen: look for VLC on this computer"
		}
		b.add(list("vlc", "VLC",
			item("vlc-folder", "VLC's folder", folder, "Choose…", "folder", command("app.vlc.folder", "", nil)),
		))
		if b.st.VLCStatus != "" {
			b.add(notice("vlc-status", "info", "", b.st.VLCStatus))
		}
	}
	b.add(caption("vlc-credit", VLCCredit))
	b.add(list("disc-options", "Discs",
		item("obey-locks", "Obey the disc's restrictions",
			"Some discs forbid skipping warnings or changing tracks at certain points, as a disc player would.", onOff(b.st.ObeyLocks), "lock",
			command("app.disc.locks", "", props{"on": !b.st.ObeyLocks})),
		item("forced", "Show forced captions when subtitles are off", "", onOff(b.st.ForcedSubs), "subtitles",
			command("app.disc.forced", "", props{"on": !b.st.ForcedSubs})),
	))
	b.add(dock("dock", []string{"select", "back"}))
}

func onOff(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}

// modules is the disc modules and their updates.
func (b *builder) modules() {
	b.page("Disc modules", "plug")
	b.nav(PageModules)
	b.pageNotice()
	b.add(para("modules-note", "WittyDVD and WittyBluray can each be updated without a new player. Only versions signed by this project run, and the built-in copy is always there if one doesn't work. Looking for updates is the only time this player uses the internet, and it talks only to GitHub."))
	var rows []nodeT
	for _, m := range b.req.Modules {
		which := "updated"
		if m.Builtin {
			which = "built in"
		}
		sub := which
		if m.Note != "" {
			sub += ". " + m.Note
		}
		line := []nodeT{item("mod-"+m.ID, m.Name, sub, m.InUse, "plug", nil)}
		if v := b.st.Offers[m.ID]; v != "" {
			install := button("mod-"+m.ID+"-install", "Install "+v, "primary", command("app.modules.install", m.ID, props{"version": v}))
			install.Props["a11y"] = fmt.Sprintf("Install %s version %s", m.Name, v)
			line = append(line, install)
		}
		if m.CanGoBack {
			back := button("mod-"+m.ID+"-back", "Go back", "ghost", command("app.modules.back", m.ID, nil))
			back.Props["a11y"] = fmt.Sprintf("Go back to the earlier version of %s", m.Name)
			line = append(line, back)
		}
		rows = append(rows, row("mod-"+m.ID+"-row", line...))
	}
	b.add(list("modules", "Modules", rows...))
	if b.st.ModuleNote != "" {
		b.add(notice("modules-status", "info", "", b.st.ModuleNote))
	}
	check := button("check", "Look for updates now", "primary", command("app.modules.check", "", nil))
	check.Props["icon"] = "refresh"
	b.add(list("updates", "Updates",
		item("at-start", "Look when the player starts", "", onOff(b.st.CheckModules), "refresh",
			command("app.modules.atStart", "", props{"on": !b.st.CheckModules}))), check)
	b.add(dock("dock", []string{"select", "back"}))
}

// look is the theme picker, with the finish choice and theme import.
func (b *builder) look() {
	b.page("Look", "palette")
	if !b.st.FirstRun {
		b.nav(PageLook)
	} else {
		b.add(text("welcome", "Choose how WittyMediaPlayer looks", "pageTitle"),
			caption("welcome-note", "You can change this any time in Settings, Look. Every theme here is one WittyMediaServer ships, and you can bring your own."))
	}
	b.pageNotice()
	themes := append([]Theme(nil), b.st.Themes...)
	sort.SliceStable(themes, func(i, j int) bool { return themes[i].Imported && !themes[j].Imported })
	var cards []nodeT
	for _, t := range themes {
		sub := t.Scheme
		if t.Imported {
			sub = "imported, " + t.Scheme
		}
		if t.ID == b.st.Theme {
			sub += ", in use"
		}
		c := node("Card", "theme-"+t.ID, props{"style": "square", "shape": "square", "title": t.Name, "subtitle": sub, "itemId": t.ID, "kind": "view", "icon": "palette"})
		if t.Desc != "" {
			c.Props["synopsis"] = t.Desc
		}
		if t.ID == b.st.Theme {
			c.Props["badges"] = []string{"In use"}
		}
		c.Action = command("app.theme.set", t.ID, nil)
		// The card wears the theme it offers: its colours are the page's tokens for it.
		c.Tokens = t.Tokens
		cards = append(cards, c)
	}
	grid := node("Grid", "themes", props{"title": "Themes", "cardStyle": "square", "cardSize": "m"}, cards...)
	b.add(grid)
	if b.st.Finishes {
		opts := []struct{ id, label, note string }{
			{"theme", "The theme's own", "Gloss, glass or bevel where the theme asks for it"},
			{"flat", "Flat", "No gloss, glass or bevel anywhere"},
		}
		var rows []nodeT
		for _, o := range opts {
			on := b.st.Finish == o.id || (b.st.Finish == "" && o.id == "theme")
			rows = append(rows, item("finish-"+o.id, o.label, o.note, selMark(on), "", command("app.finish.set", o.id, nil)))
		}
		b.add(list("finish", "Finish", rows...))
	}
	imp := button("import", "Import a theme…", "secondary", command("app.theme.import", "", nil))
	imp.Props["icon"] = "plus"
	b.add(row("tools", imp), caption("import-note", "A ZIP with theme.json and theme.css, as WittyMediaServer reads: variables and fonts only. Anything else in the CSS is refused and the first rule is named."))
	if b.st.FirstRun {
		done := button("done", "Use this look", "primary", command("app.firstRun.done", "", nil))
		b.add(row("first-run", done))
	}
	b.add(dock("dock", []string{"select", "back"}))
}

func selMark(on bool) string {
	if on {
		return "Selected"
	}
	return ""
}

func (b *builder) about() {
	b.page("About", "info")
	b.nav(PageCredits)
	v := b.st.Version
	b.add(text("about-title", "WittyMediaPlayer "+v, "title"),
		para("about-what", "A player for video, audio, DVDs and Blu-rays on your own computer, with real disc menus. No server, no account, no telemetry."),
		para("about-built", "Built on Qt and FFmpeg, and on WittyDVD, WittyBluray, WittyCD, WittyDisc and WittyModule. Its pages and themes are WittyMediaServer's: server-driven UI, drawn here from the player's own data."),
		para("about-vlc", VLCCredit),
		para("about-cd", "If you ask, CD names come from MusicBrainz and covers from the Cover Art Archive. Nothing else leaves this computer."),
		caption("about-licence", "GNU GPL v3.0 or later. The disc libraries it is built from are under the GNU AGPL v3.0 or later."),
	)
	b.add(dock("dock", []string{"select", "back"}))
}
