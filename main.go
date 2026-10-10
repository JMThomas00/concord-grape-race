// Grape Stomp: a typing race. In a Concord channel, everyone viewing races
// the same passage live, each grape rolling down its vine as they type. Run
// on its own, it's solo practice in your terminal.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/JMThomas00/Concord/sdk/pane"
	"github.com/JMThomas00/Concord/sdk/plugin"
	"github.com/JMThomas00/Concord/sdk/wire"
)

func main() {
	cfg, underConcord := plugin.ConfigFromEnv()
	if !underConcord {
		standalone()
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := plugin.Run(ctx, cfg, handler(newApp(loadRecords(cfg.DataDir), time.Now))); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

// handler wires the app to Concord: a pane per viewer, races per channel.
func handler(a *app) plugin.Handler {
	var host *pane.Host
	host = pane.NewHost(func(v *pane.Viewer) tea.Model {
		return newModel(a, v.ChannelID, v.ID, v.DisplayName, v)
	})
	a.changed = func(ch uuid.UUID) { host.Broadcast(ch, redrawMsg{}) }
	h := host.Handler()
	h.OnReady = func(c *plugin.Conn, _ *wire.User) {
		a.post = c.Post
		a.sendRecord = func(id uuid.UUID) { _ = c.SendRecord(a.records.wire(id)) }
	}
	h.OnChannel = func(_ *plugin.Conn, ch wire.Channel) {
		a.settings[ch.ID] = ch.PluginConfig
	}
	h.OnChannelDelete = func(_ *plugin.Conn, e wire.ChannelDeletePayload) {
		delete(a.races, e.ChannelID)
		delete(a.settings, e.ChannelID)
	}
	return h
}

// standalone is solo practice in this terminal, with your bests kept in
// your config folder.
func standalone() {
	dir := ""
	if d, err := os.UserConfigDir(); err == nil {
		dir = filepath.Join(d, "concord-stomp")
	}
	a := newApp(loadRecords(dir), time.Now)
	a.countdown = 3 * time.Second
	m := newModel(a, uuid.Nil, uuid.MustParse("00000000-0000-0000-0000-000000000001"), "you", nil)
	m.standalone = true
	p := tea.NewProgram(m, tea.WithAltScreen())
	a.post = func(fn func()) { p.Send(runMsg(fn)) }
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
