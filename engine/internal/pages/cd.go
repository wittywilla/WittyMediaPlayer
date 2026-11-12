// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package pages

import (
	"fmt"
	"strings"
)

// CD is the open audio CD as the window's CD screen knows it.
type CD struct {
	Title  string    `json:"title"`
	Artist string    `json:"artist"`
	Year   string    `json:"year"`
	Tracks []CDTrack `json:"tracks"`
	// The copying choices.
	Format    string   `json:"format"`
	Formats   []string `json:"formats"`
	Quality   string   `json:"quality"`
	Qualities []string `json:"qualities"`
	Folder    string   `json:"folder"`
	Template  string   `json:"template"`
	Secure    bool     `json:"secure"`
	Offset    int      `json:"offset"`
	// Busy: a copy or an image is being made.
	Busy     bool   `json:"busy"`
	Percent  int    `json:"percent"`
	Status   string `json:"status"`
	Lookup   string `json:"lookup"`
	Report   string `json:"report"`
	ImageDir string `json:"imageDir"`
	// Drive: the disc is in a drive, where careful reading and the drive offset mean something. CanLookUp: a lookup may start.
	Drive     bool `json:"drive"`
	CanLookUp bool `json:"canLookUp"`
}

// CDTrack is one track: its tick, and how its copy is going.
type CDTrack struct {
	Number      int     `json:"number"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	DurationSec float64 `json:"durationSec"`
	Checked     bool    `json:"checked"`
	Percent     int     `json:"percent"`
	Result      string  `json:"result"`
	Playing     bool    `json:"playing"`
	// Data: a data track, which a copy leaves out.
	Data bool `json:"data"`
}

func next(list []string, cur string) string {
	if len(list) == 0 {
		return cur
	}
	for i, v := range list {
		if v == cur {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

// cd is the CD screen's panel: tracks with their ticks, the copy choices, look up, images.
func (b *builder) cd() {
	c := b.st.CD
	b.page("Audio CD", "music")
	if c == nil {
		b.add(notice("no-cd", "info", "", "No audio CD is open. Put one in a drive and open it from the start page."))
		return
	}
	title := c.Title
	if title == "" {
		title = "Audio CD"
	}
	b.page(title, "music")
	head := []string{}
	if c.Artist != "" {
		head = append(head, c.Artist)
	}
	if c.Year != "" {
		head = append(head, c.Year)
	}
	head = append(head, fmt.Sprintf("%d tracks", len(c.Tracks)))
	b.add(text("album", title, "pageTitle"), caption("album-sub", strings.Join(head, " · ")))
	b.add(list("names", "Names",
		item("name-album", "Album", c.Title, "Change…", "edit", command("app.cd.album", "album", nil)),
		item("name-artist", "Artist", c.Artist, "Change…", "edit", command("app.cd.album", "artist", nil)),
		item("name-year", "Year", c.Year, "Change…", "edit", command("app.cd.album", "year", nil)),
	))
	if c.Status != "" {
		tone := "info"
		if c.Busy {
			tone = "info"
		}
		b.add(notice("status", tone, "", c.Status))
	}
	if c.Busy {
		b.add(para("overall", fmt.Sprintf("Copying: %d%%", c.Percent)))
	}
	tracks := make([]nodeT, 0, len(c.Tracks))
	for _, t := range c.Tracks {
		p := props{"title": t.Title, "number": t.Number, "durationSec": int(t.DurationSec),
			"checked": t.Checked, "itemId": fmt.Sprint(t.Number), "toggle": props{"op": "app.cd.toggle", "item": fmt.Sprint(t.Number)},
			"rename": props{"op": "app.cd.rename", "item": fmt.Sprint(t.Number)}}
		if p["title"] == "" {
			p["title"] = fmt.Sprintf("Track %d", t.Number)
		}
		if t.Artist != "" {
			p["artist"] = t.Artist
		}
		if t.Playing {
			p["playing"] = true
		}
		if t.Percent > 0 && t.Percent < 100 {
			p["progress"] = float64(t.Percent) / 100
		}
		if t.Result != "" {
			p["result"] = t.Result
		}
		if t.Data {
			p["note"] = "Data track: not copied"
		}
		n := node("Track", fmt.Sprintf("track-%d", t.Number), p)
		if !t.Data {
			n.Action = command("app.cd.play", fmt.Sprint(t.Number), nil)
		}
		tracks = append(tracks, n)
	}
	b.add(node("TrackList", "tracks", props{"showNumbers": true, "showArtist": c.Artist == "", "showDuration": true, "selectable": true}, tracks...))
	lookup := button("lookup", "Look up names", "secondary", command("app.cd.lookup", "", nil))
	if !c.CanLookUp {
		lookup.Action = nil
	}
	b.add(row("select",
		button("select-all", "Tick all", "ghost", command("app.cd.all", "", nil)),
		button("select-none", "Tick none", "ghost", command("app.cd.none", "", nil)),
		lookup,
	))
	if c.Lookup != "" {
		b.add(caption("lookup-note", c.Lookup))
	}
	folder := c.Folder
	if folder == "" {
		folder = "Not chosen"
	}
	opts := []nodeT{
		item("format", "Format", "", c.Format, "music", command("app.cd.format", next(c.Formats, c.Format), nil)),
		item("quality", "Quality", "", c.Quality, "", command("app.cd.quality", next(c.Qualities, c.Quality), nil)),
		item("folder", "Folder", folder, "Choose…", "folder", command("app.cd.folder", "", nil)),
		item("template", "Names of files", c.Template, "Change…", "edit", command("app.cd.template", "", nil)),
	}
	if c.Drive { // an image reads the same every time: these only mean something for a drive
		opts = append(opts,
			item("secure", "Check every read twice", "Each track is read twice, and again wherever the two disagree. A spot that never reads becomes silence, and the report says how much.", onOff(c.Secure), "shield",
				command("app.cd.secure", "", props{"on": !c.Secure})),
			item("offset", "Drive offset", "Corrects a drive that reads a few samples early or late. Leave it at 0 unless you know your drive's figure.", fmt.Sprint(c.Offset), "settings",
				command("app.cd.offset", "", nil)))
	}
	b.add(list("options", "Copy", opts...))
	copyBtn := button("copy", "Copy ticked tracks", "primary", command("app.cd.copy", "", nil))
	imgBtn := button("image", "Make image", "secondary", command("app.cd.image", "", nil))
	stop := button("stop", "Stop copying", "secondary", command("app.cd.stop", "", nil))
	if c.Busy {
		copyBtn.Action, imgBtn.Action = nil, nil
	} else {
		stop.Action = nil
	}
	b.add(row("do", copyBtn, imgBtn, stop))
	if c.Report != "" {
		b.add(caption("report", c.Report))
	}
	b.add(dock("dock", []string{"select", "back"}))
}
