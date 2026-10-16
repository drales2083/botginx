// pkg/actions/actions.go
package actions

import (
	"database/sql"
)

// Deps contains shared dependencies for actions.
// Actions receive this rather than reaching into global state.
type Deps struct {
	DB *sql.DB
}

// NewDeps creates a Deps instance with the given database connection.
func NewDeps(db *sql.DB) *Deps {
	return &Deps{DB: db}
}
