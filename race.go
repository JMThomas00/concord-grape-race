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
	Place    int // 1-based, among those who finished
	WPM      float64
	Accuracy float64 // 0..1
}

// race is one channel's race. All methods run on one goroutine (the plugin's
// callbacks, or the standalone program's update loop), so it has no lock.
type race struct {
	phase     phase
	number    int
	text      []rune
	countdown time.Duration
	startAt   time.Time // GO
	deadline  time.Time
	racers    []*racer
	now       func() time.Time

	// onFinish runs when a racer completes the passage, with how many raced.
	onFinish func(r *racer, field int)
}

func newRace(now func() time.Time) *race {
	return &race{now: now, countdown: 5 * time.Second}
}

// Join starts a countdown (when nothing is running) or joins the one in
// progress. It reports whether anything changed.
func (r *race) Join(id uuid.UUID, name, length string) bool {
	switch r.phase {
	case idle, finished:
		last := string(r.text)
		*r = race{now: r.now, countdown: r.countdown, onFinish: r.onFinish, number: r.number + 1}
		r.text = []rune(pickPassage(length, last))
		r.phase = countdown
		r.startAt = r.now().Add(r.countdown)
		limit := max(time.Minute, time.Duration(float64(len(r.text))/1.25*float64(time.Second))) // 15 wpm
		r.deadline = r.startAt.Add(limit)
	case countdown:
		if r.racer(id) != nil {
			return false
		}
	default:
		return false
	}
	r.racers = append(r.racers, &racer{ID: id, Name: name})
	return true
}

// Tick moves the race on with the clock: countdown to racing, racing to
// finished once everyone is done or time runs out. It reports whether the
// race is still live (worth ticking again).
func (r *race) Tick() bool {
	now := r.now()
	if r.phase == countdown && !now.Before(r.startAt) {
		r.phase = racing
	}
	if r.phase == racing && (r.allDone() || !now.Before(r.deadline)) {
		r.phase = finished
	}
	return r.phase == countdown || r.phase == racing
}

func (r *race) allDone() bool {
	for _, p := range r.racers {
		if !p.done {
			return false
		}
	}
	return true
}

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
	if p := r.racer(id); p != nil && r.phase == racing && !p.done {
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
	p.typed = p.typed[:len(p.typed)-1]
	return true
}

// DeleteWord removes back to the start of the current word.
func (r *race) DeleteWord(id uuid.UUID) bool {
	p := r.active(id)
	if p == nil || len(p.typed) == 0 {
		return false
	}
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
	if r.allDone() {
		r.phase = finished
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
	d := r.now().Sub(r.startAt)
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
