#pragma once

typedef unsigned int uint32_t;
typedef unsigned long uint64_t;

#define __section(NAME) __attribute__((section(NAME), used))
#define __uint(name, val) int (*name)[val]
#define __type(name, val) typeof(val) *name

#define BPF_MAP_TYPE_ARRAY (1)
#define BPF_MAP_TYPE_PERF_EVENT_ARRAY (4)
#define BPF_MAP_TYPE_ARRAY_OF_MAPS (12)
#define BPF_MAP_TYPE_HASH_OF_MAPS (13)

#define BPF_F_NO_PREALLOC (1U << 0)
#define BPF_F_CURRENT_CPU (0xffffffffULL)

/* From tools/lib/bpf/libbpf.h */
struct bpf_map_def {
	unsigned int type;
	unsigned int key_size;
	unsigned int value_size;
	unsigned int max_entries;
	unsigned int map_flags;
};

static void* (*map_lookup_elem)(const void *map, const void *key) = (void*)1;
static int (*perf_event_output)(const void *ctx, const void *map, uint64_t index, const void *data, uint64_t size) = (void*)25;
static uint32_t (*get_smp_processor_id)(void) = (void*)8;
