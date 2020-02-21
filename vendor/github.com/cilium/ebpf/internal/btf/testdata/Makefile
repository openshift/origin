# Usage: make KDIR=/path/to/foo

vmlinux-btf.gz: $(KDIR)/vmlinux
	objcopy --dump-section .BTF=/dev/stdout "$<" | gzip > "$@"
