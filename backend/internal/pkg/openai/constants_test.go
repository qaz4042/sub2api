package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelIDsExcludeRetiredOpenAIModels(t *testing.T) {
	t.Parallel()

	ids := DefaultModelIDs()

	require.NotContains(t, ids, "gpt-5.2")
	require.NotContains(t, ids, "gpt-5.3-codex")
	require.NotContains(t, ids, "gpt-5.3-codex-spark")
	require.NotContains(t, ids, "codex-auto-review")
	require.Contains(t, ids, "gpt-5.4")
	require.Contains(t, ids, "gpt-5.4-mini")
	require.Contains(t, ids, "gpt-5.5")
}
