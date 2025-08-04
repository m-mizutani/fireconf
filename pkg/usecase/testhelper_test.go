package usecase_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/m-mizutani/fireconf/pkg/domain/model"
	"github.com/m-mizutani/fireconf/pkg/usecase"
	"github.com/m-mizutani/gt"
)

// LoadTestConfig loads a test configuration from embedded test data
func LoadTestConfig(t *testing.T, testData string) *model.Config {
	t.Helper()

	var config model.Config
	err := yaml.Unmarshal([]byte(testData), &config)
	gt.NoError(t, err)

	return &config
}

// LoadBasicTestConfig loads the basic test configuration
func LoadBasicTestConfig(t *testing.T) *model.Config {
	return LoadTestConfig(t, usecase.TestDataBasic)
}

// LoadVectorTestConfig loads the vector test configuration
func LoadVectorTestConfig(t *testing.T) *model.Config {
	return LoadTestConfig(t, usecase.TestDataVector)
}

// LoadArrayTestConfig loads the array test configuration
func LoadArrayTestConfig(t *testing.T) *model.Config {
	return LoadTestConfig(t, usecase.TestDataArray)
}
