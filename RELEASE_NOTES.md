# FlamingoDB v1.3.1 release notes

FlamingoDB v1.3.1 fixes a Windows startup failure during database recovery. It also lets you calculate grouped averages and run vector functions directly on stored scientific data.

## Windows recovery and upgrade

In v1.3.0, the server could exit at startup with `truncate …flamingo.db.wal: Access is denied` in `server.log`. The server now opens the write-ahead log (WAL) with the write access Windows requires to truncate it after replaying committed changes.

1. Close any running instance using **Stop FlamingoDB** in the Start menu.
2. Back up the entire `%LOCALAPPDATA%\FlamingoDB\data` directory. Keep the database and its WAL together so recovery can replay pending changes.
3. Run `FlamingoDB-Setup-1.3.1-x64.exe`, open **FlamingoDB**, and sign in with your existing account. Run `SHOW TABLES;` to verify that the expected database opened.

## SQL aggregates

Use `AVG`, `COUNT`, `SUM`, `MIN`, and `MAX` over a whole table or per group. You can filter computed groups with `HAVING` and sort them with `ORDER BY`. For the `readings` table in the guide, this query returns West with an average temperature of 24 and excludes East, whose average is exactly 22.

```sql
SELECT station, AVG(temp) FROM readings
GROUP BY station
HAVING AVG(temp) > 22.0
ORDER BY AVG(temp) DESC;
```

## Native vector functions

`NORM`, `DOT`, and `CROSS` now accept stored `VECTOR` columns and numeric array literals. For a row where `v` is `[3,4,0]`, the following query returns `5`, `11`, and `[4,-3,0]`.

```sql
SELECT NORM(v), DOT(v, [1,2,0]), CROSS(v, [0,0,1]) FROM vectors;
```

Existing queries that pass bracketed text still work. `CROSS` returns text when both inputs are text and a native vector when either input is native.

## Documentation and known limitation

The new 14-page manual covers installation, first queries, dashboard imports, accounts, AI tools, the HTTP and Go interfaces, storage, backups, and recovery. [Read the FlamingoDB Technical Guide (PDF)](docs/FlamingoDB-Technical-Guide.pdf).

**Known limitation:** In this version, SQL `UPDATE` reports the number of matching rows without persisting the new values.
