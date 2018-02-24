include ../Makefile.inc

src = $(wildcard *.c)
obj = $(src:.c=.o)

override LIBS += $(shell pkg-config --libs glib-2.0)

VERSION = $(shell sed -n -e 's/^const Version = "\([^"]*\)"/\1/p' ../version/version.go)

override CFLAGS += -std=c99 -Os -Wall -Wextra $(shell pkg-config --cflags glib-2.0) -DVERSION=\"$(VERSION)\" -DGIT_COMMIT=\"$(GIT_COMMIT)\"

conmon: $(obj)
	$(CC) -o ../bin/$@ $^ $(CFLAGS) $(LIBS)

.PHONY: clean
clean:
	rm -f $(obj) ../bin/conmon
