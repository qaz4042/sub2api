package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayMessageAttemptReleaseIsIdempotent(t *testing.T) {
	queueReleased := 0
	accountReleased := 0
	parsedReq := &service.ParsedRequest{
		OnUpstreamAccepted: func() {},
	}
	attempt := &gatewayMessageAttempt{
		Parsed: parsedReq,
		QueueRelease: func() {
			queueReleased++
		},
		AccountRelease: func() {
			accountReleased++
		},
	}

	attempt.release()
	attempt.release()

	require.Equal(t, 1, queueReleased)
	require.Equal(t, 1, accountReleased)
	require.Nil(t, parsedReq.OnUpstreamAccepted)
	require.Nil(t, attempt.QueueRelease)
	require.Nil(t, attempt.AccountRelease)
}
