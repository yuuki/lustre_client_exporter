# Security policy

lustre_client_exporter usually runs as root on Lustre client nodes (debugfs is
readable by root only). It reads procfs, sysfs and debugfs, runs `lnetctl` and,
when enabled, `lpcc` at the paths given on the command line, and serves the
metrics over plain HTTP. Security issues in that position are taken seriously,
for instance:

- a crash, hang or unbounded memory or goroutine growth caused by the content
  of a Lustre file or of `lnetctl`/`lpcc` output;
- a way for a network client to make the exporter execute commands, read
  files other than the Lustre sources, or stop serving other scrapes;
- a scrape that exceeds its configured timeouts without bound.

Not vulnerabilities: what the operator controls (command-line flags such as
`-path.lnetctl` or `-path.rootfs`), and the lack of TLS and authentication,
which is documented (`-web.config.file` is not supported): restrict access to
the listen address with network controls.

## Supported versions

Security fixes are made on the `main` branch and released in the next
version. Only the latest release is supported.

## Reporting a vulnerability

Please do **not** open a public issue, pull request or discussion.

Report the vulnerability privately with GitHub's
[private vulnerability reporting](https://github.com/yuuki/lustre_client_exporter/security/advisories/new)
(the "Report a vulnerability" button of the Security tab). If that is not
possible, contact the author privately via the contact information on
https://github.com/yuuki.

Please include the exporter version (`lustre_client_exporter -version`), the
command line, the Lustre version, what an attacker controls (a file under
`/proc/fs/lustre`, the `lnetctl` output, network access to the exporter, ...),
a reproducer if possible, and the impact. You should receive an answer within a
few weeks; the project is maintained on a best-effort basis.
