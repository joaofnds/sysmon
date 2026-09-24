# sysmon

A local, self-contained monitor for this Mac. [mactop](https://github.com/metaspartan/mactop)
samples CPU, GPU, power, temperatures, memory, network, disk and battery, and exposes them
as Prometheus metrics. Single-node [VictoriaMetrics](https://victoriametrics.com) scrapes
them, stores them, and serves the UI.

Nix provides both programs, pinned by `flake.lock`. Nothing is installed system-wide and
nothing starts on its own.

## Use

    bin/start     # starts mactop, waits for its first sample, then VictoriaMetrics
    bin/status    # running or not, PIDs, CPU% and RSS, scrape target health, data size
    bin/stop      # stops both and removes their PID files

UI: http://127.0.0.1:8428/vmui, with the prepared dashboard under the Dashboards tab.

Both listen on 127.0.0.1 only: mactop on port 2112, VictoriaMetrics on 8428.

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

It asks for confirmation, stops everything, and prints the commands that finish the job,
without running them:

    rm -rf ~/code/sysmon
    nix store gc

`nix store gc` is optional. It frees the store paths sysmon used, and also anything else on
this machine that no GC root holds.
