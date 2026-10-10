package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/wire"
)

// Achievement ids, declared in plugin.toml.
const (
	achFirst    = "first_stomp"
	ach40       = "wpm_40"
	ach60       = "wpm_60"
	ach80       = "wpm_80"
	ach100      = "wpm_100"
	achSpotless = "spotless"
	achFirstWin = "first_win"
	achWins10   = "wins_10"
	achRaces50  = "races_50"
)

// record is one member's history.
type record struct {
	Name     string               `json:"name"`
	Races    int                  `json:"races"` // finished
	Wins     int                  `json:"wins"`  // first, against at least one other racer
	BestWPM  float64              `json:"best_wpm"`
	TotalWPM float64              `json:"total_wpm"`
	Unlocked map[string]time.Time `json:"unlocked,omitempty"`
}

func (r *record) unlock(id string, at time.Time) {
	if r.Unlocked == nil {
		r.Unlocked = map[string]time.Time{}
	}
	if _, ok := r.Unlocked[id]; !ok {
		r.Unlocked[id] = at
	}
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

// add records a finished race and reports whether it was a new best.
func (rs *records) add(p *racer, field, textLen int, at time.Time) (best bool) {
	r := rs.byID[p.ID]
	if r == nil {
		r = &record{}
		rs.byID[p.ID] = r
	}
	r.Name = p.Name
	r.Races++
	r.TotalWPM += p.WPM
	if p.WPM > r.BestWPM {
		r.BestWPM, best = p.WPM, true
	}
	if field > 1 && p.Place == 1 {
		r.Wins++
	}
	r.unlock(achFirst, at)
	for _, a := range []struct {
		id  string
		wpm float64
	}{{ach40, 40}, {ach60, 60}, {ach80, 80}, {ach100, 100}} {
		if p.WPM >= a.wpm {
			r.unlock(a.id, at)
		}
	}
	if p.mistakes == 0 && textLen >= 180 {
		r.unlock(achSpotless, at)
	}
	if r.Wins >= 1 {
		r.unlock(achFirstWin, at)
	}
	if r.Wins >= 10 {
		r.unlock(achWins10, at)
	}
	if r.Races >= 50 {
		r.unlock(achRaces50, at)
	}
	rs.save()
	return best
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
	rec.Stats = []wire.PluginStat{
		{Key: "best_wpm", Label: "Best WPM", Value: fmt.Sprintf("%.0f", r.BestWPM), Num: math.Round(r.BestWPM*10) / 10},
		{Key: "avg_wpm", Label: "Average WPM", Value: fmt.Sprintf("%.0f", avg), Num: avg},
		{Key: "races", Label: "Races", Value: fmt.Sprint(r.Races), Num: float64(r.Races)},
		{Key: "wins", Label: "Wins", Value: fmt.Sprint(r.Wins), Num: float64(r.Wins)},
	}
	for id, at := range r.Unlocked {
		rec.Unlocked = append(rec.Unlocked, wire.PluginUnlock{ID: id, At: at})
	}
	sort.Slice(rec.Unlocked, func(i, j int) bool { return rec.Unlocked[i].At.Before(rec.Unlocked[j].At) })
	return rec
}

type best struct {
	Name string
	WPM  float64
}

// top is the fastest n members by best WPM.
func (rs *records) top(n int) []best {
	var out []best
	for _, r := range rs.byID {
		if r.Races > 0 {
			out = append(out, best{r.Name, r.BestWPM})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WPM > out[j].WPM })
	return out[:min(n, len(out))]
}
