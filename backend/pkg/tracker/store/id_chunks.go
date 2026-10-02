package store

import "gorm.io/gorm"

// idChunkSize bounds how many ids one statement binds when an id list of unbounded length is
// split across statements. SQLite caps the variables a single statement may bind (32766 in the
// bundled mattn/go-sqlite3) and fails past it with "too many SQL variables"; this leaves room for
// the query's other placeholders. A var only so tests can scale it down with the cap they run under.
var idChunkSize = 5000

// idChunks yields ids in slices of at most idChunkSize.
func idChunks(ids []string) [][]string {
	var out [][]string
	for len(ids) > idChunkSize {
		out = append(out, ids[:idChunkSize])
		ids = ids[idChunkSize:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

// gatherInChunks runs load once per idChunks slice of ids and concatenates the rows each call puts
// in part, for a read whose id list has no bound. Rows keyed by those ids (a GROUP BY on the id
// column, say) are each read whole by the one chunk that holds their id. Like GORM's Find, it
// returns an empty slice rather than nil when nothing matched.
func gatherInChunks[T any](ids []string, load func(chunk []string, part *[]T) error) ([]T, error) {
	out := make([]T, 0)
	for _, chunk := range idChunks(ids) {
		var part []T
		if err := load(chunk, &part); err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
}

func chunkedDelete(tx *gorm.DB, model interface{}, column string, ids []string) error {
	for _, chunk := range idChunks(ids) {
		if err := tx.Where(column+" IN ?", chunk).Delete(model).Error; err != nil {
			return err
		}
	}
	return nil
}
