# gapwatch

Make silence visible in streaming log output.

## Example

```
tail -f /var/log/syslog | gapwatch
```

```
Feb 26 10:00:00 host app[1234]: request handled in 42ms
Feb 26 10:00:01 host app[1234]: request handled in 38ms
.
.
.
Feb 26 10:01:00 host app[1234]: connection timeout to upstream
```

The dots show you where nothing happened for a while.

## Why?

When you're tailing a log and nothing happens for 60 seconds, then a burst of errors appears — it's hard to tell if that gap was 5 seconds or 5 minutes. `gapwatch` makes that visible.

## Install

```bash
go install github.com/skinnybinder/gapwatch/cmd/gapwatch@latest
```

Or grab a binary from [Releases](https://github.com/skinnybinder/gapwatch/releases).

## Usage

```bash
# Default — dot after each second of silence
tail -f /var/log/syslog | gapwatch

# Skip gap detection for the first 10 seconds (let the service warm up)
kubectl logs -f deploy/myapp | gapwatch --start-delay 10s

# Custom marker
journalctl -f | gapwatch --marker '---'

# Timestamp each marker so you know exactly when silence hit
tail -f /var/log/syslog | gapwatch --timestamp

# One marker every 5 ticks instead of every tick
journalctl -f | gapwatch --fold 5

# Stop after 10 markers total — log lines keep passing through
tail -f access.log | gapwatch --max 10

# Combine
docker logs -f mycontainer | gapwatch --start-delay 5s --marker '  ⏳' --fold 3 --max 10
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--interval`, `--gap` | `1s` | How long silence must last before a marker appears, and how often it repeats during a gap. Takes `ms`, `s`, `m`, `h` (e.g. `500ms`, `2m`) |
| `--marker` | `.` | What to print during a gap |
| `--start-delay`, `--delay-start` | `0` | Ignore silence for this long at startup. Same duration units as `--interval` |
| `--fold` | `0` | Print one marker every N ticks — suppresses the rest. `0` means every tick |
| `--max` | `0` | Stop printing markers after N total. Log lines still pass through. `0` means unlimited |
| `--timestamp` | off | Prefix each marker with the current time (`10:00:01`) |
| `--version` | | Print version and exit |

`--max` is a session total — it does not reset between gaps. `--fold` resets each time a log line arrives.

## License

MIT — see [LICENSE](LICENSE) for details.

## Contributing

Issues and PRs welcome.
