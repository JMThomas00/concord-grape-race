package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/arcade"
	"github.com/JMThomas00/Concord/sdk/pane"
	"github.com/JMThomas00/Concord/sdk/wire"
)

// screen is where a viewer is in the game.
type screen int

const (
	scrTitle screen = iota
	scrMenu
	scrRace     // the channel's race
	scrPractice // the viewer's own race
	scrGarage
	scrDraft
	scrShowroom
	scrHOF
	scrHowTo
	scrOptions
)

// redrawMsg asks a viewer's model to draw again (shared state changed).
type redrawMsg struct{}

// runMsg carries work onto the standalone program's update loop.
type runMsg func()

// attractFor is how long the title screen animates after the last key.
const attractFor = 30 * time.Second

// revealFor is how long an unlock is celebrated before the garage returns.
const revealFor = 3200 * time.Millisecond

// model is one viewer's screen. The races and records live in app.
type model struct {
	a          *app
	ch, id     uuid.UUID
	name       string
	v          *pane.Viewer // nil standalone
	w, h       int
	standalone bool
	watch      uuid.UUID // the racer this viewer follows while not typing (zero: the leader)

	scr                              screen
	menu, gRow, gModel, draftSel, op int
	lastKey                          time.Time
	revealAt                         time.Time
	revealID                         string
	revealOffer                      []string // the cards on show while an unlock is revealed
}

func newModel(a *app, ch, id uuid.UUID, name string, v *pane.Viewer) *model {
	m := &model{a: a, ch: ch, id: id, name: name, v: v, lastKey: a.now()}
	a.models[id] = m
	a.startTicker() // the title screen's attract mode
	return m
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case runMsg:
		msg()
	case tea.KeyMsg:
		m.lastKey = m.a.now()
		cmd = m.key(msg)
	}
	if !m.revealAt.IsZero() && m.a.now().Sub(m.revealAt) >= revealFor {
		m.revealAt, m.scr = time.Time{}, scrGarage
	}
	return m, cmd
}

// ClaimedKeys: Esc is Concord's on the title screen (it leaves the pane);
// everywhere else it means back.
func (m *model) ClaimedKeys() []string {
	if m.scr == scrTitle {
		return nil
	}
	return []string{wire.PaneKeyEsc}
}

// animated reports whether this screen moves on its own: the title's
// attract mode, an unlock being revealed.
func (m *model) animated() bool {
	switch {
	case m.scr == scrTitle:
		return m.effects() == 0 && m.a.now().Sub(m.lastKey) < attractFor
	case m.scr == scrDraft:
		return !m.revealAt.IsZero()
	}
	return false
}

// cur is the race this viewer is looking at.
func (m *model) cur() *race {
	if m.scr == scrPractice {
		return m.a.practiceFor(m.id)
	}
	return m.a.race(m.ch)
}

func (m *model) rec() *record { return m.a.records.ensure(m.id, m.name) }

func (m *model) effects() int {
	if r := m.a.records.get(m.id); r != nil {
		return r.Options.Effects
	}
	return 0
}

// blink is true half the time (twice a second at five frames), or always
// when effects are calm or off.
func (m *model) blink() bool { return m.effects() > 0 || (m.a.frame/3)%2 == 0 }

func (m *model) go_(s screen) {
	m.scr = s
	if s == scrTitle {
		m.a.startTicker()
	}
}

// menuItems are the main menu, with what each does.
func (m *model) menuItems() []string {
	if m.standalone {
		return []string{"PRACTICE", "GARAGE", "HALL OF FAME", "HOW TO PLAY", "OPTIONS", "QUIT"}
	}
	return []string{"RACE", "PRACTICE", "GARAGE", "HALL OF FAME", "HOW TO PLAY", "OPTIONS"}
}

func (m *model) key(k tea.KeyMsg) tea.Cmd {
	if m.standalone && k.Type == tea.KeyCtrlC {
		return tea.Quit
	}
	s := k.String()
	switch m.scr {
	case scrTitle:
		switch s {
		case "enter":
			m.a.play(m.id, arcade.SoundCoin)
			m.go_(scrMenu)
		case "esc", "q":
			if m.standalone {
				return tea.Quit
			}
		}
	case scrMenu:
		items := m.menuItems()
		switch s {
		case "up", "k":
			m.menu = (m.menu + len(items) - 1) % len(items)
			m.a.play(m.id, arcade.SoundBlip)
		case "down", "j":
			m.menu = (m.menu + 1) % len(items)
			m.a.play(m.id, arcade.SoundBlip)
		case "esc":
			m.a.play(m.id, arcade.SoundBack)
			m.go_(scrTitle)
		case "enter":
			m.a.play(m.id, arcade.SoundSelect)
			switch items[m.menu] {
			case "RACE":
				m.go_(scrRace)
				m.a.join(m.ch, m.id, m.name)
			case "PRACTICE":
				m.go_(scrPractice)
				m.a.startPractice(m.ch, m.id, m.name)
			case "GARAGE":
				m.openGarage()
			case "HALL OF FAME":
				m.go_(scrHOF)
			case "HOW TO PLAY":
				m.go_(scrHowTo)
			case "OPTIONS":
				m.op = 0
				m.go_(scrOptions)
			case "QUIT":
				return tea.Quit
			}
		}
	case scrRace, scrPractice:
		m.raceKey(k)
	case scrGarage:
		m.garageKey(s)
	case scrDraft:
		m.draftKey(s)
	case scrShowroom, scrHOF, scrHowTo:
		if s == "esc" {
			m.a.play(m.id, arcade.SoundBack)
			if m.scr == scrShowroom {
				m.go_(scrGarage)
			} else {
				m.go_(scrMenu)
			}
		}
	case scrOptions:
		m.optionsKey(s)
	}
	return nil
}

// raceKey handles keys on a race screen: typing while racing, a red light
// for typing during the countdown, Enter to start or join, ←/→ to watch.
func (m *model) raceKey(k tea.KeyMsg) {
	r := m.cur()
	redraw := func() {
		if m.scr == scrPractice {
			m.a.sendTo(m.id)
		} else {
			m.a.changed(m.ch)
		}
	}
	if p := r.active(m.id); p != nil {
		if k.String() == "esc" {
			m.a.play(m.id, arcade.SoundBack)
			m.go_(scrMenu)
			return
		}
		mistakes := p.mistakes
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
		if p.mistakes > mistakes {
			m.a.play(m.id, arcade.SoundMistake)
		}
		if changed {
			redraw()
		}
		return
	}
	if r.phase == countdown && r.racer(m.id) != nil && (k.Type == tea.KeyRunes || k.Type == tea.KeySpace) {
		if r.Jump(m.id) {
			m.a.play(m.id, arcade.SoundRed)
			redraw()
		}
		return
	}
	switch k.String() {
	case "enter":
		if m.scr == scrPractice {
			m.a.startPractice(m.ch, m.id, m.name)
		} else {
			m.a.join(m.ch, m.id, m.name)
		}
	case "left", "right":
		m.cycleWatch(r, k.String() == "right")
	case "esc":
		m.a.play(m.id, arcade.SoundBack)
		m.go_(scrMenu)
	}
}

func (m *model) openGarage() {
	m.gRow = 0
	m.gModel = 0
	cur := m.a.records.car(m.id).Model
	for i, c := range cars {
		if c.ID == cur {
			m.gModel = i
		}
	}
	m.go_(scrGarage)
}

func (m *model) garageKey(s string) {
	rec := m.rec()
	car := m.a.records.car(m.id)
	d := map[string]int{"left": -1, "h": -1, "right": 1, "l": 1}[s]
	switch {
	case s == "up" || s == "k":
		m.gRow = (m.gRow + 2) % 3
	case s == "down" || s == "j":
		m.gRow = (m.gRow + 1) % 3
	case d != 0 && m.gRow == 0:
		m.gModel = (m.gModel + len(cars) + d) % len(cars)
	case d != 0 && m.gRow == 1:
		i := 0
		for j, p := range arcade.Paints {
			if p == car.Paint {
				i = j
			}
		}
		car.Paint = arcade.Paints[(i+len(arcade.Paints)+d)%len(arcade.Paints)]
		rec.Car = car
		m.a.records.save()
	case d != 0 && m.gRow == 2:
		car.Number = (car.Number-1+d+99)%99 + 1
		rec.Car = car
		m.a.records.save()
	case s == "enter" && m.gRow == 0:
		c := cars[m.gModel]
		switch {
		case rec.Rewards.Owns(c.ID, carItems):
			car.Model = c.ID
			rec.Car = car
			m.a.records.save()
			m.a.play(m.id, arcade.SoundSelect)
			m.a.changed(m.ch) // your lane changes too
			return
		case rec.Rewards.Passes > 0:
			rec.Rewards.Deal(carItems, m.a.rnd)
			m.a.records.save()
			m.draftSel = 1
			m.a.play(m.id, arcade.SoundSelect)
			m.go_(scrDraft)
			return
		}
		m.a.play(m.id, arcade.SoundMistake)
		return
	case s == "enter":
		m.a.play(m.id, arcade.SoundSelect)
		m.go_(scrMenu)
		return
	case s == "c":
		m.a.play(m.id, arcade.SoundSelect)
		m.go_(scrShowroom)
		return
	case s == "esc":
		m.a.play(m.id, arcade.SoundBack)
		m.go_(scrMenu)
		return
	default:
		return
	}
	m.a.play(m.id, arcade.SoundBlip)
}

func (m *model) draftKey(s string) {
	if !m.revealAt.IsZero() {
		return
	}
	rec := m.rec()
	offer := rec.Rewards.Deal(carItems, m.a.rnd)
	switch s {
	case "left", "h":
		m.draftSel = (m.draftSel + len(offer) - 1) % max(1, len(offer))
		m.a.play(m.id, arcade.SoundBlip)
	case "right", "l":
		m.draftSel = (m.draftSel + 1) % max(1, len(offer))
		m.a.play(m.id, arcade.SoundBlip)
	case "enter":
		if m.draftSel >= len(offer) {
			return
		}
		id := offer[m.draftSel]
		m.revealOffer = append([]string(nil), offer...)
		if !rec.Rewards.Pick(id) {
			return
		}
		car := m.a.records.car(m.id)
		car.Model = id
		rec.Car = car
		m.a.records.save()
		for i, c := range cars {
			if c.ID == id {
				m.gModel = i
			}
		}
		m.revealID, m.revealAt = id, m.a.now()
		m.a.play(m.id, arcade.SoundRecord)
		m.a.startTicker()
		if m.a.sendRecord != nil {
			m.a.sendRecord(m.id)
		}
	case "esc":
		m.a.play(m.id, arcade.SoundBack)
		m.go_(scrGarage)
	}
}

// optionRows are the options screen's rows.
var optionRows = []string{"SOUND", "VOLUME", "EFFECTS", "COUNTDOWN BEEPS", "BACK"}

func (m *model) optionsKey(s string) {
	rec := m.rec()
	o := &rec.Options
	d := map[string]int{"left": -1, "h": -1, "right": 1, "l": 1, "enter": 1}[s]
	switch {
	case s == "up" || s == "k":
		m.op = (m.op + len(optionRows) - 1) % len(optionRows)
	case s == "down" || s == "j":
		m.op = (m.op + 1) % len(optionRows)
	case s == "esc" || (s == "enter" && optionRows[m.op] == "BACK"):
		m.a.play(m.id, arcade.SoundBack)
		m.go_(scrMenu)
		return
	case d != 0:
		switch optionRows[m.op] {
		case "SOUND":
			o.Mute = !o.Mute
		case "VOLUME":
			o.Volume = max(1, min(10, o.volume()+d))
		case "EFFECTS":
			o.Effects = (o.Effects + 3 + d) % 3
		case "COUNTDOWN BEEPS":
			o.NoBeeps = !o.NoBeeps
		}
		m.a.records.save()
	default:
		return
	}
	m.a.play(m.id, arcade.SoundBlip)
}

// View draws the arcade screens, or the plain layout in a small pane.
func (m *model) View() string {
	w, h := m.w, m.h
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	if w < 64 || h < 24 {
		return m.smallView(w, h)
	}
	var theme *wire.PaneTheme
	if m.v != nil {
		theme = m.v.Theme
	}
	c := arcade.New(w, h, arcade.NewPalette(theme))
	f := frame{c: c, ox: max(0, (w-80)/2), cw: min(w, 80), h: h}
	switch m.scr {
	case scrTitle:
		m.drawTitle(f)
	case scrMenu:
		m.drawMenu(f)
	case scrRace, scrPractice:
		m.drawRace(f)
	case scrGarage:
		m.drawGarage(f)
	case scrDraft:
		m.drawDraft(f)
	case scrShowroom:
		m.drawShowroom(f)
	case scrHOF:
		m.drawHOF(f)
	case scrHowTo:
		m.drawHowTo(f)
	case scrOptions:
		m.drawOptions(f)
	}
	return c.String()
}

// smallView is for panes under 64 x 24: the races use the plain layout,
// and the menus become plain lists.
func (m *model) smallView(w, h int) string {
	if m.scr == scrRace || m.scr == scrPractice {
		return m.compactView()
	}
	st := m.styles()
	lines := []string{" " + st.title.Render("🍇 Grape Race"), ""}
	add := func(s string) { lines = append(lines, s) }
	pick := func(i, sel int, s string) {
		if i == sel {
			add(" " + st.title.Render("▶ ") + st.warn.Render(s))
		} else {
			add("   " + st.fg.Render(s))
		}
	}
	car := m.a.records.car(m.id)
	rec := m.rec()
	switch m.scr {
	case scrTitle:
		add(" " + st.fg.Render("A typing race. Press Enter."))
	case scrMenu:
		for i, it := range m.menuItems() {
			pick(i, m.menu, it)
		}
	case scrGarage:
		c := cars[m.gModel]
		name := c.Name
		if !rec.Rewards.Owns(c.ID, carItems) {
			name = "? ? ? (" + c.Tier.Name() + ", locked)"
		}
		pick(0, m.gRow, "MODEL  ◂ "+name+" ▸")
		pick(1, m.gRow, "PAINT  ◂ "+arcade.PaintNames[car.Paint]+" ▸")
		pick(2, m.gRow, fmt.Sprintf("NUMBER ◂ %02d ▸", car.Number))
		add("")
		add(" " + st.dim.Render(fmt.Sprintf("%d of %d cars · %d Pit Passes", rec.Rewards.Count(carItems), len(carItems), rec.Rewards.Passes)))
	case scrDraft:
		if !m.revealAt.IsZero() {
			add(" " + st.good.Render("UNLOCKED: "+carByID[m.revealID].Name))
			break
		}
		for i, id := range rec.Rewards.Deal(carItems, m.a.rnd) {
			c := carByID[id]
			pick(i, m.draftSel, c.Tier.Stars()+" "+c.Name)
		}
	case scrOptions:
		for i, row := range optionRows {
			pick(i, m.op, row+"  "+m.optionValue(row))
		}
	default:
		add(" " + st.dim.Render("Make the pane bigger (64 x 24) for this screen."))
	}
	lines = append(lines, "", st.dim.Render(" ↑↓ ←→ Enter · Esc back"))
	return fill(pane.Fit(strings.Join(lines, "\n"), w, h), w)
}

func (m *model) optionValue(row string) string {
	o := m.rec().Options
	switch row {
	case "SOUND":
		if o.Mute {
			return "◂ OFF ▸"
		}
		return "◂ ON ▸"
	case "VOLUME":
		return strings.Repeat("▮", o.volume()) + strings.Repeat("▯", 10-o.volume()) + fmt.Sprintf("  %d%%", o.volume()*10)
	case "EFFECTS":
		return [...]string{"◂ FULL ▸", "◂ CALM ▸", "◂ OFF ▸"}[o.Effects]
	case "COUNTDOWN BEEPS":
		if o.NoBeeps {
			return "[ ]"
		}
		return "[■]"
	}
	return ""
}
