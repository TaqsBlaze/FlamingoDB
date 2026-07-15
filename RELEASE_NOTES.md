# FlamingoDB v1.2.0 — Release Notes

FlamingoDB version `1.2.0` introduces major user-facing improvements, including schema discovery, data integrity guarantees, and a completely revamped, secure web administration interface. 

---

## 🖥️ Web Administration Dashboard
The **Web Administration Dashboard** is now fully integrated and embedded directly into the database engine. It offers a beautiful, modern, and dark-themed graphical interface to manage and interact with your FlamingoDB server.

### Key Capabilities:
*   **Visual SQL Console**: Run database queries interactively and view beautifully formatted data tables with full support for advanced scientific, geospatial, and numerical data types.
*   **Table & Schema Browser**: Explore all tables in the database catalog, inspect column configurations, and check column data types without writing SQL queries.
*   **Access Control & Roles**: Create named security policies (e.g. read-only, read-write, schema-admin) and grant granular permissions (Select, Insert, Update, Delete, Create, Drop) directly through a visual settings panel.
*   **User Account Administration**: Register new users, manage active permissions, and safely update user passwords in real time.
*   **Live Server Monitor**: Keep track of the server's online status, active connection counts, and transactional states.

---

## 🚀 Key Database Features & Enhancements

### 1. Schema Discovery (`SHOW TABLES`)
*   **Functional Change**: Users can now execute the standard `SHOW TABLES;` SQL query in the console or terminal client.
*   **User Benefit**: Retrieve a clean, alphabetically sorted list of all active tables currently registered in the database catalog.

### 2. Auto-Enforced Unique Record IDs
*   **Functional Change**: The storage engine now automatically detects the presence of an `id`/`ID` column in any table schema.
*   **User Benefit**: It prevents duplicate rows with identical IDs. Attempting to insert a record with an ID that already exists will safely abort the transaction and raise a clear duplicate key error, ensuring robust data integrity.

---

## 🔒 Security Improvements

### SHA-256 Password Hashing
*   **Functional Change**: User passwords stored on disk (`users.json`) are now secured using standard SHA-256 encryption instead of plain text.
*   **User Benefit**: Complete peace of mind for account credentials. 
*   **Zero-Overhead Upgrade**: When starting up with version `1.2.0`, the server daemon automatically scans the user database, encrypts any legacy plain-text passwords on-the-fly, and commits the secured hashes back to disk.
