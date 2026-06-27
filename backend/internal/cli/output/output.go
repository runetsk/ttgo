package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Column defines a table column.
type Column struct {
	Header string
	Key    string
}

// Print formats data based on the output mode.
func Print(w io.Writer, mode string, data interface{}, columns []Column) error {
	switch mode {
	case "json":
		return printJSON(w, data)
	case "plain":
		return printPlain(w, data, columns)
	default:
		return printTable(w, data, columns)
	}
}

// PrintRaw outputs raw JSON bytes, pretty-printed for "json" mode.
func PrintRaw(w io.Writer, mode string, raw json.RawMessage) error {
	if mode == "json" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err != nil {
			_, err := w.Write(raw)
			return err
		}
		pretty.WriteByte('\n')
		_, err := w.Write(pretty.Bytes())
		return err
	}
	_, err := w.Write(raw)
	return err
}

// PrintMessage prints a simple text message.
func PrintMessage(w io.Writer, mode string, msg string) {
	if mode == "json" {
		data, _ := json.Marshal(map[string]string{"message": msg})
		fmt.Fprintln(w, string(data))
	} else {
		fmt.Fprintln(w, msg)
	}
}

func printJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func printTable(w io.Writer, data interface{}, columns []Column) error {
	rows, err := toRows(data)
	if err != nil {
		return printJSON(w, data)
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, "No results.")
		return nil
	}

	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col.Header)
	}
	for _, row := range rows {
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col.Key])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	for i := range widths {
		if widths[i] > 50 {
			widths[i] = 50
		}
	}

	for i, col := range columns {
		fmt.Fprintf(w, "%-*s  ", widths[i], col.Header)
		_ = i
	}
	fmt.Fprintln(w)

	for _, row := range rows {
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col.Key])
			if len(val) > 50 {
				val = val[:47] + "..."
			}
			fmt.Fprintf(w, "%-*s  ", widths[i], val)
			_ = i
		}
		fmt.Fprintln(w)
	}
	return nil
}

func printPlain(w io.Writer, data interface{}, columns []Column) error {
	rows, err := toRows(data)
	if err != nil {
		return printJSON(w, data)
	}

	for _, row := range rows {
		vals := make([]string, len(columns))
		for i, col := range columns {
			vals[i] = fmt.Sprintf("%v", row[col.Key])
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	return nil
}

func toRows(data interface{}) ([]map[string]interface{}, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
