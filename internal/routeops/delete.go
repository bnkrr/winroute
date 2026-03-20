package routeops

import "fmt"

// ErrorAction defines how batch deletion behaves after a route deletion error.
type ErrorAction int

const (
	ErrorActionContinue ErrorAction = iota
	ErrorActionStop
)

// DeleteRoutes applies deleteFn to each route and either aggregates or stops on errors.
func DeleteRoutes[T any](
	routes []T,
	deleteFn func(T) error,
	describeFn func(T) string,
	errorAction ErrorAction,
) (partialErrs []error, err error) {
	if len(routes) == 0 {
		return nil, nil
	}

	for _, route := range routes {
		if delErr := deleteFn(route); delErr != nil {
			wrappedErr := fmt.Errorf("failed to delete route (%s): %w", describeFn(route), delErr)
			if errorAction == ErrorActionStop {
				return nil, wrappedErr
			}
			partialErrs = append(partialErrs, wrappedErr)
		}
	}

	return partialErrs, nil
}
