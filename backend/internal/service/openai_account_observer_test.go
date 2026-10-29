package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type observerStub struct {
	successes, failures, serverErrors int
	model                             string
}

func (o *observerStub) ObserveOpenAIAccount(_ *Account, model, _ string, success, _, http5xx bool, _ *int) {
	o.model = model
	if success {
		o.successes++
	} else if http5xx {
		o.serverErrors++
	} else {
		o.failures++
	}
}

func TestAccountObserverRecordsSuccessAndTransportOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	observer := &observerStub{}
	s := &OpenAIGatewayService{}
	s.SetOpenAIAccountObserver(observer)
	a := &Account{ID: 1, Platform: PlatformOpenAI}
	ttft := 250
	s.ReportOpenAIAccountScheduleResult(a, "gpt-test", true, &ttft)
	require.Equal(t, 1, observer.successes)
	s.ReportOpenAIAccountScheduleResult(a, "gpt-test", false, nil, &UpstreamFailoverError{StatusCode: 429})
	require.Zero(t, observer.failures)
	s.ReportOpenAIAccountScheduleResultWithTransport(a, "gpt-test", "http", false, nil, &UpstreamFailoverError{StatusCode: 503})
	require.Equal(t, 1, observer.serverErrors)
	s.ReportOpenAIAccountScheduleResultWithTransport(a, "gpt-test", "http", false, nil, &UpstreamFailoverError{StatusCode: 502, TransportFailure: true})
	s.ReportOpenAIAccountScheduleResultWithTransport(a, "gpt-test", "http", false, nil, &UpstreamFailoverError{StatusCode: 503, RequestScopedTransient: true})
	s.ReportOpenAIAccountScheduleResultWithTransport(a, "gpt-test", "http", false, nil, &UpstreamFailoverError{StatusCode: 503, Stage: GatewayFailureStageAccountAuth})
	require.Equal(t, 1, observer.serverErrors)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(OpsUpstreamModelKey, "gpt-test")
	_ = s.handleOpenAIUpstreamTransportError(context.Background(), c, a, errors.New("temporary network failure"), false)
	require.Equal(t, 1, observer.failures)
	require.Equal(t, "gpt-test", observer.model)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = s.handleOpenAIUpstreamTransportError(ctx, c, a, context.Canceled, false)
	require.Equal(t, 1, observer.failures)
}
