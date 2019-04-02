Migrillian Tool
===============

*Status: in development, not ready for production use*

**Migrillian** is a tool that transfers data from existing Certificate
Transparency logs to Trillian *PREORDERED_LOG* trees.

It can be used for:
 - One-off data migrations, e.g. from legacy CT implementation to the new
   Trillian-based [solution](https://github.com/google/certificate-transparency-go).
 - Continuous migration for keeping the copy up-to-date with the remote log,
   i.e. log mirroring.
