package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Edit tokens are opaque content hashes used for optimistic concurrency on the
// first-party record editors (concurrent-edit detection). A token is derived
// only from the persisted *editable* content of a record — never from
// timestamps, counters, display labels or linked records the save does not
// replace — so that any writer that changes that content (another editor, an
// import, a restore, an association change) naturally invalidates an open
// editor's token, while a change followed by an exact revert does not.
//
// The SAME function must produce a token on the read path (detail DTO) and on
// the write precondition (inside the save transaction), both over state loaded
// from the database, or an unchanged record would hash differently and every
// save would be a false conflict. The canonical shapes below pin the fields and
// sort the set-like associations so the hash is invariant to load order.

type tcTokenStep struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
	OrderIndex     int    `json:"order_index"`
}

type tcTokenCustomValue struct {
	FieldID string `json:"field_id"`
	Value   string `json:"value"`
}

type tcTokenShape struct {
	ID          string               `json:"id"`
	FolderID    string               `json:"folder_id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Steps       []tcTokenStep        `json:"steps"`
	CategoryIDs []string             `json:"category_ids"`
	CustomVals  []tcTokenCustomValue `json:"custom_values"`
}

// TestCaseEditToken returns the opaque edit token for a test case's persisted
// editable content.
func TestCaseEditToken(tc *TestCase) string {
	shape := tcTokenShape{
		ID:          tc.ID,
		FolderID:    tc.FolderID,
		Name:        tc.Name,
		Description: tc.Description,
	}
	// Steps keep their meaning through order_index, so sort by it (then id) for a
	// canonical sequence independent of the slice order the DB returned.
	shape.Steps = make([]tcTokenStep, 0, len(tc.Steps))
	for _, s := range tc.Steps {
		if s == nil {
			continue
		}
		shape.Steps = append(shape.Steps, tcTokenStep{
			ID:             s.ID,
			Action:         s.Action,
			ExpectedResult: s.ExpectedResult,
			OrderIndex:     s.OrderIndex,
		})
	}
	sort.Slice(shape.Steps, func(i, j int) bool {
		if shape.Steps[i].OrderIndex != shape.Steps[j].OrderIndex {
			return shape.Steps[i].OrderIndex < shape.Steps[j].OrderIndex
		}
		return shape.Steps[i].ID < shape.Steps[j].ID
	})

	shape.CategoryIDs = make([]string, 0, len(tc.Categories))
	for _, c := range tc.Categories {
		if c != nil {
			shape.CategoryIDs = append(shape.CategoryIDs, c.ID)
		}
	}
	sort.Strings(shape.CategoryIDs)

	shape.CustomVals = make([]tcTokenCustomValue, 0, len(tc.CustomValues))
	for _, cv := range tc.CustomValues {
		if cv == nil {
			continue
		}
		shape.CustomVals = append(shape.CustomVals, tcTokenCustomValue{
			FieldID: cv.CustomFieldID,
			// The stored bytes are identical on read and write; do not re-canonicalise.
			Value: string(cv.Value),
		})
	}
	sort.Slice(shape.CustomVals, func(i, j int) bool {
		if shape.CustomVals[i].FieldID != shape.CustomVals[j].FieldID {
			return shape.CustomVals[i].FieldID < shape.CustomVals[j].FieldID
		}
		return shape.CustomVals[i].Value < shape.CustomVals[j].Value
	})

	return hashShape(shape)
}

type reqTokenShape struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// RequirementEditToken returns the opaque edit token for a requirement's
// persisted editable content (the three fields the editor can change).
func RequirementEditToken(r *Requirement) string {
	return hashShape(reqTokenShape{
		ID:          r.ID,
		Identifier:  r.Identifier,
		Title:       r.Title,
		Description: r.Description,
	})
}

func hashShape(shape interface{}) string {
	// json.Marshal emits struct fields in declaration order and the slices above
	// are pre-sorted, so the encoding is deterministic.
	b, err := json.Marshal(shape)
	if err != nil {
		// The shapes are plain strings/ints/slices; marshalling cannot fail.
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
