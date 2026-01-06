// Package loader provides loaders for Starlark builtin definitions.
package loader

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/albertocavalcante/sky/internal/starlark/builtins"
	builtinspb "github.com/albertocavalcante/sky/internal/starlark/builtins/proto"
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Proto data files embedded via Bazel embedsrcs
//
//go:embed data/proto/bazel_build.pb
var bazelBuildPB []byte

//go:embed data/proto/bazel_workspace.pb
var bazelWorkspacePB []byte

//go:embed data/proto/bazel_bzl.pb
var bazelBzlPB []byte

//go:embed data/proto/bazel_module.pb
var bazelModulePB []byte

// ProtoProvider loads builtin definitions from proto files.
type ProtoProvider struct {
	// mu protects the cache
	mu sync.RWMutex

	// cache stores parsed builtins by dialect and file kind
	cache map[string]map[filekind.Kind]builtins.Builtins

	// dataFS holds the proto data files (filesystem or mock in tests)
	dataFS fsReader
}

// embeddedProtoFS provides access to embedded proto files.
type embeddedProtoFS struct{}

func (e *embeddedProtoFS) ReadFile(name string) ([]byte, error) {
	switch name {
	case "data/proto/bazel_build.pb":
		return bazelBuildPB, nil
	case "data/proto/bazel_workspace.pb":
		return bazelWorkspacePB, nil
	case "data/proto/bazel_bzl.pb":
		return bazelBzlPB, nil
	case "data/proto/bazel_module.pb":
		return bazelModulePB, nil
	default:
		return nil, fmt.Errorf("file not found: %s", name)
	}
}

// NewProtoProvider creates a new proto-based builtin provider.
// Uses Go embed to load proto data files at compile time.
func NewProtoProvider() *ProtoProvider {
	return &ProtoProvider{
		cache:  make(map[string]map[filekind.Kind]builtins.Builtins),
		dataFS: &embeddedProtoFS{},
	}
}

// newTestProtoProvider creates a proto provider for testing with injectable data.
func newTestProtoProvider() *ProtoProvider {
	return &ProtoProvider{
		cache:  make(map[string]map[filekind.Kind]builtins.Builtins),
		dataFS: &embedFS{files: make(map[string][]byte)},
	}
}

// injectTestData injects test data into the provider (for testing only).
func (p *ProtoProvider) injectTestData(filename string, data []byte) {
	if fs, ok := p.dataFS.(*embedFS); ok {
		fs.files[filename] = data
	}
}

// Builtins implements the Provider interface.
// Returns builtin definitions for the specified dialect and file kind.
func (p *ProtoProvider) Builtins(dialect string, kind filekind.Kind) (builtins.Builtins, error) {
	// Check cache first
	p.mu.RLock()
	if dialectCache, ok := p.cache[dialect]; ok {
		if cached, ok := dialectCache[kind]; ok {
			p.mu.RUnlock()
			return cached, nil
		}
	}
	p.mu.RUnlock()

	// Determine the proto filename
	filename := p.protoFilename(dialect, kind)
	if filename == "" {
		return builtins.Builtins{}, fmt.Errorf("unsupported dialect %q or file kind %q", dialect, kind)
	}

	// Load the proto file
	data, loadedFilename, err := p.loadProtoData(filename)
	if err != nil {
		return builtins.Builtins{}, fmt.Errorf("failed to load proto data for %s/%s: %w", dialect, kind, err)
	}

	// Parse the proto file
	protoBuiltins, err := p.parseProtoFile(data, loadedFilename)
	if err != nil {
		return builtins.Builtins{}, fmt.Errorf("failed to parse proto file %s: %w", loadedFilename, err)
	}

	// Convert proto to Go struct
	result := p.convertProtoToBuiltins(protoBuiltins)

	// Cache the result
	p.mu.Lock()
	if p.cache[dialect] == nil {
		p.cache[dialect] = make(map[filekind.Kind]builtins.Builtins)
	}
	p.cache[dialect][kind] = result
	p.mu.Unlock()

	return result, nil
}

// SupportedDialects implements the Provider interface.
// Returns the list of dialects this provider supports.
func (p *ProtoProvider) SupportedDialects() []string {
	return []string{"bazel", "buck2", "starlark"}
}

// protoFilename maps a dialect and file kind to a proto filename.
// Returns an empty string if the combination is not supported.
func (p *ProtoProvider) protoFilename(dialect string, kind filekind.Kind) string {
	// Normalize dialect name
	dialect = strings.ToLower(dialect)

	// Build the filename based on dialect and kind
	var basename string
	switch dialect {
	case "bazel":
		switch kind {
		case filekind.KindBUILD:
			basename = "bazel_build"
		case filekind.KindBzl:
			basename = "bazel_bzl"
		case filekind.KindWORKSPACE:
			basename = "bazel_workspace"
		case filekind.KindMODULE:
			basename = "bazel_module"
		case filekind.KindBzlmod:
			basename = "bazel_bzlmod"
		default:
			return ""
		}
	case "buck2":
		switch kind {
		case filekind.KindBUCK:
			basename = "buck2_buck"
		case filekind.KindBzlBuck:
			basename = "buck2_bzl"
		case filekind.KindBuckconfig:
			basename = "buck2_buckconfig"
		default:
			return ""
		}
	case "starlark":
		switch kind {
		case filekind.KindStarlark:
			basename = "starlark_generic"
		case filekind.KindSkyI:
			basename = "starlark_skyi"
		default:
			return ""
		}
	default:
		return ""
	}

	// Try binary format first (.pb), fall back to text format (.pbtxt)
	return fmt.Sprintf("data/proto/%s.pb", basename)
}

// loadProtoData loads proto data from the embedded filesystem.
func (p *ProtoProvider) loadProtoData(filename string) ([]byte, string, error) {
	// Try to read the requested file from embedded FS
	data, err := p.dataFS.ReadFile(filename)
	if err == nil {
		return data, filename, nil
	}

	// Try alternative extension (.pbtxt instead of .pb)
	if strings.HasSuffix(filename, ".pb") {
		altFilename := strings.TrimSuffix(filename, ".pb") + ".pbtxt"
		data, err = p.dataFS.ReadFile(altFilename)
		if err == nil {
			return data, altFilename, nil
		}
	}

	return nil, "", fmt.Errorf("proto data file not found: %s", filename)
}

// parseProtoFile parses a proto file in either binary (.pb) or text (.pbtxt) format.
func (p *ProtoProvider) parseProtoFile(data []byte, filename string) (*builtinspb.Builtins, error) {
	pb := &builtinspb.Builtins{}

	// Determine format based on extension
	if strings.HasSuffix(filename, ".pbtxt") {
		// Parse text format
		if err := prototext.Unmarshal(data, pb); err != nil {
			return nil, fmt.Errorf("failed to parse text proto: %w", err)
		}
	} else {
		// Parse binary format
		if err := proto.Unmarshal(data, pb); err != nil {
			return nil, fmt.Errorf("failed to parse binary proto: %w", err)
		}
	}

	return pb, nil
}

// convertProtoToBuiltins converts a proto Builtins message to the Go struct.
func (p *ProtoProvider) convertProtoToBuiltins(pb *builtinspb.Builtins) builtins.Builtins {
	result := builtins.Builtins{
		Functions: make([]builtins.Signature, 0),
		Types:     make([]builtins.TypeDef, 0),
		Globals:   make([]builtins.Field, 0),
	}

	// Convert types
	for _, pbType := range pb.GetTypes() {
		typeDef := builtins.TypeDef{
			Name:    pbType.GetName(),
			Doc:     pbType.GetDoc(),
			Fields:  make([]builtins.Field, 0),
			Methods: make([]builtins.Signature, 0),
		}

		// Convert fields
		for _, pbField := range pbType.GetFields() {
			field := builtins.Field{
				Name: pbField.GetName(),
				Type: pbField.GetType(),
				Doc:  pbField.GetDoc(),
			}

			// If the field has a callable, it's a method
			if pbField.Callable != nil {
				method := p.convertCallableToSignature(pbField.GetName(), pbField.Callable)
				typeDef.Methods = append(typeDef.Methods, method)
			} else {
				// Otherwise, it's a regular field
				typeDef.Fields = append(typeDef.Fields, field)
			}
		}

		result.Types = append(result.Types, typeDef)
	}

	// Convert values (can be functions or globals)
	for _, pbValue := range pb.GetValues() {
		if pbValue.Callable != nil {
			// It's a function
			fn := p.convertCallableToSignature(pbValue.GetName(), pbValue.Callable)
			result.Functions = append(result.Functions, fn)
		} else {
			// It's a global variable/constant
			global := builtins.Field{
				Name: pbValue.GetName(),
				Type: pbValue.GetType(),
				Doc:  pbValue.GetDoc(),
			}
			result.Globals = append(result.Globals, global)
		}
	}

	return result
}

// convertCallableToSignature converts a proto Callable to a Signature.
func (p *ProtoProvider) convertCallableToSignature(name string, pbCallable *builtinspb.Callable) builtins.Signature {
	sig := builtins.Signature{
		Name:       name,
		Doc:        pbCallable.GetDoc(),
		ReturnType: pbCallable.GetReturnType(),
		Params:     make([]builtins.Param, 0, len(pbCallable.GetParams())),
	}

	// Convert parameters
	for _, pbParam := range pbCallable.GetParams() {
		param := builtins.Param{
			Name:     pbParam.GetName(),
			Type:     pbParam.GetType(),
			Default:  pbParam.GetDefaultValue(),
			Required: pbParam.GetIsMandatory(),
			Variadic: pbParam.GetIsStarArg(),
			KWArgs:   pbParam.GetIsStarStarArg(),
		}
		sig.Params = append(sig.Params, param)
	}

	return sig
}
