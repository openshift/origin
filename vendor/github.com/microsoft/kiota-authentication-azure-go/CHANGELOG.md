# Changelog

All notable changes to this project will be documented in this file.

## [1.3.0](https://github.com/microsoft/kiota-authentication-azure-go/compare/v1.2.1...v1.3.0) (2025-04-02)


### Features

* add support for Continous Access Evaluation (CAE) ([e33d461](https://github.com/microsoft/kiota-authentication-azure-go/commit/e33d46169c9a88a463a20c6cee61eedcaa83099c))


### Bug Fixes

* Don't error on CAE claims ([e33d461](https://github.com/microsoft/kiota-authentication-azure-go/commit/e33d46169c9a88a463a20c6cee61eedcaa83099c))
* removes common go dependency ([f58f8df](https://github.com/microsoft/kiota-authentication-azure-go/commit/f58f8df7ded784abbb66030eaf6b26c08c078696))
* removes common go dependency ([b05ac0b](https://github.com/microsoft/kiota-authentication-azure-go/commit/b05ac0bb228526bc56a9d2172ffb975e7ca24f73))

## [1.2.1](https://github.com/microsoft/kiota-authentication-azure-go/compare/v1.2.0...v1.2.1) (2025-03-24)


### Bug Fixes

* upgrades common go dependency to solve triming issues ([5ab48d3](https://github.com/microsoft/kiota-authentication-azure-go/commit/5ab48d33da0a32efcbd44c75a43d8e1d4cc7e0ff))
* upgrades common go dependency to solve triming issues ([9360b98](https://github.com/microsoft/kiota-authentication-azure-go/commit/9360b98797f0d00bc31fc3bbe17af772479da5af))

## [1.2.0](https://github.com/microsoft/kiota-authentication-azure-go/compare/v1.1.0...v1.2.0) (2025-03-13)


### Features

* upgrades required go version from go1.18 to go 1.22 ([35f8bd7](https://github.com/microsoft/kiota-authentication-azure-go/commit/35f8bd73366e25d6ba19b2e2a19056a2baae0356))

## [1.1.0] - 2024-08-08

### Changed

- Continuous Access Evaluation is now enabled by default.

## [1.0.2] - 2024-01-19

### Changed

- Validates that provided valid hosts don't start with a scheme.

## [1.0.1] - 2023-10-13

### Changed

- Allow http on localhost.

## [1.0.0] - 2023-05-04

### Changed

- GA Release.

## [0.6.0] - 2023-01-17

### Changed

- Removes the Microsoft Graph specific default values.

## [0.5.0] - 2022-09-27

### Added

- Added tracing through OpenTelemetry.

## [0.4.1] - 2022-09-02

### Changed

- Upgraded abstractions and yaml dependencies.

### Changed

## [0.4.0] - 2022-08-31

### Changed

- Pass `context.Context` for on `GetAuthorizationToken` method.

## [0.3.1] - 2022-06-07

### Changed

- Upgraded abstractions and yaml dependencies.

## [0.3.0] - 2022-05-18

### Added

- Added preliminary work to support continuous access evaluation.

## [0.2.1] - 2022-04-19

### Changed

- Upgraded abstractions to 0.4.0.

## [0.2.0] - 2022-04-18

### Changed

- Bumped required go version to 1.18 as Azure Identity now requires it.

## [0.1.0] - 2022-03-30

### Added

- Initial tagged release of the library.
