# Sample datasets

These datasets contain **synthetic, fictional records** generated for testing and demonstrations. They are not real measurements or patient data, and they should not be used for clinical, research, or geographic decisions.

| File | Rows | Description |
| --- | ---: | --- |
| `spatial_sites.csv` | 144 | Fictional monitoring sites near six Zimbabwean cities, with latitude, longitude, elevation, and a sample score. |
| `weather_observations.csv` | 180 | Fictional daily weather readings for three stations, including temperature, humidity, rainfall, wind, and pressure. |
| `scientific_experiments.csv` | 150 | Fictional experimental runs with temperature, concentration, duration, signal, and uncertainty. |
| `medical_observations.csv` | 160 | Fictional patient visits with demographic fields and example measurements. |

## Importing

1. In the web interface, open **Table Browser → Import Data** and select a CSV file.
2. Choose **Create new table** and enter a table name, such as `weather`.
3. Review the column names and inferred `INT`, `FLOAT`, and `VARCHAR` types, then select **Continue → Import**. The first CSV row contains headers.
4. Open the table in Table Browser or query it from SQL Console.

For example, after importing the weather file as `weather`:

```sql
SELECT station_city, temperature_c, rainfall_mm FROM weather WHERE temperature_c > 24;
```

Each row has a distinct integer `id`. Dates use `YYYY-MM-DD` text. Column names indicate the measurement units; they are ordinary numeric columns, not unit-aware values. Spatial locations are represented by separate numeric latitude and longitude columns in decimal degrees so that they can be imported using the current CSV importer; the importer does not create native `POINT` columns. A coordinate pair is latitude, longitude (not a native spatial literal).
