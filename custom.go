// custom some custom thing for myself to use

package gin

// Binder provider a customized way to bind request
type Binder interface {
	Bind(c *Context) error
}

// Path the same with Param
func (c *Context) Path(key string) string {
	return c.Params.ByName(key)
}
