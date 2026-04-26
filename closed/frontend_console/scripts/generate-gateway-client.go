package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type document struct {
	path string
	root map[string]any
}

type resolver struct {
	docs map[string]*document
}

func main() {
	rootPath, err := filepath.Abs(filepath.Join("..", "..", "open", "api", "openapi", "gateway.yaml"))
	if err != nil {
		panic(err)
	}
	res := &resolver{docs: map[string]*document{}}
	globalResolver = res
	if _, err := res.load(rootPath); err != nil {
		panic(err)
	}
	for _, name := range []string{"experiments.yaml", "dataset-registry.yaml", "quality.yaml", "lineage.yaml", "audit.yaml"} {
		path := filepath.Join(filepath.Dir(rootPath), name)
		if _, err := res.load(path); err != nil {
			panic(err)
		}
	}
	if err := res.loadAllRefs(rootPath); err != nil {
		panic(err)
	}

	schemas := res.collectSchemas()
	operations := res.collectOperations(rootPath)
	paths := res.collectPaths(rootPath, operations)

	buf := &bytes.Buffer{}
	writeHeader(buf, rootPath)
	writeSchemas(buf, schemas, res)
	writeOperations(buf, operations)
	writePaths(buf, paths)

	outPath := filepath.Join("lib", "gateway-openapi.ts")
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

func (r *resolver) load(path string) (*document, error) {
	path = filepath.Clean(path)
	if doc, ok := r.docs[path]; ok {
		return doc, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	root, ok := normalize(raw).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("document %s is not a map", path)
	}
	doc := &document{path: path, root: root}
	r.docs[path] = doc
	return doc, nil
}

func (r *resolver) loadAllRefs(path string) error {
	doc, err := r.load(path)
	if err != nil {
		return err
	}
	seen := map[string]bool{path: true}
	return r.scanRefs(doc.root, path, seen)
}

func (r *resolver) scanRefs(node any, basePath string, seen map[string]bool) error {
	switch v := node.(type) {
	case map[string]any:
		if refRaw, ok := v["$ref"].(string); ok {
			parts := strings.Split(refRaw, "#")
			filePart := strings.TrimSpace(parts[0])
			if filePart != "" {
				filePath := filepath.Join(filepath.Dir(basePath), filePart)
				filePath = filepath.Clean(filePath)
				if !seen[filePath] {
					seen[filePath] = true
					doc, err := r.load(filePath)
					if err != nil {
						return err
					}
					if err := r.scanRefs(doc.root, filePath, seen); err != nil {
						return err
					}
				}
			}
		}
		for _, val := range v {
			if err := r.scanRefs(val, basePath, seen); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := r.scanRefs(item, basePath, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *resolver) resolveRef(basePath, ref string) (any, string, error) {
	parts := strings.Split(ref, "#")
	filePart := strings.TrimSpace(parts[0])
	fragment := ""
	if len(parts) > 1 {
		fragment = parts[1]
	}
	filePath := basePath
	if filePart != "" {
		filePath = filepath.Join(filepath.Dir(basePath), filePart)
	}
	doc, err := r.load(filePath)
	if err != nil {
		return nil, "", err
	}
	if fragment == "" {
		return doc.root, filePath, nil
	}
	if strings.HasPrefix(fragment, "/") {
		fragment = fragment[1:]
	}
	cursor := any(doc.root)
	for _, raw := range strings.Split(fragment, "/") {
		key := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch node := cursor.(type) {
		case map[string]any:
			cursor = node[key]
		default:
			return nil, filePath, fmt.Errorf("ref %s not found", ref)
		}
	}
	return cursor, filePath, nil
}

func (r *resolver) collectSchemas() map[string]schemaRef {
	out := map[string]schemaRef{}
	for _, doc := range r.docs {
		components, ok := doc.root["components"].(map[string]any)
		if !ok {
			continue
		}
		schemas, ok := components["schemas"].(map[string]any)
		if !ok {
			continue
		}
		for name, raw := range schemas {
			if _, exists := out[name]; exists {
				continue
			}
			if schemaMap, ok := raw.(map[string]any); ok {
				out[name] = schemaRef{value: schemaMap, file: doc.path, name: name}
			}
		}
	}
	return out
}

func (r *resolver) collectOperations(rootPath string) map[string]operationRef {
	doc, _ := r.load(rootPath)
	pathsRaw, ok := doc.root["paths"].(map[string]any)
	if !ok {
		return nil
	}
	ops := map[string]operationRef{}
	for path, raw := range pathsRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			opRaw, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			opID, _ := opRaw["operationId"].(string)
			if strings.TrimSpace(opID) == "" {
				opID = method + "_" + sanitizeName(path)
			}
			ops[opID] = operationRef{path: path, method: method, value: opRaw, item: item, file: doc.path}
		}
	}
	return ops
}

func (r *resolver) collectPaths(rootPath string, ops map[string]operationRef) map[string]pathRef {
	doc, _ := r.load(rootPath)
	pathsRaw, ok := doc.root["paths"].(map[string]any)
	if !ok {
		return nil
	}
	paths := map[string]pathRef{}
	for path, raw := range pathsRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		paths[path] = pathRef{path: path, value: item, file: doc.path}
	}
	return paths
}

func writeHeader(buf *bytes.Buffer, source string) {
	buf.WriteString("// Code generated by scripts/generate-gateway-client.go; DO NOT EDIT.\n")
	buf.WriteString("// Source: " + source + "\n\n")
	buf.WriteString("export type components = {\n  schemas: {\n")
}

type schemaRef struct {
	name  string
	value map[string]any
	file  string
}

type operationRef struct {
	path   string
	method string
	value  map[string]any
	item   map[string]any
	file   string
}

type pathRef struct {
	path  string
	value map[string]any
	file  string
}

func writeSchemas(buf *bytes.Buffer, schemas map[string]schemaRef, res *resolver) {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		schema := schemas[name]
		buf.WriteString("    " + sanitizeName(name) + ": " + schemaToType(res, schema.file, schema.value) + ";\n")
	}
	buf.WriteString("  }\n}\n\n")
}

func writeOperations(buf *bytes.Buffer, ops map[string]operationRef) {
	buf.WriteString("export type operations = {\n")
	keys := make([]string, 0, len(ops))
	for key := range ops {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, opID := range keys {
		op := ops[opID]
		buf.WriteString("  \"" + opID + "\": {\n")
		buf.WriteString(writeOperationType(op))
		buf.WriteString("  };\n")
	}
	buf.WriteString("}\n\n")
}

func writePaths(buf *bytes.Buffer, paths map[string]pathRef) {
	buf.WriteString("export type paths = {\n")
	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, path := range keys {
		item := paths[path]
		buf.WriteString("  \"" + path + "\": {\n")
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			opRaw, ok := item.value[method].(map[string]any)
			if !ok {
				continue
			}
			opID, _ := opRaw["operationId"].(string)
			if strings.TrimSpace(opID) == "" {
				opID = method + "_" + sanitizeName(path)
			}
			buf.WriteString("    " + method + "?: operations[\"" + opID + "\"];\n")
		}
		buf.WriteString("  };\n")
	}
	buf.WriteString("}\n")
}

func writeOperationType(op operationRef) string {
	buf := &bytes.Buffer{}
	params := collectParameters(op)
	if len(params.path) == 0 && len(params.query) == 0 && len(params.header) == 0 {
		buf.WriteString("    parameters?: never;\n")
	} else {
		buf.WriteString("    parameters: {\n")
		if len(params.path) > 0 {
			buf.WriteString("      path: {\n")
			writeParams(buf, params.path)
			buf.WriteString("      };\n")
		}
		if len(params.query) > 0 {
			buf.WriteString("      query?: {\n")
			writeParams(buf, params.query)
			buf.WriteString("      };\n")
		}
		if len(params.header) > 0 {
			buf.WriteString("      header?: {\n")
			writeParams(buf, params.header)
			buf.WriteString("      };\n")
		}
		buf.WriteString("    };\n")
	}

	if body, ok := op.value["requestBody"].(map[string]any); ok {
		buf.WriteString("    requestBody?: " + requestBodyType(op, body) + ";\n")
	} else {
		buf.WriteString("    requestBody?: never;\n")
	}

	buf.WriteString("    responses: {\n")
	if responses, ok := op.value["responses"].(map[string]any); ok {
		keys := make([]string, 0, len(responses))
		for code := range responses {
			keys = append(keys, code)
		}
		sort.Strings(keys)
		for _, code := range keys {
			resp := responses[code]
			buf.WriteString("      " + quoteIfNeeded(code) + ": " + responseType(op, resp) + ";\n")
		}
	}
	buf.WriteString("    };\n")
	return buf.String()
}

type paramSpec struct {
	name     string
	typeExpr string
	required bool
}

type paramGroup struct {
	path   []paramSpec
	query  []paramSpec
	header []paramSpec
}

func collectParameters(op operationRef) paramGroup {
	group := paramGroup{}
	collect := func(raw any) {
		param, ok := raw.(map[string]any)
		if !ok {
			return
		}
		if ref, ok := param["$ref"].(string); ok {
			resolved, file, err := resolveRef(op, ref)
			if err == nil {
				param, ok = resolved.(map[string]any)
				if ok {
					op.file = file
					// continue with resolved
				}
			}
		}
		name, _ := param["name"].(string)
		inVal, _ := param["in"].(string)
		required, _ := param["required"].(bool)
		schemaMap, _ := param["schema"].(map[string]any)
		spec := paramSpec{name: name, typeExpr: schemaToType(globalResolver, op.file, schemaMap), required: required}
		switch inVal {
		case "path":
			group.path = append(group.path, spec)
		case "query":
			group.query = append(group.query, spec)
		case "header":
			group.header = append(group.header, spec)
		}
	}

	if params, ok := op.item["parameters"].([]any); ok {
		for _, raw := range params {
			collect(raw)
		}
	}
	if params, ok := op.value["parameters"].([]any); ok {
		for _, raw := range params {
			collect(raw)
		}
	}
	return group
}

func writeParams(buf *bytes.Buffer, params []paramSpec) {
	sort.Slice(params, func(i, j int) bool { return params[i].name < params[j].name })
	for _, param := range params {
		optional := "?"
		if param.required {
			optional = ""
		}
		buf.WriteString("        " + sanitizeName(param.name) + optional + ": " + param.typeExpr + ";\n")
	}
}

func requestBodyType(op operationRef, body map[string]any) string {
	if ref, ok := body["$ref"].(string); ok {
		resolved, file, err := resolveRef(op, ref)
		if err == nil {
			op.file = file
			if m, ok := resolved.(map[string]any); ok {
				return requestBodyType(op, m)
			}
		}
	}
	content, _ := body["content"].(map[string]any)
	appJSON, _ := content["application/json"].(map[string]any)
	schemaMap, _ := appJSON["schema"].(map[string]any)
	return schemaToType(globalResolver, op.file, schemaMap)
}

func responseType(op operationRef, resp any) string {
	if respMap, ok := resp.(map[string]any); ok {
		if ref, ok := respMap["$ref"].(string); ok {
			resolved, file, err := resolveRef(op, ref)
			if err == nil {
				op.file = file
				return responseType(op, resolved)
			}
		}
		content, _ := respMap["content"].(map[string]any)
		appJSON, _ := content["application/json"].(map[string]any)
		schemaMap, _ := appJSON["schema"].(map[string]any)
		if schemaMap == nil {
			return "unknown"
		}
		return schemaToType(globalResolver, op.file, schemaMap)
	}
	return "unknown"
}

var globalResolver *resolver

func schemaToType(res *resolver, baseFile string, schema map[string]any) string {
	if res == nil {
		res = globalResolver
	}
	if res == nil {
		return "unknown"
	}
	if schema == nil {
		return "unknown"
	}
	if ref, ok := schema["$ref"].(string); ok {
		name := sanitizeName(refName(ref))
		return "components[\"schemas\"][\"" + name + "\"]"
	}
	if enumVals, ok := schema["enum"].([]any); ok && len(enumVals) > 0 {
		parts := make([]string, 0, len(enumVals))
		for _, entry := range enumVals {
			parts = append(parts, literal(entry))
		}
		return strings.Join(parts, " | ")
	}
	if oneOf, ok := schema["oneOf"].([]any); ok && len(oneOf) > 0 {
		parts := make([]string, 0, len(oneOf))
		for _, entry := range oneOf {
			if m, ok := entry.(map[string]any); ok {
				parts = append(parts, schemaToType(res, baseFile, m))
			}
		}
		return strings.Join(parts, " | ")
	}
	if anyOf, ok := schema["anyOf"].([]any); ok && len(anyOf) > 0 {
		parts := make([]string, 0, len(anyOf))
		for _, entry := range anyOf {
			if m, ok := entry.(map[string]any); ok {
				parts = append(parts, schemaToType(res, baseFile, m))
			}
		}
		return strings.Join(parts, " | ")
	}
	if allOf, ok := schema["allOf"].([]any); ok && len(allOf) > 0 {
		parts := make([]string, 0, len(allOf))
		for _, entry := range allOf {
			if m, ok := entry.(map[string]any); ok {
				parts = append(parts, schemaToType(res, baseFile, m))
			}
		}
		return strings.Join(parts, " & ")
	}

	schemaType, _ := schema["type"].(string)
	switch schemaType {
	case "string":
		return "string"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		if items, ok := schema["items"].(map[string]any); ok {
			return "(" + schemaToType(res, baseFile, items) + ")[]"
		}
		return "unknown[]"
	case "object", "":
		return objectType(res, baseFile, schema)
	default:
		return "unknown"
	}
}

func objectType(res *resolver, baseFile string, schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	requiredList := map[string]bool{}
	if required, ok := schema["required"].([]any); ok {
		for _, entry := range required {
			if name, ok := entry.(string); ok {
				requiredList[name] = true
			}
		}
	}

	buf := &bytes.Buffer{}
	buf.WriteString("{")
	if len(props) > 0 {
		keys := make([]string, 0, len(props))
		for key := range props {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			prop, ok := props[key].(map[string]any)
			if !ok {
				continue
			}
			optional := "?"
			if requiredList[key] {
				optional = ""
			}
			buf.WriteString(fmt.Sprintf(" %s%s: %s;", sanitizeName(key), optional, schemaToType(res, baseFile, prop)))
		}
	}
	if additional, ok := schema["additionalProperties"].(map[string]any); ok {
		buf.WriteString(fmt.Sprintf(" [key: string]: %s;", schemaToType(res, baseFile, additional)))
	}
	buf.WriteString(" }")
	if len(props) == 0 && !hasAdditional(schema) {
		return "Record<string, unknown>"
	}
	return buf.String()
}

func hasAdditional(schema map[string]any) bool {
	_, ok := schema["additionalProperties"]
	return ok
}

func literal(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", escapeString(v))
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return "null"
	}
}

func escapeString(input string) string {
	return strings.ReplaceAll(input, "\"", "\\\"")
}

func refName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "{", "")
	name = strings.ReplaceAll(name, "}", "")
	if name == "default" {
		return "_default"
	}
	return name
}

func quoteIfNeeded(code string) string {
	if _, err := fmt.Sscanf(code, "%d", new(int)); err == nil {
		return code
	}
	return fmt.Sprintf("\"%s\"", code)
}

func normalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[key] = normalize(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			out[fmt.Sprintf("%v", key)] = normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	default:
		return value
	}
}

func resolveRef(op operationRef, ref string) (any, string, error) {
	if globalResolver == nil {
		return nil, "", fmt.Errorf("resolver not initialized")
	}
	return globalResolver.resolveRef(op.file, ref)
}

func init() {
	globalResolver = &resolver{docs: map[string]*document{}}
}
