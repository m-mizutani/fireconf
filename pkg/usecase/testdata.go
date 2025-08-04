package usecase

import (
	_ "embed"
)

// Embedded test data files

//go:embed testdata/basic.yaml
var TestDataBasic string

//go:embed testdata/vector.yaml
var TestDataVector string

//go:embed testdata/array.yaml
var TestDataArray string

//go:embed testdata/invalid.yaml
var TestDataInvalid string

//go:embed testdata/e2e_test.yaml
var TestDataE2E string

//go:embed testdata/e2e_simple.yaml
var TestDataE2ESimple string
