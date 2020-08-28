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
	type Request4 struct {
		Limit string `in:"query" query:"limit,default=10"`
	}

	type args struct {
		typ reflect.Type
	}
	tests := []struct {
		name string
		args args
		want requestBindingStruct
	}{
		{
			name: "args.Param should have id key when ID taged",
			args: args{reflect.TypeOf(&Request1{})},
			want: requestBindingStruct{
				Param: map[string]requestBindingArgs{
					"ID": {
						Key: "id",
					},
				}},
		},
		{
			name: "args.Boody should have person key when Person taged",
			args: args{reflect.TypeOf(&Request2{})},
			want: requestBindingStruct{
				Body: []string{"Person"},
			},
		},
		{
			name: "args.Header should have Authorization key when Authorization taged",
			args: args{reflect.TypeOf(&Request3{})},
			want: requestBindingStruct{
				Header: map[string]requestBindingArgs{
					"Authorization": {
						Key: "Authorization",
					},
				}},
		},
		{
			name: "args.Query should have Limit key when Limit taged",
			args: args{reflect.TypeOf(&Request4{})},
			want: requestBindingStruct{
				Query: map[string]requestBindingArgs{
					"Limit": {
						Key: "limit",
						options: setOptions{
							isDefaultExists: true,
							defaultValue:    "10",
						},
					},
				}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildBindingStruct(tt.args.typ); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildBindingStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}
