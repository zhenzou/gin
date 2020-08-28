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

// requestBinding binding all request info
// include body,form,query header,path
type requestBinding struct {
	cache sync.Map
}

func (a *requestBinding) Name() string {
	return "all"
}

// requestBindingStruct for requestBinding to cache binding config
// For now, support form, path, body, header only
type requestBindingStruct struct {
	Form   map[string]requestBindingArgs // fieldName -> binding args
	Param  map[string]requestBindingArgs
	Query  map[string]requestBindingArgs
	Header map[string]requestBindingArgs
	Body   []string
}

type requestBindingArgs struct {
	Key     string
	options setOptions
}

func (b *requestBindingStruct) Merge(args requestBindingStruct) {
	b.Form = mergeArgsMap(b.Form, args.Form)
	b.Param = mergeArgsMap(b.Param, args.Param)
	b.Query = mergeArgsMap(b.Query, args.Query)
	b.Header = mergeArgsMap(b.Header, args.Header)
	b.Body = append(b.Body, args.Body...)
}

func mergeArgsMap(a, b map[string]requestBindingArgs) map[string]requestBindingArgs {
	if a == nil {
		a = map[string]requestBindingArgs{}
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

// BindRequest will bind request to ptr
func (a *requestBinding) BindRequest(request *http.Request, params map[string][]string, ptr interface{}) error {
	typ := reflect.TypeOf(ptr)
	if typ.Kind() != reflect.Ptr {
		panic("bind target must be ptr")
	}
	value, ok := a.cache.Load(typ)
	if ok {
		return a.bindRequest(request, value.(requestBindingStruct), params, ptr)
	}
	args := buildBindingStruct(typ)
	a.cache.Store(typ, args)
	return a.bindRequest(request, args, params, ptr)
}

// bindRequest will bind request
func (a *requestBinding) bindRequest(req *http.Request, args requestBindingStruct,
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

func (a *requestBinding) trySetWithArgs(args map[string]requestBindingArgs,
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
func buildBindingStruct(typ reflect.Type) requestBindingStruct {
	bindingStruct := requestBindingStruct{}
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
		in := field.Tag.Get("in")

		var m map[string]requestBindingArgs
		var taged bool
		var args requestBindingArgs

		switch in {
		case "body":
			bindingStruct.Body = append(bindingStruct.Body, field.Name)
			// no need to do anything else
			continue
		case "form":
			if bindingStruct.Form == nil {
				bindingStruct.Form = map[string]requestBindingArgs{}
			}
			m = bindingStruct.Form

		case "header":
			if bindingStruct.Header == nil {
				bindingStruct.Header = map[string]requestBindingArgs{}
			}
			m = bindingStruct.Header

		case "param", "path":
			if bindingStruct.Param == nil {
				bindingStruct.Param = map[string]requestBindingArgs{}
			}
			m = bindingStruct.Param
		case "query":
			if bindingStruct.Query == nil {
				bindingStruct.Query = map[string]requestBindingArgs{}
			}
			m = bindingStruct.Query
		default:
			// just ignore
			// force to tag args
			continue
		}

		args, taged = extractBindingArgs(field, in)
		if taged && m != nil {
			m[field.Name] = args
		}
	}
	return bindingStruct
}

func extractBindingArgs(field reflect.StructField, tagName string) (requestBindingArgs, bool) {
	args := requestBindingArgs{}

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
