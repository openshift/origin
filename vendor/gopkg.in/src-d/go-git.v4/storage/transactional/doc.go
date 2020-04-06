// Package transactional is a transactional implementation of git.Storer, it
// demux the write and read operation of two separate storers, allowing to merge
// content calling Storage.Commit.
//
// The API and functionality of this package are considered EXPERIMENTAL and is
// not considered stable nor production ready.
package transactional
