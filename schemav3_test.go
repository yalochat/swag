package swag

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sv-tools/openapi/spec"
)

func TestBuildCustomSchemaV3(t *testing.T) {
	tests := []struct {
		types              []string
		expectedSchemaSpec *spec.Schema
		expectedError      string
	}{
		{
			types: []string{"string"},
			expectedSchemaSpec: &spec.Schema{
				JsonSchema: spec.JsonSchema{
					JsonSchemaCore: spec.JsonSchemaCore{
						Type: &spec.SingleOrArray[string]{"string"},
					},
				},
			},
		},
		{
			types: []string{"primitive", "string"},
			expectedSchemaSpec: &spec.Schema{
				JsonSchema: spec.JsonSchema{
					JsonSchemaCore: spec.JsonSchemaCore{
						Type: &spec.SingleOrArray[string]{"string"},
					},
				},
			},
		},
		{
			types: []string{"array", "string"},
			expectedSchemaSpec: &spec.Schema{
				JsonSchema: spec.JsonSchema{
					JsonSchemaCore: spec.JsonSchemaCore{
						Type: &spec.SingleOrArray[string]{"array"},
					},
					JsonSchemaTypeArray: spec.JsonSchemaTypeArray{
						Items: &spec.BoolOrSchema{
							Schema: &spec.RefOrSpec[spec.Schema]{
								Ref: nil,
								Spec: &spec.Schema{
									JsonSchema: spec.JsonSchema{
										JsonSchemaCore: spec.JsonSchemaCore{
											Type: &spec.SingleOrArray[string]{"string"},
										},
									},
								},
							},
							Allowed: true,
						},
					},
				},
			},
		},
		{
			types:         []string{"array"},
			expectedError: "need array item type after array",
		},
		{
			types:         []string{"primitive"},
			expectedError: "need primitive type after primitive",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.types), func(t *testing.T) {

			schema, err := BuildCustomSchemaV3(test.types)
			require.Equal(t, test.expectedError != "", err != nil, "expected error: %v", test.expectedError)
			if test.expectedSchemaSpec != nil {
				expectedSchema := &spec.RefOrSpec[spec.Schema]{
					Spec: test.expectedSchemaSpec,
				}
				require.Equal(t, expectedSchema, schema)
			}
			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
			}
		})
	}
}
