package core

import (
	"testing"
	"time"
	"k8s.io/contrib/cluster-autoscaler/config/dynamic"
	"github.com/stretchr/testify/mock"
)

type AutoscalerMock struct {
	mock.Mock
}

func (m *AutoscalerMock) RunOnce(currentTime time.Time) {
	m.Called(currentTime)
}

type ConfigFetcherMock struct {
	mock.Mock
}

func (m *ConfigFetcherMock) FetchConfigIfUpdated() (*dynamic.Config, error) {
	args := m.Called()
	return args.Get(0).(*dynamic.Config), args.Error(1)
}

type AutoscalerBuilderMock struct {
	mock.Mock
}

func (m *AutoscalerBuilderMock) SetDynamicConfig(config dynamic.Config) AutoscalerBuilder {
	args := m.Called(config)
	return args.Get(0).(AutoscalerBuilder)
}

func (m *AutoscalerBuilderMock) Build() Autoscaler {
	args := m.Called()
	return args.Get(0).(Autoscaler)
}

func TestRunOnceWhenNoUpdate(t *testing.T) {
	currentTime := time.Now()

	autoscaler := &AutoscalerMock{}
	autoscaler.On("RunOnce", currentTime).Once()

	configFetcher := &ConfigFetcherMock{}
	configFetcher.On("FetchConfigIfUpdated").Return((*dynamic.Config)(nil), nil).Once()

	builder := &AutoscalerBuilderMock{}
	builder.On("Build").Return(autoscaler).Once()

	a := NewDynamicAutoscaler(builder, configFetcher)
	a.RunOnce(currentTime)

	autoscaler.AssertExpectations(t)
	configFetcher.AssertExpectations(t)
	builder.AssertExpectations(t)
}

func TestRunOnceWhenUpdated(t *testing.T) {
	currentTime := time.Now()

	newConfig := dynamic.NewDefaultConfig()

	initialAutoscaler := &AutoscalerMock{}

	newAutoscaler := &AutoscalerMock{}
	newAutoscaler.On("RunOnce", currentTime).Once()

	configFetcher := &ConfigFetcherMock{}
	configFetcher.On("FetchConfigIfUpdated").Return(&newConfig, nil).Once()

	builder := &AutoscalerBuilderMock{}
	builder.On("Build").Return(initialAutoscaler).Once()
	builder.On("SetDynamicConfig", newConfig).Return(builder).Once()
	builder.On("Build").Return(newAutoscaler).Once()

	a := NewDynamicAutoscaler(builder, configFetcher)
	a.RunOnce(currentTime)

	initialAutoscaler.AssertNotCalled(t, "RunOnce", mock.AnythingOfType("time.Time"))
	newAutoscaler.AssertExpectations(t)
	configFetcher.AssertExpectations(t)
	builder.AssertExpectations(t)
}
