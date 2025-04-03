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

func addPkgToSchemaTypeIfNeeded(schemaType string, astFile *ast.File) string {
	hasPkgName := strings.Contains(schemaType, ".")
	if hasPkgName {
		return schemaType
	}
	filePkg := astFile.Name.Name
	return fmt.Sprintf("%s.%s", filePkg, schemaType)
}

func (parser *Parser) getTypeSpecDefFromSchemaType(schemaType string, astFile *ast.File) (*TypeSpecDef, error) {
	if IsPrimitiveType(schemaType) {
		return nil, nil
	}

	schemaWithPkg := addPkgToSchemaTypeIfNeeded(schemaType, astFile)
	return parser.getTypeSpecDefFromSchemaTypeWithPkgName(schemaWithPkg)
}

func (parser *Parser) getTypeSpecDefFromSchemaTypeV3(schemaType string, astFile *ast.File) (*TypeSpecDef, error) {
	if IsPrimitiveType(schemaType) {
		return nil, nil
	}

	schemaWithPkg := addPkgToSchemaTypeIfNeeded(schemaType, astFile)
	return parser.getTypeSpecDefFromSchemaTypeWithPkgNameV3(schemaWithPkg)
}

func (parser *Parser) getExampleByInstance(currASTFile *ast.File, currTypeSpecDef *TypeSpecDef, attr string) (interface{}, error) {
	if currASTFile == nil {
		return nil, fmt.Errorf("astFile cannot be nil")
	}

	// Walk through the AST to find the declaration of `attr`
	example := parser.resolveIdentValue(attr, currTypeSpecDef, currASTFile, false)

	if example == nil {
		return nil, fmt.Errorf("example instance not found or unsupported type")
	}

	return example, nil
}


func (parser *Parser) findPackagePath(pkgName string, imports []*ast.ImportSpec) (string, error) {
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

func (parser *Parser) getTypeSpecDefFromSchemaTypeWithPkgName(schemaType string) (*TypeSpecDef, error) {
	for typeDef, schema := range parser.parsedSchemas {
		if schema.Name == schemaType {
			return typeDef, nil
		}
	}
	return nil, fmt.Errorf("could not find typeSpecDef for schemaType: %s", schemaType)
}

func (parser *Parser) getTypeSpecDefFromSchemaTypeWithPkgNameV3(schemaType string) (*TypeSpecDef, error) {
	for typeDef, schema := range parser.parsedSchemasV3 {
		if schema.Name == schemaType {
			return typeDef, nil
		}
	}
	return nil, fmt.Errorf("could not find typeSpecDef for schemaType: %s", schemaType)
}

func (parser *Parser) findPackagePathFromPackageName(pkgName string) (string, error) {
	for pkgPath, pkgDefinition := range parser.packages.packages {
		if pkgDefinition.Name == pkgName {
			return pkgPath, nil
		}
	}
	return "", fmt.Errorf("could not find package path for package name: %s", pkgName)
}

func (parser *Parser) getTypeSpecDefFromSchemaNameAndPkgPath(schemaName string, pkgPath string) (*TypeSpecDef, error) {
	if typeSpecDef, ok := parser.packages.packages[pkgPath].TypeDefinitions[schemaName]; ok {
		return typeSpecDef, nil
	}
	return nil, fmt.Errorf("could not find typeSpecDef for schemaName: %s and pkgPath: %s", schemaName, pkgPath)
}

func (parser *Parser) parseExpr(expr ast.Expr, currSpecDef *TypeSpecDef, file *ast.File, shouldGetNew bool) interface{} {
	switch astExpr := expr.(type) {
	case *ast.BasicLit:
		return parseBasicLiteral(astExpr)
	case *ast.CompositeLit:
		if (shouldGetNew) {
			newTypeSpecDef, err := parser.getNewTypeSpecDef(astExpr, currSpecDef, file)
			if err != nil {
				fmt.Println("Error in getNewTypeSpecDef: ", err)
				return nil
			}
			return parser.parseCompositeLiteral(astExpr, newTypeSpecDef, file) // Nested structs or arrays
		}
		return parser.parseCompositeLiteral(astExpr, currSpecDef, file) // Nested structs or arrays
	case *ast.UnaryExpr:
		if astExpr.Op == token.AND {
			return parser.parseExpr(astExpr.X, currSpecDef, file, shouldGetNew) // Dereference and parse the value
		}
		return nil
	case *ast.Ident:
		if astExpr.Name == BOOLEAN_STRING_TRUE {
			return true
		} else if astExpr.Name == BOOLEAN_STRING_FALSE {
			return false
		}
		// Handle variable references
		return parser.resolveIdentValue(astExpr.Name, currSpecDef, file, shouldGetNew)

	case *ast.SelectorExpr:
		// Handle package references
		pkgName := astExpr.X.(*ast.Ident).Name
		typeName := astExpr.Sel.Name
		pkgPath, err := parser.findPackagePath(pkgName, file.Imports)
		if err != nil {
			fmt.Println("Error in selector expr: ", err)
			return nil
		}

		fmt.Println("[parseExpr - Selector Expr] PKG PATH: ", pkgPath)
		fmt.Println("[parseExpr - Selector Expr] TYPE NAME: ", typeName)

		if packageDefinition, ok := parser.packages.packages[pkgPath]; ok {
			fmt.Println("[parseExpr - Selector Expr] PACKAGE FOUND: ", packageDefinition)
			var value interface{}
			for _, pkgFile := range packageDefinition.Files {
				ast.Inspect(pkgFile, func(n ast.Node) bool {
					decl, ok := n.(*ast.ValueSpec)
					if !ok {
						return true
					}

					for i, name := range decl.Names {
						if name.Name == typeName && len(decl.Values) > i {
							samplePkgType := TypeSpecDef{ // NEED TO REPLACE THIS - MAYBE WE ONLY NEED THE PACKAGE PATH?
								PkgPath: pkgPath,
								File: 	pkgFile,
							}
							value = parser.parseExpr(decl.Values[i], &samplePkgType, pkgFile, true)
							return false // Stop walking
						}
					}
					return true
				})
			}
			return value
		}

		return nil
	default:
		fmt.Println("[parseExpr] Unsupported type: ", fmt.Sprintf("%T", astExpr))
		return nil // Unsupported type
	}
}

func (parser *Parser) resolveIdentValue(identName string, currTypeSpecDef *TypeSpecDef, file *ast.File, shoulGetNew bool) interface{} {
	var value interface{}
	fmt.Println("[resolveIdentValue] IDENT NAME: ", identName)
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}

		for i, name := range decl.Names {
			if name.Name == identName && len(decl.Values) > i {
				value = parser.parseExpr(decl.Values[i], currTypeSpecDef, file, shoulGetNew)
				return false // Stop walking
			}
		}
		return true
	})

	return value
}

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
		return nil
	}
}

func (parser *Parser) handleMapCompositeLiteral(literal *ast.CompositeLit, currSpecDef *TypeSpecDef, file *ast.File) map[string]interface{} {
	obj := make(map[string]interface{})
	for _, compositeElement := range literal.Elts {
		if keyValueExpr, ok := compositeElement.(*ast.KeyValueExpr); ok {
			key := parser.parseExpr(keyValueExpr.Key, currSpecDef, file, true)
			value := parser.parseExpr(keyValueExpr.Value, currSpecDef, file, true)

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

func (parser *Parser) handleArrayCompositeLiteral(literal *ast.CompositeLit, currSpecDef *TypeSpecDef, file *ast.File) []interface{} {
	var arr []interface{}
	for _, elem := range literal.Elts {
		value := parser.parseExpr(elem, currSpecDef, file, true)
		arr = append(arr, value)
	}
	return arr
}


func getJSONTag(field *ast.Field) string {
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`") // Remove backticks
		jsonTag := reflect.StructTag(tagValue).Get("json")
		if jsonTag != "" {
			return strings.Split(jsonTag, ",")[0] // Ignore omitempty, etc.
		}
	}
	return "" // Return empty string if no JSON tag exists
}

func (parser *Parser) getFieldJSONTag(typeSpecDef *TypeSpecDef, fieldName string) (string, bool) {
	if typeSpecDef == nil || typeSpecDef.TypeSpec == nil {
		return fieldName, false
	}

	fmt.Println("[GET FIELD JSON TAG] PKG PATH: ", typeSpecDef.PkgPath);
	fmt.Println("[GET FIELD JSON TAG] TYPE NAME: ", typeSpecDef.TypeSpec.Name.Name);
	wasFieldFound := false

	if structType, ok := typeSpecDef.TypeSpec.Type.(*ast.StructType); ok {
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				if name.Name == fieldName {
					fmt.Println("[GET FIELD JSON TAG] FIELD FOUND")
					wasFieldFound = true
					tag := getJSONTag(field)
					if tag != "" {
						fmt.Println("[GET FIELD JSON TAG] TAG: ", tag);
						return tag, false
					}
				}
			}
		}
	}
	if !wasFieldFound {
		fmt.Printf("%s IS A EMBEDDED STRUCT!!\n", fieldName)
		return fieldName, true
	}
	fmt.Println("[GET FIELD JSON TAG] TAG: ", fieldName);
	return fieldName, false
}

func (parser *Parser) getArrayTypeInfo(arrType *ast.ArrayType, imports []*ast.ImportSpec) (pkg string, typeName string) {
	if arrType == nil {
		return "", ""
	}

	return parser.getTypePackageAndName(arrType.Elt, imports)
}

func (parser *Parser) getMapValueTypeInfo(mapType *ast.MapType, imports []*ast.ImportSpec) (valuePkg, valueType string) {
	if mapType == nil {
		return "", ""
	}

	valuePkg, valueType = parser.getTypePackageAndName(mapType.Value, imports)
	return valuePkg, valueType
}

// Helper function to extract package and type from an arbitrary expression
func (parser *Parser) getTypePackageAndName(expr ast.Expr, imports []*ast.ImportSpec) (pkg string, typeName string) {
	switch t := expr.(type) {
	case *ast.Ident:
		// Local type (no package)
		return "", t.Name

	case *ast.SelectorExpr:
		// Selector expression: `pkg.Type`
		if ident, ok := t.X.(*ast.Ident); ok {
			pkgPath, err := parser.findPackagePath(ident.Name, imports) // Find full package path
			if err != nil {
				fmt.Println("Error in getTypePackageAndName: ", err)
			}
			return pkgPath, t.Sel.Name
		}

	case *ast.StarExpr:
		// Pointer type: *SomeType
		return parser.getTypePackageAndName(t.X, imports)

	case *ast.ArrayType:
		// Nested array type (e.g., [][]MyType)
		return parser.getTypePackageAndName(t.Elt, imports)

	case *ast.MapType:
		// Map key/value types
		return parser.getTypePackageAndName(t.Value, imports)

	case *ast.StructType:
		// Anonymous struct
		return "", "struct"

	case *ast.InterfaceType:
		// Anonymous interface
		return "", "interface"
	}

	return "", ""
}

func (parser *Parser) getNewTypeSpecDef(compositeLit *ast.CompositeLit, currTypeSpecDef *TypeSpecDef, astFile *ast.File) (*TypeSpecDef, error) {
	switch compositeType := compositeLit.Type.(type) {
	case *ast.Ident:
		pkgPath, _ := parser.findPackagePathFromPackageName(astFile.Name.Name)
		typeSpecDef, err := parser.getTypeSpecDefFromSchemaNameAndPkgPath(compositeType.Name, pkgPath)
		if err != nil {
			return nil, fmt.Errorf("error getting new type spec def from *ast.Ident: %s", err)
		}
		return typeSpecDef, nil
	case *ast.SelectorExpr:
		compositePkg := compositeType.X.(*ast.Ident).Name
		pkgPath, err := parser.findPackagePath(compositePkg, astFile.Imports)
		if err != nil {
			return nil, fmt.Errorf("error getting package path from *ast.SelectorExpr: %s", err)
		}
		typeSpecDef, err := parser.getTypeSpecDefFromSchemaNameAndPkgPath(compositeType.Sel.Name, pkgPath)
		if err != nil {
			return nil, fmt.Errorf("error getting new type spec def from *ast.SelectorExpr: %s", err)
		}
		return typeSpecDef, nil
	case *ast.ArrayType:
		pkgPath, typeName := parser.getArrayTypeInfo(compositeType, astFile.Imports)
		if pkgPath == "" {
			pkgPath, _ = parser.findPackagePathFromPackageName(astFile.Name.Name)
		}
		typeSpecDef, err := parser.getTypeSpecDefFromSchemaNameAndPkgPath(typeName, pkgPath)
		if err != nil {
			return nil, fmt.Errorf("error getting new type spec def from *ast.ArrayType: %s", err)
		}
		return typeSpecDef, nil
	case *ast.MapType:
		pkgPath, typeName := parser.getMapValueTypeInfo(compositeType, astFile.Imports)
		if pkgPath == "" {
			pkgPath, _ = parser.findPackagePathFromPackageName(astFile.Name.Name)
		}
		typeSpecDef, err := parser.getTypeSpecDefFromSchemaNameAndPkgPath(typeName, pkgPath)
		if err != nil {
			return nil, fmt.Errorf("error getting new type spec def from *ast.MapType: %s", err)
		}
		return typeSpecDef, nil
	case nil:
		return currTypeSpecDef, nil
	default:
		fmt.Printf("unsupported composite type: %T\n", compositeType)
		return nil, nil
	}
}

func (parser *Parser) handleStructCompositeLiteral(literal *ast.CompositeLit, typeSpecDef *TypeSpecDef, file *ast.File) interface{} {	
	obj := make(map[string]interface{})
	embeddedKeys := []string{} 
	for _, compositeElement := range literal.Elts {
		if keyValueExpr, ok := compositeElement.(*ast.KeyValueExpr); ok {
			if key, ok := keyValueExpr.Key.(*ast.Ident); ok {
				tag, isEmbedded := parser.getFieldJSONTag(typeSpecDef, key.Name)
				if isEmbedded {
					embeddedKeys = append(embeddedKeys, tag)
				}
				obj[tag] = parser.parseExpr(keyValueExpr.Value, typeSpecDef, file, true)
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

func (parser *Parser) parseCompositeLiteral(literal *ast.CompositeLit, typeSpecDef *TypeSpecDef, file *ast.File) interface{} {
	switch literal.Type.(type) {
	case *ast.ArrayType:
		return parser.handleArrayCompositeLiteral(literal, typeSpecDef, file)
	case *ast.MapType:
		return parser.handleMapCompositeLiteral(literal, typeSpecDef, file)
	default:
		return parser.handleStructCompositeLiteral(literal, typeSpecDef, file)
	}
}
