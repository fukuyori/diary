# Changelog

All notable changes to this project will be documented in this file.

## [0.9.3] - 2026-04-19

### Added

- Added `-A` to append text to an existing entry for the same date.
- Appending uses `" / "` as the separator when an entry already exists.
- If no entry exists for the target date, `-A` creates a new entry with the given text.

### Documentation

- Updated the help text and README files to describe `-A`.

## [0.9.2] - 2026-04-15

### Added

- Added listing by year and month with `-m YYYY-MM`.
- Added case-insensitive search with `-s`.
- Added interactive narrowing search with `-i`.
- Added automatic backups on add, update, and delete.
- Added manual backup creation with `-b`.
- Added restore from backup with `-R`.
- Added restore confirmation that requires typing `diary`.

## [0.9.1] - 2026-04-13

### Added

- Added a Japanese README.
- Added prebuilt release archives for multiple platforms.

### Changed

- Refined project documentation and packaging for release use.

## [0.9.0] - 2026-03-26

### Added

- Initial release.
- Added one-line diary entry storage in JSONL format.
- Added per-date add or update behavior with `-a`.
- Added recent entry listing with `-l`.
- Added reverse listing with `-r`.
- Added numbered listing with `-n`.
- Added entry deletion by serial ID with `-d`.
- Added TOML-based configuration with configurable data path and max length.
