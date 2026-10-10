package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/arcade"
	"github.com/JMThomas00/Concord/sdk/wire"
)

// Achievement ids, declared in plugin.toml.
const (
	achFirst    = "first_race"
	ach40       = "wpm_40"
	ach60       = "wpm_60"
	ach80       = "wpm_80"
	ach100      = "wpm_100"
	achSpotless = "spotless"
	achFirstWin = "first_win"
	achWins10   = "wins_10"
	achRaces50  = "races_50"
)

// carSetup is what a member drives: the model, its paint and number.
type carSetup struct {
	Model  string `json:"model,omitempty"`
	Paint  string `json:"paint,omitempty"`
	Number int    `json:"number,omitempty"`
}

// options are a member's own settings. Zero values are the defaults.
type options struct {
	Mute    bool `json:"mute,omitempty"`
	Volume  int  `json:"volume,omitempty"`  // 1-10; 0 means 7
	Effects int  `json:"effects,omitempty"` // 0 full, 1 calm, 2 off
	NoBeeps bool `json:"no_beeps,omitempty"`
}

func (o options) volume() int {
	if o.Volume <= 0 {
		return 7
	}
	return o.Volume
}

// record is one member's history, garage and settings.
type record struct {
	Name      string               `json:"name"`
	Races     int                  `json:"races"` // finished
	Wins      int                  `json:"wins"`  // first, against at least one other racer
	BestWPM   float64              `json:"best_wpm"`
	BestAcc   float64              `json:"best_acc,omitempty"`
	BestAt    time.Time            `json:"best_at,omitempty"`
	TotalWPM  float64              `json:"total_wpm"`
	WinStreak int                  `json:"win_streak,omitempty"`
	Unlocked  map[string]time.Time `json:"unlocked,omitempty"`
	Car       carSetup             `json:"car"`
	Rewards   arcade.Rewards       `json:"rewards"`
	Options   options              `json:"options"`
}

// unlock records an achievement and reports whether it's new.
func (r *record) unlock(id string, at time.Time) bool {
	if r.Unlocked == nil {
		r.Unlocked = map[string]time.Time{}
	}
	if _, ok := r.Unlocked[id]; ok {
		return false
	}
	r.Unlocked[id] = at
	return true
}

// records keeps every member's record in records.json in the data folder.
type records struct {
	path string
	byID map[uuid.UUID]*record
}

func loadRecords(dir string) *records {
	rs := &records{byID: map[uuid.UUID]*record{}}
	if dir == "" {
		return rs
	}
	rs.path = filepath.Join(dir, "records.json")
	if b, err := os.ReadFile(rs.path); err == nil {
		_ = json.Unmarshal(b, &rs.byID)
	}
	return rs
}

func (rs *records) save() {
	if rs.path == "" {
		return
	}
	b, err := json.MarshalIndent(rs.byID, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(rs.path), 0o700)
	tmp := rs.path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, rs.path)
	}
}

func (rs *records) get(id uuid.UUID) *record { return rs.byID[id] }

// ensure returns id's record, making one (with a starter car) if needed.
func (rs *records) ensure(id uuid.UUID, name string) *record {
	r := rs.byID[id]
	if r == nil {
		r = &record{}
		rs.byID[id] = r
	}
	if name != "" {
		r.Name = name
	}
	return r
}

// car is what id drives, with defaults for anyone new: a go-kart, and a
// paint and number picked from their ID so racers start out different.
func (rs *records) car(id uuid.UUID) carSetup {
	var c carSetup
	if r := rs.byID[id]; r != nil {
		c = r.Car
	}
	if _, ok := carByID[c.Model]; !ok {
		c.Model = "kart"
	}
	if !validPaint(c.Paint) {
		c.Paint = arcade.Paints[int(id[0])%len(arcade.Paints)]
	}
	if c.Number < 1 || c.Number > 99 {
		c.Number = int(id[1])%99 + 1
	}
	return c
}

func validPaint(p string) bool {
	for _, q := range arcade.Paints {
		if p == q {
			return true
		}
	}
	return false
}

// earned is what a finish brought: a new best, and Pit Passes and why.
type earned struct {
	Best   bool
	Passes int
	Why    string
}

// add records a finished race: bests, achievements, and Pit Passes (one
// per new achievement, one per 3 wins in a row, one per 10 races).
func (rs *records) add(p *racer, field, textLen int, at time.Time) earned {
	r := rs.ensure(p.ID, p.Name)
	var e earned
	r.Races++
	r.TotalWPM += p.WPM
	if p.WPM > r.BestWPM {
		r.BestWPM, r.BestAcc, r.BestAt, e.Best = p.WPM, p.Accuracy, at, true
	}
	var why []string
	passes := 0
	if field > 1 {
		if p.Place == 1 {
			r.Wins++
			r.WinStreak++
			if r.WinStreak%3 == 0 {
				passes++
				why = append(why, fmt.Sprintf("%d wins in a row", r.WinStreak))
			}
		} else {
			r.WinStreak = 0
		}
	}
	if r.Races%10 == 0 {
		passes++
		why = append(why, fmt.Sprintf("%d races finished", r.Races))
	}
	unlocks := []string{achFirst}
	for _, a := range []struct {
		id  string
		wpm float64
	}{{ach40, 40}, {ach60, 60}, {ach80, 80}, {ach100, 100}} {
		if p.WPM >= a.wpm {
			unlocks = append(unlocks, a.id)
		}
	}
	if p.mistakes == 0 && textLen >= 180 {
		unlocks = append(unlocks, achSpotless)
	}
	if r.Wins >= 1 {
		unlocks = append(unlocks, achFirstWin)
	}
	if r.Wins >= 10 {
		unlocks = append(unlocks, achWins10)
	}
	if r.Races >= 50 {
		unlocks = append(unlocks, achRaces50)
	}
	fresh := 0
	for _, id := range unlocks {
		if r.unlock(id, at) {
			fresh++
		}
	}
	if fresh > 0 {
		passes += fresh
		if fresh == 1 {
			why = append(why, "new achievement")
		} else {
			why = append(why, fmt.Sprintf("%d new achievements", fresh))
		}
	}
	e.Passes = r.Rewards.Grant(passes, carItems)
	if e.Passes > 0 {
		e.Why = strings.Join(why, " · ")
	}
	rs.save()
	return e
}

// wire is a member's record as Concord shows it (Settings > About >
// Achievements), ranked on the leaderboard by best_wpm.
func (rs *records) wire(id uuid.UUID) wire.PluginRecord {
	r := rs.byID[id]
	rec := wire.PluginRecord{UserID: id}
	if r == nil {
		return rec
	}
	avg := 0.0
	if r.Races > 0 {
		avg = r.TotalWPM / float64(r.Races)
	}
	cars := r.Rewards.Count(carItems)
	rec.Stats = []wire.PluginStat{
		{Key: "best_wpm", Label: "Best WPM", Value: fmt.Sprintf("%.0f", r.BestWPM), Num: math.Round(r.BestWPM*10) / 10},
		{Key: "avg_wpm", Label: "Average WPM", Value: fmt.Sprintf("%.0f", avg), Num: avg},
		{Key: "races", Label: "Races", Value: fmt.Sprint(r.Races), Num: float64(r.Races)},
		{Key: "wins", Label: "Wins", Value: fmt.Sprint(r.Wins), Num: float64(r.Wins)},
		{Key: "cars", Label: "Cars", Value: fmt.Sprintf("%d of %d", cars, len(carItems)), Num: float64(cars)},
	}
	for id, at := range r.Unlocked {
		rec.Unlocked = append(rec.Unlocked, wire.PluginUnlock{ID: id, At: at})
	}
	sort.Slice(rec.Unlocked, func(i, j int) bool { return rec.Unlocked[i].At.Before(rec.Unlocked[j].At) })
	return rec
}

type best struct {
	ID   uuid.UUID
	Name string
	WPM  float64
	Acc  float64
	At   time.Time
}

// top is the fastest n members by best WPM.
func (rs *records) top(n int) []best {
	var out []best
	for id, r := range rs.byID {
		if r.Races > 0 {
			out = append(out, best{id, r.Name, r.BestWPM, r.BestAcc, r.BestAt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WPM > out[j].WPM })
	return out[:min(n, len(out))]
}

// rank is id's place by best WPM (0 if they haven't finished a race), and
// how many have.
func (rs *records) rank(id uuid.UUID) (int, int) {
	all := rs.top(len(rs.byID))
	for i, b := range all {
		if b.ID == id {
			return i + 1, len(all)
		}
	}
	return 0, len(all)
}
