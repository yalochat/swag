package inner

type InnerStruct struct {
	AwesomeField string              `json:"awesomeField"`
	MapField     map[string]int      `json:"mapField"`
	MapToArray   map[string][]string `json:"mapToArray"`
}
