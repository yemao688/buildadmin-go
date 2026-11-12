package upload

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBusinessMessageMapsUploadDomainErrors(t *testing.T) {
	message, ok := BusinessMessage(ErrMimetypeNotAllowed)
	require.True(t, ok)
	require.Equal(t, ErrMimetypeNotAllowed.Error(), message)

	message, ok = BusinessMessage(&FileTooLargeError{Message: "too large"})
	require.True(t, ok)
	require.Equal(t, "too large", message)

	_, ok = BusinessMessage(errors.New("plain"))
	require.False(t, ok)
}
