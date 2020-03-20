LLVM_PREFIX ?= /usr/bin
CLANG ?= $(LLVM_PREFIX)/clang

all: loader-clang-6.0.elf loader-clang-7.elf loader-clang-8.elf rewrite.elf invalid_map.elf

clean:
	-$(RM) *.elf

loader-%.elf: loader.c
	$* -target bpf -O2 -g \
		-Wall -Werror \
		-c $< -o $@

%.elf : %.c
	$(CLANG) -target bpf -O2 -g \
		-Wall -Werror \
		-c $< -o $@

