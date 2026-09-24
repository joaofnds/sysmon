package main

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <libproc.h>
#include <stdio.h>
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
*/
import "C"

import "errors"

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
	DiskReadBytes, DiskWrittenBytes, EnergyJoules float64
}

// resourceUsageOf reads what macOS accounts to a process. It fails for processes of
// other users, since that needs root.
func resourceUsageOf(pid int) (resourceUsage, bool) {
	var info C.struct_rusage_info_v6
	if C.pid_rusage(C.int(pid), &info) != 0 {
		return resourceUsage{}, false
	}
	return resourceUsage{
		DiskReadBytes:    float64(info.ri_diskio_bytesread),
		DiskWrittenBytes: float64(info.ri_diskio_byteswritten),
		EnergyJoules:     float64(info.ri_energy_nj) / 1e9,
	}, true
}
