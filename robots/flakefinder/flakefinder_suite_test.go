package main_test

import (
	"flag"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/reporters"
)

type options struct {
	printTestOutput bool
	junitOutput     string
}

var testOptions = options{}

func TestMain(m *testing.M) {
	flag.BoolVar(&testOptions.printTestOutput, "print_test_output", false, "Whether test output should be printed via logger")
	flag.StringVar(&testOptions.junitOutput, "junit-output", "", "Set path to Junit report.")
	flag.Parse()
	os.Exit(m.Run())
}

func TestFlakefinder(t *testing.T) {
	RegisterFailHandler(Fail)
	testReporters := []Reporter{}
	if testOptions.junitOutput != "" {
		testReporters = append(testReporters, reporters.NewJUnitReporter(testOptions.junitOutput))
	}
	RunSpecsWithDefaultAndCustomReporters(t, "Flakefinder Suite", testReporters)
}
