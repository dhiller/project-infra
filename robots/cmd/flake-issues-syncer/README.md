# flake-issues-syncer

Using the json data that is created when the flakefinder runs, creates, updates and closes issues from flakefinder data.

## Commands

### `create`

Takes the latest x flakes from the flakefinder data and for each of them creates a new issue if it doesn't exist

### `update`

Takes the latest x flakes from the flakefinder data updates the existing issues with the new data.

### `close`

Fetches the existing issues and closes any issue that is not seen in the flakefinder data any more.

### `sync`

Creates, updates and closes issues accordingly to each of the above commands.
