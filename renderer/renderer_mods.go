// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the renderer models and structures.

package renderer

type MethodInfo struct {
	Name       string
	Parameters []string
	ReturnType string
}

type FieldInfo struct {
	Name string
	Type string
}

type ClassInfo struct {
	Name    string
	Fields  []FieldInfo
	Methods []MethodInfo
}

type ObjectInfo struct {
	Name string
	Type string
}
