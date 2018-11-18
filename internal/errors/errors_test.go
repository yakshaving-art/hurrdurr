package errors_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
)

func TestNoErrors(t *testing.T) {
	errs := errors.New()

	assert.Nil(t, errs.ErrorOrNil())
	assert.EqualError(t, errs, "Formatting 0 errors shouldn't happen, this is clearly a programming failure")
}
func TestSingleError(t *testing.T) {
	errs := errors.New()
	errs.Append(fmt.Errorf("my error"))

	assert.EqualError(t, errs.ErrorOrNil(), "1 error: my error")
	assert.EqualError(t, errs, "1 error: my error")
}

func TestMultipleErrors(t *testing.T) {
	errs := errors.New()
	errs.Append(fmt.Errorf("my error"))
	errs.Append(fmt.Errorf("my other error"))

	assert.EqualError(t, errs.ErrorOrNil(), "2 errors: my error; my other error")
	assert.EqualError(t, errs, "2 errors: my error; my other error")
}
