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

func chunkedDelete(tx *gorm.DB, model interface{}, column string, ids []string) error {
	for _, chunk := range idChunks(ids) {
		if err := tx.Where(column+" IN ?", chunk).Delete(model).Error; err != nil {
			return err
		}
	}
	return nil
}
