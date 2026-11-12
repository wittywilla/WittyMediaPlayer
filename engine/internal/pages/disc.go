// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors

package pages

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Disc is the open disc as the window knows it.
type Disc struct {
	// Kind is dvd or bluray; Label the disc's name.
	Kind  string `json:"kind"`
	Label string `json:"label"`
	// Guide is the disc module's answer, nil from a module that predates it.
	Guide *Guide `json:"guide"`
	// Chosen is the id of the choice the viewer made the main feature, "" for the engine's pick.
	Chosen string `json:"chosen"`
	// PlayingTitle is the title or playlist playing now, 0 when none.
	PlayingTitle int `json:"playingTitle"`
}

// Choice is one thing the viewer can ask the engine to play.
type Choice struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	Detail      string  `json:"detail"`
	Title       int     `json:"title"`
	Titles      []int   `json:"titles"`
	DurationSec float64 `json:"durationSec"`
	Chapters    int     `json:"chapters"`
	StartSec    float64 `json:"startSec"`
	Main        bool    `json:"main"`
	OneStream   bool    `json:"oneStream"`
	Source      string  `json:"source"`
}

// Guide is what WittyDVD and WittyBluray say about a disc: the same shape from both.
type Guide struct {
	Feature       Choice   `json:"feature"`
	Why           string   `json:"why"`
	Reasons       []string `json:"reasons"`
	Confidence    float64  `json:"confidence"`
	Level         string   `json:"level"`
	Versions      []Choice `json:"versions"`
	Episodes      []Choice `json:"episodes"`
	Others        []Choice `json:"others"`
	PlayAll       Choice   `json:"playAll"`
	Decoys        int      `json:"decoys"`
	Hidden        int      `json:"hidden"`
	Warnings      []string `json:"warnings"`
	Repairs       []string `json:"repairs"`
	RepairCount   int      `json:"repairCount"`
	PaddedSectors int64    `json:"paddedSectors"`
	ReadRetries   int64    `json:"readRetries"`
}

// A guide is a menu, not a catalogue: lists are cut, as the window always did.
const (
	maxChoices = 100
	maxNotes   = 16
)

func (g *Guide) clean() {
	cut := func(c []Choice) []Choice {
		out := make([]Choice, 0, len(c))
		for _, x := range c {
			if x.ID != "" && len(out) < maxChoices {
				out = append(out, x)
			}
		}
		return out
	}
	lines := func(l []string) []string {
		out := []string{}
		for _, s := range l {
			if s = strings.TrimSpace(s); s != "" && len(out) < maxNotes {
				out = append(out, s)
			}
		}
		return out
	}
	g.Versions, g.Episodes, g.Others = cut(g.Versions), cut(g.Episodes), cut(g.Others)
	g.Reasons, g.Warnings, g.Repairs = lines(g.Reasons), lines(g.Warnings), lines(g.Repairs)
	if g.Confidence < 0 || math.IsNaN(g.Confidence) {
		g.Confidence = 0
	}
	if g.Confidence > 1 {
		g.Confidence = 1
	}
	if g.Level != "high" && g.Level != "medium" {
		g.Level = "low"
	}
	if g.Decoys < 0 {
		g.Decoys = 0
	}
	if g.Hidden < 0 {
		g.Hidden = 0
	}
	if g.RepairCount < len(g.Repairs) {
		g.RepairCount = len(g.Repairs)
	}
	if g.PaddedSectors < 0 {
		g.PaddedSectors = 0
	}
	if g.ReadRetries < 0 {
		g.ReadRetries = 0
	}
	for _, c := range []*Choice{&g.Feature, &g.PlayAll} {
		if math.IsNaN(c.DurationSec) || c.DurationSec < 0 {
			c.DurationSec = 0
		}
	}
}

func (d *Disc) unit() string {
	if d.Kind == "bluray" {
		return "playlist"
	}
	return "title"
}

func (d *Disc) versionsName() string {
	if d.Kind == "bluray" {
		return "Editions"
	}
	return "Versions"
}

// FormatTime is h:mm:ss, or m:ss under an hour.
func FormatTime(sec float64) string {
	if math.IsNaN(sec) || sec < 0 {
		sec = 0
	}
	s := int(sec)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func counted(n int64, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

var errNoDisc = errors.New("pages: no disc is open")

func (b *builder) disc() (*Disc, error) {
	d := b.st.Disc
	if d == nil {
		return nil, errNoDisc
	}
	if d.Guide != nil {
		d.Guide.clean()
	}
	return d, nil
}

func choiceDetail(c Choice) (string, string) {
	sub := c.Detail
	if c.Main {
		sub = "main feature"
		if c.Detail != "" {
			sub += ", " + c.Detail
		}
	}
	trailing := ""
	if c.DurationSec > 0 {
		trailing = FormatTime(c.DurationSec)
	}
	return sub, trailing
}

// discPage is the disc's guide: the film as picked, and the other things to play.
func (b *builder) discPage() error {
	d, err := b.disc()
	if err != nil {
		return err
	}
	name := d.Label
	if name == "" {
		name = "This disc"
	}
	b.page(name, "disc")
	g := d.Guide
	if g == nil {
		b.add(notice("no-guide", "info", "", "This disc's module is from before it could say which title is the film. A newer one may be in Settings, Disc modules."))
		b.add(dock("dock", []string{"select", "back"}, button("dock-modules", "Disc modules", "ghost", navigate("/page/"+PageModules))))
		return nil
	}
	if g.Feature.ID == "" {
		b.add(notice("no-feature", "warning", "", "The disc engine found nothing on this disc that could be its main feature."))
	} else {
		b.add(notice("summary", toneOf(g.Level), "", b.summary(d)))
		sub, trailing := choiceDetail(g.Feature)
		rows := []nodeT{item("feature", g.Feature.Label, sub, trailing, "play", command("app.disc.choose", g.Feature.ID, nil))}
		if d.PlayingTitle != 0 && !contains(g.Feature.Titles, d.PlayingTitle) && g.Feature.Title != d.PlayingTitle {
			rows = append(rows, item("make-main", fmt.Sprintf("Play %s %d as the main feature from now on", d.unit(), d.PlayingTitle), "", "", "star",
				command("app.disc.makeMain", "", nil)))
		}
		if d.Chosen != "" {
			rows = append(rows, item("forget-main", "Forget your main feature for this disc", "The engine's pick plays again", "", "undo",
				command("app.disc.forget", "", nil)))
		}
		b.add(list("main", "Main feature", rows...))
	}
	if len(g.Versions) > 0 {
		b.add(b.choices("versions", d.versionsName(), g.Versions))
	}
	if len(g.Episodes) > 0 {
		var all []Choice
		if g.PlayAll.ID != "" {
			pa := g.PlayAll
			if pa.Label == "" {
				pa.Label = "Play all"
			}
			all = append(all, pa)
		}
		b.add(b.choices("episodes", "Episodes", append(all, g.Episodes...)))
	}
	if len(g.Others) > 0 {
		b.add(b.choices("titles", "More titles", g.Others))
	}
	b.add(dock("dock", []string{"select", "back"},
		button("dock-why", "Why this "+d.unit(), "ghost", navigate("/page/"+PageWhy)),
		button("dock-health", "Disc health", "ghost", navigate("/page/"+PageDiagnosis)),
	))
	return nil
}

func contains(l []int, v int) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func toneOf(level string) string {
	if level == "high" {
		return "success"
	}
	if level == "medium" {
		return "info"
	}
	return "warning"
}

func (b *builder) summary(d *Disc) string {
	g := d.Guide
	s := fmt.Sprintf("The main feature is %s, %s.", lowerFirst(g.Feature.Label), FormatTime(g.Feature.DurationSec))
	switch g.Level {
	case "high":
		return s + " The disc engine is sure of it."
	case "medium":
		return s + " The disc engine is fairly sure of it."
	}
	return s + fmt.Sprintf(" The disc engine isn't sure of it: see Why this %s.", d.unit())
}

func (b *builder) choices(id, title string, cs []Choice) nodeT {
	rows := make([]nodeT, 0, len(cs))
	for i, c := range cs {
		sub, trailing := choiceDetail(c)
		label := c.Label
		if c.StartSec > 0 {
			sub = strings.TrimLeft(sub+fmt.Sprintf(" from %s", FormatTime(c.StartSec)), ", ")
		}
		rows = append(rows, item(fmt.Sprintf("%s-%d", id, i), label, sub, trailing, "play", command("app.disc.choose", c.ID, nil)))
	}
	return list(id, title, rows...)
}

// why is the whole answer to "why this title", in lines a person can read.
func (b *builder) why() error {
	d, err := b.disc()
	if err != nil {
		return err
	}
	b.page("Why this "+d.unit(), "info")
	g := d.Guide
	if g == nil {
		b.add(notice("old", "info", "", "This disc's module is from before it could say why it picks a title. A newer one may be in Settings, Disc modules."))
		return nil
	}
	if g.Feature.ID == "" {
		b.add(notice("none", "warning", "", "The disc engine found nothing on this disc that could be its main feature."))
		return nil
	}
	why := g.Why
	if strings.TrimSpace(why) == "" {
		why = fmt.Sprintf("%s was picked as the main feature.", g.Feature.Label)
	}
	b.add(text("pick", why, "title"))
	b.add(notice("sure", toneOf(g.Level), "How sure", fmt.Sprintf("%s (%d%%).", g.Level, int(math.Round(g.Confidence*100)))))
	length := "Length: " + FormatTime(g.Feature.DurationSec)
	if g.Feature.Chapters > 0 {
		length += ", " + counted(int64(g.Feature.Chapters), "chapter", "chapters")
	}
	b.add(para("length", length+"."))
	if n := len(g.Feature.Titles); n > 1 {
		nums := make([]string, n)
		for i, t := range g.Feature.Titles {
			nums[i] = fmt.Sprint(t)
		}
		if g.Feature.OneStream {
			b.add(para("parts", fmt.Sprintf("The disc cuts the film into %d titles (%s). They play here as one film, in the disc's order.", n, strings.Join(nums, ", "))))
		} else {
			b.add(para("parts", fmt.Sprintf("The disc cuts the film into %d titles (%s), and its own program plays them one after another.", n, strings.Join(nums, ", "))))
		}
	} else if g.Feature.Source == "commands" {
		b.add(para("parts", "The film is put together by the disc's own commands, and plays here in that order."))
	}
	if len(g.Reasons) > 0 {
		rows := make([]nodeT, len(g.Reasons))
		for i, r := range g.Reasons {
			rows[i] = item(fmt.Sprintf("reason-%d", i), r, "", "", "", nil)
		}
		b.add(list("reasons", "What the engine noticed", rows...))
	}
	for i, w := range g.Warnings {
		b.add(notice(fmt.Sprintf("warn-%d", i), "warning", "", w))
	}
	b.add(caption("never", "This comes from reading the disc's index, never from unlocking anything: a copy-protected disc is refused."))
	b.add(dock("dock", []string{"back"}, button("dock-health", "Disc health", "ghost", navigate("/page/"+PageDiagnosis)),
		button("dock-disc", "This disc", "ghost", navigate("/page/"+PageDisc))))
	return nil
}

// diagnosis is how the reading of the disc has gone: what was left out, mended or unreadable.
func (b *builder) diagnosis() error {
	d, err := b.disc()
	if err != nil {
		return err
	}
	b.page("Disc health", "heart")
	g := d.Guide
	if g == nil {
		b.add(notice("old", "info", "", "This disc's module is from before it reported how the disc reads. A newer one may be in Settings, Disc modules."))
		return nil
	}
	clean := g.Decoys == 0 && g.RepairCount == 0 && g.PaddedSectors == 0 && g.ReadRetries == 0 && len(g.Warnings) == 0
	if clean {
		b.add(notice("fine", "success", "", "Nothing needed mending, and every sector read so far was read."))
	}
	var rows []nodeT
	add := func(id, title, sub, trailing string) { rows = append(rows, item(id, title, sub, trailing, "", nil)) }
	add("decoys", "Decoy "+d.unit()+"s left out of the menus", "Settings, More titles still lists them", fmt.Sprint(g.Decoys))
	add("hidden", "Hidden "+d.unit()+"s", "Not offered because they are too short, repeated or past the disc's limits", fmt.Sprint(g.Hidden))
	add("repairs", "Places the disc's index disagreed with itself", "", fmt.Sprint(g.RepairCount))
	add("padded", "Sectors that couldn't be read and were taken as empty", "So far", fmt.Sprint(g.PaddedSectors))
	add("retries", "Reads that had to be tried again", "So far", fmt.Sprint(g.ReadRetries))
	b.add(list("facts", "What the engine found", rows...))
	if g.RepairCount > 0 {
		head := "The disc's tables disagreed in " + counted(int64(g.RepairCount), "place", "places") + "."
		if d.Kind == "bluray" {
			head = "The disc's playlists, clip information and stream files disagree in " + counted(int64(g.RepairCount), "place", "places") + "."
		}
		rr := make([]nodeT, 0, 5)
		for i, r := range g.Repairs {
			if i == 4 {
				break
			}
			rr = append(rr, item(fmt.Sprintf("repair-%d", i), r, "", "", "", nil))
		}
		if more := g.RepairCount - len(rr); more > 0 {
			rr = append(rr, item("repair-more", fmt.Sprintf("and %d more", more), "", "", "", nil))
		}
		b.add(list("repairs-list", head, rr...))
	}
	for i, w := range g.Warnings {
		b.add(notice(fmt.Sprintf("warn-%d", i), "warning", "", w))
	}
	b.add(dock("dock", []string{"back"}, button("dock-why", "Why this "+d.unit(), "ghost", navigate("/page/"+PageWhy)),
		button("dock-disc", "This disc", "ghost", navigate("/page/"+PageDisc))))
	return nil
}
