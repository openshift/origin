# Contributing to service-catalog

This document should concisely express the project development status,
methodology, and contribution process.  As the community makes progress, we
should keep this document in sync with reality.

## Submitting a Pull Request (PR)

The following outlines the general rules we follow:

- All PRs must have the appropriate documentation changes made within the
same PR. Note, not all PRs will necessarily require a documentation change
but if it does please include it in the same PR so the PR is complete and
not just a partial solution.
- All PRs must have the appropriate testcases. For example, bug fixes should
include tests that demonstrates the issue w/o your fix. New features should
include as many testcases, within reason, to cover any variants of use of the
feature.
- PR authors will need to have CLA on-file with the Linux Foundation before 
the PR will be merged.
See Kubernete's [contributing guidelines](https://github.com/kubernetes/kubernetes/blob/master/CONTRIBUTING.md) for more information.

See our [reviewing PRs](REVIEWING.md) documentation for how your PR will
be reviewed.

## Development status

We're currently collecting use-cases and requirements for our [v1 milestone](./docs/v1).

## Methodology

Each milestone will have a directory within the [`docs`](./docs) directory of
this project.   We will keep a complete record of all supported use-cases and
major designs for each milestone.

## Contributing to a release

If you would like to propose or change a use-case, open a pull request to the
project, adding or altering a file within the `docs` directory.

We'll update this space as we begin developing code with relevant dev
information.
