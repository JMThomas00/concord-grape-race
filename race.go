package main

import (
	_ "embed"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// The passages: blank-line separated, plain ASCII so every character can be
// typed. Their length puts them in a group (short, medium, long).
//
//go:embed passages.txt
var passagesText string

var passages = func() map[string][]string {
	out := map[string][]string{}
	for _, p := range strings.Split(strings.ReplaceAll(passagesText, "\r\n", "\n"), "\n\n") {
		p = strings.Join(strings.Fields(p), " ")
		if p == "" {
			continue
		}
		out[lengthOf(p)] = append(out[lengthOf(p)], p)
	}
	return out
}()

func lengthOf(p string) string {
	switch n := len(p); {
	case n < 180:
		return "short"
	case n < 330:
		return "medium"
	default:
		return "long"
	}
}

// pickPassage chooses a passage of the given length ("mixed" or anything
// unknown: any), avoiding the one just raced.
func pickPassage(length, last string) string {
	var pool []string
	if ps, ok := passages[length]; ok {
		pool = ps
	} else {
		for _, l := range []string{"short", "medium", "long"} {
			pool = append(pool, passages[l]...)
		}
	}
	for range 10 {
		if p := pool[rand.IntN(len(pool))]; p != last || len(pool) == 1 {
			return p
		}
	}
	return pool[0]
}

type phase int

const (
	idle      phase = iota // nothing yet: Enter starts a countdown
	countdown              // joining is open
	racing
	finished // results until someone starts the next race
)

// maxWrong is how far past a mistake you can type before it must be fixed.
const maxWrong = 8

// maxWPM is faster than any person sustains: a finish above it is a
// script or a paste, so it doesn't count toward records.
const maxWPM = 250

// Timing rules, so a race always ends: nobody can hold a channel by walking
// away, disconnecting or refusing to finish.
const (
	idleOut            = 20 * time.Second // no keys for this long: out of the race
	idleWarn           = 10 * time.Second // when "Still there?" shows
	resultsHold        = 5 * time.Second  // results stay up this long before a queued race starts
	defaultFinishTimer = 15 * time.Second
)

// timeLimit is the whole race's limit, by passage length.
func timeLimit(text []rune) time.Duration {
	switch lengthOf(string(text)) {
	case "short":
		return time.Minute
	case "medium":
		return 2 * time.Minute
	}
	return 3 * time.Minute
}

// queued is someone waiting for the next race.
type queued struct {
	ID   uuid.UUID
	Name string
}

type racer struct {
	ID       uuid.UUID
	Name     string
	typed    []rune
	keys     int // characters typed, mistakes included
	mistakes int
	NewBest  bool // set when the finish beat their record
	TooFast  bool // above maxWPM: shown, never recorded
	done     bool
	doneAt   time.Time
	lastKey  time.Time // last keystroke (GO, before the first)
	Out      string    // why they stopped short: "idle" or "time"
	outAt    time.Time
	Place    int // 1-based, among those who finished
	WPM      float64
	Accuracy float64 // 0..1
}

// race is one channel's race. All methods run on one goroutine (the plugin's
// callbacks, or the standalone program's update loop), so it has no lock.
type race struct {
	phase     phase
	number    int
	length    string // the channel's passage length, for the next race
	text      []rune
	countdown time.Duration
	grace     time.Duration // the finish timer: how long the rest get once someone finishes
	startAt   time.Time     // GO
	deadline  time.Time     // the whole race's limit
	finishBy  time.Time     // set by the first finish
	nextAt    time.Time     // when the queued race starts, after the results
	racers    []*racer
	next      []queued // waiting for the next race
	now       func() time.Time

	// onFinish runs when a racer completes the passage, with how many raced.
	onFinish func(r *racer, field int)
}

func newRace(now func() time.Time) *race {
	return &race{now: now, countdown: 5 * time.Second, grace: defaultFinishTimer}
}

// Join starts a countdown (when nothing is running), joins the one counting
// down, or, while a race is on, puts you in the next one. It reports
// whether anything changed.
func (r *race) Join(id uuid.UUID, name, length string) bool {
	switch r.phase {
	case idle, finished:
		r.start(length, append(r.next, queued{id, name}))
	case countdown:
		if r.racer(id) != nil {
			return false
		}
		r.racers = append(r.racers, &racer{ID: id, Name: name, lastKey: r.startAt})
	default:
		if r.active(id) != nil || r.Queued(id) {
			return false
		}
		r.next = append(r.next, queued{id, name})
		r.length = length
	}
	return true
}

// start begins a countdown for a new passage with who as its racers.
func (r *race) start(length string, who []queued) {
	last := string(r.text)
	*r = race{now: r.now, countdown: r.countdown, grace: r.grace, onFinish: r.onFinish, number: r.number + 1, length: length}
	r.text = []rune(pickPassage(length, last))
	r.phase = countdown
	r.startAt = r.now().Add(r.countdown)
	r.deadline = r.startAt.Add(timeLimit(r.text))
	for _, q := range who {
		if r.racer(q.ID) == nil {
			r.racers = append(r.racers, &racer{ID: q.ID, Name: q.Name, lastKey: r.startAt})
		}
	}
}

// Queued reports whether id is waiting for the next race.
func (r *race) Queued(id uuid.UUID) bool {
	for _, q := range r.next {
		if q.ID == id {
			return true
		}
	}
	return false
}

// Unqueue drops id from the next race (they left the channel).
func (r *race) Unqueue(id uuid.UUID) bool {
	for i, q := range r.next {
		if q.ID == id {
			r.next = append(r.next[:i], r.next[i+1:]...)
			return true
		}
	}
	return false
}

// Tick moves the race on with the clock: countdown to racing; racers with
// no keys for idleOut are out; the race ends when everyone has finished or
// is out, at its time limit, or when the finish timer runs out; and after
// the results, a queued race starts.
func (r *race) Tick() {
	now := r.now()
	switch r.phase {
	case countdown:
		if !now.Before(r.startAt) {
			r.phase = racing
		}
	case finished:
		if len(r.next) > 0 && !r.nextAt.IsZero() && !now.Before(r.nextAt) {
			r.start(r.length, r.next)
		}
		return
	}
	if r.phase != racing {
		return
	}
	for _, p := range r.racers {
		if !p.done && p.Out == "" && now.Sub(p.lastKey) >= idleOut {
			p.Out, p.outAt = "idle", now
		}
	}
	if r.settled() || !now.Before(r.deadline) || (!r.finishBy.IsZero() && !now.Before(r.finishBy)) {
		r.end()
	}
}

// Live reports whether the race needs the clock: counting down, racing,
// or showing results before a queued race.
func (r *race) Live() bool {
	return r.phase == countdown || r.phase == racing || (r.phase == finished && len(r.next) > 0)
}

// end finishes the race: anyone still typing is out of time.
func (r *race) end() {
	now := r.now()
	for _, p := range r.racers {
		if !p.done && p.Out == "" {
			p.Out, p.outAt = "time", now
		}
	}
	r.phase = finished
	if len(r.next) > 0 {
		r.nextAt = now.Add(resultsHold)
	}
}

// settled: everyone has finished or is out.
func (r *race) settled() bool {
	for _, p := range r.racers {
		if !p.done && p.Out == "" {
			return false
		}
	}
	return true
}

// TimeLeft is what's left of the whole race's limit.
func (r *race) TimeLeft() time.Duration { return max(0, r.deadline.Sub(r.now())) }

// FinishLeft is what's left on the finish timer, and whether it's running.
func (r *race) FinishLeft() (time.Duration, bool) {
	if r.phase != racing || r.finishBy.IsZero() {
		return 0, false
	}
	return max(0, r.finishBy.Sub(r.now())), true
}

// IdleLeft is how long the racer has before going idle counts them out.
func (r *race) IdleLeft(p *racer) time.Duration { return idleOut - r.now().Sub(p.lastKey) }

func (r *race) racer(id uuid.UUID) *racer {
	for _, p := range r.racers {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// active is the racer id while they're still typing this race.
func (r *race) active(id uuid.UUID) *racer {
	if p := r.racer(id); p != nil && r.phase == racing && !p.done && p.Out == "" {
		return p
	}
	return nil
}

// correct is how much of the passage the racer has typed correctly.
func (r *race) correct(p *racer) int {
	n := 0
	for n < len(p.typed) && n < len(r.text) && p.typed[n] == r.text[n] {
		n++
	}
	return n
}

// Type adds one typed character. Mistakes count against accuracy and must
// be backspaced before the passage can be finished.
func (r *race) Type(id uuid.UUID, ch rune) bool {
	p := r.active(id)
	if p == nil || !unicode.IsPrint(ch) {
		return false
	}
	p.lastKey = r.now()
	good := r.correct(p)
	if len(p.typed)-good >= maxWrong || len(p.typed) >= len(r.text)+maxWrong {
		return false
	}
	p.keys++
	if good != len(p.typed) || len(p.typed) >= len(r.text) || r.text[len(p.typed)] != ch {
		p.mistakes++
	}
	p.typed = append(p.typed, ch)
	if len(p.typed) == len(r.text) && r.correct(p) == len(r.text) {
		r.finish(p)
	}
	return true
}

// Backspace removes the last typed character.
func (r *race) Backspace(id uuid.UUID) bool {
	p := r.active(id)
	if p == nil || len(p.typed) == 0 {
		return false
	}
	p.lastKey = r.now()
	p.typed = p.typed[:len(p.typed)-1]
	return true
}

// DeleteWord removes back to the start of the current word.
func (r *race) DeleteWord(id uuid.UUID) bool {
	p := r.active(id)
	if p == nil || len(p.typed) == 0 {
		return false
	}
	p.lastKey = r.now()
	i := len(p.typed)
	for i > 0 && p.typed[i-1] == ' ' {
		i--
	}
	for i > 0 && p.typed[i-1] != ' ' {
		i--
	}
	p.typed = p.typed[:i]
	return true
}

func (r *race) finish(p *racer) {
	p.done, p.doneAt = true, r.now()
	places := 0
	for _, q := range r.racers {
		if q.done {
			places++
		}
	}
	p.Place = places
	p.WPM = wpm(len(r.text), p.doneAt.Sub(r.startAt))
	p.Accuracy = accuracy(p)
	p.TooFast = p.WPM > maxWPM
	if r.onFinish != nil {
		r.onFinish(p, len(r.racers))
	}
	switch {
	case r.settled():
		r.end()
	case r.finishBy.IsZero() && r.grace > 0:
		r.finishBy = p.doneAt.Add(r.grace) // the rest have this long now
	}
}

// LiveWPM is the racer's speed so far (or final, once done).
func (r *race) LiveWPM(p *racer) float64 {
	if p.done {
		return p.WPM
	}
	if r.phase != racing && r.phase != finished {
		return 0
	}
	end := r.now()
	if p.Out != "" {
		end = p.outAt
	}
	d := end.Sub(r.startAt)
	if d < 2*time.Second { // a few characters in, the figure would be noise
		return 0
	}
	return wpm(r.correct(p), d)
}

// Elapsed is time since GO (0 before it).
func (r *race) Elapsed() time.Duration {
	if r.phase == idle || r.phase == countdown {
		return 0
	}
	return max(0, r.now().Sub(r.startAt))
}

// Standings are the racers in finishing order, then by progress.
func (r *race) Standings() []*racer {
	out := append([]*racer(nil), r.racers...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.done != b.done {
			return a.done
		}
		if a.done {
			return a.Place < b.Place
		}
		return r.correct(a) > r.correct(b)
	})
	return out
}

// wpm counts five characters as a word, the usual measure.
func wpm(chars int, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(chars) / 5 / d.Minutes()
}

func accuracy(p *racer) float64 {
	if p.keys == 0 {
		return 1
	}
	return float64(p.keys-p.mistakes) / float64(p.keys)
}
