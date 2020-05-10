package binding

import (
	"github.com/gin-gonic/gin/internal/json"
	"go/ast"
	"io/ioutil"
	"net/http"
	"reflect"
	"sync"
)

type allBinding struct {
	cache sync.Map
}

func (a *allBinding) Name() string {
	return "all"
}

// bindingArgs for allBinding
// For now, support form, param, body, header only
type bindingArgs struct {
	Form   map[string]string
	Param  map[string]string
	Query  map[string]string
	Header map[string]string
	Body   []string
}

// BindAll will bind request
func (a *allBinding) BindAll(request *http.Request, params map[string][]string, ptr interface{}) error {
	typ := reflect.TypeOf(ptr)
	value, ok := a.cache.Load(typ)
	if ok {
		return a.bindAll(request, value.(bindingArgs), params, ptr)
	}
	args := extractBindingArgs(typ)
	a.cache.Store(typ, args)
	return a.bindAll(request, args, params, ptr)
}

// bindAll will bind request
func (a *allBinding) bindAll(req *http.Request, args bindingArgs,
	params map[string][]string, ptr interface{}) error {
	typ := reflect.TypeOf(ptr).Elem()
	value := reflect.ValueOf(ptr).Elem()
	if len(args.Param) > 0 {
		err := a.trySetWithArgs(args.Param, params, typ, value)
		if err != nil {
			return err
		}
	}
	if len(args.Header) > 0 {
		err := a.trySetWithArgs(args.Header, req.Header, typ, value)
		if err != nil {
			return err
		}
	}
	if len(args.Form) > 0 {
		if err := req.ParseForm(); err != nil {
			return err
		}
		err := a.trySetWithArgs(args.Form, req.PostForm, typ, value)
		if err != nil {
			return err
		}
	}
	if len(args.Query) > 0 {
		err := a.trySetWithArgs(args.Query, req.URL.Query(), typ, value)
		if err != nil {
			return err
		}
	}
	if len(args.Body) > 0 {
		data, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
		for _, fn := range args.Body {
			fv := value.FieldByName(fn)
			if fv.Kind() != reflect.Ptr {
				fv = fv.Addr()
			}
			err := json.Unmarshal(data, fv.Interface())
			if err != nil {
				return err
			}
		}

	}
	return validate(ptr)
}

// Bind will bind request
func (a *allBinding) trySetWithArgs(args map[string]string,
	values map[string][]string, typ reflect.Type, value reflect.Value) error {
	if values == nil {
		return nil
	}
	for f, key := range args {
		vs, ok := values[key]
		if ok {
			field, _ := typ.FieldByName(f)
			err := setWithProperType(vs[0], value.FieldByName(f), field)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Bind will bind request
func extractBindingArgs(typ reflect.Type) bindingArgs {
	args := bindingArgs{}
	if typ.Kind() != reflect.Ptr {
		panic("bind target must be ptr")
	}
	typ = typ.Elem()
	if typ.Kind() != reflect.Struct {
		panic("bind target must be ptr of struct")
	}
	for i := typ.NumField() - 1; i >= 0; i-- {
		field := typ.Field(i)
		if !ast.IsExported(field.Name) {
			continue
		}
		from := field.Tag.Get("from")

		switch from {
		case "body":
			args.Body = append(args.Body, field.Name)
		case "form":
			key := field.Tag.Get("form")
			if key == "-" {
				continue
			}
			if args.Form == nil {
				args.Form = map[string]string{}
			}
			args.Form[field.Name] = key
		case "header":
			key := field.Tag.Get("header")
			if key == "-" {
				continue
			}
			if args.Header == nil {
				args.Header = map[string]string{}
			}
			args.Header[field.Name] = key
		case "param":
			key := field.Tag.Get("param")
			if key == "-" {
				continue
			}
			if args.Param == nil {
				args.Param = map[string]string{}
			}
			args.Param[field.Name] = key
		case "query":
			key := field.Tag.Get("query")
			if key == "-" {
				continue
			}
			if args.Query == nil {
				args.Query = map[string]string{}
			}
			args.Query[field.Name] = key
		}

	}
	return args
}
