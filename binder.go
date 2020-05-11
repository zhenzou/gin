package gin

// Binder provider a customized way to bind request
type Binder interface {
	Bind(c *Context) error
}
