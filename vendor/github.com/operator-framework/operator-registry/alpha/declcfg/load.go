package declcfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"sync"

	"github.com/joelanford/ignore"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/api/pkg/operators"

	"github.com/operator-framework/operator-registry/alpha/property"
)

const (
	indexIgnoreFilename = ".indexignore"
)

type WalkMetasFSFunc func(path string, meta *Meta, err error) error

// WalkMetasFS walks the filesystem rooted at root and calls walkFn for each individual meta object found in the root.
// By default, WalkMetasFS is not thread-safe because it invokes walkFn concurrently. In order to make it thread-safe,
// use the WithConcurrency(1) to avoid concurrent invocations of walkFn.
func WalkMetasFS(ctx context.Context, root fs.FS, walkFn WalkMetasFSFunc, opts ...LoadOption) error {
	if root == nil {
		return fmt.Errorf("no declarative config filesystem provided")
	}

	options := LoadOptions{
		concurrency: runtime.NumCPU(),
	}
	for _, opt := range opts {
		opt(&options)
	}

	pathChan := make(chan string, options.concurrency)

	// Create an errgroup to manage goroutines. The context is closed when any
	// goroutine returns an error. Goroutines should check the context
	// to see if they should return early (in the case of another goroutine
	// returning an error).
	eg, ctx := errgroup.WithContext(ctx)

	// Walk the FS and send paths to a channel for parsing.
	eg.Go(func() error {
		return sendPaths(ctx, root, pathChan)
	})

	// Parse paths concurrently. The waitgroup ensures that all paths are parsed
	// before the cfgChan is closed.
	for i := 0; i < options.concurrency; i++ {
		eg.Go(func() error {
			return parseMetaPaths(ctx, root, pathChan, walkFn, options)
		})
	}
	return eg.Wait()
}

type WalkMetasReaderFunc func(meta *Meta, err error) error

func WalkMetasReader(r io.Reader, walkFn WalkMetasReaderFunc) error {
	dec := yaml.NewYAMLOrJSONDecoder(r, 4096)
	for {
		var in Meta
		if err := dec.Decode(&in); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return walkFn(nil, err)
		}

		if err := walkFn(&in, nil); err != nil {
			return err
		}
	}
	return nil
}

type WalkFunc func(path string, cfg *DeclarativeConfig, err error) error

// WalkFS walks root using a gitignore-style filename matcher to skip files
// that match patterns found in .indexignore files found throughout the filesystem.
// It calls walkFn for each declarative config file it finds. If WalkFS encounters
// an error loading or parsing any file, the error will be immediately returned.
func WalkFS(root fs.FS, walkFn WalkFunc) error {
	return walkFiles(root, func(root fs.FS, path string, err error) error {
		if err != nil {
			return walkFn(path, nil, err)
		}

		cfg, err := LoadFile(root, path)
		if err != nil {
			return walkFn(path, cfg, err)
		}

		return walkFn(path, cfg, nil)
	})
}

func walkFiles(root fs.FS, fn func(root fs.FS, path string, err error) error) error {
	if root == nil {
		return fmt.Errorf("no declarative config filesystem provided")
	}

	matcher, err := ignore.NewMatcher(root, indexIgnoreFilename)
	if err != nil {
		return err
	}

	return fs.WalkDir(root, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return fn(root, path, err)
		}
		// avoid validating a directory, an .indexignore file, or any file that matches
		// an ignore pattern outlined in a .indexignore file.
		if info.IsDir() || info.Name() == indexIgnoreFilename || matcher.Match(path, false) {
			return nil
		}

		return fn(root, path, nil)
	})
}

type LoadOptions struct {
	concurrency int
}

type LoadOption func(*LoadOptions)

func WithConcurrency(concurrency int) LoadOption {
	return func(opts *LoadOptions) {
		opts.concurrency = concurrency
	}
}

// LoadFS loads a declarative config from the provided root FS. LoadFS walks the
// filesystem from root and uses a gitignore-style filename matcher to skip files
// that match patterns found in .indexignore files found throughout the filesystem.
// If LoadFS encounters an error loading or parsing any file, the error will be
// immediately returned.
func LoadFS(ctx context.Context, root fs.FS, opts ...LoadOption) (*DeclarativeConfig, error) {
	builder := fbcBuilder{}
	if err := WalkMetasFS(ctx, root, func(path string, meta *Meta, err error) error {
		if err != nil {
			return err
		}
		return builder.addMeta(meta)
	}, opts...); err != nil {
		return nil, err
	}
	return &builder.cfg, nil
}

func sendPaths(ctx context.Context, root fs.FS, pathChan chan<- string) error {
	defer close(pathChan)
	return walkFiles(root, func(_ fs.FS, path string, err error) error {
		if err != nil {
			return err
		}
		select {
		case pathChan <- path:
		case <-ctx.Done(): // don't block on sending to pathChan
			return ctx.Err()
		}
		return nil
	})
}

func parseMetaPaths(ctx context.Context, root fs.FS, pathChan <-chan string, walkFn WalkMetasFSFunc, _ LoadOptions) error {
	for {
		select {
		case <-ctx.Done(): // don't block on receiving from pathChan
			return ctx.Err()
		case path, ok := <-pathChan:
			if !ok {
				return nil
			}
			err := func() error { // using closure to ensure file is closed immediately after use
				file, err := root.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				return WalkMetasReader(file, func(meta *Meta, err error) error {
					return walkFn(path, meta, err)
				})
			}()
			if err != nil {
				return err
			}
		}
	}
}

func readBundleObjects(b *Bundle) error {
	var obj property.BundleObject
	for i, props := range b.Properties {
		if props.Type != property.TypeBundleObject {
			continue
		}
		if err := json.Unmarshal(props.Value, &obj); err != nil {
			return fmt.Errorf("package %q, bundle %q: parse property at index %d as bundle object: %v", b.Package, b.Name, i, err)
		}
		objJSON, err := yaml.ToJSON(obj.Data)
		if err != nil {
			return fmt.Errorf("package %q, bundle %q: convert bundle object property at index %d to JSON: %v", b.Package, b.Name, i, err)
		}
		b.Objects = append(b.Objects, string(objJSON))
	}
	b.CsvJSON = extractCSV(b.Objects)
	return nil
}

func extractCSV(objs []string) string {
	for _, obj := range objs {
		u := unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(obj), &u); err != nil {
			continue
		}
		if u.GetKind() == operators.ClusterServiceVersionKind {
			return obj
		}
	}
	return ""
}

// LoadReader reads yaml or json from the passed in io.Reader and unmarshals it into a DeclarativeConfig struct.
func LoadReader(r io.Reader) (*DeclarativeConfig, error) {
	builder := fbcBuilder{}
	if err := WalkMetasReader(r, func(meta *Meta, err error) error {
		if err != nil {
			return err
		}
		return builder.addMeta(meta)
	}); err != nil {
		return nil, err
	}
	return &builder.cfg, nil
}

// LoadFile will unmarshall declarative config components from a single filename provided in 'path'
// located at a filesystem hierarchy 'root'
func LoadFile(root fs.FS, path string) (*DeclarativeConfig, error) {
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg, err := LoadReader(file)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadSlice will compose declarative config components from a slice of Meta objects
func LoadSlice(metas []*Meta) (*DeclarativeConfig, error) {
	builder := fbcBuilder{}
	for _, meta := range metas {
		if err := builder.addMeta(meta); err != nil {
			return nil, err
		}
	}
	return &builder.cfg, nil
}

type fbcBuilder struct {
	cfg DeclarativeConfig

	packagesMu     sync.Mutex
	channelsMu     sync.Mutex
	bundlesMu      sync.Mutex
	deprecationsMu sync.Mutex
	othersMu       sync.Mutex
}

func (c *fbcBuilder) addMeta(in *Meta) error {
	switch in.Schema {
	case SchemaPackage:
		var p Package
		if err := json.Unmarshal(in.Blob, &p); err != nil {
			return fmt.Errorf("parse package: %v", err)
		}
		c.packagesMu.Lock()
		c.cfg.Packages = append(c.cfg.Packages, p)
		c.packagesMu.Unlock()
	case SchemaChannel:
		var ch Channel
		if err := json.Unmarshal(in.Blob, &ch); err != nil {
			return fmt.Errorf("parse channel: %v", err)
		}
		c.channelsMu.Lock()
		c.cfg.Channels = append(c.cfg.Channels, ch)
		c.channelsMu.Unlock()
	case SchemaBundle:
		var b Bundle
		if err := json.Unmarshal(in.Blob, &b); err != nil {
			return fmt.Errorf("parse bundle: %v", err)
		}
		if err := readBundleObjects(&b); err != nil {
			return fmt.Errorf("read bundle objects: %v", err)
		}
		c.bundlesMu.Lock()
		c.cfg.Bundles = append(c.cfg.Bundles, b)
		c.bundlesMu.Unlock()
	case SchemaDeprecation:
		var d Deprecation
		if err := json.Unmarshal(in.Blob, &d); err != nil {
			return fmt.Errorf("parse deprecation: %w", err)
		}
		c.deprecationsMu.Lock()
		c.cfg.Deprecations = append(c.cfg.Deprecations, d)
		c.deprecationsMu.Unlock()
	case "":
		return fmt.Errorf("object '%s' is missing root schema field", string(in.Blob))
	default:
		c.othersMu.Lock()
		c.cfg.Others = append(c.cfg.Others, *in)
		c.othersMu.Unlock()
	}
	return nil
}
