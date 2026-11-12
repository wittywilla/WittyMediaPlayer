// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package disc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/wittywilla/wittybluray/wittyblurayplayer"
	"github.com/wittywilla/wittydisc/isofs"
	"github.com/wittywilla/wittydisc/session"
	"github.com/wittywilla/wittydvd/wittydvdplayer"
	"github.com/wittywilla/wittymediaplayer/engine/internal/modhost"
	"github.com/wittywilla/wittymodule"
)

// Kinds of thing a path can be.
const (
	KindFile   = "file"
	KindDVD    = "dvd"
	KindBluray = "bluray"
	KindData   = "data" // a disc or image with no video structure
)

// imageExt names the extensions treated as disc images.
func imageExt(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".iso", ".img":
		return true
	}
	return false
}

// Classify says what a path holds without reading more than it has to.
func Classify(p string) (string, error) {
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		base := strings.ToUpper(filepath.Base(filepath.Clean(p)))
		if base == "VIDEO_TS" || exists(p, "VIDEO_TS") {
			return KindDVD, nil
		}
		if base == "BDMV" || base == "BDAV" || exists(p, "BDMV") || exists(p, "BDAV") || exists(p, "AVCHD") || exists(p, "PRIVATE/AVCHD") {
			return KindBluray, nil
		}
		return KindData, nil
	}
	if !imageExt(p) {
		return KindFile, nil
	}
	d, err := isofs.Open(p)
	if err != nil {
		return KindData, nil // not a file system this reader knows
	}
	defer d.Close()
	var dvd, bd bool
	for _, f := range d.Files() {
		up := strings.ToUpper(f.Path)
		dvd = dvd || strings.HasPrefix(up, "VIDEO_TS/")
		bd = bd || strings.HasPrefix(up, "BDMV/") || strings.HasPrefix(up, "BDAV/") || strings.Contains(up, "AVCHD/")
	}
	switch {
	case bd:
		return KindBluray, nil
	case dvd:
		return KindDVD, nil
	}
	return KindData, nil
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil
}

// RefusalError and the rest of the contract come from WittyDisc.
type (
	RefusalError = session.RefusalError
	Session      = session.Session
	Prefs        = session.Prefs
	Presentation = session.Presentation
	Action       = session.Action
	Info         = session.Info
	TitleInfo    = session.TitleInfo
)

// Error codes and plain messages, as the modules give them.
const (
	CodeProtected      = session.CodeProtected
	CodeNoMedia        = session.CodeNoMedia
	CodeNotVideo       = session.CodeNotVideo
	CodeAudioCD        = session.CodeAudioCD
	CodeUnreadable     = session.CodeUnreadable
	CodeMissing        = session.CodeMissing
	MsgDVDProtected    = session.MsgDVDProtected
	MsgBlurayProtected = session.MsgBlurayProtected
	MsgDriveUnreadable = session.MsgDriveUnreadable
	MsgNoMedia         = session.MsgNoMedia
	MsgAudioCD         = session.MsgAudioCD
	MsgNotVideo        = session.MsgNotVideo
)

// IsProtected says an error is the refusal of a protected disc.
func IsProtected(err error) bool { return session.IsProtected(err) }

// Open opens a disc of a known kind with the built-in copy of its module. drive says it
// is read straight from a disc drive.
func Open(path, kind string, drive bool, prefs Prefs) (Session, error) {
	switch kind {
	case KindDVD:
		return wittydvdplayer.Open(path, drive, prefs)
	case KindBluray:
		return wittyblurayplayer.Open(path, drive, prefs)
	}
	return nil, &RefusalError{Code: CodeNotVideo, Message: MsgNotVideo}
}

// CheckProtected refuses a disc of a known kind that carries copy protection, without
// starting a session. It always uses the built-in copies, so what it says doesn't rest
// on anything downloaded.
func CheckProtected(path, kind string, drive bool) error {
	switch kind {
	case KindDVD:
		return wittydvdplayer.Check(path, drive)
	case KindBluray:
		return wittyblurayplayer.Check(path, drive)
	}
	return nil
}

// Module ids, for the kinds of disc.
const (
	ModuleDVD    = "wittydvdplayer"
	ModuleBluray = "wittyblurayplayer"
)

// ModuleFor says which module plays a kind of disc.
func ModuleFor(kind string) string {
	if kind == KindBluray {
		return ModuleBluray
	}
	return ModuleDVD
}

// NewManager knows this player's modules, each with its built-in copy.
func NewManager(hostVersion string) *modhost.Manager {
	mods := []modhost.Module{}
	for _, raw := range [][]byte{wittydvdplayer.Manifest, wittyblurayplayer.Manifest} {
		man, err := wittymodule.ParseManifest(raw)
		if err != nil {
			panic(err) // built in: a test catches this long before anyone sees it
		}
		mods = append(mods, modhost.Module{ID: man.ID, Name: man.Name, Repo: man.Repo, Builtin: man.Version})
	}
	return &modhost.Manager{Dir: modhost.DefaultDir(), Host: hostVersion, Keys: modhost.DefaultKeys(), Modules: mods}
}

// Host answers the calls about one open disc: the session contract's, and the two a
// disc module adds about the engine's ranking of the disc (MethodGuide, MethodChoose).
type Host interface {
	Handle(method string, raw json.RawMessage) (any, []byte, error)
	Close()
}

// The calls a disc module answers on top of the session contract. Both modules answer
// them with the same shape, so the window draws one chooser.
const (
	// MethodGuide gives the title or playlist the engine picked as the film, why,
	// how sure it is, and what else the viewer can choose: versions, episodes, other
	// titles. A module from before these calls refuses it, and the window does without.
	MethodGuide = wittydvdplayer.MethodGuide
	// MethodChoose plays one of the guide's choices by its id.
	MethodChoose = wittydvdplayer.MethodChoose
)

// BuiltinHost answers calls with the built-in copy of a kind of disc's module.
func BuiltinHost(kind string) Host {
	switch kind {
	case KindDVD:
		return wittydvdplayer.NewHost()
	case KindBluray:
		return wittyblurayplayer.NewHost()
	}
	return &session.Host{Open: func(string, bool, Prefs) (Session, error) {
		return nil, &RefusalError{Code: CodeNotVideo, Message: MsgNotVideo}
	}}
}
