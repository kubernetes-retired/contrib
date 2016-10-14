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

package cors

import (
	"k8s.io/contrib/ingress/controllers/nginx/pkg/ingress/annotations/parser"

	"k8s.io/kubernetes/pkg/apis/extensions"
)

const (
	cors = "ingress.kubernetes.io/enable-cors"
)

// ParseAnnotations parses the annotations contained in the ingress
// rule used to indicate if the location/s should allows CORS
func ParseAnnotations(ing *extensions.Ingress) (bool, error) {
	return parser.GetBoolAnnotation(cors, ing)
}
