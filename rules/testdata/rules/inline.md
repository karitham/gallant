---
appliesTo: ["*.sql", "*.ddl"]
type: noul
---
# No unbounded deletes

Report a violation when a `DELETE` or `UPDATE` statement has no `WHERE` clause.
