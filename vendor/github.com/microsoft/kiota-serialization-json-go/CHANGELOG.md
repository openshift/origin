# Changelog

All notable changes to this project will be documented in this file.

## [1.1.2](https://github.com/microsoft/kiota-serialization-json-go/compare/v1.1.1...v1.1.2) (2025-04-02)


### Bug Fixes

* removes common go dependency ([93eee3a](https://github.com/microsoft/kiota-serialization-json-go/commit/93eee3ae3298de814ea2a4d034a65aa925ccef41))
* removes common go dependency ([5c4a75c](https://github.com/microsoft/kiota-serialization-json-go/commit/5c4a75c9a22d47d2ae4ef82016526324f268169e))

## [1.1.1](https://github.com/microsoft/kiota-serialization-json-go/compare/v1.1.0...v1.1.1) (2025-03-24)


### Bug Fixes

* upgrades common go dependency to solve triming issues ([90291c0](https://github.com/microsoft/kiota-serialization-json-go/commit/90291c0a596279e3c45e2eb0cb814db19ce1df99))
* upgrades common go dependency to solve triming issues ([ce4848e](https://github.com/microsoft/kiota-serialization-json-go/commit/ce4848ed57426b7669e0587a6f2971cc923630ee))

## [1.1.0](https://github.com/microsoft/kiota-serialization-json-go/compare/v1.0.9...v1.1.0) (2025-03-13)


### Features

* upgrades required go version from go1.18 to go 1.22 ([22ec699](https://github.com/microsoft/kiota-serialization-json-go/commit/22ec69982c170452e7aa565465f4729effbe9695))

## [1.0.9](https://github.com/microsoft/kiota-serialization-json-go/compare/v1.0.8...v1.0.9) (2024-12-17)


### Bug Fixes

* changelog cleanup ([1c16092](https://github.com/microsoft/kiota-serialization-json-go/commit/1c1609263beb65dedf4793304411aa22e8cf768e))

## [1.0.8] - 2024-08-13

### Changed

- Modified how number values are derived, allowing values to be cast as the various types.

### Fixed

- Panicing when type is asserted to be what it isn't.

## [1.0.7] - 2024-02-29

### Added

- Adds support for serialization and deserialization untyped nodes.

## [1.0.6] - 2024-02-12

### Changed

- Fixes serilaization of `null` values in collections of Objects.

## [1.0.5] - 2024-01-10

### Changed

- Fixes some special character escaping when serializing strings to JSON. Previous incorrect escaping could lead to deserialization errors if old serialized data is read again.

## [1.0.4] - 2023-07-12

### Changed

- Fixes parsing time parsing without timezone information.

## [1.0.3] - 2023-06-28

### Changed

- Fixes serialization of composed types for scalar values.

## [1.0.2] - 2023-06-14

- Safely serialize null values in collections of Objects, Enums or primitives.

### Changed

## [1.0.1] - 2023-05-25

- Fixes bug where slices backing data from `GetSerializedContent` could be overwritten before they were used but after `JsonSerializationWriter.Close()` was called.

### Added

## [1.0.0] - 2023-05-04

### Changed

- GA Release.

## [0.9.3] - 2023-04-24

### Changed

- Use buffer pool for `JsonSerializationWriter`.

## [0.9.2] - 2023-04-17

### Changed

- Improve `JsonSerializationWriter` serialization performance.

## [0.9.1] - 2023-04-05

### Added

- Improve error messaging for serialization error.

## [0.9.0] - 2023-03-30

### Added

- Add Unmarshal and Marshal helpers.

## [0.8.3] - 2023-03-20

### Added

- Validates json content before parsing.

## [0.8.2] - 2023-03-01

### Added

- Fixes bug that returned `JsonParseNode` as value for nested maps when `GetRawValue` is called.

## [0.8.1] - 2023-02-20

### Added

- Fixes bug that returned `JsonParseNode` as value for collections when `GetRawValue` is called.

## [0.8.0] - 2023-01-23

### Added

- Added support for backing store.

## [0.7.2] - 2022-09-29

### Changed

- Fix: Bug on GetRawValue results to invalid memory address when server responds with a `null` on the request body field.

## [0.7.1] - 2022-09-26

### Changed

- Fixed method name for write any value.

## [0.7.0] - 2022-09-22

### Added

- Implement additional serialization method `WriteAnyValues` and parse method `GetRawValue`.

## [0.6.0] - 2022-09-02

### Added

- Added support for composed types serialization.

## [0.5.6] - 2022-09-02

- Upgrades abstractions and yaml dependencies.

### Added

## [0.5.5] - 2022-07-12

- Fixed bug where string literals of `\t` and `\r` would result in generating an invalid JSON.

### Changed

## [0.5.4] - 2022-06-30

### Changed

- Fixed a bug where a backslash in a string would result in an invalid payload.

## [0.5.3] - 2022-06-09

### Changed

- Fixed a bug where new lines in string values would not be escaped generating invalid JSON.

## [0.5.2] - 2022-06-07

### Changed

- Upgrades abstractions and yaml dependencies.

## [0.5.1] - 2022-05-30

### Changed

- Updated supported types for Additional Data, unsupported types now throwing an error instead of ignoring.
- Changed logic that trims excessive commas to be called only once on serialization.

## [0.5.0] - 2022-05-26

### Changed

- Updated reference to abstractions to support enum responses.

## [0.4.0] - 2022-05-19

### Changed

- Upgraded abstractions version.

## [0.3.2] - 2022-05-11

### Changed

- Serialization writer close method now clears the internal array and can be used to reset the writer.

## [0.3.1] - 2022-05-03

### Changed

- Fixed an issue where quotes in string values would not be escaped. #11
- Fixed an issue where int64 and byte values would get a double key. #12, #13

## [0.3.0] - 2022-04-19

### Changed

- Upgraded abstractions to 0.4.0.
- Upgraded to go 18.

## [0.2.1] - 2022-04-14

### Changed

- Fixed a bug where dates, date only, time only and duration would not serialize properly.

## [0.2.0] - 2022-04-04

### Changed

- Breaking: simplifies the field deserializers.

## [0.1.0] - 2022-03-30

### Added

- Initial tagged release of the library.
