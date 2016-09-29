package controller

const (
	// ingressClassKey picks a specific "class" for the Ingress. The controller
	// only processes Ingresses with this annotation either unset, or set
	// to either gceIngessClass or the empty string.
	ingressClassKey = "kubernetes.io/ingress.class"
)

// ingAnnotations represents Ingress annotations.
type ingAnnotations map[string]string

func (ing ingAnnotations) ingressClass() string {
	val, ok := ing[ingressClassKey]
	if !ok {
		return ""
	}
	return val
}

func (ing ingAnnotations) getClass() string {
	val, ok := ing[ingressClassKey]
	if !ok {
		return ""
	}
	return val
}

// filterClass returns true if the ingress class value is in the strings slice
func (ing ingAnnotations) filterClass(values []string) bool {
	c := ing.getClass()
	for _, v := range values {
		if v == c {
			return true
		}
	}
	return false
}
