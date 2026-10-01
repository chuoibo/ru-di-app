"""The place catalogue behind `GET /places` (rd-be-05).

Split by what can be wrong with each:

* `catalog` -- seed data. Wrong here means a typo in a demo fixture.
* `scoring` -- deterministic arithmetic over that data. Wrong here means a
  number on screen nobody can reproduce, which the work item exists to prevent.

The third part, the model's sentence about each place, left this package with
ADR-0051: the Go core writes it (internal/aiharness/timquan), and nothing here
touches the network.
"""
