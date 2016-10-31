/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package timeout

import (
	"errors"
	"strconv"

	"k8s.io/kubernetes/pkg/apis/extensions"

	"k8s.io/contrib/ingress/controllers/nginx/nginx/config"
)

const (
	proxyConnectTimeout = "ingress.kubernetes.io/proxy-connect-timeout"
	proxyReadTimeout    = "ingress.kubernetes.io/proxy-read-timeout"
	proxySendTimeout    = "ingress.kubernetes.io/proxy-send-timeout"
)

var (
	// ErrMissingConnectTimeout returned error when the ingress does not contains the
	// proxy-connect-timeout annotation
	ErrMissingConnectTimeout = errors.New("proxy-connect-timeout is missing")

	// ErrMissingReadTimeout returned error when the ingress does not contains the
	// proxy-read-timeout annotation
	ErrMissingReadTimeout = errors.New("proxy-read-timeout is missing")

	// ErrMissingSendTimeout returned error when the ingress does not contains the
	// proxy-send-timeout annotation
	ErrMissingSendTimeout = errors.New("proxy-send-timeout is missing")

	// ErrInvalidNumber returned error when the annotation is not a number
	ErrInvalidNumber = errors.New("the annotation does not contains a number")
)

// Timeout returns the specific number (in seconds) for
// connect, read and send
type Timeout struct {
	connect int
	read    int
	send    int
}

type ingAnnotations map[string]string

func (a ingAnnotations) connectTimeout() (int, error) {
	val, ok := a[proxyConnectTimeout]
	if !ok {
		return 0, ErrMissingConnectTimeout
	}

	ft, err := strconv.Atoi(val)
	if err != nil {
		return 0, ErrInvalidNumber
	}

	return ft, nil
}

func (a ingAnnotations) readTimeout() (int, error) {
	val, ok := a[proxyReadTimeout]
	if !ok {
		return 0, ErrMissingReadTimeout
	}

	mf, err := strconv.Atoi(val)
	if err != nil {
		return 0, ErrInvalidNumber
	}

	return mf, nil
}

func (a ingAnnotations) sendTimeout() (int, error) {
	val, ok := a[proxySendTimeout]
	if !ok {
		return 0, ErrMissingSendTimeout
	}

	ft, err := strconv.Atoi(val)
	if err != nil {
		return 0, ErrInvalidNumber
	}

	return ft, nil
}

// ParseAnnotations parses the annotations contained in the ingress
// rule used to configure timeouts
func ParseAnnotations(cfg config.Configuration, ing *extensions.Ingress) *Timeout {

	if ing.GetAnnotations() == nil {
		return &Timeout{cfg.ProxyConnectTimeout,
			cfg.ProxyReadTimeout,
			cfg.ProxySendTimeout}
	}

	ct, err := ingAnnotations(ing.GetAnnotations()).connectTimeout()
	if err != nil {
		ct = cfg.ProxyConnectTimeout
	}

	rt, err := ingAnnotations(ing.GetAnnotations()).readTimeout()
	if err != nil {
		rt = cfg.ProxyReadTimeout
	}

	st, err := ingAnnotations(ing.GetAnnotations()).sendTimeout()
	if err != nil {
		st = cfg.ProxySendTimeout
	}

	return &Timeout{
		connect: ct,
		read:    rt,
		send:    st,
	}
}
