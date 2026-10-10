package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/pane"
)

// redrawMsg asks a viewer's model to draw again (shared state changed).
type redrawMsg struct{}

// runMsg carries work onto the standalone program's update loop.
type runMsg func()

// model is one viewer's screen. The race itself lives in app.
type model struct {
	a          *app
	ch, id     uuid.UUID
	name       string
	v          *pane.Viewer // nil standalone
	w, h       int
	standalone bool
	watch      uuid.UUID // the racer this viewer follows while not typing (zero: the leader)
}

func newModel(a *app, ch, id uuid.UUID, name string, v *pane.Viewer) *model {
	return &model{a: a, ch: ch, id: id, name: name, v: v}
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case runMsg:
		msg()
	case tea.KeyMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *model) key(k tea.KeyMsg) tea.Cmd {
	if m.standalone && k.Type == tea.KeyCtrlC {
		return tea.Quit
	}
	r := m.a.race(m.ch)
	if r.active(m.id) != nil {
		changed := false
		switch {
		case k.Type == tea.KeyRunes && len(k.Runes) == 1 && !k.Alt:
			changed = r.Type(m.id, k.Runes[0]) // several runes at once is a paste: ignored
		case k.Type == tea.KeySpace:
			changed = r.Type(m.id, ' ')
		case k.Type == tea.KeyCtrlW, k.Type == tea.KeyBackspace && k.Alt:
			changed = r.DeleteWord(m.id)
		case k.Type == tea.KeyBackspace, k.Type == tea.KeyCtrlH:
			changed = r.Backspace(m.id)
		}
		if changed {
			m.a.changed(m.ch)
		}
		return nil
	}
	switch k.String() {
	case "enter":
		m.a.join(m.ch, m.id, m.name)
	case "left", "right":
		m.cycleWatch(r, k.String() == "right")
	case "esc", "q":
		if m.standalone {
			return tea.Quit
		}
	}
	return nil
}

// target is whose typing the passage shows: your own while you're typing,
// otherwise the racer you chose with left/right, or else whoever's ahead
// among those still going.
func (m *model) target(r *race) *racer {
	if p := r.active(m.id); p != nil {
		return p
	}
	if p := r.racer(m.watch); p != nil && p.ID != m.id {
		return p
	}
	for _, p := range r.Standings() {
		if p.ID != m.id && !p.done && p.Out == "" {
			return p
		}
	}
	return r.racer(m.id)
}

// cycleWatch follows the next (or previous) racer.
func (m *model) cycleWatch(r *race, forward bool) {
	var others []*racer
	for _, p := range r.racers {
		if p.ID != m.id {
			others = append(others, p)
		}
	}
	if len(others) == 0 || r.phase != racing {
		return
	}
	i := 0
	if t := m.target(r); t != nil {
		for j, p := range others {
			if p.ID == t.ID {
				i = j
			}
		}
	}
	if forward {
		i = (i + 1) % len(others)
	} else {
		i = (i + len(others) - 1) % len(others)
	}
	m.watch = others[i].ID
}

// styles, from the viewer's Concord theme (Dracula when standalone).
type styles struct {
	title, fg, dim, done, wrong, cursor, good, warn, bad, accent, vine lipgloss.Style
}

func (m *model) styles() styles {
	c := func(name, hex string) lipgloss.TerminalColor {
		if m.v != nil {
			return m.v.Color(name, lipgloss.Color(hex))
		}
		return lipgloss.Color(hex)
	}
	purple, fg, comment := c("purple", "#bd93f9"), c("foreground", "#f8f8f2"), c("comment", "#6272a4")
	s := lipgloss.NewStyle()
	return styles{
		title:  s.Bold(true).Foreground(purple),
		fg:     s.Foreground(fg),
		dim:    s.Foreground(comment),
		done:   s.Foreground(fg).Bold(true),
		wrong:  s.Foreground(c("background", "#282a36")).Background(c("red", "#ff5555")),
		cursor: s.Foreground(purple).Bold(true).Underline(true),
		good:   s.Foreground(c("green", "#50fa7b")),
		warn:   s.Foreground(c("yellow", "#f1fa8c")).Bold(true),
		bad:    s.Foreground(c("red", "#ff5555")).Bold(true),
		accent: s.Foreground(c("cyan", "#8be9fd")),
		vine:   s.Foreground(purple),
	}
}

func (m *model) View() string {
	w, h := m.w, m.h
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	st := m.styles()
	r := m.a.race(m.ch)
	var out []string
	out = append(out, m.header(st, r, w), "")
	switch r.phase {
	case idle:
		out = append(out, m.lobby(st, w)...)
	case finished:
		out = append(out, m.track(st, r, w)...)
		out = append(out, "")
		out = append(out, m.results(st, r, w)...)
	default:
		out = append(out, m.track(st, r, w)...)
		out = append(out, "")
		out = append(out, m.status(st, r)...)
		out = append(out, "")
		out = append(out, m.passage(st, r, w)...)
	}
	// the finish timer and the hints sit at the bottom of the pane
	bottom := []string{st.dim.Render(m.hint(r))}
	if left, on := r.FinishLeft(); on {
		style := st.warn
		if left <= 5*time.Second {
			style = st.bad
		}
		bottom = append([]string{" " + style.Render(fmt.Sprintf("⏱ %s left to finish", clock(left+time.Second-1)))}, bottom...)
	}
	if pad := h - len(out) - len(bottom); pad > 0 {
		out = append(out, make([]string, pad)...)
	} else {
		out = append(out, "")
	}
	out = append(out, bottom...)
	return pane.Fit(strings.Join(out, "\n"), w, h)
}

func (m *model) header(st styles, r *race, w int) string {
	left := st.title.Render("🍇 Grape Race")
	var right string
	switch r.phase {
	case countdown:
		right = fmt.Sprintf("race %d · starts in %d", r.number, ceilSecs(r.startAt.Sub(r.now())))
	case racing:
		right = fmt.Sprintf("race %d · %s left", r.number, clock(r.TimeLeft()+time.Second-1))
	case finished:
		right = fmt.Sprintf("race %d · results", r.number)
	}
	right = st.dim.Render(right)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		return left
	}
	return " " + left + strings.Repeat(" ", gap-1) + right
}

func (m *model) lobby(st styles, w int) []string {
	text := []string{
		"Race the room. Press Enter and a countdown starts; everyone who presses Enter before GO joins in. Type the passage as fast as you can.",
		"",
		"Mistakes turn red and have to be fixed before you can finish. Fastest clean finish wins.",
	}
	if m.standalone {
		text = []string{
			"Type the passage as fast as you can. Mistakes turn red and have to be fixed before you can finish.",
			"",
			"Press Enter to start.",
		}
	}
	var out []string
	for _, t := range text {
		for _, l := range wrapText(t, min(w-2, 72)) {
			out = append(out, " "+st.fg.Render(l))
		}
	}
	return append(out, m.leaders(st)...)
}

// leaders is the server's fastest typists (your own bests, standalone).
func (m *model) leaders(st styles) []string {
	top := m.a.records.top(5)
	if len(top) == 0 {
		return nil
	}
	title := "Fastest on this server"
	if m.standalone {
		title = "Your best"
		if r := m.a.records.get(m.id); r != nil {
			return []string{"", " " + st.title.Render(title) + "  " + st.fg.Render(fmt.Sprintf("%.0f wpm over %d races", r.BestWPM, r.Races))}
		}
	}
	out := []string{"", " " + st.title.Render(title)}
	for i, b := range top {
		out = append(out, fmt.Sprintf(" %s %s %s", st.dim.Render(fmt.Sprintf("%d.", i+1)), st.fg.Render(fmt.Sprintf("%-16s", trunc(b.Name, 16))), st.accent.Render(fmt.Sprintf("%3.0f wpm", b.WPM))))
	}
	return out
}

// track draws each racer's grape rolling down the vine.
func (m *model) track(st styles, r *race, w int) []string {
	racers := r.Standings()
	watched := m.target(r)
	nameW := 4
	for _, p := range racers {
		nameW = max(nameW, min(14, lipgloss.Width(p.Name)))
	}
	const statW = 13 // " 123 wpm  1st"
	lane := w - 3 - nameW - 1 - statW
	var out []string
	for i, p := range racers {
		if i == 8 {
			out = append(out, st.dim.Render(fmt.Sprintf("   and %d more", len(racers)-8)))
			break
		}
		mark := "  "
		name := st.fg.Render(fmt.Sprintf("%-*s", nameW, trunc(p.Name, nameW)))
		switch {
		case p.ID == m.id:
			mark, name = st.title.Render("▶ "), st.done.Render(fmt.Sprintf("%-*s", nameW, trunc(p.Name, nameW)))
		case r.phase == racing && watched != nil && p.ID == watched.ID:
			mark = st.accent.Render("▷ ")
		}
		line := " " + mark + name + " "
		if lane >= 8 {
			line += vine(st, lane, float64(r.correct(p))/float64(max(1, len(r.text))))
		}
		place := "   "
		switch {
		case p.done && p.Place == 1:
			place = st.warn.Render("1st")
		case p.done:
			place = st.fg.Render(ordinal(p.Place))
		case p.Out == "idle" && r.phase == racing:
			place = st.dim.Render("idle")
		case p.Out != "":
			place = st.dim.Render("DNF")
		}
		line += st.accent.Render(fmt.Sprintf(" %3.0f wpm", r.LiveWPM(p))) + "  " + place
		out = append(out, line)
	}
	return out
}

// vine is a lane of width cells with the grape at progress (0..1).
func vine(st styles, width int, progress float64) string {
	room := width - 2 // the grape is two cells wide
	pos := min(room, int(progress*float64(room)+0.5))
	return st.vine.Render(strings.Repeat("━", pos)) + "🍇" + st.dim.Render(strings.Repeat("┄", room-pos))
}

func (m *model) status(st styles, r *race) []string {
	p := r.racer(m.id)
	if r.phase == countdown {
		if p != nil {
			return []string{" " + st.warn.Render(fmt.Sprintf("Get ready… %d", ceilSecs(r.startAt.Sub(r.now()))))}
		}
		return []string{" " + st.fg.Render("A race is about to start. Press Enter to join!")}
	}
	var out []string
	switch {
	case r.active(m.id) != nil && r.IdleLeft(p) <= idleOut-idleWarn:
		return []string{" " + st.bad.Render(fmt.Sprintf("Still there? You're out in %ds unless you type.", ceilSecs(r.IdleLeft(p))))}
	case r.active(m.id) != nil:
		return []string{" " + st.warn.Render("GO!") + "  " + st.fg.Render(fmt.Sprintf("%.0f wpm · %.0f%% accuracy · %d/%d", r.LiveWPM(p), accuracy(p)*100, r.correct(p), len(r.text)))}
	case p != nil && p.done:
		out = append(out, " "+st.good.Render(fmt.Sprintf("Finished %s! %.0f wpm, %.0f%% accuracy.", ordinal(p.Place), p.WPM, p.Accuracy*100)))
	case p != nil && p.Out == "idle":
		out = append(out, " "+st.dim.Render("You're out: no keys for 20 seconds."))
	case p == nil:
		out = append(out, " "+st.dim.Render("Spectating."))
	}
	if t := m.target(r); t != nil && t.ID != m.id {
		out = append(out, " "+st.accent.Render("Watching "+t.Name)+st.fg.Render(fmt.Sprintf(" · %.0f wpm · %d/%d", r.LiveWPM(t), r.correct(t), len(r.text))))
	}
	if r.Queued(m.id) {
		out = append(out, " "+st.good.Render("You're in the next race."))
	}
	return out
}

// passage draws the text: typed (bright), mistakes (red), the next
// character (underlined), the rest (dim).
func (m *model) passage(st styles, r *race, w int) []string {
	width := min(w-2, 78)
	if width < 10 {
		width = max(1, w)
	}
	p := m.target(r)
	good, typed := 0, 0
	if p != nil && r.phase != countdown {
		good, typed = r.correct(p), len(p.typed)
	}
	classes := []lipgloss.Style{st.dim, st.done, st.wrong, st.cursor}
	classAt := func(i int) int {
		switch {
		case p == nil || r.phase == countdown:
			return 0
		case i < good:
			return 1
		case i < typed:
			return 2
		case i == typed && !p.done && p.Out == "":
			return 3
		}
		return 0
	}
	var out []string
	for _, ln := range wrapRanges(r.text, width) {
		var b strings.Builder
		b.WriteString(" ")
		start := ln[0]
		for i := ln[0]; i <= ln[1]; i++ {
			if i == ln[1] || classAt(i) != classAt(start) {
				b.WriteString(classes[classAt(start)].Render(string(r.text[start:i])))
				start = i
			}
		}
		out = append(out, b.String())
	}
	return out
}

func (m *model) results(st styles, r *race, w int) []string {
	out := []string{" " + st.title.Render("Results")}
	for _, p := range r.Standings() {
		if !p.done {
			why := "out of time"
			if p.Out == "idle" {
				why = "idle"
			}
			out = append(out, st.dim.Render(fmt.Sprintf("  DNF  %-20s %3.0f wpm  %3d%% done  (%s)", trunc(p.Name, 20), r.LiveWPM(p), r.correct(p)*100/max(1, len(r.text)), why)))
			continue
		}
		line := fmt.Sprintf("  %-4s %-20s %3.0f wpm  %3.0f%%  %s", ordinal(p.Place), trunc(p.Name, 20), p.WPM, p.Accuracy*100, clock(p.doneAt.Sub(r.startAt)))
		if p.Place == 1 {
			line = st.warn.Render(line)
		} else {
			line = st.fg.Render(line)
		}
		switch {
		case p.TooFast:
			line += "  " + st.dim.Render("too fast to count")
		case p.NewBest:
			line += "  " + st.good.Render("new best!")
		}
		out = append(out, line)
	}
	if next := r.next; len(next) > 0 && !r.nextAt.IsZero() {
		var names []string
		for _, q := range next {
			names = append(names, q.Name)
		}
		wait := max(0, r.nextAt.Sub(r.now()))
		out = append(out, "", " "+st.warn.Render(fmt.Sprintf("Next race in %ds: ", ceilSecs(wait)))+st.fg.Render(trunc(strings.Join(names, ", "), max(10, w-20))))
	}
	return append(out, m.leaders(st)...)
}

func (m *model) hint(r *race) string {
	var h string
	switch {
	case r.phase == idle:
		h = " Enter start a race"
	case r.phase == finished && len(r.next) > 0:
		h = " Enter start it now"
	case r.phase == finished:
		h = " Enter race again"
	case r.phase == countdown && r.racer(m.id) == nil:
		h = " Enter join the race"
	case r.phase == countdown:
		h = " Type when it says GO!"
	case r.active(m.id) != nil:
		h = " Backspace fixes a mistake · Ctrl+W deletes a word"
	case r.Queued(m.id):
		h = " ←/→ watch someone else"
	default:
		h = " Enter join the next race · ←/→ watch someone else"
	}
	if m.standalone && r.active(m.id) == nil {
		h += " · Esc quit"
	}
	return h
}

// wrapRanges splits text into lines of at most width runes, breaking after
// spaces; each range is [start, end).
func wrapRanges(text []rune, width int) [][2]int {
	var out [][2]int
	start := 0
	for start < len(text) {
		end := min(len(text), start+width)
		if end < len(text) {
			if i := lastSpace(text[start:end]); i > 0 {
				end = start + i + 1
			}
		}
		out = append(out, [2]int{start, end})
		start = end
	}
	return out
}

func lastSpace(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] == ' ' {
			return i
		}
	}
	return -1
}

func wrapText(s string, width int) []string {
	var out []string
	for _, r := range wrapRanges([]rune(s), max(1, width)) {
		out = append(out, strings.TrimRight(string([]rune(s)[r[0]:r[1]]), " "))
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func trunc(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:max(0, n-1)]) + "…"
}

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

func clock(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// ceilSecs is d in whole seconds, rounded up: a countdown reads 5 at 5s.
func ceilSecs(d time.Duration) int { return int((max(0, d) + time.Second - 1) / time.Second) }
