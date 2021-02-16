package flakefinder

import (
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Options struct {
	printTestOutput bool
}

var testOptions = Options{}

func TestMain(m *testing.M) {
	flag.BoolVar(&testOptions.printTestOutput, "print_test_output", false, "Whether test output should be printed via logger")
	flag.Parse()
	os.Exit(m.Run())
}

func TestFlakefinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flakefinder Suite")
}
