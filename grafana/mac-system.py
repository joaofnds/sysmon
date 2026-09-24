import json
from pathlib import Path

DS = {"type": "prometheus", "uid": "victoriametrics"}
panels = []


def panel(kind, title, x, y, w, h, targets, unit=None, desc=None, defaults=None, overrides=None, **extra):
    field_defaults = dict(defaults or {})
    if unit:
        field_defaults["unit"] = unit
    p = {
        "id": len(panels) + 1,
        "type": kind,
        "title": title,
        "datasource": DS,
        "gridPos": {"x": x, "y": y, "w": w, "h": h},
        "targets": [{"refId": chr(65 + i), "datasource": DS, "expr": e, "legendFormat": l}
                    for i, (e, l) in enumerate(targets)],
        "fieldConfig": {"defaults": field_defaults, "overrides": overrides or []},
        "options": {},
        **extra,
    }
    if desc:
        p["description"] = desc
    panels.append(p)
    return p


def instant(p, **fields):
    for target in p["targets"]:
        target.update(instant=True, range=False, **fields)


def row(title, y):
    panels.append({"id": len(panels) + 1, "type": "row", "title": title, "collapsed": False,
                   "gridPos": {"x": 0, "y": y, "w": 24, "h": 1}, "panels": []})


def fixed(color):
    return {"mode": "fixed", "fixedColor": color}


def color_of(name, color):
    return {"matcher": {"id": "byName", "options": name},
            "properties": [{"id": "color", "value": fixed(color)}]}


def series(fill=18, stack=False, gradient="opacity", width=2, draw="line"):
    c = {"drawStyle": draw, "lineWidth": width, "fillOpacity": fill, "gradientMode": gradient,
         "showPoints": "never", "spanNulls": 30000, "axisSoftMin": 0,
         "lineInterpolation": "smooth"}
    if stack:
        c["stacking"] = {"mode": "normal"}
    return c


TS_OPTS = {"legend": {"showLegend": True, "displayMode": "list", "placement": "bottom", "calcs": []},
           "tooltip": {"mode": "multi", "sort": "desc"}}
LAST = {"calcs": ["lastNotNull"], "fields": "", "values": False}

PCT = {"min": 0, "max": 100}
LOAD_STEPS = {"mode": "absolute", "steps": [
    {"color": "green", "value": None}, {"color": "yellow", "value": 60},
    {"color": "orange", "value": 80}, {"color": "red", "value": 90}]}
ALL_GREEN = {"mode": "absolute", "steps": [{"color": "green", "value": None}]}
THERMAL_STATES = [{"type": "value", "options": {
    "0": {"text": "Nominal", "color": "green", "index": 0},
    "1": {"text": "Fair", "color": "yellow", "index": 1},
    "2": {"text": "Serious", "color": "orange", "index": 2},
    "3": {"text": "Critical", "color": "red", "index": 3}}}]
DASHED = [{"id": "custom.stacking", "value": {"mode": "none"}}, {"id": "custom.fillOpacity", "value": 0},
          {"id": "color", "value": fixed("text")},
          {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}}]
MIRROR = [{"matcher": {"id": "byRegexp", "options": "Upload|Write"},
           "properties": [{"id": "custom.transform", "value": "negative-Y"}]}]

MEM_PCT = 'mactop_memory_gb{type="used"} / ignoring(type) mactop_memory_gb{type="total"} * 100'
POWER = '(mactop_power_watts{component="%s"} unless on() mactop_power_watts{component="system"} == 0)'
TOTAL_W = POWER % "total"
GPU_W = POWER % "gpu"
NET = '(mactop_network_kbytes_per_sec{direction="%s"} < 1e7) * 1024'
DISK = 'mactop_disk_kbytes_per_sec{operation="%s"} * 1024'
GIB = 'mactop_memory_gb{type="%s"} * 1073741824'
TEMPS = [
    ('max(mactop_temp_sensor_celsius{name=~"CPU.*"})', "CPU"),
    ('max(mactop_temp_sensor_celsius{name=~"GPU.*"})', "GPU"),
    ('max(mactop_temp_sensor_celsius{name=~"SoC Package.*"})', "SoC package"),
    ('max(mactop_temp_sensor_celsius{name=~"Memory.*"})', "Memory"),
    ('max(mactop_temp_sensor_celsius{name=~"NVMe.*|NAND.*|SSD.*"})', "Storage"),
    ('max(mactop_temp_sensor_celsius{name=~"Ambient.*"})', "Ambient"),
]

APP_CPU = "sum by (app) (rate(sysmon_process_cpu_seconds_total[%s])) * 100"
APP_MEM = "sum by (app) (sysmon_process_resident_bytes)"
APP_NET = "sum by (app) (rate(sysmon_process_network_received_bytes_total[%s]) + rate(sysmon_process_network_sent_bytes_total[%s]))"
APP_DISK = "sum by (app) (rate(sysmon_process_disk_read_bytes_total[%s]) + rate(sysmon_process_disk_written_bytes_total[%s]))"
APP_GPU = "sum by (app) (rate(sysmon_process_gpu_seconds_total[%s])) * 100"
APP_POWER = "sum by (app) (rate(sysmon_process_energy_joules_total[%s]))"
NOW = "1m"
OVER = "$__rate_interval"

APP = '{app="$app"}'
RATE = "rate(%s" + APP + "[%s])"
BY_PROCESS = "sum by (process) (%s)"

OWN = " Only processes running as you: macOS does not let other users read the counters of system daemons."
CPU_DESC = "100% is one core fully busy, as in Activity Monitor."
PICK = " Pick the app in the menu at the top."


def gauge(title, x, expr, desc):
    panel("gauge", title, x, 0, 3, 6, [(expr, title)], "percent", desc,
          defaults={**PCT, "decimals": 0, "thresholds": LOAD_STEPS, "color": {"mode": "thresholds"}},
          options={"reduceOptions": LAST, "showThresholdMarkers": False, "showThresholdLabels": False,
                   "minVizWidth": 75, "minVizHeight": 75, "sizing": "auto"})


def spark(title, x, y, w, expr, unit, color, desc, decimals=None):
    d = {"color": fixed(color)}
    if decimals is not None:
        d["decimals"] = decimals
    panel("stat", title, x, y, w, 3, [(expr, title)], unit, desc, defaults=d,
          options={"reduceOptions": LAST, "graphMode": "area", "colorMode": "value", "textMode": "value",
                   "justifyMode": "center", "wideLayout": True})


def top_now(title, x, y, expr, unit, color, desc):
    instant(panel("table", title, x, y, 8, 9, [("sort_desc(topk(10, (%s) > 0))" % expr, "{{app}}")], unit, desc,
                  defaults={"min": 0, "decimals": 1, "color": fixed(color),
                            "custom": {"cellOptions": {"type": "gauge", "mode": "basic", "valueDisplayMode": "text"}}},
                  overrides=[{"matcher": {"id": "byName", "options": "app"},
                              "properties": [{"id": "custom.cellOptions", "value": {"type": "auto"}},
                                             {"id": "custom.width", "value": 190}]}],
                  options={"showHeader": False, "cellHeight": "sm", "sortBy": [{"displayName": "Value", "desc": True}]},
                  transformations=[{"id": "organize", "options": {
                      "excludeByName": {"Time": True}, "indexByName": {"app": 0, "Value": 1}}}]),
            format="table")


def by_app(title, x, y, expr, unit, whole, whole_name, desc):
    targets = [('topk_avg(8, %s, "app=Other")' % expr, "{{app}}")]
    if whole:
        targets.append((whole, whole_name))
    panel("timeseries", title, x, y, 12, 8, targets, unit,
          desc + " Apps outside the 8 busiest over the time range are summed as Other.",
          defaults={"custom": series(fill=45, stack=True, gradient="none", width=1), "min": 0},
          overrides=[{"matcher": {"id": "byName", "options": whole_name}, "properties": DASHED},
                     color_of("Other", "#808080")],
          options=TS_OPTS)


def rate_by_process(metric, window=OVER):
    return BY_PROCESS % (RATE % (metric, window))


def held_by_process(metric):
    return BY_PROCESS % (metric + APP)


def column_unit(name, unit):
    properties = [{"id": "unit", "value": unit}]
    if unit == "none":
        properties.append({"id": "decimals", "value": 0})
    return {"matcher": {"id": "byName", "options": name}, "properties": properties}


def by_process(title, x, y, targets, unit, desc, mirrored=False, w=8, h=8):
    if mirrored:
        defaults = {"custom": series(fill=10, gradient="none", width=1)}
        overrides = [{"matcher": {"id": "byRegexp", "options": ".* (sent|written)$"},
                      "properties": [{"id": "custom.transform", "value": "negative-Y"}]}]
    else:
        defaults = {"custom": series(fill=45, stack=True, gradient="none", width=1), "min": 0}
        overrides = []
    panel("timeseries", title, x, y, w, h, targets, unit, desc + PICK,
          defaults=defaults, overrides=overrides, options=TS_OPTS)


gauge("CPU", 0, "mactop_cpu_usage_percent", "Share of all cores busy, averaged over the scrape interval.")
gauge("GPU", 3, "mactop_gpu_usage_percent", "GPU active residency.")
gauge("Memory", 6, MEM_PCT, "Memory in use as a share of installed memory.")
panel("stat", "Thermal state", 9, 0, 3, 6, [("mactop_thermal_state", "Thermal state")], None,
      "macOS thermal pressure. Only Critical means heat is cutting the system's performance. Fair and Serious ask apps to cut back first.",
      defaults={"mappings": THERMAL_STATES, "color": {"mode": "thresholds"}, "thresholds": ALL_GREEN},
      options={"reduceOptions": LAST, "graphMode": "none", "colorMode": "background", "textMode": "value",
               "justifyMode": "center"})
spark("Power", 12, 0, 4, TOTAL_W, "watt", "orange",
      "Whole-system power draw.", 1)
spark("SoC temperature", 16, 0, 4, "mactop_soc_temp_celsius", "celsius", "red",
      "Average SoC die temperature.", 0)
spark("Swap used", 20, 0, 4, GIB % "swap_used", "bytes", "purple",
      "Memory paged out to disk. Growth here means memory is the bottleneck.", 1)
spark("Download", 12, 3, 3, NET % "download", "Bps", "blue", "All interfaces, received.", 1)
spark("Upload", 15, 3, 3, NET % "upload", "Bps", "green", "All interfaces, sent.", 1)
spark("Disk read", 18, 3, 3, DISK % "read", "Bps", "yellow", "All disks.", 1)
spark("Disk write", 21, 3, 3, DISK % "write", "Bps", "orange", "All disks.", 1)

row("Where the pressure is", 6)
instant(panel("bargauge", "Utilization by resource", 0, 7, 8, 8, [
    ("mactop_pcore_usage_percent", "Performance cores"),
    ("mactop_score_usage_percent", "Super cores"),
    ("mactop_gpu_usage_percent", "GPU"),
    (MEM_PCT, "Memory"),
    ('mactop_memory_gb{type="swap_used"} / ignoring(type) mactop_memory_gb{type="swap_total"} * 100', "Swap"),
    ("mactop_thermal_state / 3 * 100", "Thermal headroom used"),
], "percent",
    "The longest bar is the resource closest to its limit. Swap is its share of the swap currently allocated, which macOS grows on demand.",
    defaults={**PCT, "decimals": 0, "thresholds": LOAD_STEPS, "color": {"mode": "thresholds"}},
    options={"orientation": "horizontal", "displayMode": "gradient", "valueMode": "color",
             "showUnfilled": True, "namePlacement": "left", "sizing": "auto", "reduceOptions": LAST}))
panel("timeseries", "Load, power and heat together", 8, 7, 16, 8, [
    ("mactop_cpu_usage_percent", "CPU"),
    ("mactop_gpu_usage_percent", "GPU"),
    (TOTAL_W, "Power"),
    ("mactop_soc_temp_celsius", "SoC temperature"),
    ('avg(mactop_fan_rpm)', "Fans"),
], "percent",
    "Hover to line the series up in time: a CPU or GPU burst should show as power first, then heat, then fan speed.",
    defaults={"custom": series(fill=10), "min": 0},
    overrides=[
        color_of("CPU", "blue"), color_of("GPU", "purple"),
        {"matcher": {"id": "byName", "options": "Power"},
         "properties": [{"id": "unit", "value": "watt"}, {"id": "color", "value": fixed("orange")},
                        {"id": "custom.axisPlacement", "value": "right"}]},
        {"matcher": {"id": "byName", "options": "SoC temperature"},
         "properties": [{"id": "unit", "value": "celsius"}, {"id": "color", "value": fixed("red")},
                        {"id": "custom.axisPlacement", "value": "right"}, {"id": "custom.fillOpacity", "value": 0}]},
        {"matcher": {"id": "byName", "options": "Fans"},
         "properties": [{"id": "unit", "value": "rotrpm"}, {"id": "color", "value": fixed("text")},
                        {"id": "custom.axisPlacement", "value": "hidden"}, {"id": "custom.fillOpacity", "value": 0},
                        {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}}]},
    ], options=TS_OPTS)

row("Top apps now", 15)
top_now("CPU", 0, 16, APP_CPU % NOW, "percent", "blue", CPU_DESC)
top_now("Memory", 8, 16, APP_MEM, "bytes", "green",
        "Resident memory of each app's processes. Memory shared between processes counts once for each of them.")
top_now("Network", 16, 16, "round(%s)" % (APP_NET % (NOW, NOW)), "Bps", "cyan", "Received plus sent, on external interfaces.")
top_now("Disk", 0, 25, "round(%s)" % (APP_DISK % (NOW, NOW)), "Bps", "yellow", "Read plus written." + OWN)
top_now("GPU", 8, 25, APP_GPU % NOW, "percent", "purple", "Share of GPU time.")
top_now("Energy", 16, 25, APP_POWER % NOW, "watt", "orange", "macOS's own estimate of each app's power draw." + OWN)

row("Apps over time", 35)
by_app("CPU by app", 0, 36, APP_CPU % OVER, "percent",
       "mactop_cpu_usage_percent * scalar(count(mactop_cpu_core_usage_percent))", "All CPU (mactop)",
       CPU_DESC + " The dashed line is all CPU time mactop measured, and the gap above the stack is mostly the kernel.")
by_app("Memory by app", 12, 36, APP_MEM, "bytes", None, None,
       "Resident memory. Memory shared between processes counts once for each of them, so the stack is larger than the memory in use.")
by_app("Network by app", 0, 44, APP_NET % (OVER, OVER), "Bps",
       "sum(%s)" % (NET % "download" + " + ignoring(direction) " + NET % "upload"), "All traffic (mactop)",
       "Received plus sent on external interfaces.")
by_app("Disk by app", 12, 44, APP_DISK % (OVER, OVER), "Bps",
       DISK % "read" + " + ignoring(operation) " + DISK % "write", "All disk I/O (mactop)",
       "Read plus written. The gap under the dashed line is system daemons." + OWN)
by_app("GPU by app", 0, 52, APP_GPU % OVER, "percent", "mactop_gpu_usage_percent", "GPU busy (mactop)",
       "Share of GPU time, from each app's GPU clients.")
by_app("Energy by app", 12, 52, APP_POWER % OVER, "watt", None, None,
       "macOS's estimate of each app's power draw. It leaves out the display, the rest of the hardware and system daemons, which the Power chart above includes." + OWN)

row("Inside $app", 60)
PROCESS_COLUMNS = [
    ("Processes", held_by_process("sysmon_process_count"), "none"),
    ("CPU", rate_by_process("sysmon_process_cpu_seconds_total", NOW) + " * 100", "percent"),
    ("Resident", held_by_process("sysmon_process_resident_bytes"), "bytes"),
    ("Footprint", held_by_process("sysmon_process_footprint_bytes"), "bytes"),
    ("Network", "round(%s)" % BY_PROCESS % (RATE % ("sysmon_process_network_received_bytes_total", NOW) + " + " + RATE % ("sysmon_process_network_sent_bytes_total", NOW)), "Bps"),
    ("Disk", "round(%s)" % BY_PROCESS % (RATE % ("sysmon_process_disk_read_bytes_total", NOW) + " + " + RATE % ("sysmon_process_disk_written_bytes_total", NOW)), "Bps"),
    ("GPU", rate_by_process("sysmon_process_gpu_seconds_total", NOW) + " * 100", "percent"),
    ("Power", rate_by_process("sysmon_process_energy_joules_total", NOW), "watt"),
    ("Wakeups", rate_by_process("sysmon_process_idle_wakeups_total", NOW), "suffix:/s"),
    ("Threads", held_by_process("sysmon_process_threads"), "none"),
    ("Open files", held_by_process("sysmon_process_open_files"), "none"),
    ("Sockets", held_by_process("sysmon_process_open_sockets"), "none"),
]
instant(panel("table", "Processes now", 0, 61, 24, 8, [(expr, "") for _, expr, _ in PROCESS_COLUMNS], None,
              "Every process of the app, with what it used over the last minute and what it holds now. A blank cell has no reading behind it. macOS does not let this user read the footprint, disk, power, wakeups, threads, files or sockets of other users' processes, and network stays blank for a process that has held no external connection since sysmon started." + PICK,
              defaults={"decimals": 1, "custom": {"minWidth": 70}},
              overrides=[column_unit(name, unit) for name, _, unit in PROCESS_COLUMNS] +
                        [{"matcher": {"id": "byName", "options": "process"}, "properties": [{"id": "custom.width", "value": 260}]}],
              options={"showHeader": True, "cellHeight": "sm", "sortBy": [{"displayName": "CPU", "desc": True}]},
              transformations=[{"id": "merge", "options": {}},
                               {"id": "organize", "options": {
                                   "excludeByName": {"Time": True},
                                   "renameByName": {"Value #%s" % chr(65 + i): name for i, (name, _, _) in enumerate(PROCESS_COLUMNS)}}}]),
        format="table")
by_process("CPU by process", 0, 69, [(rate_by_process("sysmon_process_cpu_seconds_total") + " * 100", "{{process}}")], "percent", CPU_DESC)
by_process("Resident memory by process", 8, 69, [(held_by_process("sysmon_process_resident_bytes"), "{{process}}")], "bytes",
           "Memory in RAM. Memory shared between processes counts once for each of them.")
by_process("Footprint by process", 16, 69, [(held_by_process("sysmon_process_footprint_bytes"), "{{process}}")], "bytes",
           "What Activity Monitor shows as Memory: what the process alone is responsible for, compressed and swapped memory included, shared memory left out. A line that only climbs points at a leak." + OWN)
by_process("Network by process", 0, 77, [
    (rate_by_process("sysmon_process_network_received_bytes_total"), "{{process}} received"),
    (rate_by_process("sysmon_process_network_sent_bytes_total"), "{{process}} sent")], "Bps",
    "Received above the line, sent below it, on external interfaces.", mirrored=True, w=12, h=9)
by_process("Disk by process", 12, 77, [
    (rate_by_process("sysmon_process_disk_read_bytes_total"), "{{process}} read"),
    (rate_by_process("sysmon_process_disk_written_bytes_total"), "{{process}} written")], "Bps",
    "Read above the line, written below it." + OWN, mirrored=True, w=12, h=9)
by_process("GPU by process", 0, 86, [(rate_by_process("sysmon_process_gpu_seconds_total") + " * 100", "{{process}}")], "percent",
           "Share of GPU time.")
by_process("Power by process", 8, 86, [(rate_by_process("sysmon_process_energy_joules_total"), "{{process}}")], "watt",
           "macOS's estimate of each process's power draw." + OWN)
by_process("Idle wakeups by process", 16, 86, [(rate_by_process("sysmon_process_idle_wakeups_total"), "{{process}}")], "suffix:/s",
           "Times per second the process woke the CPU from idle. Frequent wakeups keep the chip out of its low-power states and cost battery even when CPU use looks low." + OWN)
by_process("Page-ins by process", 0, 94, [(rate_by_process("sysmon_process_pageins_total"), "{{process}}")], "suffix:pages/s",
           "Memory pages per second read in from disk because the process touched memory that was not in RAM, swapped out or in a mapped file. Steady page-ins while swap is in use mean the Mac is short of memory." + OWN, w=6)
by_process("Threads by process", 6, 94, [(held_by_process("sysmon_process_threads"), "{{process}}")], "none",
           "A count that only climbs points at a thread leak." + OWN, w=6)
by_process("Open files by process", 12, 94, [(held_by_process("sysmon_process_open_files"), "{{process}}")], "none",
           "Files and directories held open. A count that only climbs points at a leaked file handle." + OWN, w=6)
by_process("Sockets by process", 18, 94, [(held_by_process("sysmon_process_open_sockets"), "{{process}}")], "none",
           "Network and local sockets held open. Many sockets with little traffic are idle connections kept open." + OWN, w=6)

row("CPU", 102)
panel("timeseries", "CPU usage", 0, 103, 9, 8, [
    ("mactop_cpu_usage_percent", "All cores"),
    ("mactop_pcore_usage_percent", "Performance cores"),
    ("mactop_score_usage_percent", "Super cores"),
], "percent", "Super cores are the M5's highest-clocked cores. Performance cores take the rest of the work.",
    defaults={"custom": series(fill=12), **PCT},
    overrides=[color_of("All cores", "blue"), color_of("Performance cores", "green"), color_of("Super cores", "purple")],
    options=TS_OPTS)
instant(panel("bargauge", "Cores now", 9, 103, 5, 8, [
    ('sort_by_label_numeric(mactop_cpu_core_usage_percent, "core")', "{{core}}")], "percent",
    "Each core's usage in the latest sample. Cores 0 to 11 are performance cores, 12 to 17 super cores.",
    defaults={**PCT, "decimals": 0, "thresholds": LOAD_STEPS, "color": {"mode": "thresholds"}},
    options={"orientation": "vertical", "displayMode": "lcd", "valueMode": "hidden",
             "showUnfilled": True, "namePlacement": "auto", "sizing": "auto", "reduceOptions": LAST}))
panel("state-timeline", "Cores over time", 14, 103, 10, 8, [
    ('sort_by_label_numeric(mactop_cpu_core_usage_percent, "core")', "{{type}}{{core}}")], "percent",
    "One lane per core. Green is idle, red is saturated. A lane that stays red means one thread is the bottleneck.",
    defaults={**PCT, "color": {"mode": "continuous-GrYlRd"}, "custom": {"fillOpacity": 90, "lineWidth": 0}},
    options={"showValue": "never", "rowHeight": 0.9, "mergeValues": False, "alignValue": "left",
             "legend": {"showLegend": False}, "tooltip": {"mode": "multi", "sort": "desc"}})

row("GPU and power", 111)
panel("timeseries", "GPU", 0, 112, 12, 8, [
    ("mactop_gpu_usage_percent", "Usage"),
    ("mactop_gpu_freq_mhz", "Frequency"),
], "percent", None,
    defaults={"custom": series(fill=20), **PCT},
    overrides=[color_of("Usage", "purple"),
               {"matcher": {"id": "byName", "options": "Frequency"},
                "properties": [{"id": "unit", "value": "rotmhz"}, {"id": "max", "value": None},
                               {"id": "color", "value": fixed("text")}, {"id": "custom.axisPlacement", "value": "right"},
                               {"id": "custom.fillOpacity", "value": 0},
                               {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}}]}],
    options=TS_OPTS)
panel("timeseries", "Power", 12, 112, 12, 8, [
    (TOTAL_W + " - ignoring(component) " + GPU_W, "SoC and rest of system"),
    (GPU_W, "GPU"),
], "watt",
    "mactop 2.1.5 reads CPU, DRAM and Neural Engine power as zero on this M5 Pro, so the SoC's share cannot be split further. Samples where those readings spike past the whole-machine reading are left out.",
    defaults={"custom": series(fill=35, stack=True, gradient="none")},
    overrides=[color_of("SoC and rest of system", "orange"), color_of("GPU", "purple")],
    options=TS_OPTS)

row("Memory", 120)
panel("timeseries", "Memory", 0, 121, 12, 8, [
    (GIB % "used", "Used"),
    (GIB % "total", "Installed"),
], "bytes", None,
    defaults={"custom": series(fill=25), "min": 0},
    overrides=[color_of("Used", "green"),
               {"matcher": {"id": "byName", "options": "Installed"},
                "properties": [{"id": "color", "value": fixed("text")}, {"id": "custom.fillOpacity", "value": 0},
                               {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}}]}],
    options=TS_OPTS)
panel("timeseries", "Swap", 12, 121, 12, 8, [
    (GIB % "swap_used", "Used"),
    (GIB % "swap_total", "Allocated"),
], "bytes", "macOS allocates swap files as it needs them, so Allocated rising means memory ran short at that moment.",
    defaults={"custom": series(fill=25), "min": 0},
    overrides=[color_of("Used", "purple"),
               {"matcher": {"id": "byName", "options": "Allocated"},
                "properties": [{"id": "color", "value": fixed("text")}, {"id": "custom.fillOpacity", "value": 0},
                               {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}}]}],
    options=TS_OPTS)

row("Network and disk", 129)
panel("timeseries", "Network", 0, 130, 12, 8, [
    (NET % "download", "Download"), (NET % "upload", "Upload")], "Bps",
    "Download above the line, upload below. Samples above 10 GB/s, faster than any link this Mac has, are mactop counter glitches and are left out.",
    defaults={"custom": series(fill=30)},
    overrides=[color_of("Download", "blue"), color_of("Upload", "green")] + MIRROR, options=TS_OPTS)
panel("timeseries", "Disk throughput", 12, 130, 6, 8, [
    (DISK % "read", "Read"), (DISK % "write", "Write")], "Bps", "Read above the line, write below.",
    defaults={"custom": series(fill=30)},
    overrides=[color_of("Read", "yellow"), color_of("Write", "orange")] + MIRROR, options=TS_OPTS)
panel("timeseries", "Disk operations", 18, 130, 6, 8, [
    ('mactop_disk_iops{operation="read"}', "Read"), ('mactop_disk_iops{operation="write"}', "Write")], "iops",
    "Many operations with little throughput means small random I/O.",
    defaults={"custom": series(fill=30)},
    overrides=[color_of("Read", "yellow"), color_of("Write", "orange")] + MIRROR, options=TS_OPTS)

row("Thermals", 138)
panel("timeseries", "Hottest sensor by area", 0, 139, 12, 8, TEMPS, "celsius",
      "The hottest of each area's sensors.",
      defaults={"custom": series(fill=0)}, options=TS_OPTS)
panel("timeseries", "Fans", 12, 139, 6, 8, [("mactop_fan_rpm", "{{fan_name}}")], "rotrpm", None,
      defaults={"custom": series(fill=15), "min": 0}, options=TS_OPTS)
panel("state-timeline", "Thermal state", 18, 139, 6, 8, [("mactop_thermal_state", "Thermal state")], None,
      "Any band other than green is time the Mac spent throttled.",
      defaults={"mappings": THERMAL_STATES, "color": {"mode": "thresholds"}, "thresholds": ALL_GREEN,
                "custom": {"fillOpacity": 85, "lineWidth": 0}},
      options={"showValue": "never", "rowHeight": 0.8, "mergeValues": True,
               "legend": {"showLegend": False}, "tooltip": {"mode": "single"}})

row("Battery", 147)
panel("timeseries", "Battery", 0, 148, 24, 6, [
    ("mactop_battery_percent", "Charge"), ("mactop_battery_charging * 100", "Charging")], "percent", None,
    defaults={"custom": series(fill=20), **PCT},
    overrides=[color_of("Charge", "green"),
               {"matcher": {"id": "byName", "options": "Charging"},
                "properties": [{"id": "color", "value": fixed("yellow")},
                               {"id": "custom.drawStyle", "value": "bars"},
                               {"id": "custom.fillOpacity", "value": 15}, {"id": "custom.lineWidth", "value": 0}]}],
    options=TS_OPTS)

dashboard = {
    "uid": "mac-system",
    "title": "Mac system",
    "tags": ["mactop"],
    "templating": {"list": [{
        "name": "app", "label": "App", "type": "query", "datasource": DS,
        "query": "query_result(sort_desc(sum by (app) (sysmon_process_resident_bytes)))",
        "definition": "query_result(sort_desc(sum by (app) (sysmon_process_resident_bytes)))",
        "regex": "/app=\"([^\"]+)\"/", "refresh": 2, "sort": 0, "includeAll": False, "multi": False,
    }]},
    "graphTooltip": 1,
    "time": {"from": "now-1h", "to": "now"},
    "refresh": "10s",
    "schemaVersion": 41,
    "panels": panels,
}
with open(Path(__file__).parent / "dashboards" / "mac-system.json", "w") as f:
    json.dump(dashboard, f, indent=2)
    f.write("\n")
