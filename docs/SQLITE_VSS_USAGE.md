# SQLite-VSS Integration & Usage

## Overview

gosqueal integrates **sqlite-vss** to provide vector similarity search
capabilities directly within SQLite. This allows for storing and querying
high-dimensional vectors (embeddings) using standard SQL syntax.

**Upstream Reference**: [asg017/sqlite-vss](https://github.com/asg017/sqlite-vss)

## Implementation Details

### 1. Docker Build Process

The integration relies on a multi-stage Docker build to fetch the required
shared libraries.

- **Source**: The Dockerfile downloads pre-compiled Linux x86_64 binaries from
  the upstream GitHub Releases.
- **Files**:
  - vss0.so: The main vector similarity search extension.
  - vector0.so: Helper math functions for vectors.
- **Placement**: These files are placed in /usr/lib/ in the final container
  image.

### 2. Go Driver Registration

To load these extensions, gosqueal registers a custom SQLite driver in main.go.
Standard SQLite drivers do not load extensions by default for security reasons.

```go
sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
    Extensions: []string{
        "/usr/lib/vector0",
        "/usr/lib/vss0",
    },
})
```

### 3. Schema Definition

The application initializes a virtual table specifically for vector storage on
startup:

```sql
CREATE VIRTUAL TABLE vectors USING vss0(
    headline_embedding(384),
    description_embedding(384)
);
```

*Note: The dimension (384) is optimized for models like all-MiniLM-L6-v2.*

## Usage Guide

### Storing Vectors

Vectors are stored as JSON arrays. You can insert them using standard SQL
INSERT statements.

```sql
INSERT INTO vectors(rowid, headline_embedding)
VALUES (1, '[0.1, -0.2, 0.5, ...]');
```

### Querying (Similarity Search)

To find the nearest neighbors (semantic search), use the vss_search function in
the WHERE clause. The distance column represents the similarity score.

```sql
SELECT rowid, distance
FROM vectors
WHERE vss_search(headline_embedding, '[0.1, -0.2, 0.5, ...]')
LIMIT 10;
```

### References

- **SQL Usage**: [sqlite-vss SQL Documentation](https://github.com/asg017/sqlite-vss?tab=readme-ov-file#sql-api-reference)
- **Python/JS Examples**: [sqlite-vss Usage Guide](https://github.com/asg017/sqlite-vss?tab=readme-ov-file#usage)
