package main

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <libproc.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

typedef struct {
	int pid;
	uint64_t gpu_ns;
} gpu_client;

// The pid in an IOUserClientCreator value such as "pid 682, WindowServer", or -1.
static int creator_pid(CFTypeRef creator) {
	char buf[256];
	int pid = -1;
	if (creator == NULL || CFGetTypeID(creator) != CFStringGetTypeID()) return -1;
	if (!CFStringGetCString(creator, buf, sizeof(buf), kCFStringEncodingUTF8)) return -1;
	if (sscanf(buf, "pid %d,", &pid) != 1) return -1;
	return pid;
}

// The sum of accumulatedGPUTime, in nanoseconds, over an AppUsage array.
static uint64_t gpu_ns(CFTypeRef usage) {
	uint64_t total = 0;
	if (usage == NULL || CFGetTypeID(usage) != CFArrayGetTypeID()) return 0;
	for (CFIndex i = 0; i < CFArrayGetCount(usage); i++) {
		CFTypeRef entry = CFArrayGetValueAtIndex(usage, i);
		if (entry == NULL || CFGetTypeID(entry) != CFDictionaryGetTypeID()) continue;
		CFTypeRef n = CFDictionaryGetValue(entry, CFSTR("accumulatedGPUTime"));
		int64_t ns = 0;
		if (n != NULL && CFGetTypeID(n) == CFNumberGetTypeID() &&
			CFNumberGetValue(n, kCFNumberSInt64Type, &ns) && ns > 0) {
			total += (uint64_t)ns;
		}
	}
	return total;
}

// Fills clients with one entry per GPU client of the AGX accelerator, the way mactop
// reads them, and returns how many it wrote, or -1 when there is no accelerator.
static int gpu_clients(gpu_client *clients, int max) {
	io_service_t accelerator = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching("AGXAccelerator"));
	if (accelerator == 0) return -1;
	io_iterator_t children;
	if (IORegistryEntryGetChildIterator(accelerator, kIOServicePlane, &children) != kIOReturnSuccess) {
		IOObjectRelease(accelerator);
		return -1;
	}
	int n = 0;
	io_registry_entry_t child;
	while (n < max && (child = IOIteratorNext(children))) {
		io_name_t class;
		IOObjectGetClass(child, class);
		if (strcmp(class, "AGXDeviceUserClient") == 0) {
			CFTypeRef creator = IORegistryEntryCreateCFProperty(child, CFSTR("IOUserClientCreator"), kCFAllocatorDefault, 0);
			CFTypeRef usage = IORegistryEntryCreateCFProperty(child, CFSTR("AppUsage"), kCFAllocatorDefault, 0);
			int pid = creator_pid(creator);
			if (pid > 0) {
				clients[n].pid = pid;
				clients[n].gpu_ns = gpu_ns(usage);
				n++;
			}
			if (creator) CFRelease(creator);
			if (usage) CFRelease(usage);
		}
		IOObjectRelease(child);
	}
	IOObjectRelease(children);
	IOObjectRelease(accelerator);
	return n;
}

static int pid_rusage(int pid, struct rusage_info_v6 *info) {
	return proc_pid_rusage(pid, RUSAGE_INFO_V6, (rusage_info_t *)info);
}

// The threads of a process, or -1.
static int pid_threads(int pid) {
	struct proc_taskinfo info;
	if (proc_pidinfo(pid, PROC_PIDTASKINFO, 0, &info, sizeof info) != sizeof info) return -1;
	return info.pti_threadnum;
}

// Counts the open descriptors of a process, and of those the files and the sockets.
// Returns -1 when the process cannot be read.
static int pid_descriptors(int pid, int *files, int *sockets) {
	int size = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, NULL, 0);
	if (size <= 0) return -1;
	struct proc_fdinfo *fds = malloc(size);
	if (fds == NULL) return -1;
	size = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, fds, size);
	if (size <= 0) {
		free(fds);
		return -1;
	}
	int n = size / (int)sizeof *fds;
	*files = *sockets = 0;
	for (int i = 0; i < n; i++) {
		if (fds[i].proc_fdtype == PROX_FDTYPE_VNODE) (*files)++;
		if (fds[i].proc_fdtype == PROX_FDTYPE_SOCKET) (*sockets)++;
	}
	free(fds);
	return n;
}
*/
import "C"

import (
	"errors"
	"math"
)

const maxGPUClients = 4096

// gpuSecondsByPID returns the GPU time each process has used through its open GPU clients.
// A process with no open client is absent.
func gpuSecondsByPID() (map[int]float64, error) {
	clients := make([]C.gpu_client, maxGPUClients)
	n := int(C.gpu_clients(&clients[0], maxGPUClients))
	if n < 0 {
		return nil, errors.New("no AGXAccelerator in the IO registry")
	}
	seconds := make(map[int]float64, n)
	for _, c := range clients[:n] {
		seconds[int(c.pid)] += float64(c.gpu_ns) / 1e9
	}
	return seconds, nil
}

type resourceUsage struct {
	DiskReadBytes, DiskWrittenBytes, EnergyJoules, IdleWakeups, PageIns, FootprintBytes float64
}

// resourceUsageOf reads what macOS accounts to a process. Every field is NaN for a
// process of another user, since reading those needs root.
func resourceUsageOf(pid int) resourceUsage {
	var info C.struct_rusage_info_v6
	if C.pid_rusage(C.int(pid), &info) != 0 {
		nan := math.NaN()
		return resourceUsage{nan, nan, nan, nan, nan, nan}
	}
	return resourceUsage{
		DiskReadBytes:    float64(info.ri_diskio_bytesread),
		DiskWrittenBytes: float64(info.ri_diskio_byteswritten),
		EnergyJoules:     float64(info.ri_energy_nj) / 1e9,
		IdleWakeups:      float64(info.ri_pkg_idle_wkups),
		PageIns:          float64(info.ri_pageins),
		FootprintBytes:   float64(info.ri_phys_footprint),
	}
}

type openResources struct {
	Threads, Files, Sockets, Descriptors float64
}

// openResourcesOf counts the threads and open descriptors of a process. Each count is
// NaN where macOS refuses to read it, as it does for processes of other users.
func openResourcesOf(pid int) openResources {
	nan := math.NaN()
	r := openResources{nan, nan, nan, nan}
	if n := C.pid_threads(C.int(pid)); n >= 0 {
		r.Threads = float64(n)
	}
	var files, sockets C.int
	if n := C.pid_descriptors(C.int(pid), &files, &sockets); n >= 0 {
		r.Files, r.Sockets, r.Descriptors = float64(files), float64(sockets), float64(n)
	}
	return r
}
