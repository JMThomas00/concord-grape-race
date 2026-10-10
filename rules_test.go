package main

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestFinishTimer: once someone finishes, the rest have the finish timer,
// then they're out of time and the race ends.
func TestFinishTimer(t *testing.T) {
	clk := newClock()
	a, b := uuid.New(), uuid.New()
	r := started(t, clk, "grape", a, b)
	clk.add(5 * time.Second)
	typeAll(r, a, "grape")
	left, on := r.FinishLeft()
	if !on || left != defaultFinishTimer {
		t.Fatalf("finish timer on=%v left=%v", on, left)
	}
	for range 3 { // b keeps typing (never idle) but doesn't finish in time
		clk.add(5 * time.Second)
		r.Type(b, 'x')
		r.Backspace(b)
		r.Tick()
	}
	if r.phase != finished || r.racer(b).Out != "time" {
		t.Fatalf("phase %v, b out %q", r.phase, r.racer(b).Out)
	}
}

// TestSoloIdlePlayerFreesTheChannel: someone starts alone and walks away;
// 20 seconds without a key and the race is over.
func TestSoloIdlePlayerFreesTheChannel(t *testing.T) {
	clk := newClock()
	a := uuid.New()
	r := started(t, clk, strings.Repeat("grape ", 30), a)
	clk.add(idleOut - time.Second)
	r.Tick()
	if r.phase != racing {
		t.Fatal("ended before the idle limit")
	}
	clk.add(time.Second)
	r.Tick()
	if r.phase != finished || r.racer(a).Out != "idle" {
		t.Fatalf("phase %v, out %q", r.phase, r.racer(a).Out)
	}
}

// TestTypingKeepsYouIn: each key resets the idle clock; a quiet racer
// drops out while the other keeps going.
func TestTypingKeepsYouIn(t *testing.T) {
	clk := newClock()
	a, b := uuid.New(), uuid.New()
	r := started(t, clk, strings.Repeat("grape ", 30), a, b)
	for range 4 {
		clk.add(8 * time.Second)
		r.Type(a, 'g')
		r.Backspace(a)
		r.Tick()
	}
	if r.racer(a).Out != "" || r.racer(b).Out != "idle" || r.phase != racing {
		t.Fatalf("a out %q, b out %q, phase %v", r.racer(a).Out, r.racer(b).Out, r.phase)
	}
	if r.Type(b, 'g') {
		t.Fatal("an idle racer typed")
	}
}

// TestTimeLimitByLength: short 1:00, medium 2:00, long 3:00.
func TestTimeLimitByLength(t *testing.T) {
	for length, want := range map[string]time.Duration{"short": time.Minute, "medium": 2 * time.Minute, "long": 3 * time.Minute} {
		if got := timeLimit([]rune(passages[length][0])); got != want {
			t.Errorf("%s: %v, want %v", length, got, want)
		}
	}
	clk := newClock()
	r := newRace(clk.now)
	r.Join(uuid.New(), "a", "long")
	if got := r.deadline.Sub(r.startAt); got != 3*time.Minute {
		t.Fatalf("long race limit %v", got)
	}
}

// TestQueueStartsTheNextRace: Enter during a race queues you; after the
// results, the next countdown starts with everyone queued.
func TestQueueStartsTheNextRace(t *testing.T) {
	clk := newClock()
	a, c, d := uuid.New(), uuid.New(), uuid.New()
	r := started(t, clk, "grape", a)
	r.Join(c, "c", "short")
	r.Join(d, "d", "short")
	r.Unqueue(d) // d closed the channel
	typeAll(r, a, "grape")
	if r.phase != finished || !r.Live() {
		t.Fatal("results with someone queued should keep the clock going")
	}
	clk.add(resultsHold - time.Second)
	r.Tick()
	if r.phase != finished {
		t.Fatal("results cut short")
	}
	clk.add(time.Second)
	r.Tick()
	if r.phase != countdown || r.number != 2 || len(r.racers) != 1 || r.racers[0].ID != c || len(r.next) != 0 {
		t.Fatalf("next race: phase %v, number %d, racers %d", r.phase, r.number, len(r.racers))
	}
}

// TestEnterDuringResultsStartsWithTheQueue: no waiting if someone presses
// Enter on the results; the queue comes along.
func TestEnterDuringResultsStartsWithTheQueue(t *testing.T) {
	clk := newClock()
	a, c := uuid.New(), uuid.New()
	r := started(t, clk, "grape", a)
	r.Join(c, "c", "short")
	typeAll(r, a, "grape")
	r.Join(a, "a", "short")
	if r.phase != countdown || r.racer(a) == nil || r.racer(c) == nil {
		t.Fatal("both should be in the new race")
	}
}

// TestSpectatorFollowsTheLeader: a spectator sees whoever's ahead, and
// left/right switches.
func TestSpectatorFollowsTheLeader(t *testing.T) {
	clk := newClock()
	a, b, me := uuid.New(), uuid.New(), uuid.New()
	r := started(t, clk, "grapes on the vine", a, b)
	app := newApp(loadRecords(""), clk.now)
	app.races[uuid.Nil] = r
	m := newModel(app, uuid.Nil, me, "me", nil)
	m.scr = scrRace
	typeAll(r, b, "grapes")
	typeAll(r, a, "gr")
	if got := m.target(r); got.ID != b {
		t.Fatal("should follow the leader")
	}
	m.Update(keyType("right"))
	if got := m.target(r); got.ID == b {
		t.Fatal("right should switch to someone else")
	}
	m.Update(keyType("left"))
	if got := m.target(r); got.ID != b {
		t.Fatal("left should switch back")
	}
}
