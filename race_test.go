package main

import (
	"math"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time      { return c.t }
func (c *fakeClock) add(d time.Duration) { c.t = c.t.Add(d) }
func newClock() *fakeClock               { return &fakeClock{time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)} }
func typeAll(r *race, id uuid.UUID, s string) {
	for _, ch := range s {
		r.Type(id, ch)
	}
}

// started is a race past its countdown, with a known passage.
func started(t *testing.T, clk *fakeClock, text string, ids ...uuid.UUID) *race {
	t.Helper()
	r := newRace(clk.now)
	for i, id := range ids {
		if !r.Join(id, "p"+string(rune('a'+i)), "short") {
			t.Fatal("join refused")
		}
	}
	r.text = []rune(text)
	clk.add(r.countdown)
	r.Tick()
	if r.phase != racing {
		t.Fatalf("phase %v after the countdown", r.phase)
	}
	return r
}

func TestPassagesAreTypeable(t *testing.T) {
	for _, l := range []string{"short", "medium", "long"} {
		if len(passages[l]) < 5 {
			t.Errorf("only %d %s passages", len(passages[l]), l)
		}
		for _, p := range passages[l] {
			for _, ch := range p {
				if ch > 126 || ch < 32 {
					t.Errorf("%q has %q, which not every keyboard types", p[:20], ch)
				}
			}
		}
	}
}

func TestJoinOnlyBeforeGo(t *testing.T) {
	clk := newClock()
	r := newRace(clk.now)
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	if !r.Join(a, "a", "short") || r.phase != countdown {
		t.Fatal("first Enter should start a countdown")
	}
	if !r.Join(b, "b", "short") || r.Join(b, "b", "short") {
		t.Fatal("second racer joins once")
	}
	clk.add(r.countdown)
	r.Tick()
	if !r.Join(c, "c", "short") || r.racer(c) != nil || !r.Queued(c) {
		t.Fatal("Enter after GO should queue for the next race, not join this one")
	}
	if len(r.racers) != 2 {
		t.Fatalf("%d racers", len(r.racers))
	}
}

func TestMistakesMustBeFixed(t *testing.T) {
	clk := newClock()
	me := uuid.New()
	r := started(t, clk, "abc def", me)
	typeAll(r, me, "abx")
	if r.correct(r.racer(me)) != 2 {
		t.Fatal("x should not count")
	}
	typeAll(r, me, " def") // typed on past the mistake
	if r.racer(me).done {
		t.Fatal("finished with a mistake in it")
	}
	for range 5 {
		r.Backspace(me)
	}
	typeAll(r, me, "c def")
	p := r.racer(me)
	if !p.done || p.mistakes != 5 || p.keys != 12 {
		t.Fatalf("done=%v mistakes=%d keys=%d", p.done, p.mistakes, p.keys)
	}
	if r.phase != finished {
		t.Fatal("the only racer finished, so the race should end")
	}
}

func TestMistakesCapped(t *testing.T) {
	clk := newClock()
	me := uuid.New()
	r := started(t, clk, strings.Repeat("a", 40), me)
	typeAll(r, me, strings.Repeat("b", 20))
	if got := len(r.racer(me).typed); got != maxWrong {
		t.Fatalf("typed %d wrong characters, want the cap %d", got, maxWrong)
	}
}

func TestDeleteWord(t *testing.T) {
	clk := newClock()
	me := uuid.New()
	r := started(t, clk, "one two three", me)
	typeAll(r, me, "one tw")
	r.DeleteWord(me)
	if string(r.racer(me).typed) != "one " {
		t.Fatalf("%q", string(r.racer(me).typed))
	}
}

func TestWPMAndPlaces(t *testing.T) {
	clk := newClock()
	a, b := uuid.New(), uuid.New()
	text := strings.Repeat("grape ", 10)[:50] // 50 characters = 10 words
	r := started(t, clk, text, a, b)
	var finished []*racer
	r.onFinish = func(p *racer, field int) {
		if field != 2 {
			t.Errorf("field %d", field)
		}
		finished = append(finished, p)
	}
	clk.add(15 * time.Second)
	typeAll(r, b, text) // 10 words in 15s = 40 wpm
	clk.add(15 * time.Second)
	typeAll(r, a, text) // 10 words in 30s = 20 wpm
	if len(finished) != 2 || finished[0].ID != b || finished[0].Place != 1 || finished[1].Place != 2 {
		t.Fatalf("finish order wrong: %+v", finished)
	}
	if math.Abs(finished[0].WPM-40) > 0.01 || math.Abs(finished[1].WPM-20) > 0.01 {
		t.Fatalf("wpm %.2f and %.2f", finished[0].WPM, finished[1].WPM)
	}
	if s := r.Standings(); s[0].ID != b {
		t.Fatal("standings should put the winner first")
	}
}

func TestDeadlineEndsTheRace(t *testing.T) {
	clk := newClock()
	a := uuid.New()
	r := started(t, clk, "slow and steady", a)
	clk.add(time.Hour)
	r.Tick()
	if r.Live() || r.phase != finished {
		t.Fatal("the race should end at its deadline")
	}
	if r.Type(a, 's') {
		t.Fatal("typed after the race ended")
	}
}

func TestRecordsAndAchievements(t *testing.T) {
	rs := loadRecords(t.TempDir())
	me := uuid.New()
	at := time.Now()
	p := &racer{ID: me, Name: "me", WPM: 65, Place: 1, keys: 200}
	e := rs.add(p, 2, 200, at)
	if !e.Best {
		t.Fatal("first race should be a new best")
	}
	// first race, 40, 60, spotless and first win: five new achievements, a pass each
	if e.Passes != 5 || e.Why != "5 new achievements" {
		t.Fatalf("passes %d (%q)", e.Passes, e.Why)
	}
	if e := rs.add(&racer{ID: me, Name: "me", WPM: 50, Place: 2, keys: 10, mistakes: 1}, 2, 200, at); e.Best || e.Passes != 0 {
		t.Fatalf("a slower race with nothing new: %+v", e)
	}
	r := rs.get(me)
	for _, id := range []string{achFirst, ach40, ach60, achSpotless, achFirstWin} {
		if _, ok := r.Unlocked[id]; !ok {
			t.Errorf("%s not unlocked", id)
		}
	}
	if _, ok := r.Unlocked[ach80]; ok {
		t.Error("80 wpm unlocked at 65")
	}
	again := loadRecords(rs.path[:len(rs.path)-len("records.json")])
	if again.get(me) == nil || again.get(me).Races != 2 || again.get(me).BestWPM != 65 {
		t.Fatal("records weren't saved")
	}
	w := rs.wire(me)
	if w.Stats[0].Key != "best_wpm" || w.Stats[0].Num != 65 {
		t.Fatalf("leaderboard stat %+v", w.Stats[0])
	}
}

func TestWrapKeepsEveryCharacter(t *testing.T) {
	for _, p := range passages["long"] {
		text := []rune(p)
		next := 0
		for _, ln := range wrapRanges(text, 37) {
			if ln[0] != next || ln[1]-ln[0] > 37 {
				t.Fatalf("bad line %v", ln)
			}
			next = ln[1]
		}
		if next != len(text) {
			t.Fatal("wrap lost text")
		}
	}
}

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func keyType(name string) tea.KeyMsg {
	return map[string]tea.KeyMsg{"left": {Type: tea.KeyLeft}, "right": {Type: tea.KeyRight}}[name]
}

// TestPitPasses: three wins in a row and every ten races each earn a pass,
// and passes stop once every car is unlocked.
func TestPitPasses(t *testing.T) {
	rs := loadRecords(t.TempDir())
	me := uuid.New()
	at := time.Now()
	win := func() earned {
		return rs.add(&racer{ID: me, Name: "me", WPM: 30, Place: 1, keys: 50, mistakes: 1}, 2, 100, at)
	}
	win() // first race + first win achievements
	if e := win(); e.Passes != 0 {
		t.Fatalf("second win: %+v", e)
	}
	if e := win(); e.Passes != 1 || e.Why != "3 wins in a row" {
		t.Fatalf("third win in a row: %+v", e)
	}
	rs.add(&racer{ID: me, Name: "me", WPM: 30, Place: 2, keys: 50, mistakes: 1}, 2, 100, at) // streak broken
	if rs.get(me).WinStreak != 0 {
		t.Fatal("a loss should end the streak")
	}
	for rs.get(me).Races < 9 {
		rs.add(&racer{ID: me, Name: "me", WPM: 30, Place: 1, keys: 50, mistakes: 1}, 1, 100, at) // solo: no wins
	}
	if e := rs.add(&racer{ID: me, Name: "me", WPM: 30, Place: 1, keys: 50, mistakes: 1}, 1, 100, at); e.Passes != 1 || e.Why != "10 races finished" {
		t.Fatalf("tenth race: %+v", e)
	}
	r := rs.get(me)
	r.Rewards.Passes = 0
	for _, c := range carItems {
		r.Rewards.Owned = append(r.Rewards.Owned, c.ID)
	}
	if r.Rewards.Grant(1, carItems) != 0 {
		t.Fatal("no passes once everything is unlocked")
	}
}
