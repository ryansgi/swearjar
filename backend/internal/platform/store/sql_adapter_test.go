package store

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

/*
   pgx fakes (unique names to avoid colliding with helpers_test fakes)
*/

// pgxFakeRow implements pgx.Row
type pgxFakeRow struct {
	scan func(dest ...any) error
}

func (r *pgxFakeRow) Scan(dest ...any) error {
	if r.scan != nil {
		return r.scan(dest...)
	}
	return nil
}

// pgxFakeRows implements pgx.Rows
type pgxFakeRows struct {
	fields []pgconn.FieldDescription
	data   [][]any
	idx    int
	err    error
	closed bool
	ct     pgconn.CommandTag
}

func newPgxFakeRows(cols []string, data [][]any) *pgxFakeRows {
	fds := make([]pgconn.FieldDescription, len(cols))
	for i, c := range cols {
		// Name is a string in pgx/v5
		fds[i] = pgconn.FieldDescription{Name: c}
	}
	return &pgxFakeRows{fields: fds, data: data, idx: -1}
}

func (r *pgxFakeRows) Conn() *pgx.Conn { return nil }

func (r *pgxFakeRows) Close()                        { r.closed = true }
func (r *pgxFakeRows) Err() error                    { return r.err }
func (r *pgxFakeRows) CommandTag() pgconn.CommandTag { return r.ct }
func (r *pgxFakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return r.fields
}
func (r *pgxFakeRows) Next() bool {
	if r.err != nil {
		return false
	}
	r.idx++
	return r.idx >= 0 && r.idx < len(r.data)
}
func (r *pgxFakeRows) RawValues() [][]byte { return nil }
func (r *pgxFakeRows) Values() ([]any, error) {
	if r.idx < 0 || r.idx >= len(r.data) {
		return nil, errors.New("out of range")
	}
	return r.data[r.idx], nil
}
func (r *pgxFakeRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.idx < 0 || r.idx >= len(r.data) {
		return errors.New("scan out of range")
	}
	row := r.data[r.idx]
	if len(row) != len(dest) {
		return errors.New("dest len mismatch")
	}
	for i := range dest {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Pointer || !dv.Elem().CanSet() {
			return errors.New("dest not pointer")
		}
		val := reflect.ValueOf(row[i])
		if val.IsValid() && val.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(val)
			continue
		}
		if val.IsValid() && val.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(val.Convert(dv.Elem().Type()))
			continue
		}
		return errors.New("type mismatch")
	}
	return nil
}

// pgxFakeTx implements pgx.Tx (only the methods txQuerier uses)
type pgxFakeTx struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f *pgxFakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("OK"), nil
}
func (f *pgxFakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.queryFn != nil {
		return f.queryFn(ctx, sql, args...)
	}
	return newPgxFakeRows([]string{"n"}, [][]any{{1}}), nil
}
func (f *pgxFakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return &pgxFakeRow{scan: func(dest ...any) error {
		if len(dest) > 0 {
			if p, ok := dest[0].(*int); ok {
				*p = 7
			}
		}
		return nil
	}}
}

// Unused pgx.Tx methods to satisfy interface
func (f *pgxFakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (f *pgxFakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *pgxFakeTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (f *pgxFakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *pgxFakeTx) Conn() *pgx.Conn              { return nil }
func (f *pgxFakeTx) Commit(context.Context) error { return nil }
func (f *pgxFakeTx) Rollback(context.Context) error {
	return nil
}
func (f *pgxFakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return f, nil }

/*
   tests
*/

func TestTag_String(t *testing.T) {
	t.Parallel()

	ct := pgconn.NewCommandTag("INSERT 0 1")
	tg := tag{} // avoid keyed literal inside if-init to dodge parser weirdness
	tg.t = ct

	got := tg.String()
	if got != "INSERT 0 1" {
		t.Fatalf("tag.String mismatch got=%q", got)
	}
}

func TestRows_Columns_Next_Scan_Close(t *testing.T) {
	t.Parallel()

	fr := newPgxFakeRows([]string{"id", "name"}, [][]any{{1, "zoe"}, {2, "ada"}})
	rs := rows{r: fr}

	cols := rs.Columns()
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Fatalf("Columns mismatch: %#v", cols)
	}

	var ids []int
	var names []string
	for rs.Next() {
		var id int
		var name string
		if err := rs.Scan(&id, &name); err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		ids = append(ids, id)
		names = append(names, name)
	}
	if err := rs.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	rs.Close()
	if !fr.closed {
		t.Fatalf("underlying rows not closed")
	}
	if !reflect.DeepEqual(ids, []int{1, 2}) || !reflect.DeepEqual(names, []string{"zoe", "ada"}) {
		t.Fatalf("data mismatch ids=%v names=%v", ids, names)
	}
}

func TestRow_ScanDelegates(t *testing.T) {
	t.Parallel()

	r := row{r: &pgxFakeRow{scan: func(dest ...any) error {
		if len(dest) != 1 {
			return errors.New("want 1")
		}
		if p, ok := dest[0].(*string); ok {
			*p = "ok"
			return nil
		}
		return errors.New("bad type")
	}}}

	var s string
	if err := r.Scan(&s); err != nil {
		t.Fatalf("row.Scan error: %v", err)
	}
	if s != "ok" {
		t.Fatalf("row.Scan mismatch got=%q", s)
	}
}

func TestTxQuerier_Exec_Query_QueryRow(t *testing.T) {
	t.Parallel()

	fx := &pgxFakeTx{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if sql != "update x set n=$1 where id=$2" {
				return pgconn.NewCommandTag(""), errors.New("unexpected sql")
			}
			if len(args) != 2 || args[0] != 9 || args[1] != 1 {
				return pgconn.NewCommandTag(""), errors.New("unexpected args")
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			if sql != "select id, name from t where id=$1" || len(args) != 1 || args[0] != 1 {
				return nil, errors.New("unexpected query")
			}
			return newPgxFakeRows([]string{"id", "name"}, [][]any{{1, "zoe"}}), nil
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &pgxFakeRow{scan: func(dest ...any) error {
				if len(dest) != 1 {
					return errors.New("want 1 dest")
				}
				if p, ok := dest[0].(*int); ok {
					*p = 42
					return nil
				}
				return errors.New("bad type")
			}}
		},
	}
	q := txQuerier{tx: fx}

	// Exec path
	ct, err := q.Exec(context.Background(), "update x set n=$1 where id=$2", 9, 1)
	if err != nil {
		t.Fatalf("txQuerier.Exec error: %v", err)
	}
	if ct.String() != "UPDATE 1" {
		t.Fatalf("CommandTag mismatch got=%q", ct.String())
	}

	// Query path
	rs, err := q.Query(context.Background(), "select id, name from t where id=$1", 1)
	if err != nil {
		t.Fatalf("txQuerier.Query error: %v", err)
	}
	defer rs.Close()

	if gotCols := rs.Columns(); len(gotCols) != 2 || gotCols[0] != "id" || gotCols[1] != "name" {
		t.Fatalf("Columns mismatch: %#v", gotCols)
	}
	if !rs.Next() {
		t.Fatalf("expected one row")
	}
	var id int
	var name string
	if err := rs.Scan(&id, &name); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if id != 1 || name != "zoe" {
		t.Fatalf("row mismatch id=%d name=%q", id, name)
	}
	if rs.Next() {
		t.Fatalf("unexpected extra row")
	}

	// QueryRow path
	var n int
	if err := q.QueryRow(context.Background(), "select 1").Scan(&n); err != nil {
		t.Fatalf("txQuerier.QueryRow scan: %v", err)
	}
	if n != 42 {
		t.Fatalf("QueryRow value mismatch got=%d", n)
	}
}

func TestRows_ScanErrorsAndErrPropagation(t *testing.T) {
	t.Parallel()

	{
		fr := newPgxFakeRows([]string{"a", "b"}, [][]any{{1, "x"}})
		rs := rows{r: fr}

		if !rs.Next() {
			t.Fatal("expected Next true")
		}
		var onlyOne int
		if err := rs.Scan(&onlyOne); err == nil {
			t.Fatal("expected dest len mismatch error")
		}
	}

	{
		fr := newPgxFakeRows([]string{"n"}, [][]any{})
		fr.err = errors.New("boom") // <-- safely inside function scope

		rs := rows{r: fr}
		if rs.Next() {
			t.Fatal("expected Next false when rows has error")
		}
		if err := rs.Err(); err == nil || err.Error() != "boom" {
			t.Fatalf("rows.Err mismatch: %v", err)
		}
	}
}

func TestTxQuerier_PropagatesErrors(t *testing.T) {
	t.Parallel()

	fx := &pgxFakeTx{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("exec failed")
		},
		queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("query failed")
		},
		queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return &pgxFakeRow{scan: func(dest ...any) error { return errors.New("scan failed") }}
		},
	}
	q := txQuerier{tx: fx}

	if _, err := q.Exec(context.Background(), "x"); err == nil {
		t.Fatalf("expected Exec error")
	}

	if _, err := q.Query(context.Background(), "x"); err == nil {
		t.Fatalf("expected Query error")
	}

	var n int
	if err := q.QueryRow(context.Background(), "x").Scan(&n); err == nil {
		t.Fatalf("expected QueryRow.Scan error")
	}
}
