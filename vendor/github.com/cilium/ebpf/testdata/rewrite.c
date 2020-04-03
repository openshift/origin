/* This file tests rewriting constants from C compiled code.
 */

#include "common.h"

char __license[] __section("license") = "MIT";

struct bpf_map_def map_val __section("maps") = {
	.type        = 1,
	.key_size    = sizeof(unsigned int),
	.value_size  = sizeof(unsigned int),
	.max_entries = 1,
};

#define CONSTANT "constant"

#define LOAD_CONSTANT(param, var) asm("%0 = " param " ll" : "=r"(var))

__section("socket") int rewrite() {
	unsigned long acc = 0;
	LOAD_CONSTANT(CONSTANT, acc);
	return acc;
}

__section("socket/map") int rewrite_map() {
	unsigned int key = 0;
	unsigned int *value = map_lookup_elem(&map_val, &key);
	if (!value) {
		return 0;
	}
	return *value;
}
