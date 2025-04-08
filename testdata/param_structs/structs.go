package structs

type FormModel struct {
	Foo string `form:"f" binding:"required" validate:"max=10" json:"foo"`
	// B is another field
	B bool `json:"b"`
}

type AuthHeader struct {
	// Token is the auth token
	Token string `header:"X-Auth-Token" binding:"required"`
	// AnotherHeader is another header
	AnotherHeader int `validate:"gte=0,lte=10"`
}

type PathModel struct {
	// ID is the id
	Identifier int    `uri:"id" binding:"required"`
	Name       string `validate:"max=10"`
}

type CompositeStruct struct {
	FormModelExample *FormModel            `json:"formModelExample"`
	PathModelExample PathModel             `json:"pathModelExample"`
	MapExample       map[string]AuthHeader `json:"mapExample"`
	ArrayExample     []FormModel           `json:"arrayExample"`
}

type EmbeddedStruct struct {
	FormModel
	AwesomeField string `json:"awesomeField"`
}
