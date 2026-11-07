# Contributing

Thanks for your interest in lustre_client_exporter. Bug reports, fixes and new
client-side metrics are welcome.

- **Propose a change**: open an issue first for anything larger than a small
  fix, especially a new metric or flag, so that it can be checked against the
  GSI-HPC compatible contract and the client-only scope. Security issues go
  through [SECURITY.md](SECURITY.md), not public issues.
- **Clean room**: do not copy, translate or adapt code, regular expressions,
  fixtures or prose from GSI-HPC/lustre_exporter or other exporters. See
  [docs/license-policy.md](docs/license-policy.md).
- **Fixtures**: a parser change comes with a fixture under `testdata/`,
  hand-written or captured from a real client with hostnames, NIDs and
  filesystem names anonymized.
- **Before you open a pull request**, run:

  ```sh
  make check
  ```

  It runs the linters, the workflow audit, the tests with the race detector
  and the build. When you change a parser, also run `make test-fuzz` and
  `make bench`. [AGENTS.md](AGENTS.md) has the conventions.
- **Docs**: a change to metrics, flags or failure behaviour updates `README.md`
  and `docs/metrics.md` (and `docs/design.md` where it applies).
- **Commits, branches and pull request titles** follow the Git conventions
  in [AGENTS.md](AGENTS.md#git-conventions).

By contributing you agree that your contribution is licensed under the
Apache License 2.0.
