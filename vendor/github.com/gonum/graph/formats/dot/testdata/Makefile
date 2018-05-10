# Dependencies:
#
# * imgcmp
#    go get github.com/mewkiz/cmd/imgcmp
# * dotfmt
#    go get github.com/graphism/dot/cmd/dotfmt
# * dot
#    sudo pacman -S graphviz
# * recode
#    sudo pacman -S recode

DOT=$(wildcard *.dot)

# Skip DOT files for which the generated PNG images mismatch.
#
# ref: https://github.com/graphism/dot/issues/2
#
#    pixel colors differ at x=550, y=1885
DOT:=$(filter-out b51.dot, $(DOT))
#    pixel colors differ at x=5395, y=1920
DOT:=$(filter-out b106.dot, $(DOT))

# Skip segfaulting files.
#
#    Segmentation fault (core dumped)
DOT:=$(filter-out b15.dot, $(DOT))
#    Segmentation fault (core dumped)
DOT:=$(filter-out b81.dot, $(DOT))
#    *** stack smashing detected ***: dot terminated
DOT:=$(filter-out sides.dot, $(DOT))
#    *** stack smashing detected ***: dot terminated
DOT:=$(filter-out tee.dot, $(DOT))

# Skip DOT files above 100 kB.
DOT:=$(filter-out 4elt.dot, $(DOT))
DOT:=$(filter-out b29.dot, $(DOT))
DOT:=$(filter-out b81.dot, $(DOT))
DOT:=$(filter-out b100.dot, $(DOT))
DOT:=$(filter-out b102.dot, $(DOT))
DOT:=$(filter-out b103.dot, $(DOT))
DOT:=$(filter-out b104.dot, $(DOT))
DOT:=$(filter-out root.dot, $(DOT))
DOT:=$(filter-out root_circo.dot, $(DOT))
DOT:=$(filter-out root_twopi.dot, $(DOT))

# Skip invalid DOT file.
#
#    Error: No or improper image file="eqn.png"
#    in label of node struct1
DOT:=$(filter-out html4.dot, $(DOT))

# Skip multi-graph DOT file which outputs to standard output.
DOT:=$(filter-out multi.dot, $(DOT))

# *.dot -> *.png
PNG=$(DOT:.dot=.png)

INPUT_PNG=$(addprefix input/,$(PNG))
OUTPUT_PNG=$(addprefix output/,$(PNG))

all:

test: input $(INPUT_PNG) output $(OUTPUT_PNG)
	@echo "PASS"

input:
	mkdir -p $@
	dot -V

input/%.png: %.dot
	dot -Tpng -o $@ $<

output:
	mkdir -p $@

output/%.png: %.dot
	dotfmt -o "output/$<" $<
	dot -Tpng -o $@ "output/$<"
	imgcmp "input/$(notdir $@)" $@

fetch: graphviz
	# Copy *.gv and *.dot files.
	find graphviz -type f -name '*.gv' -not -wholename "graphviz/rtest/share/b545.gv" -not -name "base.gv" | xargs -I '{}' cp "{}" .
	find graphviz -type f -name '*.dot' | xargs -I '{}' cp "{}" .

	# Rename *.gv to *.dot.
	#rename .gv .dot *.gv
	ls *.gv | xargs -I '{}' basename "{}" .gv | xargs -I '{}' mv "{}.gv" "{}.dot"

	# Remove execute permissions.
	chmod 0644 *.dot

	# Convert Latin1 encoded files to UTF-8.
	grep -l "charset=latin1" *.dot | xargs -I '{}' recode ISO-8859-1..UTF8 "{}"
	recode ISO-8859-1..UTF8 Latin1.dot

	# Clean up.
	rm -rf graphviz

graphviz:
	git clone https://github.com/ellson/graphviz.git

clean:
	rm -rf *.dot input output

.PHONY: all test fetch clean
