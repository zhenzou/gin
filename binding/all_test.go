package binding

import (
	"reflect"
	"testing"
)

func Test_extractBindingArgs(t *testing.T) {
	type Request1 struct {
		ID string `in:"param" param:"id"`
	}
	type Request2 struct {
		Person struct {
			Name string `json:"name"`
		} `in:"body"`
	}
	type Request3 struct {
		Authorization string `in:"header" header:"Authorization"`
	}

	type args struct {
		typ reflect.Type
	}
	tests := []struct {
		name string
		args args
		want bindingArgs
	}{
		{
			name: "args.Param should have id key when ID taged",
			args: args{reflect.TypeOf(&Request1{})},
			want: bindingArgs{Param: map[string]string{
				"ID": "id",
			}},
		},
		{
			name: "args.Boody should have person key when Person taged",
			args: args{reflect.TypeOf(&Request2{})},
			want: bindingArgs{Body: []string{"Person"}},
		},
		{
			name: "args.Header should have Authorization key when Authorization taged",
			args: args{reflect.TypeOf(&Request3{})},
			want: bindingArgs{Header: map[string]string{
				"Authorization": "Authorization",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractBindingArgs(tt.args.typ); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractBindingArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
