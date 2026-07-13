package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsIncludeBareGPT56Alias(t *testing.T) {
	require.Contains(t, DefaultModelIDs(), "gpt-5.6")
}

func TestDefaultAccountTestModelsExcludeBareGPT56Alias(t *testing.T) {
	models := DefaultAccountTestModels()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}

	require.NotContains(t, ids, "gpt-5.6")
	require.Contains(t, ids, "gpt-5.6-sol")
}
