# License and Clean-Room Policy

## Purpose

This document defines the license-safety rules for implementing and publishing
the Lustre Client Exporter as open source software.

It is an engineering policy, not legal advice. Before a public release, the
project owner should perform any required legal review.

## Intended Project License

The intended project license is Apache-2.0.

Every source file should include:

```text
SPDX-License-Identifier: Apache-2.0
```

The repository should include a root `LICENSE` file with the Apache-2.0 text
and a `NOTICE` file with project attribution.

## Existing Projects Reviewed

Two existing Lustre exporters were reviewed during design:

- `whamcloud/lustrefs-exporter`
- `GSI-HPC/lustre_exporter`

This project must not be a source fork of either project.

## GSI-HPC License Risk

`GSI-HPC/lustre_exporter` must be treated as mixed-license input. Some files
carry Apache-2.0 headers, while other files carry GPLv3 headers. Because of
that, this project must not copy, translate, or adapt GSI-HPC source code.

The safe use of GSI-HPC is limited to metric compatibility information:

- metric family names
- label keys
- expected label meanings
- Prometheus metric types
- high-level behavior needed by existing dashboards and alerts

The exporter must not reuse GSI-HPC implementation details.

## Allowed Inputs

Implementation may use:

- public Lustre procfs, sysfs, and debugfs paths
- public command behavior such as `lnetctl`
- metric family names used for compatibility
- label keys used for compatibility
- Prometheus metric types used for compatibility
- independently collected Lustre output samples
- hand-written synthetic fixtures based on public Lustre output formats
- high-level architectural ideas such as separating collection, parsing,
  mapping, and emission

## Prohibited Inputs

Implementation must not copy, translate, or adapt:

- GSI-HPC source code
- GSI-HPC parser structure
- GSI-HPC regular expressions
- GSI-HPC metric template maps
- GSI-HPC fixtures
- GSI-HPC dashboards
- GSI-HPC README or documentation prose
- GSI-HPC HELP strings copied verbatim
- GSI-HPC copyright headers
- GPL-covered snippets from any source

The same rule applies to whamcloud source code unless a future change
explicitly decides to reuse MIT-licensed code with proper attribution. The MVP
does not reuse whamcloud source code.

## Fixture Policy

Fixtures must be created in one of two ways:

- anonymized output collected from systems where the project has the right to
  use and publish the sample
- small hand-written samples that represent public Lustre output formats

Fixtures must not be copied from GSI-HPC or whamcloud repositories.

## Documentation Policy

Documentation may state that this exporter is compatible with selected
GSI-HPC metric names and labels.

Documentation must not copy GSI-HPC prose, tables, dashboards, or examples.

HELP strings should be original. Compatibility does not require HELP string
identity. Compatibility is defined by metric family names, labels, metric
types, and semantics.

## Dependency Policy

Direct dependencies must avoid strong copyleft licenses unless explicitly
approved before introduction.

CI should reject direct dependencies licensed under:

- GPL
- AGPL
- LGPL

Apache-2.0, MIT, BSD, ISC, and similarly permissive licenses are acceptable by
default.

## CI License Guards

CI should include checks that fail when:

- direct dependencies include GPL, AGPL, or LGPL licenses
- source files lack the expected Apache-2.0 SPDX header
- GSI-HPC or Hewlett Packard copyright headers appear in this repository
- GPL license headers appear in source files
- known dashboard JSON fragments are copied into the repository
- known fixture fragments from existing exporters are copied into the
  repository

The CI guard should allow this document to mention GPL conceptually. It should
not treat policy discussion as copied GPL code.

## Attribution

The project `NOTICE` file should include a short compatibility attribution such
as:

```text
This project emits selected metric names and labels compatible with
GSI-HPC/lustre_exporter for operational continuity. It does not include
GSI-HPC/lustre_exporter source code, fixtures, dashboards, or documentation.
```

The project may also mention that its architecture was informed by common
Prometheus exporter patterns and by general observations from existing Lustre
exporters, without copying their code.

## Release Checklist

Before publishing an OSS release:

1. Confirm that all source files have Apache-2.0 SPDX headers.
2. Confirm that the dependency license check passes.
3. Confirm that no copied GSI-HPC or GPL-covered code is present.
4. Confirm that fixtures are original or publishable.
5. Confirm that HELP strings are original.
6. Confirm that metric compatibility documentation does not copy GSI-HPC prose.
7. Confirm that the project license and NOTICE files are present.
