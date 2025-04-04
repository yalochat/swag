package swag

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFieldJSONTag(t *testing.T) {
	type testCase struct {
		typeSpecDef *TypeSpecDef
		fieldName   string
		expectedTag string
		expectedEmb bool
	}

	parser := New()

	tests := []testCase{
		{
			typeSpecDef: &TypeSpecDef{
				TypeSpec: &ast.TypeSpec{
					Name: ast.NewIdent("TestStruct"),
					Type: &ast.StructType{
						Fields: &ast.FieldList{
							List: []*ast.Field{
								{
									Names: []*ast.Ident{ast.NewIdent("ID")},
									Tag:   &ast.BasicLit{Value: "`json:\"id\"`"},
								},
							},
						},
					},
				},
			},
			fieldName:   "ID",
			expectedTag: "id",
			expectedEmb: false,
		},
		{
			typeSpecDef: &TypeSpecDef{
				TypeSpec: &ast.TypeSpec{
					Name: ast.NewIdent("TestStruct"),
					Type: &ast.StructType{
						Fields: &ast.FieldList{
							List: []*ast.Field{
								{
									Names: []*ast.Ident{ast.NewIdent("Name")},
									Tag:   &ast.BasicLit{Value: "`json:\"name\"`"},
								},
							},
						},
					},
				},
			},
			fieldName:   "Name",
			expectedTag: "name",
			expectedEmb: false,
		},
		{
			// Field without JSON tag
			typeSpecDef: &TypeSpecDef{
				TypeSpec: &ast.TypeSpec{
					Name: ast.NewIdent("NoTagStruct"),
					Type: &ast.StructType{
						Fields: &ast.FieldList{
							List: []*ast.Field{
								{
									Names: []*ast.Ident{ast.NewIdent("Age")},
								},
							},
						},
					},
				},
			},
			fieldName:   "Age",
			expectedTag: "Age",
			expectedEmb: false,
		},
		{
			// Embedded struct case
			typeSpecDef: &TypeSpecDef{
				TypeSpec: &ast.TypeSpec{
					Name: ast.NewIdent("EmbeddedStruct"),
					Type: &ast.StructType{
						Fields: &ast.FieldList{
							List: []*ast.Field{}, // No explicit fields
						},
					},
				},
			},
			fieldName:   "SomeField",
			expectedTag: "SomeField",
			expectedEmb: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			tag, embedded := parser.getFieldJSONTag(tt.typeSpecDef.TypeSpec.Type, tt.fieldName, &ast.File{})
			if tag != tt.expectedTag || embedded != tt.expectedEmb {
				t.Errorf("Expected (%s, %v), got (%s, %v)", tt.expectedTag, tt.expectedEmb, tag, embedded)
			}
		})
	}
}

func TestHandleMapCompositeLiteral(t *testing.T) {
	tests := []struct {
		name     string
		input    *ast.CompositeLit
		expected map[string]interface{}
	}{
		{
			name: "empty map",
			input: &ast.CompositeLit{
				Elts: []ast.Expr{},
			},
			expected: map[string]interface{}{},
		},
		{
			name: "simple string key-value pairs",
			input: &ast.CompositeLit{
				Elts: []ast.Expr{
					&ast.KeyValueExpr{
						Key:   &ast.BasicLit{Value: `"key1"`, Kind: token.STRING},
						Value: &ast.BasicLit{Value: `"value1"`, Kind: token.STRING},
					},
					&ast.KeyValueExpr{
						Key:   &ast.BasicLit{Value: `"key2"`, Kind: token.STRING},
						Value: &ast.BasicLit{Value: `42`, Kind: token.INT},
					},
				},
			},
			expected: map[string]interface{}{
				"key1": `value1`,
				"key2": int64(42),
			},
		},
		{
			name: "non-string keys that get converted",
			input: &ast.CompositeLit{
				Elts: []ast.Expr{
					&ast.KeyValueExpr{
						Key:   &ast.BasicLit{Value: `42`, Kind: token.INT},      // int key
						Value: &ast.BasicLit{Value: `"value1"`, Kind: token.STRING}, // string value
					},
					&ast.KeyValueExpr{
						Key:   &ast.Ident{Name: "StringExample"}, // this identifier is in the testdata/param_structs/instances.go file
						Value: &ast.BasicLit{Value: `3.14`, Kind: token.FLOAT}, // float value
					},
				},
			},
			expected: map[string]interface{}{
				"42":       `value1`,
				"AwesomeString": 3.14,
			},
		},
	}

	packagePath := "testdata/param_structs"
  filePath := packagePath + "/instances.go"
  src, err := os.ReadFile(filePath)
  assert.NoError(t, err)

  fileSet := token.NewFileSet()
  fileAST, err := goparser.ParseFile(fileSet, "", src, goparser.ParseComments)
  assert.NoError(t, err)

  parser := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.handleMapCompositeLiteral(tt.input, nil, fileAST)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected map length %d, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				actualValue, exists := result[key]
				if !exists {
					t.Errorf("Expected key %q not found in result", key)
					continue
				}

				if reflect.TypeOf(actualValue) != reflect.TypeOf(expectedValue) {
					t.Errorf("For key %q, expected value type %v, got %v", key, reflect.TypeOf(expectedValue), reflect.TypeOf(actualValue))
				} else if actualValue != expectedValue {
					t.Errorf("For key %q, expected value %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}
