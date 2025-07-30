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
