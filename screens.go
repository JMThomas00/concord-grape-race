package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/arcade"
)

// frame is one arcade screen being drawn: the canvas, and the 80-column
// stage centred in it (ox, cw), and the pane's height.
type frame struct {
	c      *arcade.Canvas
	ox, cw int
	h      int
}

// X is column x of the stage.
func (f frame) X(x int) int { return f.ox + x }

var bulb = []string{".##.", "####", "####", ".##."}

var trophy = []string{".YYYYYYYY.", "Y.YhYYYY.Y", "Y.YYYYYY.Y", ".YYYYYYYY.", "...YYYY...", "....YY....", "...oooo...", "..oooooo.."}

// topbar is the arcade score line: you and your best, the server's best,
// and where you are.
func (m *model) topbar(f frame, right string) {
	c := f.c
	best := 0.0
	if r := m.a.records.get(m.id); r != nil {
		best = r.BestWPM
	}
	x := c.Text(f.X(1), 0, "1UP ", "pink", "", true)
	c.Text(x, 0, fmt.Sprintf("%s %03.0f", strings.ToUpper(trunc(m.name, 10)), best), "fg", "", false)
	if top := m.a.records.top(1); len(top) > 0 && f.cw >= 72 {
		hs, hv := "HI-SCORE ", fmt.Sprintf("%03.0f %s", top[0].WPM, strings.ToUpper(trunc(top[0].Name, 10)))
		cx := f.ox + (f.cw-arcade.TextWidth(hs+hv))/2
		c.Text(c.Text(cx, 0, hs, "pink", "", true), 0, hv, "fg", "", false)
	}
	c.Text(f.X(f.cw-1-arcade.TextWidth(right)), 0, right, "dim", "", false)
}

// logo draws text in the pixel font, centred on the stage, or as plain
// bold text when it's too wide.
func (m *model) logo(f frame, text string, py int, shade []string) {
	w := arcade.LogoWidth(text, 1)
	if w > f.cw {
		f.c.CenterIn(f.ox, f.cw, py/2+1, text, "purple", "", true)
		return
	}
	f.c.Logo(text, f.ox+(f.cw-w)/2, py, 1, shade)
}

func (m *model) raceTag() string {
	if m.scr == scrPractice {
		return "PRACTICE"
	}
	if r := m.a.race(m.ch); r.number > 0 {
		return fmt.Sprintf("RACE #%d", r.number)
	}
	return ""
}

func (m *model) drawTitle(f frame) {
	c, anim := f.c, m.animated()
	m.topbar(f, m.raceTag())
	m.logo(f, "GRAPE RACE", 4, nil)
	c.CenterIn(f.ox, f.cw, 7, "·  A   T Y P I N G   R A C E  ·", "dim", "", false)
	car := m.a.records.car(m.id)
	cx := f.ox + (f.cw-24)/2
	drawCar(c, car.Model, car.Paint, cx, 18, 2, false, false)
	shift := 0
	if anim {
		shift = m.a.frame % 4
	}
	for i, l := range []string{"═══════  ════   ══", "════  ═════  ═══ ═", "══════   ══ ═════"} {
		role := "comment"
		if i == 1 {
			role = "purple"
		}
		if x := cx - 21 + shift; x >= f.ox {
			c.Text(x, 10+i, l, role, "", false)
		}
	}
	if m.blink() {
		c.CenterIn(f.ox, f.cw, 15, "▶  PRESS ENTER  ◀", "yellow", "", true)
	}
	c.CenterIn(f.ox, f.cw, 17, m.channelLine(), "cyan", "", false)
	// the road, and attract mode: cars crossing it
	c.Fill(f.ox, 19, f.cw, "▁", "line", "")
	c.Fill(f.ox, 22, f.cw, "▔", "line", "")
	off := 0
	if anim {
		off = m.a.frame * 2
	}
	for i := 0; i < f.cw; i += 6 {
		if x := (i + off) % f.cw; x+1 < f.cw {
			c.Text(f.X(x), 21, "━━", "ghost", "", false)
		}
	}
	for _, a := range []struct {
		id, paint string
		speed     float64
		at        int
	}{{"tub", "pink", 1.6, 4}, {"monster", "green", 1.1, 32}, {"drag", "orange", 2.1, 58}} {
		x := a.at
		if anim {
			x = int(float64(m.a.frame)*a.speed+float64(a.at))%(f.cw+24) - 12
		}
		drawCar(c, a.id, a.paint, f.X(x), 40, 1, false, false)
	}
	c.Text(f.X(1), f.h-1, "© 2026 CONCORD ARCADE", "dim", "", false)
	leave := "leave"
	if m.standalone {
		leave = "quit"
	}
	c.Keys(f.X(f.cw-26), f.h-1, arcade.Key{Key: "Enter", Does: "start"}, arcade.Key{Key: "Esc", Does: leave})
}

// channelLine says what's happening in the channel right now.
func (m *model) channelLine() string {
	if m.standalone {
		return "SOLO PRACTICE · JUST YOU AND THE CLOCK"
	}
	r := m.a.race(m.ch)
	switch r.phase {
	case countdown:
		return fmt.Sprintf("RACE #%d IS STAGING · %d IN THE LANES · JOIN NOW", r.number, len(r.racers))
	case racing:
		return fmt.Sprintf("RACE #%d IN PROGRESS · %d RACING · WATCH LIVE", r.number, len(r.racers))
	case finished:
		if s := r.Standings(); len(s) > 0 && s[0].done {
			return fmt.Sprintf("LAST RACE WON BY %s AT %.0f WPM", strings.ToUpper(trunc(s[0].Name, 12)), s[0].WPM)
		}
	}
	return "NO RACE RUNNING · YOURS TO START"
}

func (m *model) drawMenu(f frame) {
	c := f.c
	m.topbar(f, m.raceTag())
	m.logo(f, "GRAPE RACE", 4, nil)
	preview := f.cw >= 78
	descW := f.cw - 26
	if preview {
		descW = f.cw - 24 - 28
	}
	for i, it := range m.menuItems() {
		y, sel := 9+i, i == m.menu
		fg, bg := "fg", ""
		if sel {
			c.Text(f.X(6), y, "▶", "purple", "", true)
			fg, bg = "yellow", "line"
		}
		c.Text(f.X(8), y, " "+fmt.Sprintf("%-13s", it), fg, bg, sel)
		d, hot := m.menuDetail(it)
		role := "dim"
		if sel {
			role = "fg"
		}
		if hot {
			role = "yellow"
		}
		c.Text(f.X(24), y, trunc(d, descW), role, "", hot)
	}
	if preview {
		car := m.a.records.car(m.id)
		drawCar(c, car.Model, car.Paint, f.X(f.cw-26), 18, 2, false, false)
		c.Text(f.X(f.cw-25), 14, fmt.Sprintf("#%02d %s", car.Number, strings.ToUpper(trunc(m.name, 14))), car.Paint, "", true)
	}
	c.Fill(f.ox, 17, f.cw, "─", "line", "")
	c.Text(f.X(1), 18, trunc(m.news(), f.cw-2), "cyan", "", false)
	c.Fill(f.ox, 19, f.cw, "─", "line", "")
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "↑↓", Does: "choose"}, arcade.Key{Key: "Enter", Does: "select"}, arcade.Key{Key: "Esc", Does: "title"})
}

// menuDetail is the line beside a menu item, and whether it wants attention.
func (m *model) menuDetail(item string) (string, bool) {
	rec := m.rec()
	switch item {
	case "RACE":
		r := m.a.race(m.ch)
		switch r.phase {
		case countdown:
			return fmt.Sprintf("Join race #%d · staging now", r.number), true
		case racing:
			return fmt.Sprintf("Race #%d on · watch, go next", r.number), false
		}
		return fmt.Sprintf("Start one · %s to join", secs(seconds(m.a.setting(m.ch, "countdown", ""), m.a.countdown))), false
	case "PRACTICE":
		return "Solo, you and the clock", false
	case "GARAGE":
		if n := rec.Rewards.Passes; n > 0 && len(rec.Rewards.Locked(carItems)) > 0 {
			return fmt.Sprintf("★ %d PIT PASS%s · NEW CAR!", n, plural(n, "ES")), true
		}
		car := m.a.records.car(m.id)
		return fmt.Sprintf("%s · #%02d", carByID[car.Model].Name, car.Number), false
	case "HALL OF FAME":
		if m.standalone {
			return "Your best runs", false
		}
		return "Top 10 on this server", false
	case "HOW TO PLAY":
		return "Keys, rules, timers", false
	case "OPTIONS":
		return "Sound " + strings.Trim(m.optionValue("SOUND"), "◂▸ ") + " · effects " + strings.ToLower(strings.Trim(m.optionValue("EFFECTS"), "◂▸ ")), false
	case "QUIT":
		return "Back to your terminal", false
	}
	return "", false
}

func plural(n int, s string) string {
	if n == 1 {
		return ""
	}
	return s
}

// news is the menu's ticker: the last race and the record.
func (m *model) news() string {
	var parts []string
	if r := m.a.race(m.ch); r.phase == finished && !m.standalone {
		var res []string
		for _, p := range r.Standings() {
			if p.done {
				res = append(res, fmt.Sprintf("%s %.0f", strings.ToUpper(trunc(p.Name, 10)), p.WPM))
			}
		}
		if len(res) > 0 {
			parts = append(parts, "LAST RACE ▸ "+strings.Join(res, " · "))
		}
	}
	if top := m.a.records.top(1); len(top) > 0 {
		parts = append(parts, fmt.Sprintf("RECORD ▸ %s %.0f WPM", strings.ToUpper(trunc(top[0].Name, 12)), top[0].WPM))
	}
	if len(parts) == 0 {
		return "NO RECORDS YET · THE FIRST FINISH SETS ONE"
	}
	return strings.Join(parts, "   ✦   ")
}

// drawRace picks the race screen for where the race is.
func (m *model) drawRace(f frame) {
	r := m.cur()
	switch {
	case r.phase == idle, r.phase == countdown, r.phase == racing && r.Elapsed() < time.Second:
		m.drawTree(f, r)
	case r.phase == racing:
		m.drawTrack(f, r)
	default:
		m.drawResults(f, r)
	}
}

// drawTree is the starting line: a drag-strip tree of lights, who's in the
// lanes, and the passage to read ahead.
func (m *model) drawTree(f frame, r *race) {
	c := f.c
	tag := m.raceTag()
	if r.phase == countdown {
		tag += " · STAGING"
	}
	m.topbar(f, tag)
	st := treeStage(r)
	me := r.racer(m.id)
	others := false
	for _, p := range r.racers {
		others = others || (p.Jumped && p.ID != m.id)
	}
	tx := f.X(10)
	c.Box(tx, 2, 20, 20, "comment", "TREE", "comment")
	lamp := func(lit bool, role string) map[rune]string {
		if lit {
			return map[rune]string{'#': role}
		}
		return map[rune]string{'#': "ghost"}
	}
	label := func(y int, s string, lit bool, role string) {
		r := "dim"
		if lit {
			r = role
		}
		c.Text(tx-1-arcade.TextWidth(s), y, s, r, "", lit)
	}
	small := func(y int, lit bool) {
		c.Sprite(tx+4, y*2, []string{"##", "##"}, lamp(lit, "fg"), 1)
		c.Sprite(tx+14, y*2, []string{"##", "##"}, lamp(lit, "fg"), 1)
	}
	big := func(y int, left, right bool, role string) {
		c.Sprite(tx+3, y*2, bulb, lamp(left, role), 1)
		c.Sprite(tx+13, y*2, bulb, lamp(right, role), 1)
	}
	on := func(k int) bool { return st >= k }
	label(4, "PRE-STAGE", on(0), "fg")
	small(4, on(0))
	label(6, "STAGE", on(1), "fg")
	small(6, on(1))
	for i, n := range []string{"3", "2", "1"} {
		label(8+i*3, n, on(2+i), "orange")
		big(8+i*3, on(2+i), on(2+i), "orange")
	}
	label(17, "GO", st == 5, "green")
	big(17, st == 5, st == 5, "green")
	foul := me != nil && me.Jumped
	label(20, "FOUL", foul || others, "red")
	big(19, foul, others, "red")

	x0, rw := f.X(31), f.cw-32
	switch {
	case st == 5:
		c.Text(x0, 2, "GO!", "green", "", true)
	case r.phase == countdown:
		c.Text(x0, 2, "STARTS IN", "pink", "", true)
		c.Digit(min(9, ceilSecs(r.startAt.Sub(r.now()))), x0, 6, "orange", 1)
	default:
		c.Text(x0, 2, "READY TO RACE", "pink", "", true)
		c.Digit(-1, x0, 6, "orange", 1)
	}
	for i, p := range r.racers {
		if i == 5 {
			c.Text(x0+6, 3+i, fmt.Sprintf("+%d MORE", len(r.racers)-5), "dim", "", false)
			break
		}
		car := m.a.records.car(p.ID)
		y := 3 + i
		c.Text(x0+6, y, "●", car.Paint, "", false)
		x := c.Text(x0+8, y, fmt.Sprintf("#%02d %s", car.Number, strings.ToUpper(trunc(p.Name, 10))), "fg", "", p.ID == m.id)
		if p.Jumped {
			c.Text(x+1, y, "RED LIGHT", "red", "", true)
		} else {
			c.Text(x+1, y, "STAGED", "green", "", false)
		}
	}
	msg, role := "", "dim"
	switch {
	case foul:
		msg, role = "RED LIGHT! +1.0s ON YOUR TIME", "red"
	case st == 5 && me != nil:
		msg, role = "TYPE!", "green"
	case r.phase == countdown && me == nil:
		msg, role = "A race is staging. Press Enter to join!", "yellow"
	case r.phase == countdown:
		msg = "Hands on the keys. Typing before green is a red light."
	case r.phase == idle && m.scr == scrPractice:
		msg = "Enter starts your run."
	case r.phase == idle:
		msg = "Enter starts a race; everyone here can join before green."
	}
	c.Text(x0, 9, trunc(msg, rw), role, "", role != "dim")
	boxH := f.h - 1 - 11
	if r.phase == idle || len(r.text) == 0 {
		c.Box(x0, 11, rw, boxH, "comment", "FASTEST HERE", "purple")
		for i, b := range m.a.records.top(min(5, boxH-2)) {
			c.Text(x0+2, 12+i, fmt.Sprintf("%d. %-14s %3.0f WPM", i+1, strings.ToUpper(trunc(b.Name, 14)), b.WPM), "fg", "", false)
		}
	} else {
		c.Box(x0, 11, rw, boxH, "comment", "THE PASSAGE", "purple")
		m.drawPassage(f, r, x0+2, 12, rw-4, boxH-2)
	}
	var keys []arcade.Key
	switch {
	case r.active(m.id) != nil:
		keys = []arcade.Key{{Key: "Backspace", Does: "fix"}, {Key: "Ctrl+W", Does: "word"}}
	case me == nil && r.phase != racing:
		keys = []arcade.Key{{Key: "Enter", Does: "join"}}
	case me != nil:
		keys = []arcade.Key{{Key: "type", Does: "at green"}}
	}
	keys = append(keys, arcade.Key{Key: "Esc", Does: "menu"})
	c.Keys(f.X(1), f.h-1, keys...)
}

// drawPassage writes the passage for whoever this viewer is following:
// typed text bright, mistakes red, the next character marked, the rest dim.
// It scrolls to keep the next character in view.
func (m *model) drawPassage(f frame, r *race, x, y, w, rows int) {
	if w < 8 || rows < 1 {
		return
	}
	p := m.target(r)
	good, typed := 0, 0
	if p != nil && r.phase != countdown {
		good, typed = r.correct(p), len(p.typed)
	}
	lines := wrapRanges(r.text, w)
	at := 0
	for i, ln := range lines {
		if typed >= ln[0] && typed < ln[1] {
			at = i
		}
	}
	start := max(0, min(at-rows/2, len(lines)-rows))
	for li := start; li < len(lines) && li-start < rows; li++ {
		ln := lines[li]
		for i := ln[0]; i < ln[1]; i++ {
			fg, bg, bold := "dim", "", false
			switch {
			case p == nil || r.phase == countdown:
			case i < good:
				fg, bold = "fg", true
			case i < typed:
				fg, bg = "bg", "red"
			case i == typed && !p.done && p.Out == "":
				fg, bg, bold = "purple", "line", true
			}
			f.c.Text(x+i-ln[0], y+li-start, string(r.text[i]), fg, bg, bold)
		}
	}
}

// lanes are the racers to draw: the top three, always including you.
func (m *model) lanes(r *race) []*racer {
	s := r.Standings()
	if len(s) <= 3 {
		return s
	}
	out := s[:3]
	for _, p := range out {
		if p.ID == m.id {
			return out
		}
	}
	if me := r.racer(m.id); me != nil {
		out = append(out[:2:2], me)
	}
	return out
}

func (m *model) drawTrack(f frame, r *race) {
	c := f.c
	m.topbar(f, fmt.Sprintf("%s · %s LEFT", m.raceTag(), clock(r.TimeLeft()+time.Second-1)))
	standings := r.Standings()
	pos := map[uuid.UUID]int{}
	for i, p := range standings {
		pos[p.ID] = i + 1
	}
	watched := m.target(r)
	anim := m.effects() == 0
	trackL, finish := f.X(12), f.X(f.cw-8)
	lanes := m.lanes(r)
	for lane, p := range lanes {
		y := 2 + lane*3
		n := pos[p.ID]
		c.Text(f.X(0), y, fmt.Sprint(n), map[bool]string{true: "yellow", false: "fg"}[n == 1], "", true)
		nameRole := "dim"
		switch {
		case p.ID == m.id:
			nameRole = "fg"
		case watched != nil && p.ID == watched.ID:
			nameRole = "cyan"
		}
		c.Text(f.X(2), y, strings.ToUpper(trunc(p.Name, 8)), nameRole, "", p.ID == m.id)
		c.Text(f.X(2), y+1, fmt.Sprintf("%3.0f WPM", r.LiveWPM(p)), "cyan", "", false)
		c.Text(f.X(11), y, "│", "line", "", false)
		c.Text(f.X(11), y+1, "│", "line", "", false)
		for rr := 0; rr < 2; rr++ {
			for k := 0; k < 2; k++ {
				ch := "▞"
				if (k+rr)%2 == 1 {
					ch = "▚"
				}
				c.Text(finish+k, y+rr, ch, "fg", "line", false)
			}
		}
		car := m.a.records.car(p.ID)
		span := max(0, finish-trackL-12)
		x := trackL + int(float64(r.correct(p))/float64(max(1, len(r.text)))*float64(span))
		if anim && !p.done && p.Out == "" && r.now().Sub(p.lastKey) < time.Second {
			for k := 1; k <= 3; k++ {
				if sx := x - k*3; sx > trackL {
					role, ch := "comment", "─"
					if k == 1 {
						role, ch = car.Paint, "━"
					}
					c.Text(sx, y+1, ch, role, "", false)
				}
			}
		}
		spin := r.Spinning(p) && m.effects() < 2
		drawCar(c, car.Model, car.Paint, x, y*2, 1, spin, p.Out != "")
		switch {
		case spin:
			c.Text(x-2, y, "✶", "red", "", false)
			c.Text(x+12, y+1, "✶", "yellow", "", false)
			if x+18 < finish {
				c.Text(x+13, y, "SPIN!", "red", "", true)
			}
		case p.Out == "idle" && x+18 < finish:
			c.Text(x+13, y, "IDLE", "dim", "", false)
		}
		if lane < len(lanes)-1 {
			c.Fill(trackL, y+2, finish-trackL, "╌", "line", "")
		}
	}

	focus := watched
	c.Box(f.X(0), 11, 28, 6, "comment", "SPEED", "purple")
	wpmNow := 0.0
	if focus != nil {
		wpmNow = r.LiveWPM(focus)
		c.Text(f.X(18), 13, fmt.Sprintf("%d%% ACC", int(accuracy(focus)*100+.5)), "dim", "", false)
	}
	c.Number(int(wpmNow+.5)%1000, 3, f.X(2), 24, "green", 1)
	c.Text(f.X(18), 12, "WPM", "fg", "", true)
	bars := min(9, int(wpmNow/10))
	for k := 0; k < 9; k++ {
		role, ch := "ghost", "▯"
		if k < bars {
			ch, role = "▮", "green"
			if k >= 5 {
				role = "orange"
			}
			if k >= 7 {
				role = "red"
			}
		}
		c.Text(f.X(18+k), 15, ch, role, "", false)
	}

	pbx, pbw := f.X(29), f.cw-29
	c.Box(pbx, 11, pbw, 6, "comment", "PIT BOARD", "purple")
	row := func(i int, k, v, role string) {
		c.Text(pbx+2, 12+i, k, "pink", "", true)
		c.Text(pbx+13, 12+i, trunc(v, pbw-15), role, "", false)
	}
	me := r.racer(m.id)
	switch {
	case r.active(m.id) != nil && r.IdleLeft(me) <= idleOut-idleWarn:
		c.Text(pbx+2, 13, "STILL THERE?", "red", "", true)
		c.Text(pbx+2, 14, fmt.Sprintf("YOU'RE OUT IN %ds UNLESS YOU TYPE", ceilSecs(r.IdleLeft(me))), "red", "", false)
	case r.active(m.id) != nil:
		i := pos[m.id] - 1
		row(0, "POSITION", fmt.Sprintf("%d OF %d", pos[m.id], len(standings)), "fg")
		gap := "ALONE ON THE TRACK"
		switch {
		case i > 0:
			gap = fmt.Sprintf("%d CHARS BEHIND %s", r.correct(standings[i-1])-r.correct(me), strings.ToUpper(trunc(standings[i-1].Name, 10)))
		case len(standings) > 1:
			gap = fmt.Sprintf("LEADING BY %d CHARS", r.correct(me)-r.correct(standings[1]))
		}
		row(1, "GAP", gap, "fg")
		row(2, "STREAK", fmt.Sprintf("%d CLEAN KEYS", me.streak), "green")
		row(3, "PASSAGE", fmt.Sprintf("%d/%d", r.correct(me), len(r.text)), "fg")
	default:
		status := "SPECTATING"
		switch {
		case me != nil && me.done:
			status = fmt.Sprintf("FINISHED %s · %.0f WPM", strings.ToUpper(ordinal(me.Place)), me.WPM)
		case me != nil && me.Out != "":
			status = "OUT · NO KEYS FOR 20s"
		}
		row(0, "YOU", status, "fg")
		if watched != nil && watched.ID != m.id {
			row(1, "WATCHING", strings.ToUpper(trunc(watched.Name, 14))+"  ←/→", "cyan")
		}
		next := "ENTER JOINS THE NEXT RACE"
		if r.Queued(m.id) {
			next = "YOU'RE IN THE NEXT RACE"
		}
		if m.scr == scrRace {
			row(2, "NEXT", next, "green")
		}
	}

	title, trole := "TYPE", "purple"
	if left, on := r.FinishLeft(); on {
		title, trole = fmt.Sprintf("TYPE · ⏱ %s TO FINISH", clock(left+time.Second-1)), "orange"
		if left <= 5*time.Second {
			trole = "red"
		}
	}
	boxH := f.h - 1 - 17
	c.Box(f.X(0), 17, f.cw, boxH, "comment", title, trole)
	m.drawPassage(f, r, f.X(2), 18, f.cw-4, boxH-2)
	if r.active(m.id) != nil {
		c.Keys(f.X(1), f.h-1, arcade.Key{Key: "Backspace", Does: "fix"}, arcade.Key{Key: "Ctrl+W", Does: "word"}, arcade.Key{Key: "Esc", Does: "menu"})
	} else {
		c.Keys(f.X(1), f.h-1, arcade.Key{Key: "←→", Does: "watch"}, arcade.Key{Key: "Enter", Does: "next race"}, arcade.Key{Key: "Esc", Does: "menu"})
	}
}

func (m *model) drawResults(f frame, r *race) {
	c := f.c
	m.topbar(f, m.raceTag()+" · FINISH")
	shift := 0
	if m.effects() == 0 {
		shift = m.a.frame
	}
	for row := 1; row <= 2; row++ {
		for x := 0; x < f.cw; x++ {
			ch := "▞"
			if (x+row+shift)%2 == 1 {
				ch = "▚"
			}
			c.Text(f.X(x), row, ch, "fg", "line", false)
		}
	}
	m.logo(f, "FINISH!", 7, nil)
	cx := f.ox + f.cw/2
	c.Sprite(cx-5, 16, trophy, map[rune]string{'Y': "yellow", 'h': "fg", 'o': "orange"}, 1)
	var done []*racer
	for _, p := range r.Standings() {
		if p.done {
			done = append(done, p)
		}
	}
	steps := []struct{ i, x, top int }{{1, cx - 21, 16}, {0, cx - 7, 14}, {2, cx + 7, 17}}
	for _, s := range steps {
		if s.i >= len(done) {
			continue
		}
		p := done[s.i]
		car := m.a.records.car(p.ID)
		drawCar(c, car.Model, car.Paint, s.x+1, (s.top-2)*2, 1, false, false)
		for y := s.top; y <= 20; y++ {
			c.Fill(s.x, y, 14, " ", "fg", "line")
		}
		c.CenterIn(s.x, 14, s.top, strings.ToUpper(ordinal(s.i+1)), [...]string{"yellow", "fg", "orange"}[s.i], "line", true)
		c.Text(s.x+1, 21, fmt.Sprintf("%-9s", strings.ToUpper(trunc(p.Name, 9))), "fg", "", true)
		c.Text(s.x+10, 21, fmt.Sprintf("%.0f", p.WPM), "cyan", "", false)
	}

	// your race, on the left
	lx, lw := f.X(1), max(10, cx-22-f.ox-1)
	me := r.racer(m.id)
	line := func(y int, s, role string, bold bool) { c.Text(lx, y, trunc(s, lw), role, "", bold) }
	switch {
	case me == nil:
		line(8, "SPECTATED", "dim", true)
	case me.done:
		line(8, "YOUR RACE", "pink", true)
		line(9, fmt.Sprintf("%.0f WPM · %.0f%%", me.WPM, me.Accuracy*100), "fg", true)
		y := 10
		if me.Jumped {
			line(y, "+1.0s RED LIGHT", "red", false)
			y++
		}
		switch {
		case me.TooFast:
			line(y, "too fast to count", "dim", false)
			y++
		case me.NewBest && m.blink():
			line(y, "NEW RECORD!", "pink", true)
			y++
		case me.NewBest:
			y++
		}
		if me.Passes > 0 {
			line(y, fmt.Sprintf("★ +%d PIT PASS%s", me.Passes, plural(me.Passes, "ES")), "yellow", true)
			for _, w := range wrapText(me.PassWhy, lw) {
				y++
				line(y, w, "dim", false)
			}
		}
	default:
		line(8, "YOUR RACE", "pink", true)
		why := "out of time"
		if me.Out == "idle" {
			why = "idle"
		}
		line(9, fmt.Sprintf("DNF · %s", why), "dim", false)
		line(10, fmt.Sprintf("%d%% done", r.correct(me)*100/max(1, len(r.text))), "dim", false)
	}
	// who didn't finish, on the right
	rx := cx + 8
	y := 8
	for _, p := range r.Standings() {
		if p.done || y > 12 {
			continue
		}
		if y == 8 {
			c.Text(rx, y, "DNF", "dim", "", true)
			y++
		}
		c.Text(rx, y, trunc(fmt.Sprintf("%s %d%%", strings.ToUpper(p.Name), r.correct(p)*100/max(1, len(r.text))), f.ox+f.cw-rx-1), "dim", "", false)
		y++
	}
	if len(r.next) > 0 && !r.nextAt.IsZero() {
		var names []string
		for _, q := range r.next {
			names = append(names, strings.ToUpper(q.Name))
		}
		c.CenterIn(f.ox, f.cw, 22, trunc(fmt.Sprintf("NEXT RACE IN %d · %s", ceilSecs(r.nextAt.Sub(r.now())), strings.Join(names, ", ")), f.cw-2), "orange", "", true)
	}
	again := "race again"
	if len(r.next) > 0 {
		again = "start it now"
	}
	if m.scr == scrPractice {
		again = "go again"
	}
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "Enter", Does: again}, arcade.Key{Key: "Esc", Does: "menu"})
}

func (m *model) drawGarage(f frame) {
	c := f.c
	rec := m.rec()
	car := m.a.records.car(m.id)
	cm := cars[m.gModel]
	owned := rec.Rewards.Owns(cm.ID, carItems)
	m.topbar(f, "GARAGE")
	c.Box(f.ox+1, 1, f.cw-2, f.h-3, "comment", "GARAGE", "purple")
	drawCar(c, cm.ID, car.Paint, f.ox+(f.cw-36)/2, 6, 3, false, !owned)
	if !owned {
		c.CenterIn(f.ox, f.cw, 5, " LOCKED ", "bg", "comment", true)
		c.CenterIn(f.ox, f.cw, 10, "Earn Pit Passes by racing to unlock cars like this one.", "dim", "", false)
	} else {
		c.CenterIn(f.ox, f.cw, 10, `"`+cm.Blurb+`"`, "dim", "", false)
	}
	name := "? ? ?"
	if owned {
		name = cm.Name
	}
	rows := []struct{ label, value, role string }{
		{"MODEL", "◂ " + name + " ▸", map[bool]string{true: "fg", false: "dim"}[owned]},
		{"PAINT", "◂ " + arcade.PaintNames[car.Paint] + " ▸", car.Paint},
		{"NUMBER", fmt.Sprintf("◂ %02d ▸", car.Number), "fg"},
	}
	for i, rw := range rows {
		y, sel := 12+i*2, i == m.gRow
		label := "pink"
		if sel {
			c.Text(f.X(4), y, "▶", "purple", "", true)
			label = "yellow"
		}
		c.Text(f.X(6), y, rw.label, label, "", true)
		end := c.Text(f.X(14), y, rw.value, rw.role, "", true)
		if i == 0 {
			c.Text(end+2, y, strings.TrimSpace(cm.Tier.Stars()+" "+cm.Tier.Name()), cm.Tier.Role(), "", true)
			if cm.ID == car.Model {
				c.Text(end+2, y+1, "✓ DRIVING", "green", "", false)
			}
		}
	}
	count, passes := rec.Rewards.Count(carItems), rec.Rewards.Passes
	if f.cw >= 78 {
		c.Text(f.X(50), 12, "COLLECTION", "pink", "", true)
		for k := 0; k < len(cars); k++ {
			ch, role := "▯", "ghost"
			if k < count {
				ch, role = "▮", "purple"
			}
			c.Text(f.X(50+k), 13, ch, role, "", false)
		}
		c.Text(f.X(50), 14, fmt.Sprintf("%d of %d cars", count, len(cars)), "fg", "", false)
		c.Text(f.X(50), 16, "PIT PASSES", "pink", "", true)
		role := "dim"
		if passes > 0 {
			role = "yellow"
		}
		c.Text(f.X(50), 17, fmt.Sprintf("★ %d", passes), role, "", true)
		c.Text(f.X(55), 17, map[bool]string{true: "ready to spend", false: "none yet"}[passes > 0], "dim", "", false)
	} else {
		c.Text(f.X(6), 18, fmt.Sprintf("%d of %d cars · ★ %d Pit Pass%s", count, len(cars), passes, plural(passes, "es")), "fg", "", false)
	}
	hint := ""
	switch {
	case m.gRow != 0:
	case owned && cm.ID != car.Model:
		hint = "Enter: drive this one"
	case !owned && passes > 0:
		hint = "Enter: spend a Pit Pass and pick 1 of 3 new cars"
	case !owned:
		hint = "Pit Passes: each new achievement, 3 wins in a row, every 10 races"
	}
	c.CenterIn(f.ox, f.cw, f.h-4, hint, "cyan", "", false)
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "↑↓", Does: "row"}, arcade.Key{Key: "←→", Does: "change"}, arcade.Key{Key: "Enter", Does: "choose"}, arcade.Key{Key: "C", Does: "showroom"}, arcade.Key{Key: "Esc", Does: "back"})
}

func (m *model) drawDraft(f frame) {
	c := f.c
	rec := m.rec()
	revealing := !m.revealAt.IsZero()
	offer := m.revealOffer
	if !revealing {
		offer = rec.Rewards.Deal(carItems, m.a.rnd)
	}
	m.topbar(f, "PIT PASS")
	title, shade := "PICK ONE", []string(nil)
	if revealing {
		title, shade = "UNLOCKED!", []string{"yellow"}
		if !m.blink() {
			shade = []string{"orange"}
		}
	}
	m.logo(f, title, 2, shade)
	paint := m.a.records.car(m.id).Paint
	cw := (f.cw - 2) / 3
	for i, id := range offer {
		cm := carByID[id]
		x := f.ox + i*(cw+1)
		sel := i == m.draftSel
		dead := revealing && !sel
		border := "comment"
		switch {
		case dead:
			border = "ghost"
		case sel && revealing && m.blink():
			border = "yellow"
		case sel && revealing:
			border = "orange"
		case sel:
			border = "purple"
		}
		c.Box(x, 6, cw, 14, border, "", "")
		if sel && !revealing {
			c.Text(x+cw/2, 5, "▼", "purple", "", true)
		}
		dim := func(role string) string {
			if dead {
				return "ghost"
			}
			return role
		}
		c.CenterIn(x, cw, 7, strings.TrimSpace(cm.Tier.Stars()+" "+cm.Tier.Name()), dim(cm.Tier.Role()), "", true)
		scale := 2
		if cw-2 < 24 {
			scale = 1
		}
		drawCar(c, id, paint, x+(cw-12*scale)/2, 20-2*scale, scale, false, dead)
		nameRole := "dim"
		if sel {
			nameRole = "fg"
		}
		c.CenterIn(x, cw, 14, cm.Name, dim(nameRole), "", true)
		for k, l := range wrapText(cm.Blurb, cw-2) {
			if k < 2 {
				c.CenterIn(x, cw, 16+k, l, dim("dim"), "", false)
			}
		}
	}
	if revealing && m.effects() == 0 && m.draftSel < len(offer) {
		x0 := f.ox + m.draftSel*(cw+1)
		for k := 0; k < 10; k++ {
			h := (k*7919 + (m.a.frame/2)*104729) % 1000
			sx, sy := x0-1+h%(cw+2), 5+(h/28)%16
			if sy == 5 || sy == 20 || sx == x0-1 || sx == x0+cw {
				c.Text(sx, sy, map[bool]string{true: "✦", false: "✧"}[k%2 == 1], map[bool]string{true: "yellow", false: "pink"}[k%3 != 0], "", true)
			}
		}
	}
	msg, role := fmt.Sprintf("Spends 1 Pit Pass · you have %d · the offer stays until you pick", rec.Rewards.Passes), "dim"
	if revealing {
		msg, role = fmt.Sprintf("%s is in your garage. %d Pit Pass%s left.", carByID[m.revealID].Name, rec.Rewards.Passes, plural(rec.Rewards.Passes, "es")), "green"
	}
	c.CenterIn(f.ox, f.cw, 21, trunc(msg, f.cw-2), role, "", revealing)
	if !revealing {
		c.Keys(f.X(1), f.h-1, arcade.Key{Key: "←→", Does: "choose"}, arcade.Key{Key: "Enter", Does: "unlock"}, arcade.Key{Key: "Esc", Does: "not now"})
	}
}

func (m *model) drawShowroom(f frame) {
	c := f.c
	rec := m.rec()
	mine := m.a.records.car(m.id)
	m.topbar(f, fmt.Sprintf("COLLECTION %d/%d", rec.Rewards.Count(carItems), len(cars)))
	c.CenterIn(f.ox, f.cw, 1, "THE SHOWROOM", "pink", "", true)
	cols := max(1, (f.cw-1)/13)
	slot := 4
	if cols >= 6 {
		slot = 6
	}
	for n, cm := range cars {
		x, y := f.X(1+(n%cols)*13), 3+(n/cols)*slot
		own := rec.Rewards.Owns(cm.ID, carItems)
		paint := arcade.Paints[n%len(arcade.Paints)]
		if cm.ID == mine.Model {
			paint = mine.Paint
		}
		drawCar(c, cm.ID, paint, x, y*2, 1, false, !own)
		name, role := "? ? ?", "dim"
		if own {
			name, role = cm.Short, "fg"
		}
		c.Text(x, y+2, name, role, "", own)
		if own {
			c.Text(x, y+3, "✓ OWNED", "green", "", false)
		} else {
			c.Text(x, y+3, cm.Tier.Name(), cm.Tier.Role(), "", false)
		}
	}
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "Esc", Does: "back"})
}

func (m *model) drawHOF(f frame) {
	c := f.c
	m.topbar(f, "HALL OF FAME")
	m.logo(f, "HALL OF FAME", 4, nil)
	top := m.a.records.top(10)
	hx := f.ox + (f.cw-46)/2
	if len(top) == 0 {
		c.CenterIn(f.ox, f.cw, 12, "No finishes yet. The first one sets the record.", "dim", "", false)
	} else {
		c.Text(hx, 8, "RANK   NAME            WPM    ACC    DATE", "pink", "", true)
	}
	roles := []string{"red", "orange", "yellow", "green", "cyan", "purple", "pink", "red", "orange", "yellow"}
	for i, b := range top {
		role, bg := roles[i], ""
		if b.ID == m.id {
			role, bg = "fg", "line"
		}
		date := "-"
		if !b.At.IsZero() {
			date = b.At.Local().Format("01-02")
		}
		c.Text(hx, 10+i, fmt.Sprintf("%-6s %-14s  %03.0f    %3.0f%%   %s", strings.ToUpper(ordinal(i+1)), strings.ToUpper(trunc(b.Name, 14)), b.WPM, b.Acc*100, date), role, bg, true)
	}
	if rank, of := m.a.records.rank(m.id); rank > 0 {
		c.CenterIn(f.ox, f.cw, 21, fmt.Sprintf("YOUR BEST %03.0f WPM · %s OF %d RACERS", m.rec().BestWPM, strings.ToUpper(ordinal(rank)), of), "fg", "", false)
	}
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "Esc", Does: "back"})
}

func (m *model) drawHowTo(f frame) {
	c := f.c
	m.topbar(f, "HOW TO PLAY")
	m.logo(f, "HOW TO PLAY", 4, nil)
	grace := seconds(m.a.setting(m.ch, "finish_timer", ""), defaultFinishTimer)
	rules := [][2]string{
		{"RACE", "Enter starts a race. Everyone who presses Enter before green joins in."},
		{"TYPE", "Type the passage. Mistakes turn red: fix them with Backspace (Ctrl+W, a word)."},
		{"RED LIGHT", "Typing before green adds 1 second to your time."},
		{"TIMERS", fmt.Sprintf("1, 2 or 3 minutes by passage length. When someone finishes, the rest get %s.", secs(grace))},
		{"IDLE", "No keys for 20 seconds and you're out of the race."},
		{"WATCH", "Not racing? ←/→ picks whose typing you watch. Enter joins the next race."},
		{"PIT PASS", "Earned for each new achievement, 3 wins in a row and every 10 races. Spend it in the garage: pick 1 of 3 new cars."},
	}
	y := 8
	for _, r := range rules {
		lines := wrapText(r[1], f.cw-15)
		if y+len(lines) > f.h-2 {
			break
		}
		c.Text(f.X(2), y, r[0], "pink", "", true)
		for _, l := range lines {
			c.Text(f.X(13), y, l, "fg", "", false)
			y++
		}
	}
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "Esc", Does: "back"})
}

func (m *model) drawOptions(f frame) {
	c := f.c
	m.topbar(f, "OPTIONS")
	m.logo(f, "OPTIONS", 4, nil)
	bw := min(56, f.cw-4)
	bx := f.ox + (f.cw-bw)/2
	extra := 0
	if !m.standalone {
		extra = 4
	}
	c.Box(bx, 8, bw, len(optionRows)+3+extra, "comment", "YOURS", "purple")
	for i, row := range optionRows {
		y, sel := 9+i, i == m.op
		if row == "BACK" {
			y++
		}
		role := "fg"
		if sel {
			c.Text(bx+2, y, "▶", "purple", "", true)
			role = "yellow"
		}
		c.Text(bx+4, y, row, role, "", sel)
		c.Text(bx+24, y, m.optionValue(row), map[bool]string{true: "fg", false: "cyan"}[sel], "", false)
	}
	if !m.standalone {
		y := 9 + len(optionRows) + 2
		c.Text(bx+4, y, "SET BY THE CHANNEL", "dim", "", true)
		c.Text(bx+4, y+1, fmt.Sprintf("Countdown %s · finish timer %s · passages %s",
			secs(seconds(m.a.setting(m.ch, "countdown", ""), m.a.countdown)),
			secs(seconds(m.a.setting(m.ch, "finish_timer", ""), defaultFinishTimer)),
			m.a.setting(m.ch, "length", "mixed")), "dim", "", false)
	}
	c.Keys(f.X(1), f.h-1, arcade.Key{Key: "↑↓", Does: "choose"}, arcade.Key{Key: "←→", Does: "change"}, arcade.Key{Key: "Esc", Does: "back"})
}

// secs is a whole number of seconds, like "15s".
func secs(d time.Duration) string { return fmt.Sprintf("%ds", int(d.Seconds())) }
