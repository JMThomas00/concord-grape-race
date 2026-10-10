package main

import (
	"math/rand/v2"
	"time"

	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/arcade"
)

// tickEvery is how often live things redraw: the race clock, the starting
// lights, the title screen's attract mode. Five frames a second keeps
// frames (full screens, sent to every viewer) affordable.
const tickEvery = 200 * time.Millisecond

// app is the state every viewer shares: one race per channel, each
// viewer's practice race, each channel's settings, everyone's records, and
// the screens that are open. Like the races, it's only touched from one
// goroutine (plugin callbacks, or the standalone program).
type app struct {
	races     map[uuid.UUID]*race // by channel
	practice  map[uuid.UUID]*race // by viewer
	settings  map[uuid.UUID]map[string]string
	records   *records
	models    map[uuid.UUID]*model // open screens, by viewer
	stages    map[*race]int        // last starting-light stage, for its beep
	now       func() time.Time
	countdown time.Duration
	rnd       *rand.Rand
	frame     int

	post       func(func())                                          // run fn on the owning goroutine, from any other
	changed    func(uuid.UUID)                                       // redraw everyone viewing a channel
	sendTo     func(uuid.UUID)                                       // redraw one viewer
	sendRecord func(uuid.UUID)                                       // a member's record changed (nil standalone)
	sound      func(ch, viewer uuid.UUID, asset string, vol float64) // nil standalone

	ticking bool
	gen     int
}

func newApp(rs *records, now func() time.Time) *app {
	return &app{
		races:     map[uuid.UUID]*race{},
		practice:  map[uuid.UUID]*race{},
		settings:  map[uuid.UUID]map[string]string{},
		models:    map[uuid.UUID]*model{},
		stages:    map[*race]int{},
		records:   rs,
		now:       now,
		countdown: 5 * time.Second,
		rnd:       rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x67726170)),
		changed:   func(uuid.UUID) {},
		sendTo:    func(uuid.UUID) {},
	}
}

// newRaceFor makes a race whose finishes go into the records.
func (a *app) newRaceFor() *race {
	r := newRace(a.now)
	r.onFinish = func(p *racer, field int) {
		if p.TooFast {
			return
		}
		e := a.records.add(p, field, len(r.text), a.now())
		p.NewBest, p.Passes, p.PassWhy = e.Best, e.Passes, e.Why
		if a.sendRecord != nil {
			a.sendRecord(p.ID)
		}
		sound := arcade.SoundFinish
		if e.Best || e.Passes > 0 {
			sound = arcade.SoundRecord
		}
		a.play(p.ID, sound)
	}
	return r
}

// race is the channel's race, made on first use.
func (a *app) race(ch uuid.UUID) *race {
	r := a.races[ch]
	if r == nil {
		r = a.newRaceFor()
		a.races[ch] = r
	}
	return r
}

// practiceFor is the viewer's own practice race, made on first use.
func (a *app) practiceFor(viewer uuid.UUID) *race {
	r := a.practice[viewer]
	if r == nil {
		r = a.newRaceFor()
		r.countdown = 3 * time.Second
		a.practice[viewer] = r
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

// startPractice starts a solo race for the viewer.
func (a *app) startPractice(ch, id uuid.UUID, name string) {
	r := a.practiceFor(id)
	if r.phase == countdown || r.phase == racing {
		return
	}
	r.Join(id, name, a.setting(ch, "length", "mixed"))
	a.startTicker()
	a.sendTo(id)
}

// leave forgets a viewer who closed the channel: their screen, their
// practice race, and their place in the next race.
func (a *app) leave(ch, id uuid.UUID) {
	delete(a.models, id)
	if r := a.practice[id]; r != nil {
		delete(a.stages, r)
		delete(a.practice, id)
	}
	if r := a.races[ch]; r != nil && r.Unqueue(id) {
		a.changed(ch)
	}
}

// play plays a sound for one viewer, by their own settings (muted, volume).
func (a *app) play(viewer uuid.UUID, asset string) {
	m := a.models[viewer]
	if a.sound == nil || m == nil {
		return
	}
	o := options{}
	if r := a.records.get(viewer); r != nil {
		o = r.Options
	}
	if o.Mute || (o.NoBeeps && (asset == arcade.SoundAmber || asset == arcade.SoundGo)) {
		return
	}
	a.sound(m.ch, viewer, asset, float64(o.volume())/10)
}

// seconds reads a whole-seconds setting between 1s and 2m, or def.
func seconds(v string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(v + "s"); err == nil && d >= time.Second && d <= 2*time.Minute {
		return d
	}
	return def
}

// treeStage is where the starting lights are: 0 pre-stage, 1 stage, 2-4
// the ambers (3, 2, 1), 5 green. -1: no race counting down or just started.
func treeStage(r *race) int {
	switch r.phase {
	case countdown:
		left := r.startAt.Sub(r.now())
		switch {
		case left > 3500*time.Millisecond:
			return 0
		case left > 3*time.Second:
			return 1
		case left > 2*time.Second:
			return 2
		case left > time.Second:
			return 3
		}
		return 4
	case racing:
		return 5
	}
	return -1
}

// beep plays the starting lights' sounds to whoever is watching this race.
func (a *app) beep(r *race, viewers []uuid.UUID) {
	st := treeStage(r)
	if st == a.stages[r] {
		return
	}
	a.stages[r] = st
	asset := ""
	switch st {
	case 2, 3, 4:
		asset = arcade.SoundAmber
	case 5:
		if r.Elapsed() < time.Second {
			asset = arcade.SoundGo
		}
	}
	if asset == "" {
		return
	}
	for _, v := range viewers {
		a.play(v, asset)
	}
}

// tickAll moves every live race on, redraws what changed and runs the
// animations. It reports whether anything still needs the clock.
func (a *app) tickAll() bool {
	a.frame++
	live := false
	for ch, r := range a.races {
		if !r.Live() {
			continue
		}
		r.Tick()
		live = live || r.Live()
		var watching []uuid.UUID
		for id, m := range a.models {
			if m.ch == ch && m.scr == scrRace {
				watching = append(watching, id)
			}
		}
		a.beep(r, watching)
		a.changed(ch)
	}
	for id, r := range a.practice {
		if !r.Live() {
			continue
		}
		r.Tick()
		live = live || r.Live()
		a.beep(r, []uuid.UUID{id})
		a.sendTo(id)
	}
	for id, m := range a.models {
		if m.animated() {
			live = true
			a.sendTo(id)
		}
	}
	return live
}

// startTicker runs the clock every tickEvery until nothing needs it. The
// clock runs on its own goroutine and hands each tick back through post,
// since a pane model's commands run synchronously (a tea.Tick would block).
func (a *app) startTicker() {
	if a.ticking || a.post == nil { // nowhere to deliver ticks yet
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
