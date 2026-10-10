package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/plugin"
	"github.com/JMThomas00/Concord/sdk/plugintest"
	"github.com/JMThomas00/Concord/sdk/wire"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// atomicClock is a clock the test moves while the plugin reads it.
type atomicClock struct{ ns atomic.Int64 }

func (c *atomicClock) now() time.Time      { return time.Unix(0, c.ns.Load()) }
func (c *atomicClock) add(d time.Duration) { c.ns.Add(int64(d)) }

// TestRaceInConcord runs a two-person race through a fake Concord server:
// one starts it, the other joins, both type, results and records follow.
func TestRaceInConcord(t *testing.T) {
	const text = "chase the grapes"
	saved := passages
	passages = map[string][]string{"short": {text}}
	defer func() { passages = saved }()

	srv := plugintest.NewServer(t)
	clk := &atomicClock{}
	clk.ns.Store(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC).UnixNano())
	a := newApp(loadRecords(t.TempDir()), clk.now)
	a.countdown = 2 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go plugin.Run(ctx, srv.Config(), handler(a))
	srv.WaitReady()

	ch := uuid.New()
	srv.Channel(wire.Channel{ID: ch, Name: "grape-race", PluginConfig: map[string]string{"length": "short"}})
	alice := srv.Enter(ch, "alice", 80, 24)
	bob := srv.Enter(ch, "bob", 80, 24)
	small := srv.Enter(ch, "carol", 43, 17)   // odd sizes catch overflow
	srv.FrameContaining(alice, "PRESS ENTER") // the title screen

	srv.Key(alice, "enter") // title -> menu
	srv.Key(alice, "enter") // RACE: starts the countdown
	srv.FrameContaining(alice, "STARTS IN")
	srv.Key(bob, "enter") // title -> menu, where RACE says a race is staging
	srv.FrameContaining(bob, "Join race #1")
	srv.Key(bob, "enter")
	srv.FrameContaining(bob, "Hands on the keys")
	clk.add(2 * time.Second) // the next tick ends the countdown
	srv.FrameContaining(alice, "GO!")
	srv.Key(small, "enter") // carol, in a small pane: title -> menu
	srv.Key(small, "enter") // RACE while it's on: watch, and go next
	if f := srv.FrameContaining(small, "Spectating"); !strings.Contains(f, "Watching ") {
		t.Fatalf("a spectator should be watching someone:\n%s", f)
	}

	clk.add(10 * time.Second) // 16 characters in 10s: about 19 wpm
	srv.Key(bob, "x")         // a mistake, fixed
	srv.Key(bob, "backspace")
	srv.Type(bob, text)
	srv.FrameContaining(bob, "FINISHED 1ST")
	srv.FrameContaining(alice, "TO FINISH") // the finish timer started
	rec := nextRecord(t, srv)
	if rec.UserID != bob.ID || rec.Stats[0].Key != "best_wpm" || rec.Stats[0].Num < 19 || rec.Stats[0].Num > 20 {
		t.Fatalf("bob's record: %+v", rec)
	}
	won := map[string]bool{}
	for _, u := range rec.Unlocked {
		won[u.ID] = true
	}
	if !won[achFirst] || !won[achFirstWin] {
		t.Fatalf("bob should have First Race and First Press: %+v", rec.Unlocked)
	}

	clk.add(5 * time.Second)
	srv.Type(alice, text)
	srv.FrameContaining(alice, "YOUR RACE")
	f := sgr.ReplaceAllString(srv.FrameContaining(small, "Results"), "")
	if !strings.Contains(f, "1st  bob") || !strings.Contains(f, "2nd  alice") {
		t.Fatalf("results:\n%s", f)
	}
	for _, l := range strings.Split(f, "\n") {
		if w := len([]rune(l)); w > 43 {
			t.Fatalf("line %d runes wide in a 43-column pane: %q", w, l)
		}
	}

	// Enter again starts the next race.
	srv.Key(small, "enter")
	srv.FrameContaining(alice, "Press Enter to join")
}

// TestPasteIsIgnored: several characters in one key event is a paste.
func TestPasteIsIgnored(t *testing.T) {
	clk := newClock()
	me := uuid.New()
	r := started(t, clk, "abc", me)
	a := newApp(loadRecords(""), clk.now)
	a.races[uuid.Nil] = r
	m := newModel(a, uuid.Nil, me, "me", nil)
	m.scr = scrRace
	m.Update(keyRunes("abc"))
	if len(r.racer(me).typed) != 0 {
		t.Fatal("a paste was typed")
	}
	m.Update(keyRunes("a"))
	if len(r.racer(me).typed) != 1 {
		t.Fatal("a single key wasn't typed")
	}
}

func nextRecord(t *testing.T, srv *plugintest.Server) wire.PluginRecord {
	t.Helper()
	for range 20 {
		ev := srv.NextEvent()
		if ev.Kind != wire.PluginEventRecord {
			continue
		}
		var rec wire.PluginRecord
		if err := json.Unmarshal(ev.Payload, &rec); err != nil {
			t.Fatal(err)
		}
		return rec
	}
	t.Fatal("no record sent")
	return wire.PluginRecord{}
}

// TestTooFastIsNotRecorded: an impossible speed shows in the results but
// never reaches records or the leaderboard.
func TestTooFastIsNotRecorded(t *testing.T) {
	clk := newClock()
	me := uuid.New()
	a := newApp(loadRecords(t.TempDir()), clk.now)
	sent := 0
	a.sendRecord = func(uuid.UUID) { sent++ }
	r := a.race(uuid.Nil)
	r.Join(me, "me", "short")
	r.text = []rune("chase the grapes")
	clk.add(r.countdown)
	r.Tick()
	clk.add(100 * time.Millisecond)
	typeAll(r, me, "chase the grapes")
	if p := r.racer(me); !p.done || !p.TooFast {
		t.Fatalf("done=%v tooFast=%v wpm=%.0f", p.done, p.TooFast, p.WPM)
	}
	if a.records.get(me) != nil || sent != 0 {
		t.Fatal("a too-fast finish was recorded")
	}
}
