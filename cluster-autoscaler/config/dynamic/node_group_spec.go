package dynamic

import (
	"fmt"
	"strconv"
	"strings"
)

// NodeGroupSpec represents a specification of a node group to be auto-scaled
type NodeGroupSpec struct {
	// The name of the autoscaling target
	Name    string `json:"name"`
	// Min size of the autoscaling target
	MinSize int    `json:"minSize"`
	// Max size of the autoscaling target
	MaxSize int    `json:"maxSize"`
}

// SpecFromString parses a node group spec represented in the form of `<minSize>:<maxSize>:<name>` and produces a node group spec object
func SpecFromString(value string) (*NodeGroupSpec, error) {
	tokens := strings.SplitN(value, ":", 3)
	if len(tokens) != 3 {
		return nil, fmt.Errorf("wrong nodes configuration: %s", value)
	}

	spec := NodeGroupSpec{}
	if size, err := strconv.Atoi(tokens[0]); err == nil {

		spec.MinSize = size
	} else {
		return nil, fmt.Errorf("failed to set min size: %s, expected integer", tokens[0])
	}

	if size, err := strconv.Atoi(tokens[1]); err == nil {
		spec.MaxSize = size
	} else {
		return nil, fmt.Errorf("failed to set max size: %s, expected integer", tokens[1])
	}

	spec.Name = tokens[2]

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid node group spec: %v", err)
	}

	return &spec, nil
}

// Validate produces an error if there's an invalid field in the node group spec
func (s NodeGroupSpec) Validate() error {
	if s.MinSize <= 0 {
		return fmt.Errorf("min size must be >= 1")
	}
	if s.MaxSize < s.MinSize {
		return fmt.Errorf("max size must be greater or equal to min size")
	}
	if s.Name == "" {
		return fmt.Errorf("name must not be blank")
	}
	return nil
}

// Represents the node group spec in the form of `<minSize>:<maxSize>:<name>`
func (s NodeGroupSpec) String() string {
	return fmt.Sprintf("%d:%d:%s", s.MinSize, s.MaxSize, s.Name)
}
