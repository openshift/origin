# Gonum Graph [![Build Status](https://travis-ci.org/gonum/graph.svg?branch=master)](https://travis-ci.org/gonum/graph) [![Coverage Status](https://coveralls.io/repos/gonum/graph/badge.svg?branch=master&service=github)](https://coveralls.io/github/gonum/graph?branch=master) [![GoDoc](https://godoc.org/github.com/gonum/graph?status.svg)](https://godoc.org/github.com/gonum/graph)

# This repository is no longer maintained. Development has moved to https://github.com/gonum/gonum.

This is a generalized graph package for the Go language. It aims to provide a clean, transparent API for common algorithms on arbitrary graphs such as finding the graph's strongly connected components, dominators, or searces.

The package is currently in testing, and the API is "semi-stable". The signatures of any functions like AStar are unlikely to change much, but the Graph, Node, and Edge interfaces may change a bit.

## Issues

If you find any bugs, feel free to file an issue on the github [issue tracker for gonum/gonum](https://github.com/gonum/gonum/issues) if the bug exists in that reposity; no code changes will be made to this repository. Other dicussions should be taken to the gonum-dev Google Group.

https://groups.google.com/forum/#!forum/gonum-dev

## License

Please see github.com/gonum/license for general license information, contributors, authors, etc on the Gonum suite of packages.
