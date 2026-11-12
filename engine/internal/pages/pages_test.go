// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package pages

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/wittywilla/wittymediaserver/pkg/sdui"
)

var idChars = regexp.MustCompile(`^[A-Za-z0-9_.:~-]{1,128}$`)

func sampleGuide() *Guide {
	return &Guide{
		Feature:    Choice{ID: "f", Label: "Title 3", DurationSec: 6000, Chapters: 12, Titles: []int{3}, Title: 3, Main: true},
		Why:        "It is the longest title with chapters.",
		Reasons:    []string{"Twelve chapters", "Longest by far"},
		Level:      "high",
		Confidence: 0.93,
		Versions:   []Choice{{ID: "v1", Label: "Extended cut", DurationSec: 6600, Title: 4}},
		Episodes:   []Choice{{ID: "e1", Label: "Episode 1", DurationSec: 1300, Title: 5, StartSec: 43}},
		PlayAll:    Choice{ID: "all", Label: "Play all", Title: 5},
		Others:     []Choice{{ID: "o1", Label: "Trailer", Detail: "trailer", DurationSec: 90, Title: 9}},
		Decoys:     2, RepairCount: 5, Repairs: []string{"a", "b", "c", "d", "e"}, PaddedSectors: 3, Warnings: []string{"A warning"},
	}
}

func state() State {
	return State{
		Version: "0.2.0",
		Recent:  []Recent{{Path: "C:\\Movies\\film.mkv"}, {Path: "/tmp/gone.iso", Gone: true}},
		Offers:  map[string]string{"wittydvdplayer": "0.2.1"},
		Themes: []Theme{{ID: "default", Name: "Witty", Scheme: "dark", Tokens: map[string]string{"bg": "#101114", "accent": "#e4e6eb"}},
			{ID: "my", Name: "Mine", Scheme: "light", Imported: true}},
		Theme: "default", Finishes: true,
		Disc: &Disc{Kind: "dvd", Label: "FILM", Guide: sampleGuide(), PlayingTitle: 9, Chosen: "v1"},
		CD: &CD{Title: "Album", Artist: "Band", Formats: []string{"FLAC", "MP3"}, Format: "FLAC", Qualities: []string{"5", "8"}, Quality: "5",
			Tracks:   []CDTrack{{Number: 1, Title: "One", DurationSec: 180, Checked: true}, {Number: 2, DurationSec: 200, Percent: 40}, {Number: 3, Data: true}},
			Template: "{artist}/{title}", Drive: true, CanLookUp: true},
		Remote: RemoteState{Kind: "dvd", Angles: 1},
	}
}

func request(page string) Request {
	return Request{Page: page, Width: 1100, Height: 760, State: state(),
		Modules: []Module{{ID: "wittydvdplayer", Name: "WittyDVD", InUse: "0.2.0", Builtin: true, CanGoBack: true, Note: "ok"}},
		Drives:  []Drive{{Path: "D:\\", Name: "D:", Media: true, Label: "FILM"}, {Path: "E:\\", Name: "E:", Media: true, Audio: true}, {Path: "F:\\", Name: "F:"}}}
}

var allPages = []string{PageStart, PageSettings, PageModules, PageLook, PageCD, PageDisc, PageWhy, PageDiagnosis, PageRemote, PageCredits}

func decode(t *testing.T, page string) sdui.Document {
	t.Helper()
	raw, err := Render(request(page))
	if err != nil {
		t.Fatalf("%s: %v", page, err)
	}
	doc, err := sdui.Unmarshal(raw) // strict: unknown fields, version
	if err != nil {
		t.Fatalf("%s: %v", page, err)
	}
	return doc
}

func walk(n sdui.Node, f func(sdui.Node)) {
	f(n)
	for _, c := range n.Children {
		walk(c, f)
	}
}

func TestEveryPageIsAValidDocument(t *testing.T) {
	for _, p := range allPages {
		doc := decode(t, p)
		if doc.Catalog != 7 || doc.Root.Type != "Page" || doc.Page.ID != p {
			t.Fatalf("%s: catalog %d root %s page %s", p, doc.Catalog, doc.Root.Type, doc.Page.ID)
		}
		seen := map[string]bool{}
		order := 0
		walk(doc.Root, func(n sdui.Node) {
			if !idChars.MatchString(n.ID) {
				t.Errorf("%s: bad id %q", p, n.ID)
			}
			if seen[n.ID] {
				t.Errorf("%s: duplicate id %q", p, n.ID)
			}
			seen[n.ID] = true
			if _, ok := sdui.Component(n.Type); !ok {
				t.Errorf("%s: %q is not in the catalogue", p, n.Type)
			}
			if n.Focus != nil {
				order++
				if n.Focus.Order != order {
					t.Errorf("%s: focus order of %s is %d, want %d", p, n.ID, n.Focus.Order, order)
				}
			}
			if sdui.Focusable(&n, false) && n.Type != "Card" && n.Focus == nil && n.Action != nil {
				t.Errorf("%s: %s can be acted on and has no focus", p, n.ID)
			}
			if a := n.Action; a != nil {
				switch a.Type {
				case sdui.ActNavigate:
					if !sdui.ValidRoute(a.To) {
						t.Errorf("%s: %s routes to %q", p, n.ID, a.To)
					}
					if strings.HasPrefix(a.To, "/page/") && !slices.Contains(allPages, strings.TrimPrefix(a.To, "/page/")) {
						t.Errorf("%s: %s opens a page that doesn't exist: %q", p, n.ID, a.To)
					}
				case sdui.ActCommand:
					if !slices.Contains(Actions, a.Op) {
						t.Errorf("%s: %s uses %q, which the window doesn't carry out", p, n.ID, a.Op)
					}
				default:
					t.Errorf("%s: %s has action type %q", p, n.ID, a.Type)
				}
			}
		})
	}
}

func TestStartListsDrivesAndRecent(t *testing.T) {
	doc := decode(t, PageStart)
	var play, image, gone []string
	walk(doc.Root, func(n sdui.Node) {
		switch {
		case n.Type == "Button" && strings.HasSuffix(n.ID, "-play"):
			play = append(play, n.Props["label"].(string))
			if n.Action == nil {
				play[len(play)-1] += "(off)"
			}
		case n.Type == "Button" && strings.HasSuffix(n.ID, "-image"):
			image = append(image, n.ID)
		case n.Type == "ListItem" && n.Props["trailing"] == "not found":
			gone = append(gone, n.ID)
			if n.Action != nil {
				t.Error("a missing file must not open")
			}
		}
	})
	if !slices.Equal(play, []string{"Play", "Open CD", "Play(off)"}) {
		t.Errorf("drive buttons: %v", play)
	}
	if len(image) != 1 { // a CD's image is made on its own screen, an empty drive has nothing
		t.Errorf("image buttons: %v", image)
	}
	if len(gone) != 1 {
		t.Errorf("missing recent: %v", gone)
	}
}

func TestRemoteKeepsEveryKeyAndDimsByKind(t *testing.T) {
	table, err := loadRemote()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"dvd", "bluray", "video", "cd"} {
		for _, angles := range []int{1, 3} {
			req := request(PageRemote)
			req.Width = 1600
			req.State.Remote = RemoteState{Kind: kind, Angles: angles}
			raw, err := Render(req)
			if err != nil {
				t.Fatal(err)
			}
			doc, _ := sdui.Unmarshal(raw)
			got := map[string]bool{}
			dimmed := map[string]bool{}
			walk(doc.Root, func(n sdui.Node) {
				if k, ok := n.Props["key"].(string); ok {
					got[k] = true
					d := n.Props["dimmed"] == true
					dimmed[k] = d
					if d && n.Action != nil || !d && n.Action == nil {
						t.Errorf("%s: key %s dimmed %v but action %v", kind, k, d, n.Action)
					}
				}
			})
			for _, k := range table.Keys {
				if !got[k.ID] {
					t.Errorf("%s: key %s is missing: keys are never hidden", kind, k.ID)
				}
			}
			want := table.dimmed(kind, angles)
			for id, d := range dimmed {
				if d != want[id] {
					t.Errorf("%s/%d angles: key %s dimmed %v, want %v", kind, angles, id, d, want[id])
				}
			}
		}
	}
	if dim := table.dimmed("dvd", 3); !dim["red"] || dim["angle"] {
		t.Errorf("a DVD dims the colour keys and not ANGLE: %v", dim)
	}
	if dim := table.dimmed("bluray", 1); !dim["angle"] || dim["red"] {
		t.Errorf("a one-angle Blu-ray dims ANGLE only: %v", dim)
	}
}

func TestNarrowRemoteIsTwoPages(t *testing.T) {
	req := request(PageRemote)
	req.Width = 420
	req.State.Remote = RemoteState{Kind: "dvd", Angles: 1, Page: 1}
	raw, _ := Render(req)
	doc, _ := sdui.Unmarshal(raw)
	keys := map[string]bool{}
	walk(doc.Root, func(n sdui.Node) {
		if k, ok := n.Props["key"].(string); ok {
			keys[k] = true
		}
	})
	if !keys["1"] || keys["play"] {
		t.Errorf("page 1 holds the digits and colours: %v", keys)
	}
}

func TestDiscPagesNeedADiscAndSayWhy(t *testing.T) {
	req := request(PageDisc)
	req.State.Disc = nil
	if _, err := Render(req); err == nil {
		t.Error("a disc page with no disc open must be an error")
	}
	doc := decode(t, PageWhy)
	var all []string
	walk(doc.Root, func(n sdui.Node) {
		if s, ok := n.Props["text"].(string); ok {
			all = append(all, s)
		}
	})
	joined := strings.Join(all, "\n")
	for _, want := range []string{"longest title", "93%", "never from unlocking"} {
		if !strings.Contains(joined, want) {
			t.Errorf("why page lacks %q:\n%s", want, joined)
		}
	}
}

func TestDiscPageOffersTheViewersChoices(t *testing.T) {
	doc := decode(t, PageDisc)
	ops := map[string]bool{}
	walk(doc.Root, func(n sdui.Node) {
		if n.Action != nil && n.Action.Type == sdui.ActCommand {
			ops[n.Action.Op] = true
		}
	})
	for _, op := range []string{"app.disc.choose", "app.disc.makeMain", "app.disc.forget"} {
		if !ops[op] {
			t.Errorf("missing %s", op)
		}
	}
}

func TestLookGivesEachThemeItsOwnColours(t *testing.T) {
	doc := decode(t, PageLook)
	var cards []sdui.Node
	walk(doc.Root, func(n sdui.Node) {
		if n.Type == "Card" {
			cards = append(cards, n)
		}
	})
	if len(cards) != 2 || cards[0].Props["itemId"] != "my" {
		t.Fatalf("imported themes come first: %+v", cards)
	}
	if cards[1].Tokens["accent"] != "#e4e6eb" {
		t.Error("a theme's card wears its own tokens")
	}
}

func TestCDPage(t *testing.T) {
	doc := decode(t, PageCD)
	var tracks, ticked int
	walk(doc.Root, func(n sdui.Node) {
		if n.Type == "Track" {
			tracks++
			if n.Props["checked"] == true {
				ticked++
			}
			if n.Props["number"] == 3 && n.Action != nil {
				t.Error("a data track can't be played from here")
			}
		}
	})
	if tracks != 3 || ticked != 1 {
		t.Errorf("tracks %d ticked %d", tracks, ticked)
	}
}

func TestUnknownPage(t *testing.T) {
	if _, err := Render(Request{Page: "nope"}); err == nil {
		t.Error("expected an error")
	}
}

func TestDocumentIsJSONAsTheServerWritesIt(t *testing.T) {
	raw, err := Render(request(PageStart))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"version", "catalog", "page", "layout", "root", "meta"} {
		if _, ok := m[k]; !ok {
			t.Errorf("no %q", k)
		}
	}
}

func textsOf(doc sdui.Document) string {
	var all []string
	walk(doc.Root, func(n sdui.Node) {
		for _, k := range []string{"text", "title", "subtitle", "trailing", "label"} {
			if s, ok := n.Props[k].(string); ok {
				all = append(all, s)
			}
		}
	})
	return strings.Join(all, "\n")
}

// What the window's own "why this title" text said before the page replaced it: the reason, how
// sure, the length, the parts, the warnings, and that nothing was unlocked.
func TestWhyPageSaysEverythingTheOldTextDid(t *testing.T) {
	req := request(PageWhy)
	req.State.Disc = &Disc{Kind: "dvd", Label: "FILM", Guide: &Guide{
		Feature: Choice{ID: "feature", Label: "Titles 7, 3, 9, 2", Title: 7, Titles: []int{7, 3, 9, 2}, DurationSec: 7200, Chapters: 30, OneStream: true},
		Why:     "Title 7 is the main feature because the disc starts it by itself.", Level: "high", Confidence: 0.99,
		Reasons: []string{"It plays first"}, Warnings: []string{"The disc has 9 decoy titles."},
	}}
	raw, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := sdui.Unmarshal(raw)
	got := textsOf(doc)
	for _, want := range []string{"Title 7 is the main feature because the disc starts it by itself.", "high (99%).", "Length: 2:00:00, 30 chapters.",
		"4 titles (7, 3, 9, 2). They play here as one film", "The disc has 9 decoy titles.", "never from unlocking anything"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// A Blu-ray says playlist, and a module with no guide says why there is none.
	req.State.Disc = &Disc{Kind: "bluray", Guide: &Guide{Feature: Choice{ID: "f", Label: "Playlist 00100", DurationSec: 6084.7, Chapters: 11}, Level: "medium", Confidence: 0.7}}
	raw, _ = Render(req)
	doc, _ = sdui.Unmarshal(raw)
	if doc.Page.Title != "Why this playlist" {
		t.Errorf("title %q", doc.Page.Title)
	}
	req.State.Disc = &Disc{Kind: "dvd"}
	raw, _ = Render(req)
	doc, _ = sdui.Unmarshal(raw)
	if !strings.Contains(textsOf(doc), "from before it could say why") {
		t.Errorf("no guide: %s", textsOf(doc))
	}
}

func TestDiagnosisCountsWhatWasMendedAndLost(t *testing.T) {
	req := request(PageDiagnosis)
	req.State.Disc = &Disc{Kind: "bluray", Guide: &Guide{Feature: Choice{ID: "f"}, RepairCount: 5, Repairs: []string{"a", "b", "c", "d", "e"}, PaddedSectors: 2, Decoys: 9}}
	raw, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := sdui.Unmarshal(raw)
	got := textsOf(doc)
	for _, want := range []string{"playlists, clip information and stream files disagree in 5 places", "and 1 more", "Sectors that couldn't be read", "9"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	req.State.Disc = &Disc{Kind: "dvd", Guide: &Guide{Feature: Choice{ID: "f"}}}
	raw, _ = Render(req)
	doc, _ = sdui.Unmarshal(raw)
	if !strings.Contains(textsOf(doc), "Nothing needed mending") {
		t.Error("a clean disc says so")
	}
}

func TestSettingsCreditsVLCWhereItIsChosen(t *testing.T) {
	doc := decode(t, PageSettings)
	if !strings.Contains(textsOf(doc), "VideoLAN") {
		t.Error("the VLC credit must be on the page that chooses it")
	}
}

func TestAGuideIsCutAndCleaned(t *testing.T) {
	g := &Guide{Level: "weird", Confidence: 7, Decoys: -3, Warnings: []string{" ", "x"}}
	for i := 0; i < 500; i++ {
		g.Others = append(g.Others, Choice{ID: "t", Label: "T"})
	}
	g.Others = append(g.Others, Choice{Label: "no id"})
	g.clean()
	if len(g.Others) != maxChoices || g.Level != "low" || g.Confidence != 1 || g.Decoys != 0 || len(g.Warnings) != 1 {
		t.Errorf("%d %s %v %d %v", len(g.Others), g.Level, g.Confidence, g.Decoys, g.Warnings)
	}
}
