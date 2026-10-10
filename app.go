package main

import (
	"time"

	"github.com/google/uuid"
)

// tickEvery is how often a live race redraws (the clock and live WPM).
const tickEvery = 200 * time.Millisecond

// app is the state every viewer shares: one race per channel, each
// channel's settings, and everyone's records. Like the races, it's only
// touched from one goroutine (plugin callbacks, or the standalone program).
type app struct {
	races     map[uuid.UUID]*race
	settings  map[uuid.UUID]map[string]string // channel → create_field values
	records   *records
	now       func() time.Time
	countdown time.Duration

	post       func(func())    // run fn on the owning goroutine, from any other
	changed    func(uuid.UUID) // redraw everyone viewing a channel
	sendRecord func(uuid.UUID) // a member's record changed (nil standalone)

	ticking bool
	gen     int
}

func newApp(rs *records, now func() time.Time) *app {
	return &app{
		races:     map[uuid.UUID]*race{},
		settings:  map[uuid.UUID]map[string]string{},
		records:   rs,
		now:       now,
		countdown: 5 * time.Second,
		post:      func(fn func()) { fn() },
		changed:   func(uuid.UUID) {},
	}
}

// race is the channel's race, made on first use.
func (a *app) race(ch uuid.UUID) *race {
	r := a.races[ch]
	if r == nil {
		r = newRace(a.now)
		r.onFinish = func(p *racer, field int) {
			if p.TooFast {
				return
			}
			p.NewBest = a.records.add(p, field, len(r.text), a.now())
			if a.sendRecord != nil {
				a.sendRecord(p.ID)
			}
		}
		a.races[ch] = r
	}
	return r
}

// setting is one of the channel's options, or def.
func (a *app) setting(ch uuid.UUID, key, def string) string {
	if v := a.settings[ch][key]; v != "" {
		return v
	}
	return def
}

// join starts or joins the channel's race.
func (a *app) join(ch, id uuid.UUID, name string) {
	r := a.race(ch)
	r.countdown = seconds(a.setting(ch, "countdown", ""), a.countdown)
	r.grace = seconds(a.setting(ch, "finish_timer", ""), defaultFinishTimer)
	if r.Join(id, name, a.setting(ch, "length", "mixed")) {
		a.startTicker()
		a.changed(ch)
	}
}

// leave drops someone who closed the channel from its next race.
func (a *app) leave(ch, id uuid.UUID) {
	if r := a.races[ch]; r != nil && r.Unqueue(id) {
		a.changed(ch)
	}
}

// seconds reads a whole-seconds setting between 1s and 2m, or def.
func seconds(v string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(v + "s"); err == nil && d >= time.Second && d <= 2*time.Minute {
		return d
	}
	return def
}

// tickAll moves every live race on and redraws its channel. It reports
// whether any race is still live.
func (a *app) tickAll() bool {
	live := false
	for ch, r := range a.races {
		if !r.Live() {
			continue
		}
		r.Tick()
		live = live || r.Live()
		a.changed(ch)
	}
	return live
}

// startTicker redraws live races every tickEvery until none is left. The
// clock runs on its own goroutine and hands each tick back through post,
// since a pane model's commands run synchronously (a tea.Tick would block).
func (a *app) startTicker() {
	if a.ticking {
		return
	}
	a.ticking = true
	a.gen++
	gen, stop := a.gen, make(chan struct{})
	go func() {
		t := time.NewTicker(tickEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				a.post(func() {
					if a.gen != gen || !a.ticking {
						return
					}
					if !a.tickAll() {
						a.ticking = false
						close(stop)
					}
				})
			}
		}
	}()
}
