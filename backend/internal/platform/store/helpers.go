package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	perr "swearjar/internal/platform/errors"
)

// Exec runs a write and returns the raw CommandTag
func Exec(ctx context.Context, q RowQuerier, sql string, args ...any) (CommandTag, error) {
	return q.Exec(ctx, sql, args...)
}

// ExecOne runs a write and asserts exactly 1 row affected
func ExecOne(ctx context.Context, q RowQuerier, sql string, args ...any) error {
	tag, err := q.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	// best-effort check; pg's CommandTag String contains affected count
	if !strings.Contains(tag.String(), "1") { // e.g. "INSERT 0 1", "UPDATE 1"
		return errors.New("expected exactly one row affected")
	}
	return nil
}

// Scalar queries the first row, first column into T
func Scalar[T any](ctx context.Context, q RowQuerier, sql string, args ...any) (T, error) {
	var zero T
	r := q.QueryRow(ctx, sql, args...)
	var v T
	if err := r.Scan(&v); err != nil {
		return zero, err
	}
	return v, nil
}

// One uses a custom scanner to map a single row into T
func One[T any](ctx context.Context, q RowQuerier, scan func(Row) (T, error), sql string, args ...any) (T, error) {
	var zero T
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return zero, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, err
		}
		return zero, perr.ErrNotFound
	}
	// build a Row adapter over Rows for a single Scan call
	r := &rowFromRows{rows: rows}
	item, err := scan(r)
	if err != nil {
		return zero, err
	}
	// ensure no extra rows (optional)
	if rows.Next() {
		return zero, fmt.Errorf("expected 1 row, got more")
	}
	return item, rows.Err()
}

// Many uses a custom scanner to map all rows into []T
func Many[T any](ctx context.Context, q RowQuerier, scan func(Row) (T, error), sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []T
	r := &rowFromRows{rows: rows}
	for rows.Next() {
		item, err := scan(r)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Map returns a single row as map[column]any
func Map(ctx context.Context, q RowQuerier, sql string, args ...any) (map[string]any, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, perr.ErrNotFound
	}
	m, err := scanMap(rows)
	if err != nil {
		return nil, err
	}
	// ensure single row
	if rows.Next() {
		return nil, fmt.Errorf("expected 1 row, got more")
	}
	return m, rows.Err()
}

// Maps returns all rows as []map[string]any
func Maps(ctx context.Context, q RowQuerier, sql string, args ...any) ([]map[string]any, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		m, err := scanMap(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// StructByName maps one row into T by matching columns to struct `db` tags or field names
func StructByName[T any](ctx context.Context, q RowQuerier, sql string, args ...any) (T, error) {
	var zero T
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return zero, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, err
		}
		return zero, perr.ErrNotFound
	}
	item, err := scanStructByName[T](rows)
	if err != nil {
		return zero, err
	}
	if rows.Next() {
		return zero, fmt.Errorf("expected 1 row, got more")
	}
	return item, rows.Err()
}

// StructsByName maps all rows into []T by matching columns to struct `db` tags or field names
func StructsByName[T any](ctx context.Context, q RowQuerier, sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []T
	for rows.Next() {
		item, err := scanStructByName[T](rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// rowFromRows gives a Row facade over a current Rows position
type rowFromRows struct{ rows Rows }

func (r *rowFromRows) Scan(dest ...any) error { return r.rows.Scan(dest...) }

// scanMap builds map[string]any using Rows.Columns
func scanMap(rows Rows) (map[string]any, error) {
	cols := rows.Columns()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		m[c] = deref(vals[i])
	}
	return m, nil
}

func deref(v any) any {
	switch x := v.(type) {
	case *time.Time:
		if x == nil {
			return nil
		}
		return *x
	default:
		return v
	}
}

// scanStructByName maps row into T based on `db` tags or lowercased field names
func scanStructByName[T any](rows Rows) (T, error) {
	var zero T
	var m map[string]any
	{
		// reuse scanMap to get a name -> value map
		mv, err := scanMap(rows)
		if err != nil {
			return zero, err
		}
		m = mv
	}

	rt := reflect.TypeOf((*T)(nil)).Elem()
	rv := reflect.New(rt).Elem()

	fieldIndex := indexStructFields(rt)

	for name, val := range m {
		if idx, ok := fieldIndex[strings.ToLower(name)]; ok {
			fv := rv.Field(idx)
			assign(fv, val)
		}
	}

	return rv.Interface().(T), nil
}

// indexStructFields returns lowercased db tag or field name -> index
func indexStructFields(t reflect.Type) map[string]int {
	out := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		tag := f.Tag.Get("db")
		key := tag
		if key == "" || key == "-" {
			key = f.Name
		}
		out[strings.ToLower(key)] = i
	}
	return out
}

func assign(dst reflect.Value, src any) {
	if !dst.CanSet() {
		return
	}
	if src == nil {
		zero := reflect.Zero(dst.Type())
		dst.Set(zero)
		return
	}
	sv := reflect.ValueOf(src)

	// fast path: assignable
	if sv.Type().AssignableTo(dst.Type()) {
		dst.Set(sv)
		return
	}

	// convertible (e.g., int32 -> int64, float32 -> float64)
	if sv.Type().ConvertibleTo(dst.Type()) {
		dst.Set(sv.Convert(dst.Type()))
		return
	}

	// []byte -> string
	if b, ok := src.([]byte); ok && dst.Kind() == reflect.String {
		dst.SetString(string(b))
		return
	}
	// string -> []byte
	if s, ok := src.(string); ok && dst.Kind() == reflect.Slice && dst.Type().Elem().Kind() == reflect.Uint8 {
		dst.SetBytes([]byte(s))
		return
	}
	// fallback: no-op
}
