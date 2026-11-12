// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

// Package pages is the player's local page provider: it makes the SDUI documents the window
// draws for every screen that is not playback (the start page, File Settings, disc modules,
// the Look page, the CD page, a disc's guide and WittyRemote), from the window's own state,
// with no server. A document is the same tree WittyMediaServer sends its apps (catalogue
// revision 7, WittyMediaServer docs/SDUI.md), built with the server's pkg/sdui types, so the
// window's renderer is tested against the shared conformance fixtures and can draw a page a
// server sent just the same.
//
// The provider is stateless: a request carries the page and a State (what the window keeps in
// its settings, and what the disc and CD sessions know). Drives and disc modules are read here,
// by the engine. Actions the page offers are routes (/page/<id>) and commands whose op starts
// with "app." (see Actions): the window carries them out and asks for the page again.
package pages

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wittywilla/wittymediaserver/pkg/sdui"
)

// Page ids.
const (
	PageStart       = "start"
	PageSettings    = "settings"
	PageModules     = "modules"
	PageLook        = "look"
	PageCD          = "cd"
	PageDisc        = "disc"
	PageWhy         = "why"
	PageDiagnosis   = "diagnosis"
	PageRemote      = "remote"
	PageCredits     = "about"
	catalogRevision = 7
)

// Request is one call: the page, what the window knows, and how wide the window is.
type Request struct {
	Page string `json:"page"`
	// Width and Height of the area the page is drawn in, in points. The breakpoint follows.
	Width  int   `json:"width"`
	Height int   `json:"height"`
	State  State `json:"state"`
	// Modules and Drives are filled in by the engine, not the window.
	Modules []Module `json:"-"`
	Drives  []Drive  `json:"-"`
}

// State is what the window keeps.
type State struct {
	Version string   `json:"version"`
	Recent  []Recent `json:"recent"`
	// What plays the media.
	UseVLC    bool   `json:"useVlc"`
	VLCFolder string `json:"vlcFolder"`
	VLCStatus string `json:"vlcStatus"`
	// Module updates.
	CheckModules bool              `json:"checkModules"`
	Offers       map[string]string `json:"offers"`
	ModuleNote   string            `json:"moduleNote"`
	// Look.
	Theme     string  `json:"theme"`
	Themes    []Theme `json:"themes"`
	Finish    string  `json:"finish"`
	Finishes  bool    `json:"finishes"`
	FirstRun  bool    `json:"firstRun"`
	ThemeNote string  `json:"themeNote"`
	// Notice is a line the window wants shown on the page (a drive that failed, a copy that finished).
	Notice string `json:"notice"`
	// The open disc, as the disc module's guide gave it, and the player's own notes about it.
	Disc *Disc `json:"disc"`
	// The open audio CD.
	CD *CD `json:"cd"`
	// WittyRemote.
	Remote RemoteState `json:"remote"`
	// Locks and forced captions, the two disc options.
	ObeyLocks   bool `json:"obeyLocks"`
	ForcedSubs  bool `json:"forcedSubs"`
	Visualizer  bool `json:"visualizer"`
	ThemeImport bool `json:"themeImport"`
}

// Recent is something opened lately.
type Recent struct {
	Path string `json:"path"`
	// Gone: the file is no longer where it was.
	Gone bool `json:"gone"`
}

// Theme is what the Look page shows of one theme: the colours of its card.
type Theme struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Scheme   string            `json:"scheme"`
	Desc     string            `json:"description"`
	Imported bool              `json:"imported"`
	Finish   string            `json:"finish"`
	Tokens   map[string]string `json:"tokens"`
}

// Module is one disc module's status (engine/internal/modhost.Status).
type Module struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	InUse     string `json:"inUse"`
	Builtin   bool   `json:"isBuiltin"`
	CanGoBack bool   `json:"canGoBack"`
	Note      string `json:"note"`
}

// Drive is an optical drive (engine/internal/drives.Drive).
type Drive struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Label string `json:"label"`
	Media bool   `json:"media"`
	Audio bool   `json:"audio"`
}

// Render makes the document for a request. An unknown page is an error: pages are a closed set.
func Render(req Request) (json.RawMessage, error) {
	b := newBuilder(req)
	var err error
	switch req.Page {
	case PageStart:
		b.start()
	case PageSettings:
		b.settings()
	case PageModules:
		b.modules()
	case PageLook:
		b.look()
	case PageCD:
		b.cd()
	case PageDisc:
		err = b.discPage()
	case PageWhy:
		err = b.why()
	case PageDiagnosis:
		err = b.diagnosis()
	case PageRemote:
		err = b.remote()
	case PageCredits:
		b.about()
	default:
		return nil, fmt.Errorf("pages: %q is not a page of this player", req.Page)
	}
	if err != nil {
		return nil, err
	}
	doc := b.document()
	out, err := sdui.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Actions the window carries out. Every op is "app." and a verb; the table is the contract
// between this package and the window (src/sdui/appactions.cpp), checked by a test.
var Actions = []string{
	"app.openFile", "app.openFolder", "app.open", "app.makeImage", "app.clearRecent", "app.forget",
	"app.vlc.set", "app.vlc.folder",
	"app.modules.check", "app.modules.install", "app.modules.back", "app.modules.atStart",
	"app.theme.set", "app.theme.import", "app.theme.remove", "app.finish.set",
	"app.cd.toggle", "app.cd.all", "app.cd.none", "app.cd.play", "app.cd.copy", "app.cd.stop", "app.cd.image", "app.cd.lookup",
	"app.cd.album", "app.cd.rename", "app.cd.format", "app.cd.quality", "app.cd.folder", "app.cd.secure", "app.cd.offset", "app.cd.template",
	"app.disc.choose", "app.disc.makeMain", "app.disc.forget", "app.disc.locks", "app.disc.forced",
	"app.remote.key", "app.remote.page", "app.remote.close", "app.firstRun.done",
}

// ---------------------------------------------------------------- builder

type props = map[string]any

type nodeT = sdui.Node

type builder struct {
	req    Request
	st     State
	title  string
	icon   string
	id     string
	layout *sdui.Layout
	root   sdui.Node
	kids   []sdui.Node
	scope  *sdui.ThemeScope
}

func newBuilder(req Request) *builder {
	l := sdui.ProfileFor(sdui.FormDesktop, "")
	if req.Width > 0 {
		l = l.WithViewport(sdui.Viewport{Width: req.Width, Height: req.Height})
	}
	return &builder{req: req, st: req.State, id: req.Page, layout: l}
}

func (b *builder) page(title, icon string) { b.title, b.icon = title, icon }
func (b *builder) add(n ...sdui.Node)      { b.kids = append(b.kids, n...) }

func (b *builder) document() sdui.Document {
	root := sdui.Node{Type: "Page", ID: "page", Props: props{"title": b.title}, Children: b.kids}
	stampFocus(&root)
	return sdui.Document{
		Version: sdui.SchemaVersion,
		Catalog: catalogRevision,
		Page:    sdui.PageInfo{ID: b.id, Kind: "custom", Title: b.title, Icon: b.icon, Route: "/page/" + b.id},
		Layout:  b.layout,
		Root:    root,
		Meta:    sdui.Meta{Source: "builtin", TTLSeconds: 0},
	}
}

func node(typ, id string, p props, kids ...sdui.Node) sdui.Node {
	return sdui.Node{Type: typ, ID: cleanID(id), Props: p, Children: kids}
}

// cleanID keeps to the characters a node ID may have.
func cleanID(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == ':', r == '-', r == '~':
			sb.WriteRune(r)
		default:
			sb.WriteByte('~')
		}
	}
	out := sb.String()
	if len(out) > 128 {
		out = out[:128]
	}
	return out
}

func navigate(to string) *sdui.Action { return &sdui.Action{Type: sdui.ActNavigate, To: to} }

// command is an app command: op is one of Actions.
func command(op, item string, params props) *sdui.Action {
	return &sdui.Action{Type: sdui.ActCommand, Op: op, Item: item, Params: params}
}

func external(url string) *sdui.Action { return &sdui.Action{Type: sdui.ActExternal, URL: url} }

func text(id, s, role string) sdui.Node {
	return node("Text", id, props{"text": s, "role": role})
}

func para(id, s string) sdui.Node { return node("Text", id, props{"text": s, "role": "body"}) }
func caption(id, s string) sdui.Node {
	return node("Text", id, props{"text": s, "role": "caption"})
}
func heading(id, s string) sdui.Node { return text(id, s, "heading") }

func notice(id, tone, title, body string) sdui.Node {
	p := props{"tone": tone, "text": body}
	if title != "" {
		p["title"] = title
	}
	return node("Notice", id, p)
}

func button(id, label, variant string, a *sdui.Action) sdui.Node {
	n := node("Button", id, props{"label": label, "variant": variant})
	n.Action = a
	return n
}

func row(id string, kids ...sdui.Node) sdui.Node {
	return node("Stack", id, props{"direction": "row", "gap": "s", "wrap": true}, kids...)
}

func column(id string, kids ...sdui.Node) sdui.Node {
	return node("Stack", id, props{"direction": "column", "gap": "s"}, kids...)
}

func list(id, title string, kids ...sdui.Node) sdui.Node {
	p := props{}
	if title != "" {
		p["title"] = title
	}
	return node("List", id, p, kids...)
}

// item is a ListItem that does something when chosen. An empty action makes a plain row.
func item(id, title, subtitle, trailing, icon string, a *sdui.Action) sdui.Node {
	p := props{"title": title}
	if subtitle != "" {
		p["subtitle"] = subtitle
	}
	if trailing != "" {
		p["trailing"] = trailing
	}
	if icon != "" {
		p["icon"] = icon
	}
	n := node("ListItem", id, p)
	n.Action = a
	return n
}

func divider(id string) sdui.Node { return node("Divider", id, nil) }

// legend is the Dock every page ends with: its keys.
func dock(id string, actions []string, buttons ...sdui.Node) sdui.Node {
	kids := append([]sdui.Node{}, buttons...)
	kids = append(kids, node("KeyLegend", id+"-keys", props{"actions": actions}))
	return node("Dock", id, props{"title": "Options", "legend": true}, kids...)
}

// stampFocus numbers what a person can act on, in reading order, as the server's renderer
// does: depth first, a Stack row or a group component making a focus section.
func stampFocus(root *sdui.Node) {
	order := 0
	var first *sdui.Node
	var walk func(n *sdui.Node, group string, inHero bool)
	walk = func(n *sdui.Node, group string, inHero bool) {
		if sdui.Focusable(n, inHero) {
			order++
			n.Focus = &sdui.Focus{Order: order, Group: group}
			if first == nil {
				first = n
			}
		}
		g := group
		if spec, ok := sdui.Component(n.Type); ok && spec.Focus == "group" || n.Type == "Stack" && n.Props["direction"] == "row" {
			g = n.ID
		}
		for i := range n.Children {
			walk(&n.Children[i], g, inHero || n.Type == "Hero")
		}
	}
	walk(root, "", false)
	if first != nil {
		first.Focus.Default = true
	}
}
