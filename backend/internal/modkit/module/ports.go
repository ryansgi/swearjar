package module

import "reflect"

// PortSet is a marker for module defined port sets
// modules should define their own concrete interface types and return them from Ports
type PortSet = any

// PortsOf pulls an interface T out of a module's Ports() bundle without using the registry
// it returns ok=false if no field/value in Ports() implements T
func PortsOf[T any](m Module) (t T, ok bool) {
	p := m.Ports()
	if p == nil {
		return t, false
	}
	// direct implement
	if v, ok2 := p.(T); ok2 {
		return v, true
	}
	rv := reflect.ValueOf(p)
	rt := rv.Type()
	// only walk exported fields of structs
	if rt.Kind() == reflect.Struct {
		for i := 0; i < rt.NumField(); i++ {
			f := rv.Field(i)
			if !f.CanInterface() {
				continue
			}
			if v, ok2 := f.Interface().(T); ok2 {
				return v, true
			}
		}
	}
	return t, false
}

// MustPortsOf is a convenience that panics with a friendly message
func MustPortsOf[T any](m Module) T {
	if v, ok := PortsOf[T](m); ok {
		return v
	}
	panic("module: requested port not found on module " + m.Name())
}
