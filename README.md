# 🍇 Grape Race

An arcade typing race for [Concord](https://github.com/JMThomas00/Concord).
Everyone in the channel races the same passage on a drag strip, cars driven
by grapes, with a garage full of cars to unlock. Run on its own, it's typing
practice in your terminal. It's the first game built on the Concord Arcade
standard (`sdk/arcade`): title screen, menu, pixel art and chiptune sounds.

```
1 BOB      │                         ▄▀▄▄▀▀▀▄▄▀▄                        ▞▚
   48 WPM  │                ─  ─  ━   ▀▀▀▀▀▀▀▀▀                         ▚▞
            ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
2 JORDAN   │          ✶      ▄█▀▄    SPIN!                              ▞▚
   24 WPM  │   ─  ─  ━   ▀▀▀▀▀▀▀▀▀▀▀✶                                   ▚▞

╭─ SPEED ──────────────────╮ ╭─ PIT BOARD ─────────────────────────────────────╮
│ █▀▀█ ▀▀▀█ █▀▀█  WPM      │ │ POSITION   2 OF 3                               │
│ █▄▄█ ▀▄▄█ █▄▄█  94% ACC  │ │ GAP        30 CHARS BEHIND BOB                  │
│ █  █ █  █ █  █           │ │ STREAK     0 CLEAN KEYS                         │
│ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀  ▮▮▯▯▯▯▯▯▯│ │ PASSAGE    30/112                               │
╰──────────────────────────╯ ╰─────────────────────────────────────────────────╯
```

## How to play

- **The menu:** RACE, PRACTICE (a solo run that's yours alone), GARAGE,
  HALL OF FAME, HOW TO PLAY and OPTIONS. ↑↓ and Enter; Esc goes back.
- **RACE** starts a race, or joins the one staging. The starting line is a
  drag-strip tree: pre-stage, stage, three ambers with a beep each, then
  green. The passage is shown beside it, so read ahead.
- **Red light:** typing before green adds 1 second to your time.
- Type the passage. Typed text turns bright, the next character is marked,
  and **mistakes turn red** (and spin your car out): fix them with
  **Backspace** (**Ctrl+W** deletes a word) before you can finish.
- Speed is in words per minute (five characters to a word), timed from
  green. The first clean finish wins. The pit board shows your position,
  the gap to the car ahead and your clean-key streak.
- **Results:** a chequered flag, the podium, new records, and any Pit
  Passes you earned. **Enter** goes again.

## The garage and Pit Passes

Everyone starts with three cars: Go-Kart, Stock Car and Hatchback. Fifteen
more are earned, from the F1 Racer and Monster Truck to the legendary
Wine Barrel, Shopping Cart, Bathtub, Couch and Giant Grape.

- A **Pit Pass** comes with each new achievement, every 3 wins in a row and
  every 10 races finished.
- In the **GARAGE**, Enter on a locked car spends a pass: you're offered
  three locked cars (rarer ones less often) and keep one. The offer is
  saved until you choose, so leaving doesn't re-roll it.
- Pick your car's paint (seven colours from your Concord theme) and its
  number. Everyone racing sees your car, paint and number on your lane.
- **C** in the garage opens the showroom: all 18 cars, locked ones as
  silhouettes.

**Spectating:** if you're in the channel but not racing, you watch one racer
type live, mistakes and all (the leader, to start with). **←/→** switches
to someone else. **Enter** puts you in the next race.

**A race always ends.** Nobody can hold the channel by walking away:

| Rule | What happens |
| --- | --- |
| Time limit | Short passages get 1 minute, medium 2, long 3 (counting down in the corner). |
| Finish timer | Once someone finishes, everyone else has 15 seconds (a channel setting) to finish too. It counts down at the bottom. |
| Idle | No keys for 20 seconds and you're out. At 10 seconds you're asked if you're still there. |

Anyone who doesn't finish shows as **DNF** with how far they got and their
speed until then; it doesn't count toward records. People who pressed
**Enter** during the race start the next one automatically, 5 seconds
after the results appear.

Pasting doesn't type anything, and a finish faster than 250 WPM shows in the
results but never counts toward records.

## Use it on a Concord server

You need to be the server owner, or have the **Manage Plugins** permission.

1. In Concord, open **Server Settings → Plugins** and press **I** (install).
2. Type `JMThomas00/concord-grape-race` and press Enter. Concord downloads the
   latest release for the server's own system, verifies it, and starts it:
   no restart, no files to edit.
3. Open **Server Settings → Channels**, create a channel, and choose
   **Grape Race** as its type. Under **Configure…** you can set:
   - **Passage length:** short (a sentence or two), medium, long (a full
     paragraph) or mixed;
   - **Countdown:** 3, 5, 10 or 15 seconds for others to join;
   - **Finish timer:** 10, 15, 30 or 60 seconds for everyone else once
     someone finishes.
4. Select the channel, then press **Tab** (or click the pane) so your keys go
   to it. **Esc** or **Ctrl+]** gives the keyboard back to Concord.

**Records:** each member's best and average WPM, races and wins appear in
**Settings → About → A** (Achievements), with the server's leaderboard
ranked by best WPM. There are achievements to collect along the way, from
First Race to Grand Cru.

To update later: select it in **Server Settings → Plugins**, press **U**, then
Enter. A failed update rolls back by itself.

## Run it on your own computer

**Download:** from this repo's [Releases](https://github.com/JMThomas00/concord-grape-race/releases),
get the zip for your system (`concord-grape-race_windows_amd64.zip`,
`concord-grape-race_darwin_arm64.zip` for Apple silicon, `concord-grape-race_linux_amd64.zip`, ...),
unzip it, and run the program inside from a terminal:

```sh
./concord-grape-race          # Windows: .\concord-grape-race.exe
```

On macOS, if it's blocked as being from an unidentified developer, run
`xattr -d com.apple.quarantine concord-grape-race` once.

**Or with Go installed:**

```sh
go install github.com/JMThomas00/concord-grape-race@latest
concord-grape-race
```

Standalone, it's solo practice with a 3-second countdown. Your best is kept
in your config folder (`concord-grape-race/records.json`). **Esc** quits.

## Develop

```sh
go run .              # run it standalone
go test ./...         # tests, including a fake Concord server (sdk/plugintest)
go run release.go     # the release zips, in dist/
```

Release a version by pushing a tag: `git tag v0.1.0 && git push --tags`.
`.github/workflows/release.yml` then attaches
`dist/concord-grape-race_<os>_<arch>.zip` for every platform to the GitHub release,
which is what Concord installs.

- `plugin.toml`: the channel type, its options, the achievements and the
  leaderboard.
- `race.go`: one race: joining, the countdown, red lights, typing, mistakes, WPM.
- `app.go`: the races per channel, practice races, and the clock that runs
  the lights, the race and the title screen's attract mode.
- `model.go`: each viewer's screens and keys (a Bubble Tea model per viewer).
- `screens.go`: the arcade screens, drawn with `sdk/arcade`.
- `compact.go`: the plain layout for panes smaller than 64 x 24.
- `cars.go`: the 18 cars, 12 x 4 pixels each.
- `records.go`: bests, achievements, the garage and Pit Passes, and each
  player's options, saved in the plugin's data folder.
- `client/sounds/`: the arcade sound kit. `go generate` writes it
  (`tools/gensounds`).
- `passages.txt`: the passages, all original. Add your own: plain ASCII,
  one blank line between passages.

## License

MIT
