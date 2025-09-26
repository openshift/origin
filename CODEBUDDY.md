# OpenShift Origin - CodeBuddy Guide

This repository maintains the `openshift-tests` binary for OpenShift, focusing on extended end-to-end testing. It previously also maintained `hyperkube` binaries, but that responsibility has transitioned to the `openshift/kubernetes` repository.

## Repository Purpose

- **Primary Purpose**: Maintains the `openshift-tests` binary for OKD (OpenShift Origin)
- **Branches**: 
  - `main` and `release-4.x` branches (4.6+): Only maintain `openshift-tests`
  - `release-4.5` and earlier: Continue to maintain hyperkube
- **Key Binary**: `openshift-tests` - compiled test binary containing all end-to-end tests

## Development Commands

### Building
```bash
# Build the main test binary
make

# Build specific target (openshift-tests)
make openshift-tests

# Alternative build method
hack/build-go.sh cmd/openshift-tests
```

### Testing
```bash
# Run all tests (uses conformance suite by default)
make test

# Run specific test suite
make test SUITE=core

# Run tests with focus
make test SUITE=conformance FOCUS=pods

# Run tools tests
make test-tools

# Run extended tests directly
openshift-tests run-test <FULL_TEST_NAME>
```

### Verification & Generation
```bash
# Run all verification checks
make verify

# Run origin-specific verification
make verify-origin

# Update all generated artifacts
make update

# Update TLS artifacts
make update-tls-ownership

# Update external examples
make update-examples

# Update bindata
make update-bindata
```

### Test Management
```bash
# List available test suites
openshift-tests help run

# Dry run to see test matches
openshift-tests run all --dry-run | grep -E "<REGEX>"

# Run filtered tests
openshift-tests run all --dry-run | grep -E "<REGEX>" | openshift-tests run -f -
```

## Key Dependencies and Vendoring

### Kubernetes Dependency
This repository vendors Kubernetes components from `openshift/kubernetes` fork:
- Uses custom `replace` directives in `go.mod` to point to OpenShift's fork
- Most staging repos (`k8s.io/api`, `k8s.io/apimachinery`, etc.) come from `openshift/kubernetes`

### Vendoring Updates
```bash
# Update Kubernetes vendor from openshift/kubernetes
hack/update-kube-vendor.sh <branch-name-or-SHA>

# Workaround for '410 Gone' errors
GOSUMDB=off hack/update-kube-vendor.sh <branch-name-or-SHA>

# Fake bump for unmerged changes
hack/update-kube-vendor.sh <branch-name> github.com/myname/kubernetes
```

## Architecture

### Directory Structure
- `cmd/openshift-tests/`: Main test binary entry point
- `pkg/`: Core packages including monitoring, test suites, and utilities
- `test/extended/`: End-to-end tests organized by functionality
- `hack/`: Build and development scripts
- `examples/`: Sample configurations and templates
- `vendor/`: Dependencies (mostly from OpenShift Kubernetes fork)

### Test Organization
- **Extended Tests**: Live under `test/extended/` with subdirectories by functionality
- **Test Labels**: Use Kubernetes e2e test conventions:
  - `[Serial]`: Tests that cannot run in parallel
  - `[Slow]`: Tests taking >5 minutes
  - `[Conformance]`: Core functionality tests
  - `[Local]`: Tests requiring local host access

### Key Components
- **Test Framework**: Uses Ginkgo for BDD-style testing
- **CLI Interface**: `exutil.NewCLI()` provides `oc` command simulation
- **Test Data**: JSON/YAML fixtures in `test/extended/testdata/`
- **Utilities**: Shared helpers in `test/extended/util/`

## Development Workflow

### Adding New Tests
1. Place tests in appropriate `test/extended/<category>/` directory
2. Use proper test labels (Serial, Slow, Conformance, Local)
3. Follow existing patterns using `exutil.NewCLI()` for `oc` commands
4. Use `test/extended/testdata/` for fixtures

### Test Exclusion Rules
- **Kubernetes e2e tests**: Managed in `openshift/kubernetes`
- **OpenShift e2e tests**: Managed in this repository (`pkg/test/extensions/`)

### Code Generation
- Uses `build-machinery-go` for build infrastructure
- Bindata generation for embedded examples and test data
- Automatic code generation via `make update`

## Build System

### Makefile Structure
- Uses `openshift/build-machinery-go` for standardized build targets
- Custom targets for origin-specific operations
- Image building support for test containers

### Environment Variables
- `JUNIT_REPORT=true`: Enable jUnit output for CI
- `IMAGE_REGISTRY=registry.ci.openshift.org`: Default image registry

## Testing Conventions

### Extended Test Structure
- Test files organized by functionality (builds, images, storage, etc.)
- Each test group can have custom launcher scripts if needed
- Use `g.Describe()` with descriptive bucket names

### CLI Integration
- Tests use `exutil.NewCLI("test-name")` to create CLI instances
- Chain commands: `oc.Run("create").Args("-f", fixture).Execute()`
- Use `By()` statements to document test steps

## Important Notes

- This repository is focused on **testing infrastructure** not production binaries
- Most changes to Kubernetes functionality should go to `openshift/kubernetes` first
- Test exclusion rules are split between this repo and `openshift/kubernetes`
- The `openshift-tests` binary is the primary output artifact

## Resources

- [Extended Test README](test/extended/README.md)
- [OpenShift Kubernetes Fork](https://github.com/openshift/kubernetes)
- [Test Exclusion Rules](pkg/test/extensions/)