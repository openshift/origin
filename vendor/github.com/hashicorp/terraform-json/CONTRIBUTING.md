# Contributing to terraform-json

## Versioning

The `github.com/hashicorp/terraform-json` Go module in its entirety is versioned according to [Go module versioning](https://golang.org/ref/mod#versions) with Git tags.

There is currently no firm plan for releasing v1.

## Releases

Releases are made on a reasonably regular basis by the Terraform team, using our custom CI workflows. There is currently no set release schedule and no requirement for _contributors_ to write changelog entries.

The following notes are only relevant to maintainers.

[Create new release](https://github.com/hashicorp/terraform-json/releases/new) via GitHub UI to point to the new tag and use GitHub to generate the changelog (`Generate release notes` button).

You can format the generated changelog before publishing - e.g. ensure entries are grouped into categories such as `ENHANCEMENTS`, `BUG FIXES` and `INTERNAL`.

## Security vulnerabilities

Please disclose security vulnerabilities by following the procedure
described at https://www.hashicorp.com/security#vulnerability-reporting.
