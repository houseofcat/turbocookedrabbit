package main_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {

	os.Exit(m.Run())
}

func TestCleanup(t *testing.T) {

}

func BenchCleanup(b *testing.B) {

}
