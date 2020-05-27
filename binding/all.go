package binding

import (
	"go/ast"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin/internal/json"
)

type allBinding struct {
	cache sync.Map
}

func (a *allBinding) Name() string {
	return "all"
}

// BindingStruct for allBinding
// For now, support form, param, body, header only
type BindingStruct struct {
	Form   map[string]BindingArgs
	Param  map[string]BindingArgs
	Query  map[string]BindingArgs
	Header map[string]BindingArgs
	Body   []string
}

type BindingArgs struct {
	Key     string
	options setOptions
}

func (b *BindingStruct) Merge(args BindingStruct) {
	b.Form = mergeArgsMap(b.Form, args.Form)
	b.Param = mergeArgsMap(b.Param, args.Param)
	b.Query = mergeArgsMap(b.Query, args.Query)
	b.Header = mergeArgsMap(b.Header, args.Header)
	b.Body = append(b.Body, args.Body...)
}

func mergeArgsMap(a, b map[string]BindingArgs) map[string]BindingArgs {
	if a == nil {
		a = map[string]BindingArgs{}
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

// BindAll will bind request
func (a *allBinding) BindAll(request *http.Request, params map[string][]string, ptr interface{}) error {
	typ := reflect.TypeOf(ptr)
	if typ.Kind() != reflect.Ptr {
		panic("bind target must be ptr")
	}
	value, ok := a.cache.Load(typ)
	if ok {
		return a.bindAll(request, value.(BindingStruct), params, ptr)
	}
	args := buildBindingStruct(typ)
	a.cache.Store(typ, args)
	return a.bindAll(request, args, params, ptr)
}

// bindAll will bind request
func (a *allBinding) bindAll(req *http.Request, args BindingStruct,
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
func (a *allBinding) trySetWithArgs(args map[string]BindingArgs,
	values map[string][]string, typ reflect.Type, value reflect.Value) error {
	if values == nil {
		return nil
	}
	for f, arg := range args {
		fv := value.FieldByName(f)
		ft, _ := typ.FieldByName(f)
		_, err := setByForm(fv, ft, values, arg.Key, arg.options)
		if err != nil {
			return err
		}
	}
	return nil
}

// buildBindingStruct
func buildBindingStruct(typ reflect.Type) BindingStruct {
	bindingStruct := BindingStruct{}
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		panic("binding target must be ptr of struct")
	}
	for i := typ.NumField() - 1; i >= 0; i-- {
		field := typ.Field(i)
		if !ast.IsExported(field.Name) {
			continue
		}
		if field.Anonymous {
			subArgs := buildBindingStruct(field.Type)
			bindingStruct.Merge(subArgs)
			continue
		}
		from := field.Tag.Get("in")

		var m map[string]BindingArgs
		var taged bool
		var args BindingArgs

		switch from {
		case "body":
			bindingStruct.Body = append(bindingStruct.Body, field.Name)
		case "form":
			if bindingStruct.Form == nil {
				bindingStruct.Form = map[string]BindingArgs{}
			}
			m = bindingStruct.Form
			args, taged = extractBindingArgs(field, "form")
		case "header":
			if bindingStruct.Header == nil {
				bindingStruct.Header = map[string]BindingArgs{}
			}
			m = bindingStruct.Header
			args, taged = extractBindingArgs(field, "header")

		case "param":
			if bindingStruct.Param == nil {
				bindingStruct.Param = map[string]BindingArgs{}
			}
			m = bindingStruct.Param
			args, taged = extractBindingArgs(field, "param")

		case "path":
			if bindingStruct.Param == nil {
				bindingStruct.Param = map[string]BindingArgs{}
			}
			m = bindingStruct.Param
			args, taged = extractBindingArgs(field, "path")

		case "query":
			if bindingStruct.Query == nil {
				bindingStruct.Query = map[string]BindingArgs{}
			}
			m = bindingStruct.Query
			args, taged = extractBindingArgs(field, "query")
		default:
			// just ignore
			// force to tag args
		}

		if taged && m != nil {
			m[field.Name] = args
		}
	}
	return bindingStruct
}

func extractBindingArgs(field reflect.StructField, tagName string) (BindingArgs, bool) {
	args := BindingArgs{}

	tagValue := field.Tag.Get(tagName)
	tagValue = strings.TrimSpace(tagValue)
	if tagValue == "-" {
		return args, false
	}
	tagValue, opts := head(tagValue, ",")

	if tagValue == "" { // default value is FieldName
		tagValue = field.Name
	}
	args.Key = tagValue

	var opt string
	for len(opts) > 0 {
		opt, opts = head(opts, ",")

		if k, v := head(opt, "="); k == "default" {
			args.options.isDefaultExists = true
			args.options.defaultValue = v
		}
	}
	return args, true
}
