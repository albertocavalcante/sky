package formatter

import (
	"fmt"

	"github.com/bazelbuild/buildtools/build"

	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Buildtools is the upstream bazelbuild/buildtools-based formatter. It is
// the current default and the stable baseline that every other engine is
// compared against during migration.
var Buildtools Engine = buildtoolsEngine{}

type buildtoolsEngine struct{}

func (buildtoolsEngine) Name() string { return "buildtools" }

func (buildtoolsEngine) Format(src []byte, path string, kind filekind.Kind) ([]byte, error) {
	f, err := parse(src, path, kind)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return build.Format(f), nil
}

// parse parses source code using the appropriate buildtools parser based
// on file kind. Lives on the buildtools engine because it's
// buildtools-specific; other engines bring their own parsers.
func parse(src []byte, path string, kind filekind.Kind) (*build.File, error) {
	switch kind {
	case filekind.KindBUILD, filekind.KindBUCK:
		return build.ParseBuild(path, src)
	case filekind.KindWORKSPACE:
		return build.ParseWorkspace(path, src)
	case filekind.KindMODULE:
		return build.ParseModule(path, src)
	case filekind.KindBzl, filekind.KindBzlmod, filekind.KindBzlBuck:
		return build.ParseBzl(path, src)
	default:
		// KindStarlark, KindSkyI, KindUnknown, or any other.
		return build.ParseDefault(path, src)
	}
}
