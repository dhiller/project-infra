package flakefinder

import (
	"bytes"
	"fmt"
	"github.com/joshdk/go-junit"
	"io/ioutil"
	"kubevirt.io/project-infra/robots/pkg/flakefinder"
	"log"
	"os"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("report.go", func() {

	RegisterFailHandler(Fail)

	reportTime, e := time.Parse("2006-01-02", "2019-08-23")
	Expect(e).ToNot(HaveOccurred())

	When("creating filename with date and merged as hours", func() {

		It("creates a filename for week", func() {
			fileName := CreateReportFileName(reportTime, 24*7*time.Hour)
			Expect(fileName).To(BeEquivalentTo("flakefinder-2019-08-23-168h.html"))
		})

		It("creates a filename for day", func() {
			fileName := CreateReportFileName(reportTime, 24*time.Hour)
			Expect(fileName).To(BeEquivalentTo("flakefinder-2019-08-23-024h.html"))
		})

	})

	When("rendering report data", func() {

		var buffer bytes.Buffer

		prepareBuffer := func(parameters Params) {
			buffer = bytes.Buffer{}
			err := flakefinder.WriteTemplateToOutput(ReportTemplate, parameters, &buffer)
			Expect(err).ToNot(HaveOccurred())
			if testOptions.printTestOutput {
				logger := log.New(os.Stdout, "report_test.go:", log.Flags())
				logger.Printf(buffer.String())
			}
		}

		prepareWithDefaultParams := func() {
			parameters := Params{Data: map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: "red", Jobs: []*Job{}}},
			}, Headers: []string{"a", "b", "c"}, Tests: []string{"t1", "t2", "t3"}, EndOfReport: "2019-08-23",
				Org: Org, Repo: Repo,
				PrNumbers: []int{17, 42},
			}

			prepareBuffer(parameters)
		}

		It("outputs something", func() {
			prepareWithDefaultParams()
			Expect(buffer.String()).ToNot(BeEmpty())
		})

		It("has rows", func() {
			prepareWithDefaultParams()
			Expect(buffer.String()).To(ContainSubstring("<td>t1</td>"))
			Expect(buffer.String()).To(ContainSubstring("<td>t2</td>"))
			Expect(buffer.String()).To(ContainSubstring("<td>t3</td>"))
		})

		It("has columns", func() {
			prepareWithDefaultParams()
			Expect(buffer.String()).To(ContainSubstring("<td>a</td>"))
			Expect(buffer.String()).To(ContainSubstring("<td>b</td>"))
			Expect(buffer.String()).To(ContainSubstring("<td>c</td>"))
		})

		It("has one filled test cell", func() {
			prepareWithDefaultParams()
			Expect(buffer.String()).To(ContainSubstring("<td class=\"red center\">"))
			Expect(buffer.String()).To(MatchRegexp("(?s)4.*1.*2"))
		})

		It("contains the date", func() {
			prepareWithDefaultParams()
			Expect(buffer.String()).To(ContainSubstring("2019-08-23"))
		})

		It("contains the pr ids", func() {
			prepareWithDefaultParams()
			Expect(buffer.String()).To(ContainSubstring("#17"))
			Expect(buffer.String()).To(ContainSubstring("#42"))
		})

		It("shows no errors if no failing tests", func() {
			parameters := Params{Data: map[string]map[string]*Details{},
				Headers: []string{}, Tests: []string{}, EndOfReport: "2019-08-23",
				Org: Org, Repo: Repo,
				PrNumbers: []int{17, 42},
			}

			prepareBuffer(parameters)

			Expect(buffer.String()).To(ContainSubstring("No failing tests!"))
		})

		It("shows pr ids if no failing tests", func() {
			parameters := Params{Data: map[string]map[string]*Details{},
				Headers: []string{}, Tests: []string{}, EndOfReport: "2019-08-23",
				Org: Org, Repo: Repo,
				PrNumbers: []int{17, 42},
			}

			prepareBuffer(parameters)

			Expect(buffer.String()).To(ContainSubstring("#17"))
			Expect(buffer.String()).To(ContainSubstring("#42"))
		})

		DescribeTable("title contains repo and org", func(org, repo string) {
			parameters := Params{Data: map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: "red", Jobs: []*Job{}}},
			}, Headers: []string{"a", "b", "c"}, Tests: []string{"t1", "t2", "t3"}, EndOfReport: "2019-08-23", Org: org, Repo: repo}

			prepareBuffer(parameters)

			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("<title>%s/%s", org, repo)))
		},
			Entry("is kubevirt/kubevirt", "kubevirt", "kubevirt"),
			Entry("is kubevirt/containerized-data-importer", "kubevirt", "containerized-data-importer"),
			Entry("is test/blah", "test", "blah"),
		)

		DescribeTable("prow link contains repo and org", func(org, repo string) {
			parameters := Params{Data: map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: "red", Jobs: []*Job{
					{BuildNumber: 1742, Severity: "red", PR: 1427, Job: "testblah"},
				}}},
			}, Headers: []string{"a", "b", "c"}, Tests: []string{"t1", "t2", "t3"}, EndOfReport: "2019-08-23", Org: org, Repo: repo}

			prepareBuffer(parameters)

			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("pr-logs/pull/%s", fmt.Sprintf("%s_%s", org, repo))))
		},
			Entry("is kubevirt/kubevirt", "kubevirt", "kubevirt"),
			Entry("is kubevirt/containerized-data-importer", "kubevirt", "containerized-data-importer"),
			Entry("is test/blah", "test", "blah"),
		)

		DescribeTable("GitHub link contains repo and org", func(org, repo string) {
			parameters := Params{Data: map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: "red", Jobs: []*Job{
					{BuildNumber: 1742, Severity: "red", PR: 1427, Job: "testblah"},
				}}},
			}, Headers: []string{"a", "b", "c"}, Tests: []string{"t1", "t2", "t3"}, EndOfReport: "2019-08-23", Org: org, Repo: repo}

			prepareBuffer(parameters)

			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("https://github.com/%s/%s", org, repo)))
		},
			Entry("is kubevirt/kubevirt", "kubevirt", "kubevirt"),
			Entry("is kubevirt/containerized-data-importer", "kubevirt", "containerized-data-importer"),
			Entry("is test/blah", "test", "blah"),
		)

		It("shows job header table", func() {
			parameters := Params{Data: map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: "red", Jobs: []*Job{
					{BuildNumber: 1742, Severity: "red", PR: 1427, Job: "testblah"},
				}}},
			}, Headers: []string{"a", "b", "c"}, Tests: []string{"t1", "t2", "t3"}, EndOfReport: "2019-08-23", Org: "kubevirt", Repo: "kubevirt",
				FailuresForJobs: map[int]*JobFailures{
					1742: {
						BuildNumber: 1742,
						PR: 17,
						Job: "k8s-1.18-whatever",
						Failures: 66,
					},
					4217:  {
						BuildNumber: 4217,
						PR: 42,
						Job: "k8s-1.19-whocares",
						Failures: 66,
					},
				},
			}

			prepareBuffer(parameters)

			Expect(buffer.String()).To(ContainSubstring("4217"))
			Expect(buffer.String()).To(ContainSubstring("k8s-1.18-whatever"))
			Expect(buffer.String()).To(ContainSubstring("k8s-1.19-whocares"))
		})

	})

	When("calculating report data", func() {

		testJunitFile, err := ioutil.ReadFile(path.Join("testdata","junit.functest.1.20.1.xml"))
		Expect(err).To(BeNil())
		report1_20_1, err := junit.Ingest(testJunitFile)
		Expect(err).To(BeNil())
		testJunitFile, err = ioutil.ReadFile(path.Join("testdata","junit.functest.1.20.2.xml"))
		Expect(err).To(BeNil())
		report1_20_2, err := junit.Ingest(testJunitFile)
		Expect(err).To(BeNil())
		testJunitFile, err = ioutil.ReadFile(path.Join("testdata","junit.functest.1.18.1.xml"))
		Expect(err).To(BeNil())
		report1_18_1, err := junit.Ingest(testJunitFile)
		Expect(err).To(BeNil())
		testJunitFile, err = ioutil.ReadFile(path.Join("testdata","junit.functest.1.18.2.xml"))
		Expect(err).To(BeNil())
		report1_18_2, err := junit.Ingest(testJunitFile)
		Expect(err).To(BeNil())

		It("doesn't break on empty data", func() {
			results := []*Result{}
			prNumbers := []int{}
			createReportData(results ,prNumbers, time.Now(), "kubevirt", "kubevirt", time.Now())
		})

		It("doesn't break on empty junit data", func() {
			results := []*Result{
				&Result{
					"pull-kubevirt-e2e-k8s-1.20",
					[]junit.Suite{},
					1742,
					4217,
				},
			}
			prNumbers := []int{}
			createReportData(results ,prNumbers, time.Now(), "kubevirt", "kubevirt", time.Now())
		})

		It("doesn't break on junit data", func() {
			results := []*Result{
				{
					"pull-kubevirt-e2e-k8s-1.20",
					report1_20_1,
					1,
					17,
								},
				{
					"pull-kubevirt-e2e-k8s-1.20",
					report1_20_2,
					2,
					17,
								},
			}
			prNumbers := []int{17}
			createReportData(results ,prNumbers, time.Now(), "kubevirt", "kubevirt", time.Now())
		})

		It("returns useful things", func() {
			results := []*Result{
				{
					"pull-kubevirt-e2e-k8s-1.20",
					report1_20_1,
					1,
					17,
				},
				{
					"pull-kubevirt-e2e-k8s-1.20",
					report1_20_2,
					2,
					17,
				},
				{
					"pull-kubevirt-e2e-k8s-1.18",
					report1_18_1,
					1,
					17,
				},
				{
					"pull-kubevirt-e2e-k8s-1.18",
					report1_18_2,
					2,
					17,
				},
			}
			prNumbers := []int{17}
			data := createReportData(results, prNumbers, time.Now(), "kubevirt", "kubevirt", time.Now())
			Expect(data.Headers).To(BeEquivalentTo([]string{
				"pull-kubevirt-e2e-k8s-1.20",
				"pull-kubevirt-e2e-k8s-1.18",
			}))
			test1 := "Storage Starting a VirtualMachineInstance Run a VMI with VirtIO-FS and a datavolume should be successfully started and virtiofs could be accessed"
			test2 := "[rfe_id:3423][crit:high][vendor:cnv-qe@redhat.com][level:component]VmWatch [test_id:3466]Should update vmi status with the proper columns using 'kubectl get vmi -w'"
			Expect(data.Tests).To(BeEquivalentTo([]string{
				test1,
				test2,
			}))
			Expect(data.Data).To(HaveLen(2))
			Expect(data.Data[test1]).To(HaveLen(2))
			Expect(data.Data[test2]).To(HaveLen(2))
		})

	})

	When("sorting test data", func() {
		tests := []string{"t1", "t2", "t3"}

		It("returns all tests", func() {
			data := map[string]map[string]*Details{
				"t3": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}},
			}

			Expect(SortTestsByRelevance(data, tests)).To(BeEquivalentTo([]string{"t3", "t1", "t2"}))
		})

		It("returns no duplicated tests", func() {
			data := map[string]map[string]*Details{
				"t1": {
					"a": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: MostlyFlaky, Jobs: []*Job{}},
					"b": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: Unimportant, Jobs: []*Job{}},
				},
				"t2": {"a": &Details{Failed: 2, Succeeded: 1, Skipped: 2, Severity: MildlyFlaky, Jobs: []*Job{}}},
				"t3": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}},
			}

			Expect(SortTestsByRelevance(data, tests)).To(BeEquivalentTo([]string{"t3", "t1", "t2"}))
		})

		It("returns no duplicated tests for the end", func() {
			data := map[string]map[string]*Details{
				"t1": {
					"a": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: MostlyFlaky, Jobs: []*Job{}},
					"b": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: Unimportant, Jobs: []*Job{}},
				},
				"t2": {
					"a": &Details{Failed: 2, Succeeded: 1, Skipped: 2, Severity: MildlyFlaky, Jobs: []*Job{}},
					"b": &Details{Failed: 2, Succeeded: 1, Skipped: 2, Severity: Unimportant, Jobs: []*Job{}},
				},
				"t3": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}},
			}

			Expect(SortTestsByRelevance(data, tests)).To(BeEquivalentTo([]string{"t3", "t1", "t2"}))
		})

		It("returns tests sorted descending by severity", func() {
			data := map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: MostlyFlaky, Jobs: []*Job{}}},
				"t2": {"a": &Details{Failed: 2, Succeeded: 1, Skipped: 2, Severity: MildlyFlaky, Jobs: []*Job{}}},
				"t3": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}},
			}

			Expect(SortTestsByRelevance(data, tests)).To(BeEquivalentTo([]string{"t3", "t1", "t2"}))
		})

		It("returns tests of same severity sorted descending by number of severity points", func() {
			data := map[string]map[string]*Details{
				"t1": {"a": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}, "b": &Details{Failed: 3, Succeeded: 1, Skipped: 2, Severity: MostlyFlaky, Jobs: []*Job{}}},
				"t2": {"a": &Details{Failed: 2, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}, "b": &Details{Failed: 2, Succeeded: 1, Skipped: 2, Severity: MildlyFlaky, Jobs: []*Job{}}},
				"t3": {"a": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}, "b": &Details{Failed: 4, Succeeded: 1, Skipped: 2, Severity: HeavilyFlaky, Jobs: []*Job{}}},
			}

			Expect(SortTestsByRelevance(data, tests)).To(BeEquivalentTo([]string{"t3", "t1", "t2"}))
		})

	})

	When("sorting test via severity", func() {

		It("returns tests of same severity sorted descending by number of severity points", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"t1": {HeavilyFlaky: 2},
				"t2": {HeavilyFlaky: 1},
				"t3": {HeavilyFlaky: 3},
			})).To(BeEquivalentTo([]string{"t3", "t1", "t2"}))
		})

		It("returns tests of same severity and same number of severity points sorted lexically", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"tb": {HeavilyFlaky: 2},
				"tc": {HeavilyFlaky: 2},
				"ta": {HeavilyFlaky: 2},
			})).To(BeEquivalentTo([]string{"ta", "tb", "tc"}))
		})

		It("returns tests of same severity sorted by lower severity", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"tb": {HeavilyFlaky: 2, MostlyFlaky: 2},
				"tc": {HeavilyFlaky: 2, MostlyFlaky: 1},
				"ta": {HeavilyFlaky: 2, MostlyFlaky: 3},
			})).To(BeEquivalentTo([]string{"ta", "tb", "tc"}))
		})

		It("returns tests of same severity sorted by lower severity if even lower values present but zero", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"tb": {HeavilyFlaky: 2, MostlyFlaky: 2, ModeratelyFlaky: 0, MildlyFlaky: 0},
				"tc": {HeavilyFlaky: 2, MostlyFlaky: 1, ModeratelyFlaky: 0, MildlyFlaky: 0},
				"ta": {HeavilyFlaky: 2, MostlyFlaky: 3, ModeratelyFlaky: 0, MildlyFlaky: 0},
			})).To(BeEquivalentTo([]string{"ta", "tb", "tc"}))
		})

		It("returns tests of same severity sorted by lower severity if inbetween values present but zero", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"tb": {HeavilyFlaky: 2, MostlyFlaky: 0, ModeratelyFlaky: 2, MildlyFlaky: 0},
				"tc": {HeavilyFlaky: 2, MostlyFlaky: 0, ModeratelyFlaky: 1, MildlyFlaky: 0},
				"ta": {HeavilyFlaky: 2, MostlyFlaky: 0, ModeratelyFlaky: 3, MildlyFlaky: 0},
			})).To(BeEquivalentTo([]string{"ta", "tb", "tc"}))
		})

		It("returns tests of same severity sorted by lower severity if some inbetween values zero", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"tb": {HeavilyFlaky: 2, MostlyFlaky: 0, ModeratelyFlaky: 2, MildlyFlaky: 0},
				"tc": {HeavilyFlaky: 2, MostlyFlaky: 0, ModeratelyFlaky: 0, MildlyFlaky: 0},
				"ta": {HeavilyFlaky: 2, MostlyFlaky: 3, ModeratelyFlaky: 0, MildlyFlaky: 0},
			})).To(BeEquivalentTo([]string{"ta", "tb", "tc"}))
		})

		It("returns tests of same severity sorted by lower severity if some inbetween values zero with more values", func() {
			Expect(BuildUpSortedTestsBySeverity(map[string]map[string]int{
				"tb": {HeavilyFlaky: 1, MostlyFlaky: 0, ModeratelyFlaky: 0, MildlyFlaky: 1, Fine: 0, Unimportant: 0},
				"tc": {HeavilyFlaky: 1, MostlyFlaky: 0, ModeratelyFlaky: 0, MildlyFlaky: 0, Fine: 0, Unimportant: 0},
				"ta": {HeavilyFlaky: 1, MostlyFlaky: 0, ModeratelyFlaky: 0, MildlyFlaky: 2, Fine: 0, Unimportant: 0},
			})).To(BeEquivalentTo([]string{"ta", "tb", "tc"}))
		})

	})

	DescribeTable("When comparing severity",
		func(a, b *TestToSeverityOccurrences, expected bool) {
			bySeverity := []*TestToSeverityOccurrences{a, b}
			Expect(BySeverity.Less(bySeverity, 0, 1)).To(BeEquivalentTo(expected))
		},

		Entry("ta -> Sev(2) less than tb -> Sev(2) is false",
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{2}},
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{2}},
			false,
		),
		Entry("tb -> Sev(2) less than ta -> Sev(2) is true",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{2}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{2}},
			true,
		),
		Entry("ta -> Sev(2) less than ta -> Sev(2) is false",
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{2}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{2}},
			false,
		),
		Entry("tb -> Sev(3) is less than ta -> Sev(2) is false",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{3}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{2}},
			false,
		),
		Entry("ta -> Sev(2) is less than tb -> Sev(3) is true",
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{2}},
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{3}},
			true,
		),
		Entry("tb -> Sev(3, 2) is less ta -> Sev(3, 3) is true",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{3, 2}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{3, 3}},
			true,
		),
		Entry("tb -> Sev(3, 3) is less ta -> Sev(3, 2) is false",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{3, 3}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{3, 2}},
			false,
		),
		Entry("tb -> Sev(3, 0, 3) is less ta -> Sev(3, 0, 2) is false",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{3, 0, 3}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{3, 0, 2}},
			false,
		),
		Entry("tb -> Sev(3, 0, 2) is not less ta -> Sev(3, 0, 3) is true",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{3, 0, 2}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{3, 0, 3}},
			true,
		),
		Entry("tb -> Sev(1,0,0,2,0,0) is less ta -> Sev(1,0,0,1,0,0) is false",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{1, 0, 0, 2, 0, 0}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{1, 0, 0, 1, 0, 0}},
			false,
		),
		Entry("ta -> Sev(1,0,0,1,0,0) is less tb -> Sev(1,0,0,2,0,0) is true",
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{1, 0, 0, 1, 0, 0}},
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{1, 0, 0, 2, 0, 0}},
			true,
		),
		Entry("tb -> Sev(1,0,1,0,0,0) is less ta -> Sev(1,0,0,1,0,0) is false",
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{1, 0, 1, 0, 0, 0}},
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{1, 0, 0, 1, 0, 0}},
			false,
		),
		Entry("ta -> Sev(1,0,0,1,0,0) is less tb -> Sev(1,0,1,0,0,0) is true",
			&TestToSeverityOccurrences{Name: "ta", SeverityOccurrences: []int{1, 0, 0, 1, 0, 0}},
			&TestToSeverityOccurrences{Name: "tb", SeverityOccurrences: []int{1, 0, 1, 0, 0, 0}},
			true,
		),
	)

	When("swapping elements", func() {

		It("Works", func() {
			bySeverity := []*TestToSeverityOccurrences{
				{Name: "tb", SeverityOccurrences: []int{3, 0, 2}},
				{Name: "ta", SeverityOccurrences: []int{3, 0, 3}},
			}
			BySeverity.Swap(bySeverity, 0, 1)
			Expect(bySeverity[0].Name).To(BeEquivalentTo("ta"))
			Expect(bySeverity[1].Name).To(BeEquivalentTo("tb"))
		})

	})

	DescribeTable("When calculating severity",
		func(details *Details, expected string) {
			SetSeverity(details)
			Expect(details.Severity).To(BeEquivalentTo(expected))
		},
		Entry("results having no failed tests but successful tests is fine", &Details{Failed: 0, Succeeded: 1, Skipped: 2, Jobs: []*Job{}}, Fine),
		Entry("results having no successful tests is heavily flaky", &Details{Failed: 1, Succeeded: 0, Skipped: 2, Jobs: []*Job{}}, HeavilyFlaky),
		Entry("results being HeavilyFlaky", &Details{Failed: 1, Succeeded: 1, Skipped: 2, Jobs: []*Job{}}, HeavilyFlaky),
		Entry("results being MostlyFlaky", &Details{Failed: 1, Succeeded: 2, Skipped: 2, Jobs: []*Job{}}, MostlyFlaky),
		Entry("results being ModeratelyFlaky", &Details{Failed: 1, Succeeded: 5, Skipped: 2, Jobs: []*Job{}}, ModeratelyFlaky),
		Entry("results being MildlyFlaky", &Details{Failed: 1, Succeeded: 10, Skipped: 2, Jobs: []*Job{}}, MildlyFlaky),
	)

})
