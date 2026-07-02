package tests

import (
	"github.com/tinywasm/router"
)

type mockContext struct {
	method  string
	path    string
	body    []byte
	headers map[string]string
	status  int
	output  []byte
	values  map[string]any
}

func newMockContext(method, path string) *mockContext {
	return &mockContext{
		method:  method,
		path:    path,
		headers: make(map[string]string),
		values:  make(map[string]any),
	}
}

var _ router.Context = (*mockContext)(nil)

func (c *mockContext) Method() string              { return c.method }
func (c *mockContext) Path() string                { return c.path }
func (c *mockContext) Body() []byte                { return c.body }
func (c *mockContext) GetHeader(k string) string   { return c.headers[k] }
func (c *mockContext) SetHeader(k, v string)       { c.headers[k] = v }
func (c *mockContext) WriteStatus(code int)        { c.status = code }
func (c *mockContext) Write(b []byte) (int, error) { c.output = append(c.output, b...); return len(b), nil }
func (c *mockContext) SetValue(k string, v any)    { c.values[k] = v }
func (c *mockContext) Value(k string) any          { return c.values[k] }
