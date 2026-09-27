import json
from pathlib import Path

VM = {"type": "prometheus", "uid": "victoriametrics"}
CH = {"type": "grafana-clickhouse-datasource", "uid": "clickhouse"}
panels = []


def prom(expr, legend=""):
    return {"datasource": VM, "expr": expr, "legendFormat": legend}


def sql(query, table=False):
    return {"datasource": CH, "editorType": "sql", "format": 1 if table else 0, "rawSql": query}


def ref_id(i):
    return chr(65 + i) if i < 26 else ref_id(i // 26 - 1) + chr(65 + i % 26)


def panel(kind, title, x, y, w, h, targets, unit=None, desc=None, defaults=None, overrides=None, **extra):
    field_defaults = dict(defaults or {})
    if unit:
        field_defaults["unit"] = unit
    p = {
        "id": len(panels) + 1,
        "type": kind,
        "title": title,
        "datasource": targets[0]["datasource"] if targets else VM,
        "gridPos": {"x": x, "y": y, "w": w, "h": h},
        "targets": [{"refId": ref_id(i), **t} for i, t in enumerate(targets)],
        "fieldConfig": {"defaults": field_defaults, "overrides": overrides or []},
        "options": {},
        **extra,
    }
    if desc:
        p["description"] = desc
    panels.append(p)
    return p


def instant(p):
    for target in p["targets"]:
        target.update(instant=True, range=False)
    return p


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


def limit_line(name):
    return {"matcher": {"id": "byName", "options": name},
            "properties": [{"id": "custom.fillOpacity", "value": 0}, {"id": "color", "value": fixed("text")},
                           {"id": "custom.lineStyle", "value": {"fill": "dash", "dash": [6, 4]}}]}


def chart(title, x, y, w, targets, unit, desc, h=7, defaults=None, overrides=None, fill=18, stack=False, **extra):
    return panel("timeseries", title, x, y, w, h, targets, unit, desc,
                 defaults={"custom": series(fill=fill, stack=stack), "min": 0, **(defaults or {})},
                 overrides=overrides, options=TS_OPTS, **extra)


TS_OPTS = {"legend": {"showLegend": True, "displayMode": "list", "placement": "bottom", "calcs": []},
           "tooltip": {"mode": "multi", "sort": "desc"}}
LAST = {"calcs": ["lastNotNull"], "fields": "", "values": False}
ALL_GREEN = {"mode": "absolute", "steps": [{"color": "green", "value": None}]}
MUTED = "#8E8E8E"
IDLE = -1

OK, SLOW = 0, 10
THROTTLED, BACKLOG, LOW_DISK, TOO_MANY_PARTS, ERRORS = 22, 23, 24, 25, 26
LOGIN_EXPIRED, NO_LOGIN, READ_FAILED, STALLED, REJECTING, DROPPING, DOWN = 33, 34, 35, 36, 37, 38, 39
STATES = {
    OK: ("OK", "green"),
    SLOW: ("Slow", "yellow"),
    THROTTLED: ("Throttled", "orange"),
    BACKLOG: ("Backlog", "orange"),
    LOW_DISK: ("Low disk", "orange"),
    TOO_MANY_PARTS: ("Too many parts", "orange"),
    ERRORS: ("Errors", "orange"),
    LOGIN_EXPIRED: ("Login expired", "red"),
    NO_LOGIN: ("No login", "red"),
    READ_FAILED: ("Read failed", "red"),
    STALLED: ("Stalled", "red"),
    REJECTING: ("Rejecting inserts", "red"),
    DROPPING: ("Dropping events", "red"),
    DOWN: ("Down", "red"),
}
STATUS_MAPPINGS = [{"type": "value", "options": {
    str(code): {"text": text, "color": color, "index": i} for i, (code, (text, color)) in enumerate(STATES.items())}}]
VERDICT_MAPPINGS = [
    {"type": "value", "options": {"0": {"text": "Healthy", "color": "green", "index": 0},
                                  "10": {"text": "Slow", "color": "yellow", "index": 1}}},
    {"type": "range", "options": {"from": 20, "to": 29, "result": {"text": "Degraded", "color": "orange", "index": 2}}},
    {"type": "range", "options": {"from": 30, "to": 39, "result": {"text": "Action needed", "color": "red", "index": 3}}},
]


def down(job):
    return '(max(up{job="%s"}) or vector(0)) == bool 0' % job


def happened(counter):
    return "(sum(increase(%s[5m])) or vector(0)) > bool 0" % counter


def present(selector):
    return "(max(%s) or vector(0)) > bool 0" % selector


def above(expr, limit):
    return "(max(%s) or vector(0)) > bool %s" % (expr, limit)


def below(expr, limit):
    return "(min(%s) or vector(1)) < bool %s" % (expr, limit)


def status(*rules):
    return "max(%s, vector(0))" % ", ".join("(%s) * %d" % (cond, code) for code, cond in rules)


def known(expr):
    return "(%s) >= 0" % expr


def idle_when_empty(expr):
    return "(%s) or vector(%d)" % (known(expr), IDLE)


def p95(histogram, by=""):
    return "histogram_quantile(0.95, sum by (le%s) (rate(%s[5m])))" % (by, histogram)


LAST_READ = "claude_limits_last_read_outcome{outcome%s}"
RECEIVE_P95 = p95('rpc_server_call_duration_bucket{job="otelcol"}')
INSERT_AVG = ("sum(rate(ClickHouseProfileEvents_InsertQueryTimeMicroseconds[5m])) / "
              "sum(rate(ClickHouseProfileEvents_InsertQuery[5m])) / 1e6")
DS_QUERY_P95 = p95('grafana_http_request_duration_seconds_bucket{handler="/api/ds/query"}')
VM_CPU = 'rate(process_cpu_seconds_total{job="victoria-metrics"}[5m]) / vm_available_cpu_cores'

COMPONENTS = [
    ("mactop", status(
        (DOWN, down("mactop")),
        (STALLED, "(max(changes(mactop_cpu_usage_percent[2m])) or vector(1)) == bool 0"),
        (SLOW, above('scrape_duration_seconds{job="mactop"}', 1)))),
    ("sysmon-procs", status(
        (DOWN, down("sysmon-procs")),
        (SLOW, above("sysmon_procs_sample_duration_seconds", 5)),
        (SLOW, above('scrape_duration_seconds{job="sysmon-procs"}', 1)))),
    ("claude-limits", status(
        (DOWN, down("claude-limits-reads")),
        (READ_FAILED, present(LAST_READ % '!~"ok|no_login|http_401|http_429|http_5.."')),
        (NO_LOGIN, present(LAST_READ % '="no_login"')),
        (LOGIN_EXPIRED, present(LAST_READ % '="http_401"')),
        (STALLED, "(max(time() - claude_limits_last_success_timestamp_seconds) * max(%s) or vector(0)) > bool 900"
         % (LAST_READ % '="ok"')),
        (ERRORS, present(LAST_READ % '=~"http_5.."')),
        (THROTTLED, present(LAST_READ % '="http_429"')),
        (SLOW, above("claude_limits_last_read_duration_seconds", 5)))),
    ("VictoriaMetrics", status(
        (DOWN, down("victoria-metrics")),
        (REJECTING, present("vm_storage_is_read_only")),
        (ERRORS, happened('vm_log_messages_total{level=~"error|fatal|panic"}')),
        (LOW_DISK, below("vm_free_disk_space_bytes / vm_total_disk_space_bytes", 0.05)),
        (THROTTLED, happened("vm_concurrent_select_limit_timeout_total")),
        (THROTTLED, happened("vm_concurrent_insert_limit_timeout_total")),
        (SLOW, happened("vm_slow_queries_total")),
        (SLOW, above(VM_CPU, 0.8)))),
    ("Collector", status(
        (DOWN, down("otelcol")),
        (DROPPING, happened("otelcol_exporter_send_failed_log_records")),
        (DROPPING, happened("otelcol_exporter_enqueue_failed_log_records")),
        (DROPPING, happened("otelcol_receiver_refused_log_records")),
        (DROPPING, happened("otelcol_receiver_failed_log_records")),
        (BACKLOG, above("otelcol_exporter_queue_size / otelcol_exporter_queue_capacity", 0.05)),
        (SLOW, above(RECEIVE_P95, 0.5)),
        (SLOW, above("min_over_time(otelcol_exporter_in_flight_requests[1m])", 0)))),
    ("ClickHouse", status(
        (DOWN, down("clickhouse")),
        (REJECTING, happened("ClickHouseProfileEvents_RejectedInserts")),
        (ERRORS, happened("ClickHouseProfileEvents_FailedQuery")),
        (TOO_MANY_PARTS, above("ClickHouseAsyncMetrics_MaxPartCountForPartition", 300)),
        (LOW_DISK, below("ClickHouseAsyncMetrics_FilesystemMainPathAvailableBytes / "
                         "ClickHouseAsyncMetrics_FilesystemMainPathTotalBytes", 0.05)),
        (THROTTLED, happened("ClickHouseProfileEvents_DelayedInserts")),
        (SLOW, above(INSERT_AVG, 1)))),
    ("Grafana", status(
        (DOWN, down("grafana")),
        (ERRORS, happened('grafana_http_request_duration_seconds_count{status_code=~"5.."}')),
        (ERRORS, happened('grafana_plugin_request_total{status="error"}')),
        (SLOW, above(DS_QUERY_P95, 2)))),
]
VERDICT = "max(%s)" % ", ".join(expr for _, expr in COMPONENTS)

SCALE = {"horizontal": "scale", "vertical": "scale"}
TINTS = {"source": ("rgba(189,193,198,0.14)", "#BDC1C6"), "metric": ("rgba(127,196,184,0.16)", "#7FC4B8"),
         "telem": ("rgba(169,144,220,0.16)", "#A990DC"), "view": ("rgba(138,180,224,0.16)", "#8AB4E0")}
LINE = 3.2
PAD = 0.8
elements = []


def place(x, y, w, h):
    return {"left": x, "top": y, "right": round(100 - x - w, 2), "bottom": round(100 - y - h, 2)}


def element(kind, box, config, name=None, **extra):
    elements.append({"type": kind, "name": name or "%s %d" % (kind, len(elements) + 1), "config": config,
                     "constraint": SCALE, "placement": place(*box), **extra})
    return elements[-1]


def color(field=None, otherwise=MUTED):
    return {"fixed": otherwise, "field": field} if field else {"fixed": otherwise}


def words(box, text, size=12, tint=MUTED, align="left"):
    element("text", box, {"text": {"mode": "fixed", "fixed": text}, "color": color(otherwise=tint),
                          "size": size, "align": align, "valign": "middle"})


def reading(box, field, size=12, tint=None, align="right"):
    element("metric-value", box, {"text": {"mode": "field", "field": field, "fixed": ""},
                                  "color": color(field, "text") if tint is None else color(otherwise=tint),
                                  "size": size, "align": align, "valign": "middle"})


def group(box, caption):
    x, y, w, _ = box
    element("rectangle", box, {"text": {"mode": "fixed", "fixed": ""}},
            background={"color": {"fixed": "rgba(128,128,128,0.04)"}},
            border={"color": {"fixed": "rgba(128,128,128,0.35)"}, "width": 1, "radius": 8})
    words((x + PAD, y + 0.5, w - 2 * PAD, LINE), caption, size=11)


def node(name, box, kind, title, subtitle, figures, watched=True):
    x, y, w, h = box
    tint, stroke = TINTS[kind]
    edge = color(name + " status", stroke) if watched else color(otherwise=stroke)
    element("rectangle", box, {"text": {"mode": "fixed", "fixed": ""}}, name=name,
            background={"color": {"fixed": tint}},
            border={"color": edge, "width": 3 if watched else 1, "radius": 6}, connections=[])
    top = y + 1
    words((x + PAD, top, w - 2 * PAD, LINE), title, size=14, tint="text")
    top += LINE
    words((x + PAD, top, w - 2 * PAD, LINE), subtitle, size=11)
    top += LINE
    if watched:
        reading((x + PAD, top, w - 2 * PAD, LINE), name + " status", size=13, align="left")
        top += LINE
    for caption, field in figures:
        words((x + PAD, top, w * 0.55, LINE), caption)
        reading((x + w * 0.45, top, w * 0.55 - PAD, LINE), field, tint="text")
        top += LINE


def anchor(box, px, py):
    x, y, w, h = box
    return {"x": round((px - x - w / 2) / (w / 2), 3), "y": round(-(py - y - h / 2) / (h / 2), 3)}


def flow(source, target, into_y=None, out_y=None, dashed=True, colored_by=None):
    sx, sy, sw, sh = BOXES[source]
    tx, ty, tw, th = BOXES[target]
    start = (sx + sw, out_y if out_y is not None else sy + sh / 2)
    end = (tx, into_y if into_y is not None else ty + th / 2)
    for e in elements:
        if e["name"] == source:
            e["connections"].append({
                "source": anchor(BOXES[source], *start), "target": anchor(BOXES[target], *end),
                "targetName": target, "path": "straight",
                "color": color(colored_by + " status"), "size": {"fixed": 2, "min": 1, "max": 10},
                "lineStyle": {"style": "dashed" if dashed else "solid", "animate": False}})
    return start, end


def flow_label(point, field=None, text=None, width=12, dy=-3.6):
    box = (point[0] - width / 2, point[1] + dy, width, LINE)
    if field:
        reading(box, field, size=11, tint=MUTED, align="center")
    else:
        words(box, text, size=11, align="center")


def midpoint(start, end):
    return ((start[0] + end[0]) / 2, (start[1] + end[1]) / 2)


BOXES = {
    "macOS": (1, 19, 15, 13),
    "Anthropic API": (1, 50.5, 15, 13),
    "Claude Code": (1, 79.5, 15, 13),
    "mactop": (23, 6, 16, 18),
    "sysmon-procs": (23, 27, 16, 18),
    "claude-limits": (23, 48, 16, 18),
    "VictoriaMetrics": (52, 23.5, 18, 25),
    "Collector": (23, 77, 16, 18),
    "ClickHouse": (52, 77, 18, 18),
    "Grafana": (84, 50, 15, 18),
}

def figure_field(name, unit):
    properties = [{"id": "unit", "value": unit},
                  {"id": "mappings", "value": [{"type": "value", "options": {str(IDLE): {"text": "idle", "index": 0}}}]}]
    if unit == "none":
        properties.append({"id": "decimals", "value": 0})
    if unit == "percent":
        properties.append({"id": "decimals", "value": 1})
    return {"matcher": {"id": "byName", "options": name}, "properties": properties}


FIGURES = [
    ("processes", "sum(sysmon_process_count)", "none"),
    ("last good read", "time() - max(claude_limits_last_success_timestamp_seconds)", "s"),
    ("last event", "time() - max(tlast_change_over_time(otelcol_receiver_accepted_log_records[1d]))", "s"),
    ("mactop scrape", 'max(scrape_duration_seconds{job="mactop"})', "s"),
    ("mactop series", 'max(scrape_samples_post_metric_relabeling{job="mactop"})', "short"),
    ("procs sample", "max(sysmon_procs_sample_duration_seconds)", "s"),
    ("procs series", 'max(scrape_samples_post_metric_relabeling{job="sysmon-procs"})', "short"),
    ("limits read", "max(claude_limits_last_read_duration_seconds)", "s"),
    ("limits ok share", 'sum(increase(claude_limits_reads_total{outcome="ok"}[6h])) / '
                        "sum(increase(claude_limits_reads_total[6h])) * 100", "percent"),
    ("vm query p99", idle_when_empty('max(vm_request_duration_seconds{quantile="0.99"})'), "s"),
    ("vm cpu", "max(%s) * 100" % VM_CPU, "percent"),
    ("vm series", 'max(vm_cache_entries{type="storage/hour_metric_ids"})', "short"),
    ("vm stored", "sum(vm_data_size_bytes)", "bytes"),
    ("collector queue", "max(otelcol_exporter_queue_size)", "none"),
    ("collector p95", idle_when_empty(RECEIVE_P95), "s"),
    ("clickhouse insert", idle_when_empty(INSERT_AVG), "s"),
    ("clickhouse parts", "max(ClickHouseAsyncMetrics_MaxPartCountForPartition)", "none"),
    ("grafana p95", idle_when_empty(DS_QUERY_P95), "s"),
    ("grafana errors", '(sum(increase(grafana_plugin_request_total{status="error"}[1h])) or vector(0)) + '
                       '(sum(increase(grafana_http_request_duration_seconds_count{status_code=~"5.."}[1h])) or vector(0))',
     "none"),
    ("mactop flow", 'sum_over_time(scrape_samples_post_metric_relabeling{job="mactop"}[5m]) / 300', "count:samples/s"),
    ("procs flow", 'sum_over_time(scrape_samples_post_metric_relabeling{job="sysmon-procs"}[5m]) / 300',
     "count:samples/s"),
    ("limits flow", 'sum(sum_over_time(scrape_samples_post_metric_relabeling{job=~"claude-limits.*"}[5m])) / 300',
     "count:samples/s"),
    ("events in", "sum(rate(otelcol_receiver_accepted_log_records[5m])) * 60", "count:events/min"),
    ("events out", "sum(rate(otelcol_exporter_sent_log_records[5m])) * 60", "count:events/min"),
    ("vm queries", 'sum(rate(grafana_plugin_request_total{plugin_id="prometheus"}[5m])) * 60', "count:queries/min"),
    ("clickhouse queries", 'sum(rate(grafana_plugin_request_total{plugin_id="grafana-clickhouse-datasource"}[5m])) * 60',
     "count:queries/min"),
]

group((20, 1, 52, 67), "bin/sysmon metrics · runs until stopped")
group((20, 71, 52, 28), "bin/sysmon telemetry · starts at login")
node("macOS", BOXES["macOS"], "source", "macOS", "the machine and each process", [("processes", "processes")], False)
node("Anthropic API", BOXES["Anthropic API"], "source", "Anthropic API", "Keychain sign-in",
     [("last good read", "last good read")], False)
node("Claude Code", BOXES["Claude Code"], "source", "Claude Code", "OTLP events", [("last event", "last event")], False)
node("mactop", BOXES["mactop"], "metric", "mactop", "whole machine :2112",
     [("scrape", "mactop scrape"), ("series", "mactop series")])
node("sysmon-procs", BOXES["sysmon-procs"], "metric", "sysmon-procs", "each app :2113",
     [("sample", "procs sample"), ("series", "procs series")])
node("claude-limits", BOXES["claude-limits"], "metric", "claude-limits", "plan limits :2114",
     [("last read", "limits read"), ("reads ok, 6h", "limits ok share")])
node("VictoriaMetrics", BOXES["VictoriaMetrics"], "metric", "VictoriaMetrics", ":8428 data/",
     [("query p99", "vm query p99"), ("CPU of 2 cores", "vm cpu"), ("series", "vm series"), ("stored", "vm stored")])
words((52, 49.5, 19, 2 * LINE), "also scrapes the Collector, ClickHouse, Grafana and itself", size=10)
node("Collector", BOXES["Collector"], "telem", "Collector", "OpenTelemetry :4327",
     [("queue", "collector queue"), ("receive p95", "collector p95")])
node("ClickHouse", BOXES["ClickHouse"], "telem", "ClickHouse", ":9327 clickhouse-data/",
     [("insert avg", "clickhouse insert"), ("max parts", "clickhouse parts")])
node("Grafana", BOXES["Grafana"], "view", "Grafana", ":3030",
     [("query p95", "grafana p95"), ("errors, 1h", "grafana errors")])
words((84, 68.5, 15, LINE), "bin/sysmon grafana · until logout", size=10)

flow("macOS", "mactop", out_y=22, colored_by="mactop")
flow("macOS", "sysmon-procs", out_y=29, colored_by="sysmon-procs")
start, end = flow("Anthropic API", "claude-limits", colored_by="claude-limits")
flow_label(midpoint(start, end), text="every 5 min", width=7)
start, end = flow("mactop", "VictoriaMetrics", into_y=28, colored_by="mactop")
flow_label(midpoint(start, end), "mactop flow", dy=-5)
start, end = flow("sysmon-procs", "VictoriaMetrics", colored_by="sysmon-procs")
flow_label(midpoint(start, end), "procs flow")
start, end = flow("claude-limits", "VictoriaMetrics", into_y=44, colored_by="claude-limits")
flow_label(midpoint(start, end), "limits flow", dy=1.5)
start, end = flow("Claude Code", "Collector", dashed=False, colored_by="Collector")
flow_label(midpoint(start, end), "events in", width=8)
start, end = flow("Collector", "ClickHouse", dashed=False, colored_by="ClickHouse")
flow_label(midpoint(start, end), "events out")
start, end = flow("VictoriaMetrics", "Grafana", into_y=55, colored_by="VictoriaMetrics")
flow_label(midpoint(start, end), "vm queries", dy=-4.5)
start, end = flow("ClickHouse", "Grafana", into_y=66, colored_by="ClickHouse")
flow_label(midpoint(start, end), "clickhouse queries", dy=1.5)

panel("stat", "Verdict", 0, 0, 4, 4, [prom(VERDICT, "Verdict")], None,
      "The worst status of any component. Healthy: all of them are OK. Slow: something works but slower than usual. "
      "Degraded: something is throttled, backed up, short of disk or failing some requests, and will usually recover "
      "on its own. Action needed: something is down or losing data until you act. With no answer from "
      "VictoriaMetrics, this panel cannot judge anything: run bin/sysmon status.",
      defaults={"mappings": VERDICT_MAPPINGS, "color": {"mode": "thresholds"}, "thresholds": ALL_GREEN,
                "noValue": "No answer"},
      options={"reduceOptions": LAST, "graphMode": "none", "colorMode": "background", "textMode": "value",
               "justifyMode": "center", "text": {"valueSize": 26}})
panel("stat", "Components, in the order data flows through them", 4, 0, 20, 4,
      [prom(expr, name) for name, expr in COMPONENTS], None,
      "Each component's status now. The table under the diagram says what each status means and what to do.",
      defaults={"mappings": STATUS_MAPPINGS, "color": {"mode": "thresholds"}, "thresholds": ALL_GREEN},
      options={"reduceOptions": LAST, "graphMode": "none", "colorMode": "background", "textMode": "value_and_name",
               "justifyMode": "center", "orientation": "vertical", "wideLayout": True,
               "text": {"titleSize": 13, "valueSize": 18}})
instant(panel("canvas", "How data flows through sysmon now", 0, 4, 24, 19,
              [prom(expr, name + " status") for name, expr in COMPONENTS] +
              [prom(expr, name) for name, expr, _ in FIGURES], None,
              "The README's diagram, live. Dashed arrows are pulls, solid arrows are pushes, and each arrow and "
              "border takes the color of the component it depends on. Figures on an arrow are what flowed through "
              "it over the last 5 minutes.",
              defaults={"noValue": "none", "color": {"mode": "thresholds"}, "thresholds": ALL_GREEN},
              overrides=[{"matcher": {"id": "byRegexp", "options": ".* status$"},
                          "properties": [{"id": "mappings", "value": STATUS_MAPPINGS}]}] +
                        [figure_field(name, unit) for name, _, unit in FIGURES],
              options={"inlineEditing": False, "showAdvancedTypes": True, "panZoom": False, "zoomToContent": False,
                       "infinitePan": False, "tooltip": {"mode": "none"},
                       "root": {"name": "root", "type": "frame", "elements": elements,
                                "background": {"color": {"fixed": "transparent"}},
                                "border": {"color": {"fixed": "transparent"}},
                                "constraint": SCALE, "placement": {"left": 0, "top": 0, "right": 0, "bottom": 0}}}))
panel("state-timeline", "Status over time", 0, 23, 24, 9, [prom(expr, name) for name, expr in COMPONENTS], None,
      "One lane per component. Hover a band to see its status. Short orange bands come from a single failed "
      "request, which the component retries, so look for bands that last or repeat.",
      defaults={"mappings": STATUS_MAPPINGS, "color": fixed("text"), "custom": {"fillOpacity": 85, "lineWidth": 0}},
      options={"showValue": "auto", "rowHeight": 0.85, "mergeValues": True, "alignValue": "left",
               "legend": {"showLegend": False}, "tooltip": {"mode": "single"}})
panel("text", "What a status means and what to do", 0, 32, 24, 17, [], None, None,
      options={"mode": "markdown", "content": """Start with `bin/sysmon status`, then the component's log in `logs/`.

| Status | Means | Do |
|---|---|---|
| Down | VictoriaMetrics cannot scrape it: it stopped, or sysmon-procs failed its last sample. | Restart its group: `bin/sysmon metrics stop`, then `bin/sysmon metrics`. For the Collector or ClickHouse, the same with `telemetry`. |
| Login expired | The usage API refused the Keychain login. | Open Claude Code, which renews it. |
| No login | The Keychain holds no Claude.ai login. | Sign in to Claude Code. |
| Read failed | claude-limits could not reach or understand the usage API. | Read `logs/claude-limits.log`. |
| Stalled | mactop's readings stopped changing, or claude-limits has not read for 15 minutes. | Restart the metrics group. |
| Rejecting inserts | A store refuses new data: VictoriaMetrics went read-only, or ClickHouse rejected an insert. | Free disk space, then read the store's log. |
| Dropping events | The Collector refused or lost Claude Code events. | Check ClickHouse, then `logs/otelcol.log`. |
| Throttled | The usage API answered 429, VictoriaMetrics refused work that waited too long for a slot, or ClickHouse delayed an insert. | Wait. claude-limits reads again in 5 minutes. |
| Backlog | Events wait in the Collector's queue for ClickHouse. | Check ClickHouse. |
| Low disk | Under 5% of the disk is free. | Free disk space before the store stops writing. |
| Too many parts | A ClickHouse partition holds over 300 parts. It slows inserts at 1000. | Check the merges below. |
| Errors | Requests failed in the last 5 minutes, or the usage API answered 5xx. | Read the component's log. |
| Slow | Work takes longer than usual. | Watch the latency charts below. |
"""})

row("Collection: what VictoriaMetrics scrapes every 10 seconds", 49)
panel("state-timeline", "Targets answering", 0, 50, 12, 7, [prom("max by (job) (up)", "{{job}}")], None,
      "Red is a scrape that failed. claude-limits answers 503 while its last read failed, so it goes red with a "
      "throttled or expired read. claude-limits-reads always answers while the service runs.",
      defaults={"mappings": [{"type": "value", "options": {"0": {"text": "Down", "color": "red", "index": 0},
                                                            "1": {"text": "Up", "color": "green", "index": 1}}}],
                "color": fixed("text"), "custom": {"fillOpacity": 85, "lineWidth": 0}},
      options={"showValue": "never", "rowHeight": 0.85, "mergeValues": True,
               "legend": {"showLegend": False}, "tooltip": {"mode": "single"}})
chart("Scrape time by target", 12, 50, 12, [prom("max by (job) (scrape_duration_seconds)", "{{job}}")], "s",
      "How long each target took to answer. A target near its 10-second timeout is about to fail its scrapes.",
      fill=0)
chart("Samples per scrape", 0, 57, 8, [prom("max by (job) (scrape_samples_post_metric_relabeling)", "{{job}}")],
      "short", "Series each target exposes, after the keep lists for ClickHouse and Grafana. sysmon-procs grows with "
               "the number of apps running.",
      defaults={"custom": {**series(fill=0), "scaleDistribution": {"type": "log", "log": 10}}})
chart("Failed and timed-out scrapes", 8, 57, 8, [
    prom("sum(increase(vm_promscrape_scrapes_failed_total[5m]))", "Failed"),
    prom("sum(increase(vm_promscrape_scrapes_timed_out_total[5m]))", "Timed out")], "none",
    "Scrapes that failed over the last 5 minutes, across all targets. Targets answering shows which one.",
    overrides=[color_of("Failed", "red"), color_of("Timed out", "orange")])
chart("Response size by target", 16, 57, 8, [prom("max by (job) (scrape_response_size_bytes)", "{{job}}")], "bytes",
      "Bytes each scrape read.", fill=0)

row("mactop, sysmon-procs and claude-limits", 64)
chart("sysmon-procs sample time", 0, 65, 8, [
    prom("max(sysmon_procs_sample_duration_seconds)", "Sample"),
    prom("vector(10)", "Budget")], "s",
    "How long reading every process took. A sample that runs past the 10-second budget fails, and the scrape "
    "answers 503 until the next one succeeds.",
    overrides=[color_of("Sample", "green"), limit_line("Budget")])
chart("Apps and processes sampled", 8, 65, 8, [
    prom("sum(sysmon_process_count)", "Processes"),
    prom("count(count by (app) (sysmon_process_count))", "Apps")], "short",
    "What sysmon-procs sees. Its series and sample time grow with these.",
    overrides=[color_of("Processes", "blue"), color_of("Apps", "purple")])
chart("mactop readings per minute", 16, 65, 8, [prom("changes(mactop_cpu_usage_percent[1m])", "Readings")], "none",
      "How often mactop's CPU reading changed. It drops to zero when mactop stops reading the machine while still "
      "answering scrapes, which is what Stalled means.", overrides=[color_of("Readings", "green")])
chart("claude-limits reads by outcome", 0, 72, 10, [
    prom("sum by (outcome) (increase(claude_limits_reads_total[5m]))", "{{outcome}}")], "none",
    "One read every 5 minutes. ok is a good read, http_429 is throttling, http_401 is an expired login, no_login is "
    "a missing login and failed is anything else.",
    defaults={"custom": {**series(fill=80, stack=True, gradient="none", width=0, draw="bars")}, "decimals": 0},
    overrides=[color_of("ok", "green"), color_of("http_429", "orange"), color_of("http_401", "red"),
               color_of("no_login", "dark-red"), color_of("failed", "purple")], interval="5m")
chart("Time since a good read", 10, 72, 7, [
    prom("time() - max(claude_limits_last_success_timestamp_seconds)", "Since a good read"),
    prom("vector(300)", "Read interval")], "s",
    "The plan limits on the Claude usage dashboard are this old. A sawtooth under the dashed line is healthy.",
    overrides=[color_of("Since a good read", "blue"), limit_line("Read interval")])
chart("Read time", 17, 72, 7, [prom("max(claude_limits_last_read_duration_seconds)", "Last read")], "s",
      "How long the last read of the usage API took.", overrides=[color_of("Last read", "green")])

row("VictoriaMetrics", 79)
chart("Samples stored", 0, 80, 8, [prom("sum by (type) (rate(vm_rows_inserted_total[5m])) > 0", "{{type}}")],
      "count:/s", "Samples written per second. A drop means a target stopped answering.")
chart("Active and new series", 8, 80, 8, [
    prom('max(vm_cache_entries{type="storage/hour_metric_ids"})', "Active this hour"),
    prom("sum(rate(vm_new_timeseries_created_total[5m])) * 60", "New per minute")], "short",
    "New series come from processes starting, since each process is its own series. Steady churn is normal here.",
    overrides=[color_of("Active this hour", "blue"),
               {"matcher": {"id": "byName", "options": "New per minute"},
                "properties": [{"id": "custom.axisPlacement", "value": "right"}, {"id": "color", "value": fixed("orange")}]}])
chart("Stored data and free disk", 16, 80, 8, [
    prom("sum(vm_data_size_bytes)", "Stored"),
    prom("min(vm_free_disk_space_bytes)", "Free disk")], "bytes",
    "VictoriaMetrics goes read-only when free disk falls under its limit.",
    overrides=[color_of("Stored", "blue"),
               {"matcher": {"id": "byName", "options": "Free disk"},
                "properties": [{"id": "custom.axisPlacement", "value": "right"}, {"id": "color", "value": fixed("green")}]}])
chart("Queries by path", 0, 87, 8, [
    prom('sum by (path) (rate(vm_http_requests_total{path=~"/api/v1/.*|/federate"}[5m])) * 60 > 0', "{{path}}")],
    "count:/min", "Requests per minute, mostly from Grafana.", fill=0)
chart("Query time, 99th percentile", 8, 87, 8, [
    prom('max by (path) (vm_request_duration_seconds{quantile="0.99"}) >= 0', "{{path}}")], "s",
    "Over the last 5 minutes of each path. VictoriaMetrics logs any query over 5 seconds as slow.", fill=0)
chart("Concurrent requests against the limit", 16, 87, 8, [
    prom("max(vm_concurrent_select_current)", "Queries"),
    prom("max(vm_concurrent_select_capacity)", "Query limit"),
    prom("max(vm_concurrent_insert_current)", "Inserts")], "none",
    "Work beyond the limit waits, which shows as Throttled.",
    overrides=[color_of("Queries", "blue"), color_of("Inserts", "green"), limit_line("Query limit")])
chart("CPU against its 2-core limit", 0, 94, 8, [
    prom('rate(process_cpu_seconds_total{job="victoria-metrics"}[5m])', "Used"),
    prom("max(vm_available_cpu_cores)", "Limit")], "none",
    "Cores busy. GOMAXPROCS caps VictoriaMetrics at 2 cores, so heavy queries queue behind each other near the line.",
    overrides=[color_of("Used", "blue"), limit_line("Limit")])
chart("Memory", 8, 94, 8, [
    prom('sum(sysmon_process_resident_bytes{app="victoria-metrics"})', "Resident"),
    prom('go_memstats_heap_inuse_bytes{job="victoria-metrics"}', "Heap in use")], "bytes", None,
    overrides=[color_of("Resident", "green"), color_of("Heap in use", "blue")])
chart("Problems", 16, 94, 8, [
    prom("sum(increase(vm_slow_queries_total[5m]))", "Slow queries"),
    prom('sum(increase(vm_log_messages_total{level=~"error|fatal|panic"}[5m])) or vector(0)', "Errors logged"),
    prom('sum(increase(vm_log_messages_total{level="warn"}[5m])) or vector(0)', "Warnings logged"),
    prom("sum(increase(vm_http_request_errors_total[5m]))", "Requests refused"),
    prom('sum(increase({__name__=~"vm_concurrent_(select|insert)_limit_timeout_total"}[5m]))', "Timed out waiting")], "none",
    "Over the last 5 minutes. A failed scrape logs a warning. A query with a typo counts as a refused request. "
    "Work waits when all 4 slots are busy, which opening a dashboard does, and a query times out after 10 seconds "
    "of waiting.",
    overrides=[color_of("Slow queries", "yellow"), color_of("Errors logged", "red"),
               color_of("Warnings logged", "orange"), color_of("Requests refused", "purple"),
               color_of("Timed out waiting", "dark-red")])

row("Collector", 101)
chart("Events in and out", 0, 102, 8, [
    prom("sum(rate(otelcol_receiver_accepted_log_records[5m])) * 60", "Accepted from Claude Code"),
    prom("sum(rate(otelcol_exporter_sent_log_records[5m])) * 60", "Written to ClickHouse")], "count:/min",
    "Events per minute. The lines should match within a batch of 10 seconds.",
    overrides=[color_of("Accepted from Claude Code", "purple"), color_of("Written to ClickHouse", "green")], fill=0)
chart("Dropped and refused events", 8, 102, 8, [
    prom("sum(increase(otelcol_receiver_refused_log_records[5m])) or vector(0)", "Refused"),
    prom("sum(increase(otelcol_receiver_failed_log_records[5m])) or vector(0)", "Failed on receipt"),
    prom("sum(increase(otelcol_exporter_enqueue_failed_log_records[5m])) or vector(0)", "Queue full"),
    prom("sum(increase(otelcol_exporter_send_failed_log_records[5m])) or vector(0)", "Lost after retries")], "none",
    "Events lost over the last 5 minutes. Anything above zero is Claude Code telemetry that never reached ClickHouse.",
    overrides=[color_of("Refused", "orange"), color_of("Failed on receipt", "red"),
               color_of("Queue full", "purple"), color_of("Lost after retries", "dark-red")])
chart("Queue against capacity", 16, 102, 8, [
    prom("max(otelcol_exporter_queue_size)", "Queued"),
    prom("max(otelcol_exporter_queue_capacity)", "Capacity")], "none",
    "Events wait here up to 10 seconds to form a batch. A queue that keeps growing means ClickHouse is not keeping up.",
    defaults={"custom": {**series(), "scaleDistribution": {"type": "log", "log": 10}}},
    overrides=[color_of("Queued", "purple"), limit_line("Capacity")])
chart("Receive time", 0, 109, 8, [
    prom(p95('rpc_server_call_duration_bucket{job="otelcol"}').replace("0.95", q) + " >= 0", name)
    for q, name in (("0.5", "Median"), ("0.95", "95th percentile"), ("0.99", "99th percentile"))], "s",
    "How long Claude Code waits for the Collector to take a batch of events.", fill=0)
chart("Batch size", 8, 109, 8, [
    prom("sum(rate(otelcol_exporter_queue_batch_send_size_sum[5m])) / sum(rate(otelcol_exporter_queue_batch_send_size_count[5m]))",
         "Events per batch"),
    prom("sum(rate(otelcol_exporter_queue_batch_send_size_bytes_sum[5m])) / "
         "sum(rate(otelcol_exporter_queue_batch_send_size_bytes_count[5m]))", "Bytes per batch")], "none",
    "Average batch written to ClickHouse. Bigger batches mean fewer inserts and fewer parts to merge.",
    overrides=[color_of("Events per batch", "purple"),
               {"matcher": {"id": "byName", "options": "Bytes per batch"},
                "properties": [{"id": "unit", "value": "bytes"}, {"id": "custom.axisPlacement", "value": "right"},
                               {"id": "color", "value": fixed("text")}]}], fill=0)
chart("CPU and memory", 16, 109, 8, [
    prom("rate(otelcol_process_cpu_seconds[5m]) * 100", "CPU"),
    prom("otelcol_process_memory_rss", "Resident"),
    prom("otelcol_process_runtime_heap_alloc_bytes", "Heap")], "percent", "100% is one core fully busy.",
    overrides=[color_of("CPU", "blue")] + [
        {"matcher": {"id": "byName", "options": name},
         "properties": [{"id": "unit", "value": "bytes"}, {"id": "custom.axisPlacement", "value": "right"},
                        {"id": "color", "value": fixed(tint)}]} for name, tint in (("Resident", "green"), ("Heap", "purple"))],
    fill=0)

row("ClickHouse", 116)
chart("Inserts and rows", 0, 117, 8, [
    prom("sum(rate(ClickHouseProfileEvents_InsertQuery[5m])) * 60", "Inserts"),
    prom("sum(rate(ClickHouseProfileEvents_InsertedRows[5m])) * 60", "Rows")], "count:/min",
    "Per minute. Each Collector batch is one insert.",
    overrides=[color_of("Inserts", "purple"),
               {"matcher": {"id": "byName", "options": "Rows"},
                "properties": [{"id": "custom.axisPlacement", "value": "right"}, {"id": "color", "value": fixed("green")}]}],
    fill=0)
chart("Query time, average", 8, 117, 8, [
    prom(known(INSERT_AVG), "Insert"),
    prom(known("sum(rate(ClickHouseProfileEvents_SelectQueryTimeMicroseconds[5m])) / "
               "sum(rate(ClickHouseProfileEvents_SelectQuery[5m])) / 1e6"), "Select")], "s",
    "Inserts come from the Collector, selects from the Claude usage dashboard.",
    overrides=[color_of("Insert", "purple"), color_of("Select", "blue")], fill=0)
chart("Failed, delayed and rejected", 16, 117, 8, [
    prom("sum(increase(ClickHouseProfileEvents_FailedInsertQuery[5m])) or vector(0)", "Failed inserts"),
    prom("sum(increase(ClickHouseProfileEvents_FailedSelectQuery[5m])) or vector(0)", "Failed selects"),
    prom("sum(increase(ClickHouseProfileEvents_DelayedInserts[5m])) or vector(0)", "Delayed inserts"),
    prom("sum(increase(ClickHouseProfileEvents_RejectedInserts[5m])) or vector(0)", "Rejected inserts")], "none",
    "Over the last 5 minutes. ClickHouse delays inserts when a partition has too many parts and rejects them past 3000.",
    overrides=[color_of("Failed inserts", "red"), color_of("Failed selects", "orange"),
               color_of("Delayed inserts", "yellow"), color_of("Rejected inserts", "dark-red")])
chart("Parts and merges", 0, 124, 8, [
    prom("max(ClickHouseAsyncMetrics_MaxPartCountForPartition)", "Most parts in a partition"),
    prom("max(ClickHouseMetrics_PartsActive)", "Active parts"),
    prom("max(ClickHouseMetrics_Merge)", "Merges running")], "none",
    "Every insert writes a part, and merges fold them together. Parts that climb faster than merges fold them "
    "lead to delayed inserts at 1000 in a partition.",
    overrides=[color_of("Most parts in a partition", "purple"), color_of("Active parts", "blue"),
               color_of("Merges running", "green")], fill=0)
chart("Memory", 8, 124, 8, [prom("max(ClickHouseMetrics_MemoryTracking)", "Tracked")], "bytes",
      "Memory ClickHouse accounts for across queries, merges and caches.", overrides=[color_of("Tracked", "purple")])
chart("Stored data and free disk", 16, 124, 8, [
    prom("max(ClickHouseAsyncMetrics_TotalBytesOfMergeTreeTables)", "Stored"),
    prom("min(ClickHouseAsyncMetrics_FilesystemMainPathAvailableBytes)", "Free disk")], "bytes", None,
    overrides=[color_of("Stored", "purple"),
               {"matcher": {"id": "byName", "options": "Free disk"},
                "properties": [{"id": "custom.axisPlacement", "value": "right"}, {"id": "color", "value": fixed("green")}]}])
panel("timeseries", "Events stored", 0, 131, 12, 7, [sql(
    "SELECT $__timeInterval(Timestamp) AS time, count() AS events FROM otel.otel_logs "
    "WHERE $__timeFilter_ms(Timestamp) GROUP BY time ORDER BY time")], "short",
    "Claude Code events in ClickHouse by the time they happened, read from the table itself.",
    defaults={"custom": series(fill=60, gradient="none", width=0, draw="bars"), "min": 0,
              "color": fixed("purple"), "displayName": "Events"}, options=TS_OPTS, interval="1m")
panel("stat", "Newest event stored", 12, 131, 4, 7, [sql(
    "SELECT dateDiff('second', max(Timestamp), now()) AS age FROM otel.otel_logs", table=True)], "s",
    "How long ago the newest Claude Code event happened. It grows while Claude Code is idle, and while events "
    "stop reaching ClickHouse.",
    defaults={"color": fixed("purple"), "decimals": 0},
    options={"reduceOptions": LAST, "graphMode": "none", "colorMode": "value", "textMode": "value",
             "justifyMode": "center", "text": {"valueSize": 48}})
instant(panel("table", "Errors in the time range, by code", 16, 131, 8, 7, [
    {**prom('sort_desc(label_replace(increase({__name__=~"ClickHouseErrorMetric_.+", __name__!="ClickHouseErrorMetric_ALL"}[$__range]) keep_metric_names > 0, '
            '"error", "$1", "__name__", "ClickHouseErrorMetric_(.+)"))'), "format": "table"}], "none",
    "Every error ClickHouse raised, by its name, including failed queries typed by hand and errors ClickHouse "
    "recovered from on its own. A query that bounds the time with toDateTime on both sides raises TYPE_MISMATCH "
    "and still succeeds, so read Failed, delayed and rejected for queries that failed.",
    defaults={"decimals": 0, "noValue": "No errors"},
    options={"showHeader": True, "cellHeight": "sm", "sortBy": [{"displayName": "Value", "desc": True}]},
    transformations=[{"id": "organize", "options": {
        "excludeByName": {"Time": True, "__name__": True, "job": True, "instance": True},
        "renameByName": {"error": "Error", "Value": "Count"}}}]))

row("Grafana", 138)
chart("Requests by status", 0, 139, 8, [
    prom("sum by (status_code) (rate(grafana_http_request_duration_seconds_count[5m])) * 60", "{{status_code}}")],
    "count:/min", "Requests per minute from the browser.", fill=0,
    overrides=[{"matcher": {"id": "byRegexp", "options": "5.."}, "properties": [{"id": "color", "value": fixed("red")}]},
               {"matcher": {"id": "byRegexp", "options": "4.."}, "properties": [{"id": "color", "value": fixed("orange")}]}])
chart("Request time by handler, 95th percentile", 8, 139, 8, [
    prom(p95("grafana_http_request_duration_seconds_bucket", ", handler") + " > 0", "{{handler}}")], "s",
    "Panels load through /api/ds/query, so that line is what a dashboard feels like.", fill=0)
chart("Data source queries by outcome", 16, 139, 8, [
    prom('sum by (plugin_id, status) (rate(grafana_plugin_request_total{endpoint="queryData"}[5m])) * 60',
         "{{plugin_id}} {{status}}")], "count:/min",
    "prometheus is VictoriaMetrics. An error that ClickHouse or VictoriaMetrics caused is marked downstream in "
    "Grafana's own metrics.", fill=0,
    overrides=[{"matcher": {"id": "byRegexp", "options": ".* error"}, "properties": [{"id": "color", "value": fixed("red")}]}])
chart("Data source query time, 95th percentile", 0, 146, 8, [
    prom(p95('grafana_plugin_request_duration_seconds_bucket{endpoint="queryData"}', ", plugin_id") + " >= 0",
         "{{plugin_id}}")], "s", "Time from Grafana asking a data source to its answer.", fill=0)
chart("Goroutines and heap", 8, 146, 8, [
    prom('go_goroutines{job="grafana"}', "Goroutines"),
    prom('go_memstats_heap_inuse_bytes{job="grafana"}', "Heap in use")], "none",
    "Goroutines that only climb point at a leak.",
    overrides=[color_of("Goroutines", "blue"),
               {"matcher": {"id": "byName", "options": "Heap in use"},
                "properties": [{"id": "unit", "value": "bytes"}, {"id": "custom.axisPlacement", "value": "right"},
                               {"id": "color", "value": fixed("green")}]}], fill=0)
chart("CPU and open files", 16, 146, 8, [
    prom('rate(process_cpu_seconds_total{job="grafana"}[5m]) * 100', "CPU"),
    prom('process_open_fds{job="grafana"}', "Open files")], "percent", "100% is one core fully busy.",
    overrides=[color_of("CPU", "blue"),
               {"matcher": {"id": "byName", "options": "Open files"},
                "properties": [{"id": "unit", "value": "none"}, {"id": "custom.axisPlacement", "value": "right"},
                               {"id": "color", "value": fixed("text")}]}], fill=0)

row("What sysmon costs the Mac", 153)
STACK = '{app=~"mactop|sysmon-procs|claude-limits|victoria-metrics|otelcol-contrib|clickhouse|grafana|gpx_.*"}'


def by_service(metric, rate=True):
    measured = "rate(%s%s[$__rate_interval])" % (metric, STACK) if rate else metric + STACK
    return 'sum by (app) (label_replace(%s, "app", "grafana", "app", "gpx_.*"))' % measured


COST = " Grafana includes its data source plugins, which run as processes of their own."
SERVICE_COLORS = [color_of(app, tint) for app, tint in (
    ("mactop", "blue"), ("sysmon-procs", "light-blue"), ("claude-limits", "yellow"), ("victoria-metrics", "green"),
    ("otelcol-contrib", "purple"), ("clickhouse", "orange"), ("grafana", "text"))]
chart("CPU by service", 0, 154, 8, [prom(by_service("sysmon_process_cpu_seconds_total") + " * 100", "{{app}}")],
      "percent", "100% is one core fully busy." + COST, h=8, fill=45, stack=True, overrides=SERVICE_COLORS)
chart("Memory by service", 8, 154, 8, [prom(by_service("sysmon_process_resident_bytes", rate=False), "{{app}}")],
      "bytes", "Resident memory." + COST, h=8, fill=45, stack=True, overrides=SERVICE_COLORS)
chart("Power by service", 16, 154, 8, [prom(by_service("sysmon_process_energy_joules_total"), "{{app}}")],
      "watt", "macOS's estimate of each service's power draw." + COST, h=8, fill=45, stack=True, overrides=SERVICE_COLORS)

dashboard = {
    "uid": "sysmon-services",
    "title": "sysmon services",
    "tags": ["sysmon"],
    "graphTooltip": 1,
    "time": {"from": "now-1h", "to": "now"},
    "refresh": "10s",
    "schemaVersion": 41,
    "panels": panels,
}
with open(Path(__file__).parent / "dashboards" / "sysmon-services.json", "w") as f:
    json.dump(dashboard, f, indent=2)
    f.write("\n")
