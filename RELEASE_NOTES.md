# FlamingoDB v1.3.0 "Plumage" release notes

FlamingoDB 1.3.0 expands the web interface, adds database import options and a Windows installer, and improves AI-assisted analysis. This release also fixes SQL pagination and makes documentation available before sign-in.

## Web interface and data import

- Import CSV, TSV, and flat JSON files into a new table or append them to an existing table. Review the preview, column mappings, and inferred `INT`, `FLOAT`, and `VARCHAR` types before importing.
- Import one table at a time from a SQLite database or a MySQL database. SQLite is read in the browser using a reader loaded from a CDN. MySQL connections are initiated by the server, require a FlamingoDB administrator, and use TLS by default; use a read-only MySQL account.
- The database importer supports nonempty tables of up to 10,000 rows. NULL and binary values are not supported, and indexes and constraints are not copied. SQLite files are limited to 32 MiB and MySQL source data to 16 MiB. Validate the mapped types, particularly for large integers and precision-sensitive values. An insertion error can leave previously inserted rows in the destination.
- Query History saves up to 30 statements and their results in the browser for the signed-in username. Selecting an entry restores the saved output without executing the query again.
- Expanded in-app documentation covers getting started, SQL, imports, users, policies, AI models, profiles, and engine internals. It can also be opened from the sign-in page.
- The `samples/` directory contains synthetic spatial, weather, scientific, and medical CSV datasets and an import guide.

## AI assistant and MCP

- The AI chat keeps multiple conversations per model, with a side panel for switching conversations. Existing browser-saved per-model histories are carried over. Conversation history remains in the browser, not on the server.
- New messages scroll into view. Chart handling supports bar, line, pie, doughnut, radar, polar-area, scatter, and bubble configurations, with visible defaults and improved handling of model-generated tool calls.
- Added `calculate_vector` for addition, subtraction, dot product, cross product, and norm, and `calculate_distance` for planar distance or approximate geographic distance in kilometers. Geographic distance uses a spherical haversine calculation.
- The MCP server reports version 1.3.0.

## SQL and Windows

- Fixed parsing of `LIMIT`, `OFFSET`, and `ORDER BY` expressions and preserved result column names through pagination. For consistent pages, specify an ordering, for example: `SELECT * FROM stars ORDER BY id LIMIT 10 OFFSET 10;`.
- Added a per-user Windows installer and launcher. The launcher starts the server on loopback and opens the browser; the Start menu includes a Stop FlamingoDB shortcut. Database files remain under `%LOCALAPPDATA%\FlamingoDB\data` when uninstalling. Stop the server before upgrading or uninstalling.
- Build the 64-bit installer with `packaging/windows/build.ps1` on Windows or `packaging/windows/build.sh` with Go and NSIS available. The installer and launcher should be verified on the target Windows environment before distribution.

## Upgrade notes

The initial Windows login for a new installation is `admin` / `admin`; change it in My Profile. Existing stored user credentials are not overwritten by the daemon's startup flags. Browser-stored query results and conversations are local to that browser and may be lost when browser data is cleared. Keep database files and configuration backed up before upgrading.
