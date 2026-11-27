package audiobooks

import (
	"database/sql"
	"time"
)

// Place is where the listener is in a book: the part and the time into it,
// whether they've finished it, and when that was.
type Place struct {
	Part     string
	Position time.Duration
	Finished bool
	Updated  time.Time
}

// Places keeps the listener's place in each book.
type Places struct{ db *sql.DB }

// NewPlaces keeps places in db.
func NewPlaces(db *sql.DB) *Places { return &Places{db: db} }

// All is every book's place, by the book's path.
func (ps *Places) All() (map[string]Place, error) {
	rows, err := ps.db.Query(`SELECT book, part, position, finished, updated FROM audiobooks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Place{}
	for rows.Next() {
		var book string
		var pl Place
		var pos, updated int64
		if err := rows.Scan(&book, &pl.Part, &pos, &pl.Finished, &updated); err != nil {
			return nil, err
		}
		pl.Position, pl.Updated = time.Duration(pos)*time.Millisecond, time.UnixMilli(updated)
		out[book] = pl
	}
	return out, rows.Err()
}

// Save keeps pl as book's place.
func (ps *Places) Save(book string, pl Place) error {
	if pl.Updated.IsZero() {
		pl.Updated = time.Now()
	}
	_, err := ps.db.Exec(`INSERT INTO audiobooks (book, part, position, finished, updated) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (book) DO UPDATE SET part = excluded.part, position = excluded.position,
		finished = excluded.finished, updated = excluded.updated`,
		book, pl.Part, pl.Position.Milliseconds(), pl.Finished, pl.Updated.UnixMilli())
	return err
}

// Forget starts book over: its place goes.
func (ps *Places) Forget(book string) error {
	_, err := ps.db.Exec(`DELETE FROM audiobooks WHERE book = ?`, book)
	return err
}
