GOMD2MAN = ../tests/tools/build/go-md2man
PREFIX ?= ${DESTDIR}/usr
MANINSTALLDIR=${PREFIX}/share/man
MANPAGES_MD = $(wildcard docs/*.5.md)
MANPAGES ?= $(MANPAGES_MD:%.md=%)

docs: $(patsubst %.md,%.1,$(filter-out %.5.md,$(wildcard *.md))) containers-storage.conf.5

%.1: %.md
	$(GOMD2MAN) -in $^ -out $@

containers-storage.conf.5: containers-storage.conf.5.md
	$(GOMD2MAN) -in $^ -out $@

install:
	install -d -m 755 ${MANINSTALLDIR}/man5
	install -m 644 *.5 ${MANINSTALLDIR}/man5/
