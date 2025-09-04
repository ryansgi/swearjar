package repokit

// Binder is a tiny factory that binds a domain repo to a specific Queryer
type Binder[T any] interface {
	Bind(Queryer) T
}

// BindFunc lets you create a Binder from a function
type BindFunc[T any] func(Queryer) T

// Bind calls the underlying function
func (f BindFunc[T]) Bind(q Queryer) T { return f(q) }

// RequireQueryer panics early on programmer error (nil q)
func RequireQueryer(q Queryer) Queryer {
	if q == nil {
		panic("repokit: nil Queryer")
	}
	return q
}

// MustBind is a convenience that validates q then binds
func MustBind[T any](b Binder[T], q Queryer) T {
	return b.Bind(RequireQueryer(q))
}
