package errors

import (
	"bytes"
	"fmt"
)

// Errors is an error aggregator, it's useful for aggregating all the errors in
// a process to fail once all the errors have been discovered
type Errors struct {
	errors    []error
	Formatter func([]error) string
}

// New returns a new Errors object
func New() Errors {
	return Errors{
		errors:    make([]error, 0),
		Formatter: formatErrors,
	}
}

func formatErrors(errors []error) string {
	if len(errors) == 0 {
		return "Formatting 0 errors shouldn't happen, this is clearly a programming failure"
	}

	if len(errors) == 1 {
		return fmt.Sprintf("1 error: %s", errors[0])
	}

	buffer := bytes.NewBufferString(fmt.Sprintf("%d errors: ", len(errors)))

	for i, e := range errors {
		if i != 0 {
			buffer.WriteString("; ")
		}
		buffer.WriteString(e.Error())
	}
	return buffer.String()
}

// Append adds a new error to the list
func (e *Errors) Append(err error) {
	e.errors = append(e.errors, err)
}

func (e Errors) Error() string {
	return e.Formatter(e.errors)
}

// ErrorOrNil builds a single error with all the errors in it or a nil, use to
// collapse reality into a single state
func (e Errors) ErrorOrNil() error {
	if len(e.errors) == 0 {
		return nil
	}
	return fmt.Errorf(e.Error())
}
