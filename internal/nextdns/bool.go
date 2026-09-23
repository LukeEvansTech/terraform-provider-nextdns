package nextdns

// Bool returns a pointer to v. Pointer booleans are used for the profile
// fields whose absence from a request must mean "leave unchanged" rather
// than "set to false": with `omitempty`, a nil pointer is omitted from the
// PATCH body while an explicit false is still serialised.
func Bool(v bool) *bool {
	return &v
}
