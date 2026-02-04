package cmdtest

import (
	"testing"
)

func TestMain(m *testing.M) {
	Main(m)
}

func TestSkylint(t *testing.T) {
	Run(t, "testdata/skylint")
}

func TestSkyfmt(t *testing.T) {
	Run(t, "testdata/skyfmt")
}

func TestSkycheck(t *testing.T) {
	Run(t, "testdata/skycheck")
}

func TestSkyquery(t *testing.T) {
	Run(t, "testdata/skyquery")
}

func TestSkydoc(t *testing.T) {
	Run(t, "testdata/skydoc")
}

func TestSkyls(t *testing.T) {
	Run(t, "testdata/skyls")
}
