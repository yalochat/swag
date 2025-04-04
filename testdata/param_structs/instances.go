package structs

import (
	outsidePkg "github.com/yalochat/swag/v2/testdata/param_structs/inner"
)

var StringExample = "AwesomeString"
var IntExample = 1
var BoolExample = true
var FloatExample = 1.1

var FormModelExample = FormModel {
	Foo: "foo",
	B: true,
}

var CompositeStructExample = CompositeStruct {
	FormModelExample: &FormModelExample,
	PathModelExample: PathModel {
		Identifier: 1,
		Name: "name",
	},
	MapExample: map[string]AuthHeader {
		"key": AuthHeader {
			Token: "token",
			AnotherHeader: 1,
		},
	},
	ArrayExample: []FormModel{FormModelExample},
}

var OutsidePkgExample = outsidePkg.InnerStruct {
	AwesomeField: "awesome",
	MapField: map[string]int {
		"key": 1,
		"key2": 2,
	},
	MapToArray: map[string][]string {
		"key": {"value1", "value2"},
		"key2": {"value3", "value4"},
	},
}

var EmbeddedStructExample = EmbeddedStruct {
	FormModel: FormModelExample,
	AwesomeField: "awesome",
}
