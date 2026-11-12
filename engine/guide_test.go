// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package main

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wittywilla/wittymediaplayer/engine/internal/disc"
	"github.com/wittywilla/wittymodule"
)

// The disc engines' synthetic discs, through the helper as the window talks to it: an
// open disc comes with the engine's guide to it, and a choice from the guide plays.
// Each engine writes its discs out on request (export_synth_test.go in its repository),
// with what the right answer for each is.

// synthDisc is the part of an engine's list that both engines share.
type synthDisc struct {
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Main          int       `json:"main"`
	Feature       []int     `json:"feature"` // DVD: the titles a film runs through
	FeatureSec    float64   `json:"featureSec"`
	Episodes      []int     `json:"episodes"`
	Editions      []int     `json:"editions"`         // Blu-ray
	EpisodeStarts []float64 `json:"episodeStartsSec"` // DVD
	PartStarts    []float64 `json:"playAllStartsSec"` // Blu-ray
	Image         bool      `json:"image"`

	fingerprint string
}

type synthSet struct {
	once  sync.Once
	dir   string
	list  []synthDisc
	skip  string
	fatal string
}

var synthSets = map[string]*synthSet{disc.KindDVD: {}, disc.KindBluray: {}}

// TestMain removes the exported discs when the tests are done.
func TestMain(m *testing.M) {
	code := m.Run()
	for _, s := range synthSets {
		if s.dir != "" {
			_ = os.RemoveAll(s.dir)
		}
	}
	os.Exit(code)
}

// synthDiscs has an engine write its scenario discs out, once for the whole test
// binary. It skips when the engine's repository isn't checked out beside this one.
func synthDiscs(t *testing.T, kind string) (string, []synthDisc) {
	t.Helper()
	s := loadSynth(kind)
	if s.skip != "" {
		t.Skip(s.skip)
	}
	if s.fatal != "" {
		t.Fatal(s.fatal)
	}
	return s.dir, s.list
}

// loadSynth is synthDiscs without a test to skip or fail: it says which in what it returns.
func loadSynth(kind string) *synthSet {
	repo, mod, env := "WittyDVD", "github.com/wittywilla/wittydvd", "WITTYDVD_SYNTH_OUT"
	if kind == disc.KindBluray {
		repo, mod, env = "WittyBluray", "github.com/wittywilla/wittybluray", "WITTYBLURAY_SYNTH_OUT"
	}
	s := synthSets[kind]
	s.once.Do(func() {
		gobin, err := exec.LookPath("go")
		if err != nil {
			s.skip = "the go command isn't on the PATH, so " + repo + " can't write its synthetic discs out"
			return
		}
		// The engine as this build uses it: a checkout a go.work names, or the
		// module cache's copy of the pinned version.
		out, err := exec.Command(gobin, "list", "-m", "-f", "{{.Dir}}", mod).Output()
		engine := strings.TrimSpace(string(out))
		if err != nil || engine == "" {
			s.skip = "the go command can't say where " + mod + " is, so there are no synthetic discs to play"
			return
		}
		if _, err := os.Stat(filepath.Join(engine, "export_synth_test.go")); err != nil {
			s.skip = "the " + repo + " this build uses (" + engine + ") can't write its synthetic discs out (no export_synth_test.go); a go.work naming a newer checkout can"
			return
		}
		dir, err := os.MkdirTemp("", "wmp-synth-"+kind+"-")
		if err != nil {
			s.fatal = err.Error()
			return
		}
		s.dir = dir
		cmd := exec.Command(gobin, "test", "-run", "^TestExportSyntheticDiscs$", "-count=1", ".")
		cmd.Dir = engine
		cmd.Env = append(os.Environ(), env+"="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			s.fatal = repo + " couldn't write its synthetic discs: " + err.Error() + "\n" + string(out)
			return
		}
		b, err := os.ReadFile(filepath.Join(dir, "expected.json"))
		if err == nil {
			err = json.Unmarshal(b, &s.list)
		}
		if err != nil {
			s.fatal = "expected.json: " + err.Error()
		}
	})
	return s
}

// guideChoice and guideReply are the guide as the window reads it.
type guideChoice struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Label    string  `json:"label"`
	Title    int     `json:"title"`
	Titles   []int   `json:"titles"`
	Duration float64 `json:"durationSec"`
	StartSec float64 `json:"startSec"`
	Main     bool    `json:"main"`
}

type guideReply struct {
	Kind       string        `json:"kind"`
	Feature    *guideChoice  `json:"feature"`
	Why        string        `json:"why"`
	Confidence float64       `json:"confidence"`
	Level      string        `json:"level"`
	Versions   []guideChoice `json:"versions"`
	Episodes   []guideChoice `json:"episodes"`
	Others     []guideChoice `json:"others"`
	Warnings   []string      `json:"warnings"`
}

func openSynth(t *testing.T, c *wittymodule.Conn, kind, path string) (openReply, guideReply) {
	t.Helper()
	var reply openReply
	if _, err := c.Call("open", map[string]any{"path": path}, &reply); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if reply.Kind != kind || reply.Info == nil || reply.Presentation == nil || len(reply.Guide) == 0 {
		t.Fatalf("%s opened as %+v", path, reply)
	}
	var g guideReply
	if err := json.Unmarshal(reply.Guide, &g); err != nil {
		t.Fatal(err)
	}
	return reply, g
}

func choose(t *testing.T, c *wittymodule.Conn, id string) disc.Presentation {
	t.Helper()
	var p disc.Presentation
	if _, err := c.Call(disc.MethodChoose, map[string]any{"id": id}, &p); err != nil {
		t.Fatalf("choosing %s: %v", id, err)
	}
	if p.Kind != "title" || p.Title == nil || p.Media == nil || len(p.Media.Extents) == 0 {
		t.Fatalf("choosing %s gave %+v", id, p)
	}
	return p
}

func nearSec(a, b float64) bool { return math.Abs(a-b) < 2 }

// Every synthetic disc of both engines: the open disc names the film the engine picked,
// says why and how sure, and playing the feature plays that film at its whole length.
func TestOpenDiscComesWithItsGuide(t *testing.T) {
	for _, kind := range []string{disc.KindDVD, disc.KindBluray} {
		t.Run(kind, func(t *testing.T) {
			dir, list := synthDiscs(t, kind)
			c := startHelper(t)
			seen := map[string]synthDisc{}
			for _, d := range list {
				reply, g := openSynth(t, c, kind, filepath.Join(dir, filepath.FromSlash(d.Path)))
				// Each disc has a name to remember choices by, its own, and the same
				// whether it was written as a folder or an image.
				if len(reply.Fingerprint) != 32 {
					t.Errorf("%s: fingerprint %q", d.Name, reply.Fingerprint)
				}
				if o, ok := seen[reply.Fingerprint]; ok && strings.TrimSuffix(o.Path, ".iso") != strings.TrimSuffix(d.Path, ".iso") {
					t.Errorf("%s has the same fingerprint as %s", d.Name, o.Name)
				}
				for _, o := range seen {
					if d.Image && o.Path+".iso" == d.Path && disc.Fingerprint(reply.Info) != o.fingerprint {
						t.Errorf("%s: the image's fingerprint differs from the folder's", d.Name)
					}
				}
				d.fingerprint = reply.Fingerprint
				seen[reply.Fingerprint] = d
				if g.Kind != kind || g.Feature == nil || g.Feature.Title != d.Main || reply.Info.Main != d.Main {
					t.Fatalf("%s: feature %+v, main title %d, want %d", d.Name, g.Feature, reply.Info.Main, d.Main)
				}
				if g.Why == "" || g.Confidence <= 0 || g.Confidence > 1 || (g.Level != "high" && g.Level != "medium" && g.Level != "low") {
					t.Errorf("%s: why %q, confidence %v, level %q", d.Name, g.Why, g.Confidence, g.Level)
				}
				if d.FeatureSec > 0 && !nearSec(g.Feature.Duration, d.FeatureSec) {
					t.Errorf("%s: the guide says the film is %v s, want %v", d.Name, g.Feature.Duration, d.FeatureSec)
				}
				if len(d.Feature) > 1 && len(g.Feature.Titles) != len(d.Feature) {
					t.Errorf("%s: the film runs through %v, want %v", d.Name, g.Feature.Titles, d.Feature)
				}
				if len(g.Versions) < len(d.Editions) || len(g.Episodes) < len(d.Episodes) {
					t.Errorf("%s: %d versions and %d episodes, want %d and %d", d.Name, len(g.Versions), len(g.Episodes), len(d.Editions), len(d.Episodes))
				}
				p := choose(t, c, "feature")
				if p.Title.TTN != d.Main || (d.FeatureSec > 0 && !nearSec(p.Title.Duration, d.FeatureSec)) {
					t.Errorf("%s: the feature plays title %d for %v s, want %d for %v s", d.Name, p.Title.TTN, p.Title.Duration, d.Main, d.FeatureSec)
				}
				// The guide asked for again is the same guide, and one line of the protocol.
				var again json.RawMessage
				if _, err := c.Call(disc.MethodGuide, nil, &again); err != nil || len(again) == 0 || len(again) >= maxGuide {
					t.Errorf("%s: the guide again: %d bytes, %v", d.Name, len(again), err)
				}
				// A choice the disc doesn't have is a plain no, and the disc stays open.
				_, err := c.Call(disc.MethodChoose, map[string]any{"id": "episode:999"}, nil)
				if ce, ok := err.(*wittymodule.CallError); !ok || ce.Code != disc.CodeMissing {
					t.Errorf("%s: a choice that isn't there: %v", d.Name, err)
				}
				if _, err := c.Call("action", map[string]any{"type": "tick"}, nil); err != nil {
					t.Errorf("%s: after a refused choice: %v", d.Name, err)
				}
			}
			// With the disc closed there is no guide and nothing to choose.
			if _, err := c.Call("close", nil, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := c.Call(disc.MethodGuide, nil, nil); err == nil {
				t.Error("a guide with no disc open")
			}
			if _, err := c.Call(disc.MethodChoose, map[string]any{"id": "feature"}, nil); err == nil {
				t.Error("a choice with no disc open")
			}
		})
	}
}

// Episodes, editions and other titles play when chosen, each as what the engine listed.
func TestChoicesPlay(t *testing.T) {
	t.Run("a DVD's episodes inside its play-all title", func(t *testing.T) {
		dir, list := synthDiscs(t, disc.KindDVD)
		c := startHelper(t)
		for _, d := range list {
			if len(d.EpisodeStarts) == 0 {
				continue
			}
			_, g := openSynth(t, c, disc.KindDVD, filepath.Join(dir, d.Path))
			if len(g.Episodes) != len(d.EpisodeStarts) {
				t.Fatalf("%d episodes, want %d", len(g.Episodes), len(d.EpisodeStarts))
			}
			for i, e := range g.Episodes {
				if p := choose(t, c, e.ID); p.Title.TTN != d.Episodes[i] || !nearSec(p.Title.StartSec, d.EpisodeStarts[i]) {
					t.Errorf("episode %d plays title %d from %v s, want title %d from %v s", i+1, p.Title.TTN, p.Title.StartSec, d.Episodes[i], d.EpisodeStarts[i])
				}
			}
			return
		}
		t.Fatal("no exported DVD has episodes inside a play-all title")
	})
	t.Run("a DVD's other cut and other titles", func(t *testing.T) {
		dir, _ := synthDiscs(t, disc.KindDVD)
		c := startHelper(t)
		_, g := openSynth(t, c, disc.KindDVD, filepath.Join(dir, "cuts"))
		if len(g.Versions) == 0 || len(g.Others) == 0 {
			t.Fatalf("versions %+v, others %+v", g.Versions, g.Others)
		}
		for _, v := range append(g.Versions, g.Others...) {
			if p := choose(t, c, v.ID); p.Title.TTN != v.Title || !nearSec(p.Title.Duration, v.Duration) {
				t.Errorf("%s plays title %d for %v s, want %d for %v s", v.ID, p.Title.TTN, p.Title.Duration, v.Title, v.Duration)
			}
		}
	})
	t.Run("a Blu-ray's editions, episodes and parts", func(t *testing.T) {
		dir, list := synthDiscs(t, disc.KindBluray)
		c := startHelper(t)
		editions, episodes, parts := false, false, false
		for _, d := range list {
			if len(d.Editions) == 0 && len(d.Episodes) == 0 && len(d.PartStarts) == 0 {
				continue
			}
			_, g := openSynth(t, c, disc.KindBluray, filepath.Join(dir, filepath.FromSlash(d.Path)))
			for i, want := range d.Editions {
				editions = true
				v := g.Versions[i]
				if p := choose(t, c, v.ID); v.Title != want || p.Title.TTN != want || v.Main != (want == d.Main) {
					t.Errorf("%s: edition %d is %+v and plays playlist %d, want %d", d.Name, i+1, v, p.Title.TTN, want)
				}
			}
			for i, want := range d.Episodes {
				episodes = true
				if p := choose(t, c, g.Episodes[i].ID); p.Title.TTN != want || p.Title.StartSec != 0 {
					t.Errorf("%s: episode %d plays playlist %d from %v s, want %d", d.Name, i+1, p.Title.TTN, p.Title.StartSec, want)
				}
			}
			if len(d.Episodes) == 0 && len(d.PartStarts) > 0 {
				parts = true
				if len(g.Episodes) != len(d.PartStarts) {
					t.Fatalf("%s: %d parts, want %d", d.Name, len(g.Episodes), len(d.PartStarts))
				}
				for i, want := range d.PartStarts {
					if p := choose(t, c, g.Episodes[i].ID); p.Title.TTN != d.Main || !nearSec(p.Title.StartSec, want) {
						t.Errorf("%s: part %d plays playlist %d from %v s, want %d from %v s", d.Name, i+1, p.Title.TTN, p.Title.StartSec, d.Main, want)
					}
				}
			}
		}
		if !editions || !episodes || !parts {
			t.Errorf("the exported discs cover editions %v, episodes %v, parts %v", editions, episodes, parts)
		}
	})
}

// A disc opened again has the same fingerprint, from the built-in copy as from the
// helper's own process (TestRealHelperPlaysADisc).
func TestFingerprintStaysTheSame(t *testing.T) {
	dir, list := synthDiscs(t, disc.KindDVD)
	path := filepath.Join(dir, filepath.FromSlash(list[0].Path))
	var prints []string
	for range 2 {
		c := startHelper(t)
		reply, _ := openSynth(t, c, disc.KindDVD, path)
		prints = append(prints, reply.Fingerprint)
	}
	if prints[0] == "" || prints[0] != prints[1] {
		t.Errorf("fingerprints %q", prints)
	}
}
