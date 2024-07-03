package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTls(t *testing.T) {
	tlsConfig := GenerateTLSConfig("test")

	require.NotNil(t, tlsConfig)
}
