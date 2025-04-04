package swag

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

const (
	BOOLEAN_STRING_TRUE = "true"
	BOOLEAN_STRING_FALSE = "false"
)

/*
Entry point to get the example instance from the AST file.
It will walk through the AST to find the declaration of `attr` and return its value.
If a struct, it will also use the type definition of the current type spec to resolve
the JSON tags (if any) and replace the keys in the returned object with the JSON tags.
*/
func (parser *Parser) getExampleByInstance(currASTFile *ast.File, attr string) (interface{}, error) {
	if currASTFile == nil {
		return nil, fmt.Errorf("astFile cannot be nil")
	}

	// Walk through the AST to find the declaration of `attr`
	example, identFound := parser.resolveIdentValue(attr, nil, currASTFile)

	if !identFound {
		return nil, fmt.Errorf("example instance not found or unsupported type")
	}

	return example, nil
}

/*
Gets the package path from the imports object. It is based on how the the package is referenced
in the file. So if the import has an alias, it will be used to find the package path.
If the import does not have an alias, it will be used the package name as it is defined in the
package itself (that is how golang resolves the package name).
*/
func (parser *Parser) findPackagePathInImports(pkgName string, imports []*ast.ImportSpec) (string, error) {
	for _, importObj := range imports {
		importPath := strings.Trim(importObj.Path.Value, `"`)
		alias := importObj.Name

		if alias != nil {
			if alias.Name == pkgName {
				return importPath, nil
			}
			continue
		}

		if currPkgDefinition, ok := parser.packages.packages[importPath]; ok {
			if currPkgDefinition.Name == pkgName {
				return importPath, nil
			}
		}
	}

	return "", fmt.Errorf("could not find package path for package name: %s", pkgName)
}

/*
Ranges through the packages and finds the package path for the package name.
The `pkgName` should be the package name as it is defined in the package itself,
and not an alias.
*/
func (parser *Parser) findPackagePathFromPackageName(pkgName string) (string, error) {
	for pkgPath, pkgDefinition := range parser.packages.packages {
		if pkgDefinition.Name == pkgName {
			return pkgPath, nil
		}
	}
	return "", fmt.Errorf("could not find package path for package name: %s", pkgName)
}

/*
Ranges through the type definitions of a particular package path and returns the type with the name
<schemaName>.
<pkgPath> should be the package path and not the package name.
<schemaName> should be the name of the type without the package prefix.
*/
func (parser *Parser) getTypeSpecDefFromSchemaNameAndPkgPath(schemaName string, pkgPath string) (*TypeSpecDef, error) {
	if pkgDefinition, ok := parser.packages.packages[pkgPath]; ok {
		if typeSpecDef, ok := pkgDefinition.TypeDefinitions[schemaName]; ok {
			return typeSpecDef, nil
		}
	}
	return nil, fmt.Errorf("could not find typeSpecDef for schemaName: %s and pkgPath: %s", schemaName, pkgPath)
}

/*
Parsers the composite literal based on its type.
It will call the appropriate handler based on the type of the composite literal.
*/
func (parser *Parser) parseCompositeLiteral(literal *ast.CompositeLit, currTypeDefinition ast.Expr, file *ast.File) interface{} {
	switch literal.Type.(type) {
	case *ast.ArrayType:
		return parser.handleArrayCompositeLiteral(literal, literal.Type.(*ast.ArrayType).Elt, file)
	case *ast.MapType:
		return parser.handleMapCompositeLiteral(literal, literal.Type.(*ast.MapType).Value, file)
	default:
		return parser.handleStructCompositeLiteral(literal, literal.Type, file)
	}
}

/*
parseExpr parses the expression and returns the value based on its type.

- If the expression type is a basic literal, it will return the value.

- If the expression type is composite literal, it will update the type definition
associated with it and then recursively parse its elements. 

- If the expression type is a unary expression, it will return the value of the
dereferenced expression recursively.

- If the expression type is an identifier, it will resolve the identifier and then
parse it recursively. It will also check if the identifier is a boolean string
(true/false) and return the corresponding boolean value.

- If the expression type is a selector expression, it will find the package path associated
with the identifier, update the current file and type definition, and recursively
parse its value.
*/
func (parser *Parser) parseExpr(expr ast.Expr, currTypeDefinition ast.Expr, file *ast.File) interface{} {
	switch astExpr := expr.(type) {
	case *ast.BasicLit:
		return parseBasicLiteral(astExpr)
	case *ast.CompositeLit:
		if astExpr.Type == nil {
			astExpr.Type = currTypeDefinition
		}
		return parser.parseCompositeLiteral(astExpr, currTypeDefinition, file) // Nested structs or arrays
	case *ast.UnaryExpr:
		if astExpr.Op == token.AND {
			return parser.parseExpr(astExpr.X, currTypeDefinition, file) // Dereference and parse the value
		}
		return nil
	case *ast.Ident:
		if astExpr.Name == BOOLEAN_STRING_TRUE {
			return true
		} else if astExpr.Name == BOOLEAN_STRING_FALSE {
			return false
		}
		// Handle variable references
		value, _ := parser.resolveIdentValue(astExpr.Name, currTypeDefinition, file)
		return value

	case *ast.SelectorExpr:
		// Handle package references
		pkgName := astExpr.X.(*ast.Ident).Name
		typeName := astExpr.Sel.Name
		pkgPath, err := parser.findPackagePathInImports(pkgName, file.Imports)
		if err != nil {
			fmt.Println("Error in selector expr: ", err)
			return nil
		}

		if packageDefinition, ok := parser.packages.packages[pkgPath]; ok {
			for _, pkgFile := range packageDefinition.Files {
				value, identFound := parser.resolveIdentValue(typeName, nil, pkgFile)
				if identFound {
					return value
				}
			}
		}

		return nil
	default:
		fmt.Println("[parseExpr] Unsupported type: ", fmt.Sprintf("%T", astExpr))
		return nil // Unsupported type
	}
}

/*
Resolves the identifier value in the AST file.
It will walk through the AST to find the declaration of `identName` and return its value.
An ast.Ident (short for AST Identifier) represents a name used in Go code. It could be the
name of a variable, a function, a type, a constant, or even a package. Essentially, an ast.
Ident is any word in Go that serves as a named entity. 

This function is first used to resolve the value of the example instance. Whenever a new
identifier is found, this function is called to resolve it.
*/
func (parser *Parser) resolveIdentValue(identName string, currTypeDefinition ast.Expr, file *ast.File) (interface{}, bool) {
		var value interface{}
		identFound := false
		ast.Inspect(file, func(n ast.Node) bool {
			decl, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}

			for i, name := range decl.Names {
				if name.Name == identName && len(decl.Values) > i {
					value = parser.parseExpr(decl.Values[i], currTypeDefinition, file)
					identFound = true
					return false // Stop walking
				}
			}
			return true
		})

		return value, identFound
}

/*
Returns the value of the basic literal based on its kind.
*/
func parseBasicLiteral(literal *ast.BasicLit) interface{} {
	switch literal.Kind {
	case token.INT:
		if asInt, err := strconv.ParseInt(literal.Value, 10, 64); err == nil {
			return asInt
		}
		return literal.Value
	case token.FLOAT:
		if asFloat, err := strconv.ParseFloat(literal.Value, 64); err == nil {
			return asFloat
		}
		return literal.Value
	case token.STRING:
		return literal.Value[1 : len(literal.Value)-1] // Remove quotes
	default:
		return literal.Value
	}
}

func (parser *Parser) handleMapCompositeLiteral(literal *ast.CompositeLit, currTypeDefinition ast.Expr, file *ast.File) map[string]interface{} {
	obj := make(map[string]interface{})
	for _, compositeElement := range literal.Elts {
		if keyValueExpr, ok := compositeElement.(*ast.KeyValueExpr); ok {
			key := parser.parseExpr(keyValueExpr.Key, currTypeDefinition, file)
			value := parser.parseExpr(keyValueExpr.Value, currTypeDefinition, file)

			// Ensure key is a string (convert if needed)
			keyStr, ok := key.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", key)
			}
			obj[keyStr] = value
		}
	}
	return obj
}

/*
Handles the parsing of array composite literals. It always uses the same type definition,
since the array elements types are always the same. It will return a slice of interface{} 
with the values of the array.
*/
func (parser *Parser) handleArrayCompositeLiteral(literal *ast.CompositeLit, currTypeDefinition ast.Expr, file *ast.File) []interface{} {
	var arr []interface{}
	for _, elem := range literal.Elts {
		value := parser.parseExpr(elem, currTypeDefinition, file)
		arr = append(arr, value)
	}
	return arr
}

/*
Get JSON tag for a field in a struct. If no JSON tag is found, returns empty string.
*/
func getJSONTag(field *ast.Field) string {
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`") // Remove backticks
		jsonTag := reflect.StructTag(tagValue).Get("json")
		if jsonTag != "" {
			return strings.Split(jsonTag, ",")[0] // Ignore omitempty, etc.
		}
	}
	return ""
}

/*
Gets the type schema from the type definition of the struct. It will check if the type definition
is a pointer, a selector expression, or a identifier.
*/
func (parser *Parser) getTypeSchemaFromTypeDefinition(typeDefinition ast.Expr, astFile *ast.File) (ast.Expr, error) {
	switch castTypeDef := typeDefinition.(type) {
	case *ast.Ident:
		pkgPath, _ := parser.findPackagePathFromPackageName(astFile.Name.Name)
		typeSpecDef, err := parser.getTypeSpecDefFromSchemaNameAndPkgPath(castTypeDef.Name, pkgPath)
		if err != nil {
			return nil, fmt.Errorf("error getting type schema from *ast.Ident: %s", err)
		}
		return typeSpecDef.TypeSpec.Type, nil
	
	case *ast.SelectorExpr:
		compositePkg := castTypeDef.X.(*ast.Ident).Name
		pkgPath, err := parser.findPackagePathInImports(compositePkg, astFile.Imports)
		if err != nil {
			return nil, fmt.Errorf("error getting package path from *ast.SelectorExpr: %s", err)
		}

		typeSpecDef, err := parser.getTypeSpecDefFromSchemaNameAndPkgPath(castTypeDef.Sel.Name, pkgPath)
		if err != nil {
			return nil, fmt.Errorf("error getting type schema from *ast.SelectorExpr: %s", err)
		}

		return typeSpecDef.TypeSpec.Type, nil
	
	case *ast.StarExpr:
		return parser.getTypeSchemaFromTypeDefinition(castTypeDef.X, astFile)

	default:
		return typeDefinition, nil
	}
}

/*
Range through the fields of a struct and find the JSON tag for a field.
If the field is found but no JSON tag is found, it returns the field name.
If the field is not found, it is considered an embedded field.
*/
func (parser *Parser) getFieldJSONTag(typeDefinition ast.Expr, fieldName string, astFile *ast.File) (string, bool) {
	isEmbedded := true

	typeSchema, err := parser.getTypeSchemaFromTypeDefinition(typeDefinition, astFile)
	if err != nil {
		fmt.Println("Error getting type schema from type definition: ", err)
		return fieldName, false
	}

	if structType, ok := typeSchema.(*ast.StructType); ok {
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				if name.Name == fieldName {
					isEmbedded = false
					tag := getJSONTag(field)
					if tag != "" {
						return tag, isEmbedded
					}
					break
				}
			}
		}
	}

	return fieldName, isEmbedded
}

/*
Handles the parsing of struct composite literals. It will go through the fields
of the struct and find the JSON tag for each field. If no JSON tag is found,
it will use the field name as the key.
It will also handle embedded structs by flattening them into the parent struct.
Returns a map[string]interface{} with the values of the struct.
*/
func (parser *Parser) handleStructCompositeLiteral(literal *ast.CompositeLit, currTypeDefinition ast.Expr, file *ast.File) interface{} {	
	obj := make(map[string]interface{})
	embeddedKeys := []string{} 
	for _, compositeElement := range literal.Elts {
		if keyValueExpr, ok := compositeElement.(*ast.KeyValueExpr); ok {
			if key, ok := keyValueExpr.Key.(*ast.Ident); ok {
				tag, isEmbedded := parser.getFieldJSONTag(currTypeDefinition, key.Name, file)
				if isEmbedded {
					embeddedKeys = append(embeddedKeys, tag)
				}
				obj[tag] = parser.parseExpr(keyValueExpr.Value, currTypeDefinition, file)
			}
		}
	}

	// If there are embedded structs, we need to flatten the struct
	if len(embeddedKeys) > 0 {
		for _, key := range embeddedKeys {
			embeddedObj, ok := obj[key].(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range embeddedObj {
				obj[k] = v
			}
			delete(obj, key)
		}
	}

	return obj
}
