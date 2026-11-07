package parser

import (
	"maps"
	"slices"
)

// The single-value file names each parser knows, sorted, so that the fuzz
// targets in fuzz_test.go (package parser_test, which may import the mapper)
// cover every entry of the maps without copying them.
var (
	LLiteSingleFileNames  = slices.Sorted(maps.Keys(lliteSingleFileMap))
	ClientSingleFileNames = slices.Sorted(maps.Keys(clientSingleFileMap))
	LNetParamNames        = slices.Sorted(maps.Keys(lnetParamMap))
)
