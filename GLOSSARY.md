# Glossary

**App**: the outermost `.app` bundle a process runs from, such as Brave Browser with all
its helper processes. Processes outside any bundle group by executable name, so eleven
`claude` processes are one app.

**Process**: one running program, identified by its pid. macOS reuses pids, so a pid can
name a new process once the old one exits.

**Super cores and performance cores**: the two CPU core clusters of the M5 Pro. Super
cores are the faster cluster. macOS names them in `sysctl hw.perflevel0.name` and `hw.perflevel1.name`. Older
Apple chips call the pair performance and efficiency cores.

**Resident memory**: the memory a process has in RAM right now. Pages shared between
processes count once for each of them, so the resident memory of all processes adds up
to more than the machine uses.

**Swap**: memory macOS has moved from RAM to disk because RAM ran short. Swap that keeps
growing means the machine needs more memory than it has.

**Thermal state**: macOS's own verdict on heat, from nominal through fair and serious to
critical. Only critical means heat is cutting the system's performance. Fair and serious
ask apps to cut back their work before that happens.

**External traffic**: network bytes on any interface other than loopback, which includes
VPN tunnels and virtual machine bridges. Traffic between processes on this Mac is not
external.

**Kernel time**: CPU time macOS spends on its own work, which belongs to no app.

**Energy**: what the chip spent running a process, in joules. Energy per second is power,
in watts.

**Cache read and cache write**: the two ways a Claude request uses the prompt cache. A
cache write stores a prompt prefix at a premium, and a cache read reuses it at a
fraction of the input price. Claude Code's telemetry calls a cache write cache
creation.

**Session limit**: the cap on how much of the Claude plan can be used in one five-hour
session. It resets when the session ends.

**Weekly limit**: the cap on how much of the Claude plan all models together can use in
a week.

**Per-model weekly limit**: a weekly cap of its own on one model, such as Fable, beside the
weekly limit that covers all models.
