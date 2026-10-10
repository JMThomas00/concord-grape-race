# 🍇 Grape Race

A typing race for [Concord](https://github.com/JMThomas00/Concord). Everyone
in the channel types the same passage at once, and each person's grape rolls
down its vine as they go. Run on its own, it's typing practice in your
terminal.

```
 🍇 Grape Race                                           race 4 · 0:20

   bob    ━━━━━━━━━━━━━━━━━━━━━━━━🍇┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  33 wpm
 ▶ jordan ━━━━━━━━━━━━━🍇┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄  18 wpm

 GO!  18 wpm · 94% accuracy · 30/112

 He tuned the old radio until the static gave way to a voice reading
 the shipping forecast, calm as a lighthouse.
```

## How to play

- **Enter** starts a race. A countdown begins, and everyone viewing the
  channel who presses **Enter** before GO joins in.
- Type the passage. Typed text turns bright, the next character is
  underlined, and **mistakes turn red**: fix them with **Backspace**
  (**Ctrl+W** deletes a word) before you can finish.
- Speed is in words per minute (five characters to a word), timed from GO.
  The first clean finish wins.
- After the race: results, new personal bests, and the server's fastest
  typists. **Enter** starts the next one.

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
- `race.go`: one race: joining, the countdown, typing, mistakes, WPM.
- `app.go`: the races per channel and the clock that keeps live ones moving.
- `model.go`: what each viewer sees (a Bubble Tea model per viewer).
- `records.go`: personal bests and achievements, saved in the plugin's data folder.
- `passages.txt`: the passages, all original. Add your own: plain ASCII,
  one blank line between passages.

## License

MIT
