package inner

import (
	structs "github.com/yalochat/swag/v2/testdata/param_structs"
)

type InnerStruct struct {
	AwesomeField string `json:"awesomeField"`
	FormModelExample *structs.FormModel `json:"formModelExample"`
}
