Close pull-requests
===================

This is a simple tool that closes pull-request that are being inactive for a
long time. This is done by checking comment activity, and events (like re-open).

It does print a warning before, so that it can get attention.

Usage
-----

One can call `./close-pull-requests --help` to get help, but here are the most
useful options:

- `--dry-run`: Try and see what would happen before actually changing everything
- `--stderrthreshold=0`: Makes various debug visible, so you see what happens
- `--token-file`: Specify your github token file (increases rate-limits)
- `--close-after`: Sets the limit (in hours) to use to close pull-requests
- `--warn-before`: Sets the limit (in hours) to start warning
- `--warn-reminder`: It will put the warning again after that many hours
