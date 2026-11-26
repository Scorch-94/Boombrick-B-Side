# Working on Boombrick B-Side

Boombrick B-Side is a fork of ujjwal313/boombrick for NextUI on the TrimUI Brick.

## Every change

- **README.md:** when a change adds a major feature or changes one, update the README in the same commit:
  - New features go under "What B-Side adds", with a section of their own further down if they need explaining.
  - Changes to how something worked in the original go under "Smaller changes".
  - Controls, files and settings changes go in their tables.
- **CHANGELOG.md and pak.json:** add the change under the next unreleased version. A version that already has a GitHub release is closed, so start the next one, and set `pak.json`'s `version` to match.
