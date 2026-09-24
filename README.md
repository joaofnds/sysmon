# sysmon

A local monitor for this Mac. [mactop](https://github.com/metaspartan/mactop) samples CPU,
GPU, power, temperatures, memory, network, disk and battery, and exposes them as Prometheus
metrics. Single-node [VictoriaMetrics](https://victoriametrics.com) scrapes them, stores
them, and serves the UI.

Nix provides both programs, pinned by `flake.lock`. Neither is installed system-wide, and
both run only between `bin/start` and `bin/stop`.

Beside them, a Docker Compose stack records Claude Code's telemetry in ClickHouse and charts
it in Grafana. That stack does start on its own, with OrbStack. See Claude telemetry below.

## Use

    bin/start     # starts mactop, waits for its first sample, then VictoriaMetrics
    bin/status    # running or not, PIDs, CPU% and RSS, scrape target health, data size
    bin/stop      # stops both and removes their PID files

UI: http://127.0.0.1:8428/vmui, with the prepared dashboard under the Dashboards tab.

Both listen on 127.0.0.1 only: mactop on port 2112, VictoriaMetrics on 8428.

## Claude telemetry

A Docker Compose stack stores Claude Code's OpenTelemetry events in ClickHouse and charts
them in Grafana, so token spend can be broken down by model, subagent, skill, MCP server,
repository, session, prompt, and tool. Claude Code exports to it through the `OTEL_*`
entries in `~/.claude/settings.json`. chezmoi renders that file from
`dot_claude/private_settings.json` in the dotfiles repository, so change them there.

Unlike mactop and VictoriaMetrics, it runs in OrbStack and restarts with it. Start it once:

    docker compose -f ~/code/sysmon/compose.yaml up -d

Check it with `docker compose -f ~/code/sysmon/compose.yaml ps`, since `bin/status` covers
only mactop and VictoriaMetrics.

Rerun it with `--force-recreate` after editing any of its files. The services read their
files only when they start, `up -d` alone leaves a running container as it is, and the
`schema` service reapplies `schema.sql` on every start.

The collected events live in the Docker volumes `claude-telemetry_clickhouse` and
`claude-telemetry_grafana`, named after the Compose project, not this folder.

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
| `logs/` | stderr of mactop and VictoriaMetrics |
| `run/` | PID files |
| `result-mactop`, `result-victoriametrics` | Links to the Nix store paths `bin/start` runs |
| `dashboards/mactop.json` | The vmui dashboard |
| `scrape.yml` | Scrape configuration |
| `flake.nix`, `flake.lock` | The pinned packages |
| `compose.yaml` | The Claude telemetry stack |
| `otelcol.yaml` | OpenTelemetry collector pipeline into ClickHouse |
| `clickhouse.xml`, `clickhouse-users.xml` | ClickHouse server settings and the read-only `grafana` user |
| `schema.sql` | Claude telemetry retention and query views |
| `grafana/` | Grafana datasource, dashboard provisioning, and the Claude usage dashboard |

The two `result-*` links are Nix GC roots: `nix store gc` keeps both packages while the
links exist.

mactop comes from nixpkgs with one patch in `flake.nix`. Upstream mactop listens on every
network interface and has no option to change that, so the patch binds its metrics server
to 127.0.0.1.

## Change settings

Retention: edit `RETENTION` at the top of `bin/start` (for example `30d`, `1y`), then
`bin/stop && bin/start`. VictoriaMetrics deletes data older than the new period.

Scrape interval: edit `scrape_interval` in `scrape.yml`, in whole seconds (for example
`30s`), then `bin/stop && bin/start`. `bin/start` sets mactop's sampling interval from the
same value, so mactop never samples faster than it is scraped. mactop needs about two
intervals to produce its first sample, so a longer interval makes `bin/start` wait longer.

## Uninstall

    bin/uninstall

It asks for confirmation, stops mactop and VictoriaMetrics, and prints the commands that
finish the job, without running them:

    rm -rf ~/code/sysmon
    nix store gc

It leaves the Claude telemetry stack running on this folder's files, so stop that stack
before the `rm` with `docker compose -f ~/code/sysmon/compose.yaml down`. Adding `-v` to it
also deletes every collected event.

`nix store gc` is optional. It frees the store paths sysmon used, and also anything else on
this machine that no GC root holds.
