/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

// TODO: move everything in this file to pkg/api/rest
package meta

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"
)

// Implements RESTScope interface
type restScope struct {
	name             RESTScopeName
	paramName        string
	argumentName     string
	paramDescription string
}

func (r *restScope) Name() RESTScopeName {
	return r.name
}
func (r *restScope) ParamName() string {
	return r.paramName
}
func (r *restScope) ArgumentName() string {
	return r.argumentName
}
func (r *restScope) ParamDescription() string {
	return r.paramDescription
}

var RESTScopeNamespace = &restScope{
	name:             RESTScopeNameNamespace,
	paramName:        "namespaces",
	argumentName:     "namespace",
	paramDescription: "object name and auth scope, such as for teams and projects",
}

var RESTScopeRoot = &restScope{
	name: RESTScopeNameRoot,
}

// DefaultRESTMapper exposes mappings between the types defined in a
// runtime.Scheme. It assumes that all types defined the provided scheme
// can be mapped with the provided MetadataAccessor and Codec interfaces.
//
// The resource name of a Kind is defined as the lowercase,
// English-plural version of the Kind string.
// When converting from resource to Kind, the singular version of the
// resource name is also accepted for convenience.
//
// TODO: Only accept plural for some operations for increased control?
// (`get pod bar` vs `get pods bar`)
// TODO these maps should be keyed based on GroupVersionKinds
type DefaultRESTMapper struct {
	defaultGroupVersions []unversioned.GroupVersion

	resourceToKind       map[unversioned.GroupVersionResource]unversioned.GroupVersionKind
	kindToPluralResource map[unversioned.GroupVersionKind]unversioned.GroupVersionResource
	kindToScope          map[unversioned.GroupVersionKind]RESTScope
	singularToPlural     map[unversioned.GroupVersionResource]unversioned.GroupVersionResource
	pluralToSingular     map[unversioned.GroupVersionResource]unversioned.GroupVersionResource

	interfacesFunc VersionInterfacesFunc
}

var _ RESTMapper = &DefaultRESTMapper{}

// VersionInterfacesFunc returns the appropriate codec, typer, and metadata accessor for a
// given api version, or an error if no such api version exists.
type VersionInterfacesFunc func(version unversioned.GroupVersion) (*VersionInterfaces, error)

// NewDefaultRESTMapper initializes a mapping between Kind and APIVersion
// to a resource name and back based on the objects in a runtime.Scheme
// and the Kubernetes API conventions. Takes a group name, a priority list of the versions
// to search when an object has no default version (set empty to return an error),
// and a function that retrieves the correct codec and metadata for a given version.
func NewDefaultRESTMapper(defaultGroupVersions []unversioned.GroupVersion, f VersionInterfacesFunc) *DefaultRESTMapper {
	resourceToKind := make(map[unversioned.GroupVersionResource]unversioned.GroupVersionKind)
	kindToPluralResource := make(map[unversioned.GroupVersionKind]unversioned.GroupVersionResource)
	kindToScope := make(map[unversioned.GroupVersionKind]RESTScope)
	singularToPlural := make(map[unversioned.GroupVersionResource]unversioned.GroupVersionResource)
	pluralToSingular := make(map[unversioned.GroupVersionResource]unversioned.GroupVersionResource)
	// TODO: verify name mappings work correctly when versions differ

	return &DefaultRESTMapper{
		resourceToKind:       resourceToKind,
		kindToPluralResource: kindToPluralResource,
		kindToScope:          kindToScope,
		defaultGroupVersions: defaultGroupVersions,
		singularToPlural:     singularToPlural,
		pluralToSingular:     pluralToSingular,
		interfacesFunc:       f,
	}
}

func (m *DefaultRESTMapper) Add(kind unversioned.GroupVersionKind, scope RESTScope, mixedCase bool) {
	plural, singular := KindToResource(kind, mixedCase)
	lowerPlural := plural.GroupVersion().WithResource(strings.ToLower(plural.Resource))
	lowerSingular := singular.GroupVersion().WithResource(strings.ToLower(singular.Resource))

	m.singularToPlural[singular] = plural
	m.pluralToSingular[plural] = singular
	m.singularToPlural[lowerSingular] = lowerPlural
	m.pluralToSingular[lowerPlural] = lowerSingular

	if _, mixedCaseExists := m.resourceToKind[plural]; !mixedCaseExists {
		m.resourceToKind[plural] = kind
		m.resourceToKind[singular] = kind
	}

	if _, lowerCaseExists := m.resourceToKind[lowerPlural]; !lowerCaseExists && (lowerPlural != plural) {
		m.resourceToKind[lowerPlural] = kind
		m.resourceToKind[lowerSingular] = kind
	}

	m.kindToPluralResource[kind] = plural
	m.kindToScope[kind] = scope
}

// KindToResource converts Kind to a resource name.
func KindToResource(kind unversioned.GroupVersionKind, mixedCase bool) (plural, singular unversioned.GroupVersionResource) {
	kindName := kind.Kind
	if len(kindName) == 0 {
		return
	}
	if mixedCase {
		// Legacy support for mixed case names
		singular = kind.GroupVersion().WithResource(strings.ToLower(kindName[:1]) + kindName[1:])
	} else {
		singular = kind.GroupVersion().WithResource(strings.ToLower(kindName))
	}

	singularName := singular.Resource
	if strings.HasSuffix(singularName, "endpoints") {
		plural = singular
	} else {
		switch string(singularName[len(singularName)-1]) {
		case "s":
			plural = kind.GroupVersion().WithResource(singularName + "es")
		case "y":
			plural = kind.GroupVersion().WithResource(strings.TrimSuffix(singularName, "y") + "ies")
		default:
			plural = kind.GroupVersion().WithResource(singularName + "s")
		}
	}
	return
}

// ResourceSingularizer implements RESTMapper
// It converts a resource name from plural to singular (e.g., from pods to pod)
// It must have exactly one match and it must match case perfectly.  This is congruent with old functionality
func (m *DefaultRESTMapper) ResourceSingularizer(resourceType string) (string, error) {
	partialResource := unversioned.GroupVersionResource{Resource: resourceType}
	resource, err := m.ResourceFor(partialResource)
	if err != nil {
		return resourceType, err
	}

	singular, ok := m.pluralToSingular[resource]
	if !ok {
		return resourceType, fmt.Errorf("no singular of resource %v has been defined", resource)
	}
	return singular.Resource, nil
}

func (m *DefaultRESTMapper) ResourcesFor(resource unversioned.GroupVersionResource) ([]unversioned.GroupVersionResource, error) {
	hasResource := len(resource.Resource) > 0
	hasGroup := len(resource.Group) > 0
	hasVersion := len(resource.Version) > 0

	if !hasResource {
		return nil, fmt.Errorf("a resource must be present, got: %v", resource)
	}

	ret := []unversioned.GroupVersionResource{}
	switch {
	// fully qualified.  Find the exact match
	case hasGroup && hasVersion:
		for plural, singular := range m.pluralToSingular {
			if singular == resource {
				ret = append(ret, plural)
				break
			}
			if plural == resource {
				ret = append(ret, plural)
				break
			}
		}

	case hasGroup:
		requestedGroupResource := resource.GroupResource()
		for currResource := range m.pluralToSingular {
			if currResource.GroupResource() == requestedGroupResource {
				ret = append(ret, currResource)
			}
		}

	case hasVersion:
		for currResource := range m.pluralToSingular {
			if currResource.Version == resource.Version && currResource.Resource == resource.Resource {
				ret = append(ret, currResource)
			}
		}

	default:
		for currResource := range m.pluralToSingular {
			if currResource.Resource == resource.Resource {
				ret = append(ret, currResource)
			}
		}
	}

	if len(ret) == 0 {
		return nil, fmt.Errorf("no resource %v has been defined; known resources: %v", resource, m.pluralToSingular)
	}

	sort.Sort(resourceByPreferredGroupVersion{ret, m.defaultGroupVersions})
	return ret, nil
}

func (m *DefaultRESTMapper) ResourceFor(resource unversioned.GroupVersionResource) (unversioned.GroupVersionResource, error) {
	resources, err := m.ResourcesFor(resource)
	if err != nil {
		return unversioned.GroupVersionResource{}, err
	}
	if len(resources) == 1 {
		return resources[0], nil
	}

	return unversioned.GroupVersionResource{}, fmt.Errorf("%v is ambiguous, got: %v", resource, resources)
}

func (m *DefaultRESTMapper) KindsFor(input unversioned.GroupVersionResource) ([]unversioned.GroupVersionKind, error) {
	resource := input.GroupVersion().WithResource(strings.ToLower(input.Resource))

	hasResource := len(resource.Resource) > 0
	hasGroup := len(resource.Group) > 0
	hasVersion := len(resource.Version) > 0

	if !hasResource {
		return nil, fmt.Errorf("a resource must be present, got: %v", resource)
	}

	ret := []unversioned.GroupVersionKind{}
	switch {
	// fully qualified.  Find the exact match
	case hasGroup && hasVersion:
		kind, exists := m.resourceToKind[resource]
		if exists {
			ret = append(ret, kind)
		}

	case hasGroup:
		requestedGroupResource := resource.GroupResource()
		for currResource, currKind := range m.resourceToKind {
			if currResource.GroupResource() == requestedGroupResource {
				ret = append(ret, currKind)
			}
		}

	case hasVersion:
		for currResource, currKind := range m.resourceToKind {
			if currResource.Version == resource.Version && currResource.Resource == resource.Resource {
				ret = append(ret, currKind)
			}
		}

	default:
		for currResource, currKind := range m.resourceToKind {
			if currResource.Resource == resource.Resource {
				ret = append(ret, currKind)
			}
		}
	}

	if len(ret) == 0 {
		return nil, fmt.Errorf("no kind %v has been defined; known resources: %v", resource, m.pluralToSingular)
	}

	sort.Sort(kindByPreferredGroupVersion{ret, m.defaultGroupVersions})
	return ret, nil
}

func (m *DefaultRESTMapper) KindFor(resource unversioned.GroupVersionResource) (unversioned.GroupVersionKind, error) {
	kinds, err := m.KindsFor(resource)
	if err != nil {
		return unversioned.GroupVersionKind{}, err
	}

	// TODO for each group, choose the most preferred (first) version.  This keeps us consistent with code today.
	// eventually, we'll need a RESTMapper that is aware of what's available server-side and deconflicts that with
	// user preferences
	oneKindPerGroup := []unversioned.GroupVersionKind{}
	groupsAdded := sets.String{}
	for _, kind := range kinds {
		if groupsAdded.Has(kind.Group) {
			continue
		}

		oneKindPerGroup = append(oneKindPerGroup, kind)
		groupsAdded.Insert(kind.Group)
	}

	if len(oneKindPerGroup) == 1 {
		return oneKindPerGroup[0], nil
	}

	return unversioned.GroupVersionKind{}, fmt.Errorf("%v is ambiguous, got: %v", resource, kinds)
}

type kindByPreferredGroupVersion struct {
	list      []unversioned.GroupVersionKind
	sortOrder []unversioned.GroupVersion
}

func (o kindByPreferredGroupVersion) Len() int      { return len(o.list) }
func (o kindByPreferredGroupVersion) Swap(i, j int) { o.list[i], o.list[j] = o.list[j], o.list[i] }
func (o kindByPreferredGroupVersion) Less(i, j int) bool {
	lhs := o.list[i]
	rhs := o.list[j]
	if lhs == rhs {
		return false
	}

	if lhs.GroupVersion() == rhs.GroupVersion() {
		return lhs.Kind < rhs.Kind
	}

	// otherwise, the difference is in the GroupVersion, so we need to sort with respect to the preferred order
	lhsIndex := -1
	rhsIndex := -1

	for i := range o.sortOrder {
		if o.sortOrder[i] == lhs.GroupVersion() {
			lhsIndex = i
		}
		if o.sortOrder[i] == rhs.GroupVersion() {
			rhsIndex = i
		}
	}

	if rhsIndex == -1 {
		return true
	}

	return lhsIndex < rhsIndex
}

type resourceByPreferredGroupVersion struct {
	list      []unversioned.GroupVersionResource
	sortOrder []unversioned.GroupVersion
}

func (o resourceByPreferredGroupVersion) Len() int      { return len(o.list) }
func (o resourceByPreferredGroupVersion) Swap(i, j int) { o.list[i], o.list[j] = o.list[j], o.list[i] }
func (o resourceByPreferredGroupVersion) Less(i, j int) bool {
	lhs := o.list[i]
	rhs := o.list[j]
	if lhs == rhs {
		return false
	}

	if lhs.GroupVersion() == rhs.GroupVersion() {
		return lhs.Resource < rhs.Resource
	}

	// otherwise, the difference is in the GroupVersion, so we need to sort with respect to the preferred order
	lhsIndex := -1
	rhsIndex := -1

	for i := range o.sortOrder {
		if o.sortOrder[i] == lhs.GroupVersion() {
			lhsIndex = i
		}
		if o.sortOrder[i] == rhs.GroupVersion() {
			rhsIndex = i
		}
	}

	if rhsIndex == -1 {
		return true
	}

	return lhsIndex < rhsIndex
}

// RESTMapping returns a struct representing the resource path and conversion interfaces a
// RESTClient should use to operate on the provided group/kind in order of versions. If a version search
// order is not provided, the search order provided to DefaultRESTMapper will be used to resolve which
// version should be used to access the named group/kind.
func (m *DefaultRESTMapper) RESTMapping(gk unversioned.GroupKind, versions ...string) (*RESTMapping, error) {
	// Pick an appropriate version
	var gvk *unversioned.GroupVersionKind
	hadVersion := false
	for _, version := range versions {
		if len(version) == 0 {
			continue
		}

		currGVK := gk.WithVersion(version)
		hadVersion = true
		if _, ok := m.kindToPluralResource[currGVK]; ok {
			gvk = &currGVK
			break
		}
	}
	// Use the default preferred versions
	if !hadVersion && (gvk == nil) {
		for _, gv := range m.defaultGroupVersions {
			if gv.Group != gk.Group {
				continue
			}

			currGVK := gk.WithVersion(gv.Version)
			if _, ok := m.kindToPluralResource[currGVK]; ok {
				gvk = &currGVK
				break
			}
		}
	}
	if gvk == nil {
		return nil, fmt.Errorf("no kind named %q is registered in versions %q", gk, versions)
	}

	// Ensure we have a REST mapping
	resource, ok := m.kindToPluralResource[*gvk]
	if !ok {
		found := []unversioned.GroupVersion{}
		for _, gv := range m.defaultGroupVersions {
			if _, ok := m.kindToPluralResource[*gvk]; ok {
				found = append(found, gv)
			}
		}
		if len(found) > 0 {
			return nil, fmt.Errorf("object with kind %q exists in versions %v, not %v", gvk.Kind, found, gvk.GroupVersion().String())
		}
		return nil, fmt.Errorf("the provided version %q and kind %q cannot be mapped to a supported object", gvk.GroupVersion().String(), gvk.Kind)
	}

	// Ensure we have a REST scope
	scope, ok := m.kindToScope[*gvk]
	if !ok {
		return nil, fmt.Errorf("the provided version %q and kind %q cannot be mapped to a supported scope", gvk.GroupVersion().String(), gvk.Kind)
	}

	interfaces, err := m.interfacesFunc(gvk.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("the provided version %q has no relevant versions", gvk.GroupVersion().String())
	}

	retVal := &RESTMapping{
		Resource:         resource.Resource,
		GroupVersionKind: *gvk,
		Scope:            scope,

		Codec:            interfaces.Codec,
		ObjectConvertor:  interfaces.ObjectConvertor,
		MetadataAccessor: interfaces.MetadataAccessor,
	}

	return retVal, nil
}

// aliasToResource is used for mapping aliases to resources
var aliasToResource = map[string][]string{}

// AddResourceAlias maps aliases to resources
func (m *DefaultRESTMapper) AddResourceAlias(alias string, resources ...string) {
	if len(resources) == 0 {
		return
	}
	aliasToResource[alias] = resources
}

// AliasesForResource returns whether a resource has an alias or not
func (m *DefaultRESTMapper) AliasesForResource(alias string) ([]string, bool) {
	if res, ok := aliasToResource[alias]; ok {
		return res, true
	}
	return nil, false
}

// ResourceIsValid takes a partial resource and checks if it's valid
func (m *DefaultRESTMapper) ResourceIsValid(resource unversioned.GroupVersionResource) bool {
	_, err := m.KindFor(resource)
	return err == nil
}

// MultiRESTMapper is a wrapper for multiple RESTMappers.
type MultiRESTMapper []RESTMapper

// ResourceSingularizer converts a REST resource name from plural to singular (e.g., from pods to pod)
// This implementation supports multiple REST schemas and return the first match.
func (m MultiRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	for _, t := range m {
		singular, err = t.ResourceSingularizer(resource)
		if err == nil {
			return
		}
	}
	return
}

func (m MultiRESTMapper) ResourcesFor(resource unversioned.GroupVersionResource) (gvk []unversioned.GroupVersionResource, err error) {
	for _, t := range m {
		gvk, err = t.ResourcesFor(resource)
		if err == nil {
			return
		}
	}
	return
}

// KindsFor provides the Kind mappings for the REST resources. This implementation supports multiple REST schemas and returns
// the first match.
func (m MultiRESTMapper) KindsFor(resource unversioned.GroupVersionResource) (gvk []unversioned.GroupVersionKind, err error) {
	for _, t := range m {
		gvk, err = t.KindsFor(resource)
		if err == nil {
			return
		}
	}
	return
}

func (m MultiRESTMapper) ResourceFor(resource unversioned.GroupVersionResource) (gvk unversioned.GroupVersionResource, err error) {
	for _, t := range m {
		gvk, err = t.ResourceFor(resource)
		if err == nil {
			return
		}
	}
	return
}

// KindsFor provides the Kind mapping for the REST resources. This implementation supports multiple REST schemas and returns
// the first match.
func (m MultiRESTMapper) KindFor(resource unversioned.GroupVersionResource) (gvk unversioned.GroupVersionKind, err error) {
	for _, t := range m {
		gvk, err = t.KindFor(resource)
		if err == nil {
			return
		}
	}
	return
}

// RESTMapping provides the REST mapping for the resource based on the
// kind and version. This implementation supports multiple REST schemas and
// return the first match.
func (m MultiRESTMapper) RESTMapping(gk unversioned.GroupKind, versions ...string) (mapping *RESTMapping, err error) {
	for _, t := range m {
		mapping, err = t.RESTMapping(gk, versions...)
		if err == nil {
			return
		}
	}
	return
}

// AliasesForResource finds the first alias response for the provided mappers.
func (m MultiRESTMapper) AliasesForResource(alias string) (aliases []string, ok bool) {
	for _, t := range m {
		if aliases, ok = t.AliasesForResource(alias); ok {
			return
		}
	}
	return nil, false
}

// ResourceIsValid takes a string (either group/kind or kind) and checks if it's a valid resource
func (m MultiRESTMapper) ResourceIsValid(resource unversioned.GroupVersionResource) bool {
	for _, t := range m {
		if t.ResourceIsValid(resource) {
			return true
		}
	}
	return false
}
