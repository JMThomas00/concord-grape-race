package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/arcade"
)

// TestRedLight: a key during the countdown is a red light, and the penalty
// lands on the finishing time.
func TestRedLight(t *testing.T) {
	clk := newClock()
	me := uuid.New()
	r := newRace(clk.now)
	r.Join(me, "me", "short")
	r.text = []rune("grape")
	if !r.Jump(me) || r.Jump(me) {
		t.Fatal("the first key in the countdown is a red light, once")
	}
	clk.add(r.countdown)
	r.Tick()
	clk.add(10 * time.Second)
	typeAll(r, me, "grape")
	p := r.racer(me)
	if got := r.Time(p); got != 11*time.Second {
		t.Fatalf("time %v, want 10s plus the 1s penalty", got)
	}
	if want := wpm(5, 11*time.Second); p.WPM != want {
		t.Fatalf("wpm %.2f, want %.2f", p.WPM, want)
	}
}

// TestUnlockFromTheGarage walks the whole reward flow: a locked car,
// Enter, the draft, a pick, the reveal, back to the garage driving it.
func TestUnlockFromTheGarage(t *testing.T) {
	clk := newClock()
	a := newApp(loadRecords(t.TempDir()), clk.now)
	me := uuid.New()
	m := newModel(a, uuid.New(), me, "me", nil)
	m.rec().Rewards.Passes = 1
	m.openGarage()
	if m.scr != scrGarage || cars[m.gModel].ID != "kart" {
		t.Fatal("the garage opens on the car you drive")
	}
	m.gModel = 15 // the bathtub: locked
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.scr != scrDraft || len(m.rec().Rewards.Offer) != 3 {
		t.Fatalf("Enter on a locked car with a pass opens the draft: scr %v offer %v", m.scr, m.rec().Rewards.Offer)
	}
	offer := append([]string(nil), m.rec().Rewards.Offer...)
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	picked := offer[(1+1)%3]
	rec := m.rec()
	if !rec.Rewards.Owns(picked, carItems) || rec.Rewards.Passes != 0 || rec.Car.Model != picked {
		t.Fatalf("picked %s: owned %v, passes %d, driving %s", picked, rec.Rewards.Owned, rec.Rewards.Passes, rec.Car.Model)
	}
	if !strings.Contains(stripped(m), "UNLOCKED") && !strings.Contains(stripped(m), carByID[picked].Name) {
		t.Fatal("the reveal should name the car")
	}
	clk.add(revealFor)
	m.Update(redrawMsg{})
	if m.scr != scrGarage || cars[m.gModel].ID != picked {
		t.Fatal("after the reveal, the garage shows the new car")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if rec.Rewards.Passes != 0 || m.scr != scrGarage {
		t.Fatal("Enter on a car you own just drives it")
	}
}

// TestSoundsFollowOptions: a muted player hears nothing; volume scales;
// "countdown beeps" off silences only the lights.
func TestSoundsFollowOptions(t *testing.T) {
	a := newApp(loadRecords(t.TempDir()), time.Now)
	var got []string
	var vol float64
	a.sound = func(_, _ uuid.UUID, asset string, v float64) { got, vol = append(got, asset), v }
	me := uuid.New()
	m := newModel(a, uuid.New(), me, "me", nil)
	a.play(me, arcade.SoundBlip)
	if len(got) != 1 || vol != .7 {
		t.Fatalf("default: %v at %.1f", got, vol)
	}
	m.rec().Options.NoBeeps = true
	a.play(me, arcade.SoundAmber)
	a.play(me, arcade.SoundSelect)
	if len(got) != 2 || got[1] != arcade.SoundSelect {
		t.Fatalf("beeps off: %v", got)
	}
	m.rec().Options.Mute = true
	a.play(me, arcade.SoundSelect)
	if len(got) != 2 {
		t.Fatal("muted, but a sound played")
	}
}

// TestEveryScreenFits draws every screen at small, standard and large
// pane sizes, and checks nothing spills past the pane.
func TestEveryScreenFits(t *testing.T) {
	clk := newClock()
	a := newApp(loadRecords(t.TempDir()), clk.now)
	ch, me, bob := uuid.New(), uuid.New(), uuid.New()
	a.records.add(&racer{ID: bob, Name: "a very long display name indeed", WPM: 70, Place: 1}, 1, 100, clk.now())
	m := newModel(a, ch, me, "someone with a long name", nil)
	m.rec().Rewards.Passes = 2
	race := func(phase phase) {
		r := a.race(ch)
		switch phase {
		case countdown:
			a.join(ch, me, m.name)
			a.join(ch, bob, "a very long display name indeed")
		case racing:
			clk.add(r.countdown)
			r.Tick()
			clk.add(5 * time.Second)
			typeAll(r, me, string(r.text[:10])+"x")
		case finished:
			typeAll(r, bob, string(r.text))
			clk.add(time.Hour)
			r.Tick()
		}
	}
	for _, size := range [][2]int{{43, 17}, {64, 24}, {80, 24}, {120, 40}} {
		for _, s := range []screen{scrTitle, scrMenu, scrGarage, scrDraft, scrShowroom, scrHOF, scrHowTo, scrOptions, scrRace, scrPractice} {
			m.scr = s
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			fits(t, m, size, s)
		}
	}
	m.scr = scrRace
	for _, ph := range []phase{countdown, racing, finished} {
		race(ph)
		for _, size := range [][2]int{{43, 17}, {64, 24}, {80, 24}, {120, 40}} {
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			fits(t, m, size, scrRace)
		}
	}
}

func fits(t *testing.T, m *model, size [2]int, s screen) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) > size[1] {
		t.Fatalf("screen %d at %v: %d lines", s, size, len(lines))
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > size[0] {
			t.Fatalf("screen %d at %v: a line %d wide: %q", s, size, w, sgr.ReplaceAllString(l, ""))
		}
	}
}

func stripped(m *model) string { return sgr.ReplaceAllString(m.View(), "") }
