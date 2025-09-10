package store

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	perr "swearjar/internal/platform/errors"
)

type cmdTag string

func (c cmdTag) String() string { return string(c) }
func (c cmdTag) RowsAffected() int64 {
	s := string(c)
	i := strings.LastIndexByte(s, ' ')
	if i < 0 {
		return 0
	}
	n, err := strconv.ParseInt(s[i+1:], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

type fakeRowQuerier struct {
	lastExecSQL string
	lastExecArg []any
	execTag     CommandTag
	execErr     error

	queryRows Rows
	queryErr  error

	qrRow   Row
	qrErr   error
	qrCalls int
}

func (f *fakeRowQuerier) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	f.lastExecSQL = sql
	f.lastExecArg = args
	return f.execTag, f.execErr
}

func (f *fakeRowQuerier) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return f.queryRows, f.queryErr
}

func (f *fakeRowQuerier) QueryRow(ctx context.Context, sql string, args ...any) Row {
	f.qrCalls++
	return &fakeRow{err: f.qrErr, val: f.qrRow}
}

type fakeRow struct {
	// if val != nil and is *fakeRow, delegate; else Scan first arg
	val Row
	err error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.val != nil {
		return r.val.Scan(dest...)
	}
	// default: put a constant into first dest if it's *T
	if len(dest) > 0 {
		switch p := dest[0].(type) {
		case *int:
			*p = 42
		case *string:
			*p = "ok"
		default:
			// try reflection
			rv := reflect.ValueOf(dest[0])
			if rv.Kind() == reflect.Pointer && rv.Elem().CanSet() {
				zero := reflect.Zero(rv.Elem().Type())
				rv.Elem().Set(zero)
			}
		}
	}
	return nil
}

type fakeRows struct {
	cols   []string
	data   [][]any // each row is len(cols)
	idx    int     // -1 before first
	err    error
	closed bool
}

func newRows(cols []string, data [][]any) *fakeRows {
	return &fakeRows{cols: cols, data: data, idx: -1}
}
func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Next() bool {
	if r.err != nil {
		return false
	}
	r.idx++
	return r.idx >= 0 && r.idx < len(r.data)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.idx < 0 || r.idx >= len(r.data) {
		return errors.New("scan out of bounds")
	}
	row := r.data[r.idx]
	if len(dest) != len(row) {
		return errors.New("dest len mismatch")
	}
	for i := range dest {
		// dest[i] is pointer; set underlying to row[i]
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Pointer || !dv.Elem().CanSet() {
			return errors.New("dest not settable")
		}
		val := reflect.ValueOf(row[i])
		// if types don't match, try conversion for common cases
		if val.IsValid() && val.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(val)
			continue
		}
		// []byte -> string
		if b, ok := row[i].([]byte); ok && dv.Elem().Kind() == reflect.String {
			dv.Elem().SetString(string(b))
			continue
		}
		// string -> []byte
		if s, ok := row[i].(string); ok && dv.Elem().Kind() == reflect.Slice &&
			dv.Elem().Type().Elem().Kind() == reflect.Uint8 {
			dv.Elem().SetBytes([]byte(s))
			continue
		}
		if val.IsValid() && val.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(val.Convert(dv.Elem().Type()))
			continue
		}
		dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
	}
	return nil
}
func (r *fakeRows) Err() error { return r.err }
func (r *fakeRows) Close()     { r.closed = true }

/*
	tests
*/

func TestExec_Passthrough(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{execTag: cmdTag("INSERT 0 3")}
	tag, err := Exec(context.Background(), f, "insert x", 1, "a")
	if err != nil {
		t.Fatalf("Exec err: %v", err)
	}
	if tag.String() != "INSERT 0 3" {
		t.Fatalf("tag mismatch: %q", tag.String())
	}
	if f.lastExecSQL != "insert x" || len(f.lastExecArg) != 2 {
		t.Fatalf("exec call not recorded properly")
	}
}

func TestExecOne_ExactlyOne(t *testing.T) {
	t.Parallel()

	f1 := &fakeRowQuerier{execTag: cmdTag("INSERT 0 1")}
	if err := ExecOne(context.Background(), f1, "ok"); err != nil {
		t.Fatalf("ExecOne should succeed: %v", err)
	}

	f2 := &fakeRowQuerier{execTag: cmdTag("UPDATE 2")}
	if err := ExecOne(context.Background(), f2, "bad"); err == nil {
		t.Fatalf("ExecOne expected error when affected != 1")
	}
}

func TestScalar_OK(t *testing.T) {
	t.Parallel()

	// QueryRow returns 7
	f := &fakeRowQuerier{
		qrRow: Row(&fakeRow{val: Row(&scanVal{v: 7})}),
	}
	got, err := Scalar[int](context.Background(), f, "select 7")
	if err != nil {
		t.Fatalf("Scalar err: %v", err)
	}
	if got != 7 {
		t.Fatalf("Scalar got %d want 7", got)
	}
}

// scanVal lets us force the returned Scan value
type scanVal struct{ v any }

func (s *scanVal) Scan(dest ...any) error {
	if len(dest) == 0 {
		return nil
	}
	dv := reflect.ValueOf(dest[0])
	if dv.Kind() == reflect.Pointer && dv.Elem().CanSet() {
		val := reflect.ValueOf(s.v)
		if val.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(val)
		} else if val.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(val.Convert(dv.Elem().Type()))
		}
	}
	return nil
}

func TestOne_SingleRow(t *testing.T) {
	t.Parallel()

	rows := newRows([]string{"n"}, [][]any{{5}})
	f := &fakeRowQuerier{queryRows: rows}

	item, err := One(context.Background(), f, func(r Row) (int, error) {
		var x int
		if err := r.Scan(&x); err != nil {
			return 0, err
		}
		return x, nil
	}, "select")
	if err != nil {
		t.Fatalf("One err: %v", err)
	}
	if item != 5 {
		t.Fatalf("One item %d want 5", item)
	}
	if !rows.closed {
		t.Fatalf("rows not closed")
	}
}

func TestOne_NotFoundAndTooMany(t *testing.T) {
	t.Parallel()

	// not found
	f1 := &fakeRowQuerier{queryRows: newRows([]string{"a"}, [][]any{})}
	_, err := One(context.Background(), f1, func(r Row) (int, error) {
		var x int
		return x, r.Scan(&x)
	}, "q")
	if !errors.Is(err, perr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// too many
	f2 := &fakeRowQuerier{queryRows: newRows([]string{"a"}, [][]any{{1}, {2}})}
	_, err = One(context.Background(), f2, func(r Row) (int, error) {
		var x int
		return x, r.Scan(&x)
	}, "q")
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error for >1 row")
	}
}

func TestMany_MultiRow(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{queryRows: newRows([]string{"n"}, [][]any{{1}, {2}, {3}})}
	items, err := Many(context.Background(), f, func(r Row) (int, error) {
		var x int
		return x, r.Scan(&x)
	}, "q")
	if err != nil {
		t.Fatalf("Many err: %v", err)
	}
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("Many %v want %v", items, want)
	}
}

func TestMap_And_Maps(t *testing.T) {
	t.Parallel()

	cols := []string{"id", "name"}
	data := [][]any{{1, "ryan"}, {2, "mike"}}

	// Map single
	f1 := &fakeRowQuerier{queryRows: newRows(cols, data[:1])}
	m, err := Map(context.Background(), f1, "q")
	if err != nil {
		t.Fatalf("Map err: %v", err)
	}
	if m["id"] != 1 || m["name"] != "ryan" {
		t.Fatalf("Map mismatch: %v", m)
	}

	// Map not found
	f2 := &fakeRowQuerier{queryRows: newRows(cols, nil)}
	_, err = Map(context.Background(), f2, "q")
	if !errors.Is(err, perr.ErrNotFound) {
		t.Fatalf("Map expected ErrNotFound, got %v", err)
	}

	// Map too many
	f3 := &fakeRowQuerier{queryRows: newRows(cols, data)}
	_, err = Map(context.Background(), f3, "q")
	if err == nil {
		t.Fatalf("Map expected error on >1 row")
	}

	// Maps multi
	f4 := &fakeRowQuerier{queryRows: newRows(cols, data)}
	mv, err := Maps(context.Background(), f4, "q")
	if err != nil {
		t.Fatalf("Maps err: %v", err)
	}
	if len(mv) != 2 || mv[0]["id"] != 1 || mv[1]["name"] != "mike" {
		t.Fatalf("Maps mismatch: %#v", mv)
	}
}

func TestStructByName_And_StructsByName(t *testing.T) {
	t.Parallel()

	type user struct {
		ID     int       `db:"user_id"` // tag mapping
		Name   string    // field mapping
		Raw    []byte    // string -> []byte conversion path
		Note   string    // []byte -> string conversion path
		SeenAt time.Time // pointer time deref path exercised in deref()
	}

	tm := time.Date(2025, 8, 26, 12, 0, 0, 0, time.UTC)

	cols := []string{"user_id", "name", "raw", "note", "seenat"}
	data := [][]any{
		{10, "Zoe", "hello", []byte("bytes"), &tm}, // string->[]byte, []byte->string, *time.Time
		{11, "Ada", "x", []byte("y"), &tm},
	}

	// single
	f1 := &fakeRowQuerier{queryRows: newRows(cols, data[:1])}
	u, err := StructByName[user](context.Background(), f1, "q")
	if err != nil {
		t.Fatalf("StructByName err: %v", err)
	}
	if u.ID != 10 || u.Name != "Zoe" || string(u.Raw) != "hello" || u.Note != "bytes" || u.SeenAt.IsZero() {
		t.Fatalf("StructByName mismatch: %#v", u)
	}

	// not found
	f2 := &fakeRowQuerier{queryRows: newRows(cols, nil)}
	_, err = StructByName[user](context.Background(), f2, "q")
	if !errors.Is(err, perr.ErrNotFound) {
		t.Fatalf("StructByName expected ErrNotFound, got %v", err)
	}

	// too many
	f3 := &fakeRowQuerier{queryRows: newRows(cols, data)}
	_, err = StructByName[user](context.Background(), f3, "q")
	if err == nil {
		t.Fatalf("StructByName expected error on >1 row")
	}

	// structs slice
	f4 := &fakeRowQuerier{queryRows: newRows(cols, data)}
	us, err := StructsByName[user](context.Background(), f4, "q")
	if err != nil {
		t.Fatalf("StructsByName err: %v", err)
	}
	if len(us) != 2 || us[0].ID != 10 || us[1].Name != "Ada" {
		t.Fatalf("StructsByName mismatch: %#v", us)
	}
}

func TestExecOne_PropagatesExecError(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{execErr: errors.New("boom")}
	if err := ExecOne(context.Background(), f, "update x"); err == nil || err.Error() != "boom" {
		t.Fatalf("expected exec error to bubble, got %v", err)
	}
}

func TestScalar_ScanError(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{qrErr: errors.New("scan bad")}
	_, err := Scalar[int](context.Background(), f, "select 1")
	if err == nil || err.Error() != "scan bad" {
		t.Fatalf("expected scan error, got %v", err)
	}
}

func TestOne_QueryErrorAndErrFromRowsOnNoNext(t *testing.T) {
	t.Parallel()

	// Query error
	f1 := &fakeRowQuerier{queryErr: errors.New("query bad")}
	_, err := One(context.Background(), f1, func(Row) (int, error) { return 0, nil }, "q")
	if err == nil || err.Error() != "query bad" {
		t.Fatalf("expected query error, got %v", err)
	}

	// rows.Err() when no Next
	r := newRows([]string{"a"}, nil)
	r.err = errors.New("rows-err")
	f2 := &fakeRowQuerier{queryRows: r}
	_, err = One(context.Background(), f2, func(Row) (int, error) { return 0, nil }, "q")
	if err == nil || err.Error() != "rows-err" {
		t.Fatalf("expected rows.Err, got %v", err)
	}
}

func TestMany_QueryErrorAndScanError(t *testing.T) {
	t.Parallel()

	// Query error
	f1 := &fakeRowQuerier{queryErr: errors.New("boom")}
	_, err := Many(context.Background(), f1, func(Row) (int, error) { return 0, nil }, "q")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected query error, got %v", err)
	}

	// Scan error (second row)
	rows := newRows([]string{"n"}, [][]any{{1}, {2}})
	f2 := &fakeRowQuerier{queryRows: rows}
	_, err = Many(context.Background(), f2, func(r Row) (int, error) {
		if rows.idx == 0 {
			var v int
			return v, r.Scan(&v)
		}
		return 0, errors.New("scan in mapper failed")
	}, "q")
	if err == nil || err.Error() != "scan in mapper failed" {
		t.Fatalf("expected mapper error, got %v", err)
	}
}

func TestMap_ScanErrorFromscanMap_AndNilTimeDeref(t *testing.T) {
	t.Parallel()

	// Force scanMap dest len mismatch: 2 columns but row with 1 value
	cols := []string{"a", "b"}
	bad := newRows(cols, [][]any{{1}})
	f1 := &fakeRowQuerier{queryRows: bad}
	if _, err := Map(context.Background(), f1, "q"); err == nil {
		t.Fatalf("expected scanMap error on dest mismatch")
	}

	// deref(*time.Time(nil)) -> nil value in map
	var tm *time.Time // nil pointer
	cols2 := []string{"seenat"}
	ok := newRows(cols2, [][]any{{tm}})
	f2 := &fakeRowQuerier{queryRows: ok}
	m, err := Map(context.Background(), f2, "q")
	if err != nil {
		t.Fatalf("Map err: %v", err)
	}
	if _, present := m["seenat"]; !present {
		t.Fatalf("expected seenat key present")
	}
	if m["seenat"] != nil {
		t.Fatalf("expected nil deref for *time.Time(nil), got %#v", m["seenat"])
	}
}

func TestMaps_ScanErrorOnSecondRow(t *testing.T) {
	t.Parallel()

	// First row OK (2 values), second row short (1 value) -> scanMap error on second iteration
	cols := []string{"id", "name"}
	rows := newRows(cols, [][]any{
		{1, "ok"},
		{2}, // dest mismatch triggers scanMap error
	})
	f := &fakeRowQuerier{queryRows: rows}
	_, err := Maps(context.Background(), f, "q")
	if err == nil {
		t.Fatalf("expected scanMap error on second row")
	}
}

func TestStructByName_ScanError(t *testing.T) {
	t.Parallel()

	type user struct{ ID int }

	// Columns say 1, row has 0 -> scanMap error bubbles
	cols := []string{"id"}
	rows := newRows(cols, [][]any{ /* empty row values */ })
	f := &fakeRowQuerier{queryRows: rows}
	_, err := StructByName[user](context.Background(), f, "q")
	if err == nil {
		t.Fatalf("expected scanMap error")
	}
}

func TestIndexStructFields_AndAssignConversionsAndNilSrc(t *testing.T) {
	t.Parallel()

	type demo struct {
		I64   int64  `db:"num"` // convertible from int32
		S     string // from []byte
		B     []byte // from string
		Plain int    // assignable
		Skip  string `db:"-"` // note: current implementation maps "-" to field name
	}

	cols := []string{"num", "s", "b", "plain", "skip"}
	// int32 -> int64 (ConvertibleTo), []byte -> string, string -> []byte, exact int, and regular string
	row := [][]any{{int32(5), []byte("bytes"), "str", 9, "kept"}}
	rows := newRows(cols, row)

	got, err := StructByName[demo](context.Background(), &fakeRowQuerier{queryRows: rows}, "q")
	if err != nil {
		t.Fatalf("StructByName err: %v", err)
	}
	if got.I64 != 5 || got.S != "bytes" || string(got.B) != "str" || got.Plain != 9 || got.Skip != "kept" {
		t.Fatalf("assign/convert mismatch: %#v", got)
	}

	// Also exercise assign nil to zero-value
	var dst reflect.Value
	{
		var s struct{ X *int }
		dst = reflect.ValueOf(&s).Elem().FieldByName("X")
		assign(dst, nil)
		if !dst.IsNil() {
			t.Fatalf("nil assign should set zero; got %#v", dst.Interface())
		}
	}
}

func TestRowFromRows_SingleScanFacade(t *testing.T) {
	t.Parallel()

	cols := []string{"n"}
	data := [][]any{{7}}
	fr := newRows(cols, data)
	r := &rowFromRows{rows: fr}

	// advance to first row then scan through facade
	if !fr.Next() {
		t.Fatalf("Next false")
	}
	var n int
	if err := r.Scan(&n); err != nil {
		t.Fatalf("rowFromRows.Scan err: %v", err)
	}
	if n != 7 {
		t.Fatalf("rowFromRows got %d want 7", n)
	}
}

func TestExecOne_AffectedZero(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{execTag: cmdTag("INSERT 0 0")}
	err := ExecOne(context.Background(), f, "insert nothing")
	if err == nil {
		t.Fatalf("expected error when affected != 1")
	}
}

func TestMany_EmptyRows_IsHappyPath(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{queryRows: newRows([]string{"n"}, nil)}
	items, err := Many[int](context.Background(), f, func(r Row) (int, error) {
		var v int
		return v, r.Scan(&v)
	}, "q")
	if err != nil {
		t.Fatalf("expected nil error on empty result set, got %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty slice, got %v", items)
	}
}

func TestMaps_EmptyRows_IsHappyPath(t *testing.T) {
	t.Parallel()

	f := &fakeRowQuerier{queryRows: newRows([]string{"id", "name"}, nil)}
	out, err := Maps(context.Background(), f, "q")
	if err != nil {
		t.Fatalf("expected nil error on empty result set, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty slice, got %v", out)
	}
}

func TestStructsByName_EmptyRows_IsHappyPath(t *testing.T) {
	t.Parallel()

	type u struct {
		ID   int
		Name string
	}
	f := &fakeRowQuerier{queryRows: newRows([]string{"id", "name"}, nil)}
	out, err := StructsByName[u](context.Background(), f, "q")
	if err != nil {
		t.Fatalf("expected nil error on empty result set, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty slice, got %v", out)
	}
}

func TestMany_ReturnsRowsErr_WhenIteratorErrors(t *testing.T) {
	t.Parallel()

	// rows.Err should propagate even if we never enter the loop
	r := newRows([]string{"n"}, nil)
	r.err = errors.New("iter blew up")
	f := &fakeRowQuerier{queryRows: r}

	items, err := Many[int](context.Background(), f, func(Row) (int, error) { return 0, nil }, "q")
	if err == nil || err.Error() != "iter blew up" {
		t.Fatalf("expected rows.Err to bubble, got %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil slice on error, got %v", items)
	}
}

func TestMaps_ReturnsRowsErr_WhenIteratorErrors(t *testing.T) {
	t.Parallel()

	r := newRows([]string{"id"}, nil)
	r.err = errors.New("rows kaput")
	f := &fakeRowQuerier{queryRows: r}

	out, err := Maps(context.Background(), f, "q")
	if err == nil || err.Error() != "rows kaput" {
		t.Fatalf("expected rows.Err to bubble, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil slice on error, got %v", out)
	}
}

func TestStructsByName_ReturnsRowsErr_WhenIteratorErrors(t *testing.T) {
	t.Parallel()

	type u struct{ ID int }
	r := newRows([]string{"id"}, nil)
	r.err = errors.New("boom rows")
	f := &fakeRowQuerier{queryRows: r}

	out, err := StructsByName[u](context.Background(), f, "q")
	if err == nil || err.Error() != "boom rows" {
		t.Fatalf("expected rows.Err to bubble, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil slice on error, got %v", out)
	}
}

func TestIndexStructFields_SkipsUnexported_AndCaseInsensitive(t *testing.T) {
	t.Parallel()

	type demo struct {
		ID int // exported
	}
	m := indexStructFields(reflect.TypeOf(demo{}))
	// exported present (case-insensitive)
	if _, ok := m["id"]; !ok {
		t.Fatalf("expected id key present")
	}
	// unexported absent
	if _, ok := m["name"]; ok {
		t.Fatalf("did not expect unexported field to be indexed")
	}
}

func TestAssign_Incompatible_NoOpLeavesZero(t *testing.T) {
	t.Parallel()

	type dstStruct struct {
		V int
	}
	var target dstStruct
	rv := reflect.ValueOf(&target).Elem().FieldByName("V")

	// assign a type that can't convert or assign to int
	type weird struct{ X string }
	assign(rv, weird{X: "nope"})

	if target.V != 0 {
		t.Fatalf("expected zero value on incompatible assign, got %v", target.V)
	}
}

func TestAssign_ByteStringConversions_Explicit(t *testing.T) {
	t.Parallel()

	// []byte -> string
	var s struct{ S string }
	sv := reflect.ValueOf(&s).Elem().FieldByName("S")
	assign(sv, []byte("bytes"))
	if s.S != "bytes" {
		t.Fatalf("[]byte->string assign failed, got %q", s.S)
	}

	// string -> []byte
	var b struct{ B []byte }
	bv := reflect.ValueOf(&b).Elem().FieldByName("B")
	assign(bv, "str")
	if string(b.B) != "str" {
		t.Fatalf("string->[]byte assign failed, got %q", string(b.B))
	}
}

func TestMap_SingleRow_HappyPath_Again(t *testing.T) {
	t.Parallel()

	cols := []string{"id", "name"}
	data := [][]any{{int32(9), []byte("neo")}}
	f := &fakeRowQuerier{queryRows: newRows(cols, data)}

	m, err := Map(context.Background(), f, "q")
	if err != nil {
		t.Fatalf("Map err: %v", err)
	}
	if m["id"] != int32(9) {
		t.Fatalf("id mismatch: %#v", m["id"])
	}
	v, ok := m["name"].([]byte)
	if !ok || string(v) != "neo" {
		t.Fatalf("name mismatch: %#v", m["name"])
	}
}
