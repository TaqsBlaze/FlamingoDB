# FlamingoDB v1.3.1

Windows startup recovery can now truncate the write-ahead log. Stop FlamingoDB before installing the update and keep the existing data directory and WAL together.

- Compute numeric averages over a whole table or per group. Aggregate expressions work in projections, HAVING and ORDER BY. COUNT, SUM, MIN and MAX are also supported.
- Pass native VECTOR columns and array literals to NORM, DOT and CROSS. Existing bracketed text inputs remain supported. CROSS returns a vector when either argument is native.
- Follow the new 14-page technical guide from installation and first queries through dashboard imports, APIs, storage and recovery. It documents current limitations, including UPDATE reporting matched rows without persisting changes.

The release includes the Windows x64 installer and the technical guide as a PDF.
