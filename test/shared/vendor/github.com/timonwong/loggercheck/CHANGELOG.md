# Changelog

## [0.13.0](https://github.com/timonwong/loggercheck/compare/v0.12.1...v0.13.0) (2026-10-06)


### Features

* use ssa to filter out false-positive nil Stringer reports ([#124](https://github.com/timonwong/loggercheck/issues/124)) ([c2be25e](https://github.com/timonwong/loggercheck/commit/c2be25e169266758c56996a5463463abad4c9387))


### Dependencies

* bump golang.org/x/tools from 0.50.0 to 0.51.0 ([#127](https://github.com/timonwong/loggercheck/issues/127)) ([7a79f81](https://github.com/timonwong/loggercheck/commit/7a79f816c1cdbd6f5aaddc13364b5f68092472a8))

## [0.12.1](https://github.com/timonwong/loggercheck/compare/v0.12.0...v0.12.1) (2026-10-02)


### Bug Fixes

* limit nil-unsafe Stringer check to klog and logr ([#120](https://github.com/timonwong/loggercheck/issues/120)) ([ee0b376](https://github.com/timonwong/loggercheck/commit/ee0b376b1542070fbaca105b5b0c6ae9dddbe266))
