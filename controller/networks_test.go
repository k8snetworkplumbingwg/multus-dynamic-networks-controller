package controller_test

import (
	"testing"

	"github.com/maiqueb/multus-dynamic-networks-controller/controller"
)

func TestRun(t *testing.T) {
	t.Run("networks-controller entry point (placeholder test)", func(t *testing.T) {
		if err := controller.Run(); err != nil {
			t.Errorf("failed execution of Run(): %v", err)
		}
	})
}
