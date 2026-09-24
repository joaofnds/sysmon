# sysmon

A local monitor for this Mac. [mactop](https://github.com/metaspartan/mactop) samples CPU,
GPU, power, temperatures, memory, network, disk and battery, and exposes them as Prometheus
metrics. `sysmon-procs`, a small collector in `procs/`, measures the same resources for each
app. `claude-limits`, in `claude-limits/`, reads how much of each usage limit of the Claude
plan is used. Single-node [VictoriaMetrics](https://victoriametrics.com) scrapes and stores
all three, and Grafana charts them.

Nix provides all four programs, pinned by `flake.lock`. None is installed system-wide, and
they run only between `bin/start` and `bin/stop`.

Beside them, a Docker Compose stack records Claude Code's telemetry in ClickHouse and charts
it in Grafana. That stack does start on its own, with OrbStack. See Claude telemetry below.

## Use

    bin/start     # starts mactop and sysmon-procs, waits for their first samples,
                  # then claude-limits and VictoriaMetrics
    bin/status    # running or not, PIDs, CPU% and RSS, the health of each scrape target,
                  # data size, and the state of each Claude telemetry container
    bin/stop      # stops all four and removes their PID files

Dashboard: http://localhost:3030/d/mac-system. It is served by the Grafana of the Claude
telemetry stack below, which reads VictoriaMetrics through `host.docker.internal:8428`, so
it shows data only while `bin/start` is running. Hover any chart and every other chart
marks the same moment, which lines up a load spike with the power, heat and fan speed it
caused. For ad hoc queries, VictoriaMetrics has its own UI at http://127.0.0.1:8428/vmui.

The top of the dashboard is the whole machine. Below it, Top apps now ranks apps by CPU,
memory, network, disk, GPU and energy over the last minute, Apps over time stacks the eight
biggest apps of each against mactop's whole-machine line, and Inside $app splits the app
picked at the top into its processes. Its table lists each process with its CPU, resident
memory, footprint, network, disk, GPU, power, idle wakeups, threads, open files and
sockets, and the charts below it follow each of those per process over time, with
page-ins. Footprint is the memory figure Activity Monitor shows. A blank cell has no
reading behind it. The app picker starts on the app using the most memory.

`grafana/mac-system.py` writes this dashboard's JSON, so change the script rather than the
JSON, then run it:

    nix shell nixpkgs#python3 -c python3 grafana/mac-system.py

Grafana picks up the rewritten file on its own.

An app is the outermost `.app` bundle a process runs from, so Brave's helpers count as
Brave Browser. Processes outside any bundle group by executable name. Its limits:

- WebKit's shared services (`com.apple.WebKit.WebContent`, `com.apple.WebKit.GPU`) sit
  outside Safari's and Mail's bundles, so they show under their own names.
- Footprint, disk, energy, idle wakeups, page-ins, threads, open files and sockets cover
  only this user's processes. macOS does not report them for processes of other users
  without root, so a root process such as WindowServer shows only CPU, resident memory
  and GPU.
- Network counts external interfaces only, as mactop does, so traffic between local
  processes, VictoriaMetrics scraping included, is left out. A process that has held no
  external connection since `bin/start` has no network reading.
- Byte rates in the Top apps now and Processes now tables round to whole bytes per
  second, because Grafana would otherwise show a fraction of a byte as millibytes.
- Memory is resident memory, which counts shared pages once for each process that maps
  them, so the apps add up to more than the machine's used memory.
- The kernel's own CPU time belongs to no app, which is most of the gap between the apps
  and mactop's whole-machine CPU line.

mactop 2.1.5 reads CPU, DRAM and Neural Engine power and DRAM bandwidth as zero on this
M5 Pro, so the dashboard charts only whole-machine and GPU power, and leaves out the
samples where those readings spike and the occasional impossible network sample.

All four listen on 127.0.0.1 only: mactop on port 2112, sysmon-procs on 2113,
claude-limits on 2114, VictoriaMetrics on 8428.

## Claude plan limits

`claude-limits` reads how much of the Claude plan's session limit, weekly limit and
per-model weekly limits is used, and when each resets. It asks every five minutes, from the
same endpoint Claude Code's `/usage` reads, `https://api.anthropic.com/api/oauth/usage`.
Anthropic does not document that endpoint, so it can change or go away without notice.

The Plan limits row of the Claude usage dashboard, http://localhost:3030/d/claude-usage,
charts them. It reads VictoriaMetrics, so that row is blank while `bin/start` is not
running. Its even pace line is how much of a limit would be used by now if the whole limit
were spread evenly up to the reset, so use above that line runs out before the reset.

It signs in with the Claude.ai login that Claude Code keeps in the Keychain item
`Claude Code-credentials`, read again before every request, and it never refreshes that login.
When a read fails, for example because the login expired and Claude Code has not refreshed
it yet, `/metrics` answers 503 until the next read succeeds. VictoriaMetrics then marks the
target down, `bin/status` shows it, the charts leave a gap rather than repeat an old
reading, and `logs/claude-limits.log` says why.

## Claude telemetry

A Docker Compose stack stores Claude Code's OpenTelemetry events in ClickHouse and charts
them in Grafana, so token spend can be broken down by model, subagent, skill, MCP server,
repository, session, prompt, and tool. Claude Code exports to it through the `OTEL_*`
entries in `~/.claude/settings.json`. chezmoi renders that file from
`dot_claude/private_settings.json` in the dotfiles repository, so change them there.

Unlike mactop and VictoriaMetrics, it runs in OrbStack and restarts with it. Start it once:

    docker compose -f ~/code/sysmon/compose.yaml up -d

`bin/status` reports its containers too.

Rerun it with `--force-recreate` after editing any of its files. The services read their
files only when they start, `up -d` alone leaves a running container as it is, and the
`schema` service reapplies `schema.sql` on every start.

The collected events live in the Docker volumes `claude-telemetry_clickhouse` and
`claude-telemetry_grafana`. `compose.yaml` names them outright, so renaming the Compose
project or moving this folder keeps them.

`clickhouse.xml` turns off ClickHouse's own system log tables, which otherwise double its
idle CPU and memory. It lists every log section active in the pinned image's `config.xml`
except `crash_log`, which is written only on a fatal error, so recheck it when bumping the
image.

- Dashboard: http://localhost:3030
- SQL: `docker compose -f ~/code/sysmon/compose.yaml exec clickhouse clickhouse-client -u otel --password otel -d otel`

`schema.sql` defines the views to query (`api_requests`, `prompts`, `tool_results`,
`subagent_runs`, `session_first_prompts`) and keeps 90 days of events. To change that, edit
`INTERVAL 90 DAY` on its first line and rerun with `--force-recreate`. `api_requests`
splits each request's cost into input, cache reads, cache writes and output using the
per-model prices in `model_prices`. A request whose model is missing from that list, or
that ran at a speed other than normal, shows all its spend as "Other" on the dashboard.

ClickHouse publishes no port to the host, because its HTTP interface answers any web page;
reach it through Grafana or `docker compose exec`. Grafana connects as the `grafana` user
from `clickhouse-users.xml`, which may only select from the `otel` database. A Grafana link
runs its query when opened, so that user must not gain writes or `url()`, `file()`,
`remote()` or `s3()` access. Anonymous visitors are Viewers, so a link can neither open
Explore nor add a datasource that logs in as `otel`; run ad hoc queries with the SQL command
above.

Grafana and the collector publish their ports on 127.0.0.1 only. The collector listens on
4327 rather than 4317 so it does not collide with an application's own OpenTelemetry
collector.

## Where things live

| Path | Contents |
|---|---|
| `data/` | VictoriaMetrics storage |
| `logs/` | stderr of mactop, sysmon-procs, claude-limits and VictoriaMetrics |
| `run/` | PID files |
| `result-mactop`, `result-sysmon-procs`, `result-claude-limits`, `result-victoriametrics` | Links to the Nix store paths `bin/start` runs |
| `procs/` | Source of `sysmon-procs`, the per-app collector |
| `claude-limits/` | Source of `claude-limits`, the Claude plan limits collector |
| `scrape.yml` | Scrape configuration |
| `flake.nix`, `flake.lock` | The pinned packages |
| `compose.yaml` | The Claude telemetry stack |
| `otelcol.yaml` | OpenTelemetry collector pipeline into ClickHouse |
| `clickhouse.xml`, `clickhouse-users.xml` | ClickHouse server settings and the read-only `grafana` user |
| `schema.sql` | Claude telemetry retention and query views |
| `grafana/` | Grafana datasources for ClickHouse and VictoriaMetrics, dashboard provisioning, and the Claude usage and Mac system dashboards |
| `grafana/mac-system.py` | The script that writes the Mac system dashboard |

The `result-*` links are Nix GC roots: `nix store gc` keeps the packages while the links
exist.

mactop comes from nixpkgs with one patch in `flake.nix`. Upstream mactop listens on every
network interface and has no option to change that, so the patch binds its metrics server
to 127.0.0.1.

## Change settings

Retention: edit `RETENTION` at the top of `bin/start` (for example `30d`, `1y`), then
`bin/stop && bin/start`. VictoriaMetrics deletes data older than the new period.

Scrape interval: edit `scrape_interval` in `scrape.yml`, in whole seconds (for example
`30s`), then `bin/stop && bin/start`. `bin/start` sets mactop's sampling interval from the
same value, so neither collector samples faster than it is scraped. mactop needs about two
intervals to produce its first sample, so a longer interval makes `bin/start` wait longer.

## Uninstall

    bin/uninstall

It asks for confirmation, stops mactop, sysmon-procs, claude-limits and VictoriaMetrics,
removes the Claude telemetry containers, and prints the commands that finish the job,
without running them:

    rm -rf ~/code/sysmon
    nix store gc
    docker volume rm claude-telemetry_clickhouse claude-telemetry_grafana

The last one deletes every collected Claude telemetry event, so skip it to keep them.

`nix store gc` is optional. It frees the store paths sysmon used, and also anything else on
this machine that no GC root holds.
