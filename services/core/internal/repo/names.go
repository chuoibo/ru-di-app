package repo

import "context"

// displayNames is `_display_names(set_of_ids)`: one statement for the distinct
// ids, and a name is `found.get(id) or str(id)`, so an EMPTY display_name falls
// back to the id exactly like a missing row does. No ids, no statement.
//
// The IN list has one parameter per distinct id. Python builds it from a set,
// whose order only moves bound values, never the statement text.
func (r Repository) displayNames(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	distinct := []string{}
	for _, id := range ids {
		if _, seen := out[id]; !seen {
			out[id] = id
			distinct = append(distinct, id)
		}
	}
	if len(distinct) == 0 {
		return out, nil
	}
	rows, err := r.Q.Query(ctx,
		`SELECT people.id, people.display_name
		   FROM people
		  WHERE people.id IN (`+uuidPlaceholders(1, len(distinct))+`)`, uuidArgs(distinct)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		if name != "" {
			out[id] = name
		}
	}
	return out, rows.Err()
}
