/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2022 Red Hat, Inc.
 *
 */

package jenkins

import (
	"fmt"
	"github.com/bndr/gojenkins"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"kubevirt.io/project-infra/robots/pkg/circuitbreaker"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestJenkins(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "jenkins suite")
}

type SimpleMockBuildDataGetter struct {
	callCounter int
	build       []*gojenkins.Build
	err         []error
}

func (d *SimpleMockBuildDataGetter) GetBuild(int64) (build *gojenkins.Build, err error) {
	build, err = d.build[d.callCounter], d.err[d.callCounter]
	d.callCounter++
	return build, err
}

type DurationBasedMockBuildDataGetter struct {
	callCounter   uint32
	start         time.Time
	durationIndex []time.Duration
	build         []*gojenkins.Build
	err           []error
}

func (d *DurationBasedMockBuildDataGetter) GetBuild(int64) (build *gojenkins.Build, err error) {
	atomic.AddUint32(&d.callCounter, 1)
	for index, durationInterval := range d.durationIndex {
		if time.Now().Before(d.start.Add(durationInterval)) {
			return d.build[index], d.err[index]
		}
	}
	panic(fmt.Errorf("no interval was matching!"))
}

func (d *DurationBasedMockBuildDataGetter) GetCallCounter() uint32 {
	return d.callCounter
}

var _ = Describe("builds.go", func() {

	BeforeEach(func() {
		retryDelay = 150 * time.Millisecond
		maxJitter = 10 * time.Millisecond
		circuitBreakerBuildDataGetter = circuitbreaker.NewCircuitBreaker(retryDelay)
	})

	When("retrying", func() {

		entry := logrus.WithField("dummy", "blah")

		It("should return build directly", func() {
			expectedBuild := &gojenkins.Build{}
			build, statusCode, err := getBuildFromGetterWithRetry(&SimpleMockBuildDataGetter{build: []*gojenkins.Build{expectedBuild}, err: []error{nil}}, int64(42), entry)
			Expect(build).To(BeIdenticalTo(expectedBuild))
			Expect(statusCode).To(BeEquivalentTo(0))
			Expect(err).To(BeNil())
		})

		It("should return nil if 404", func() {
			err2 := fmt.Errorf("404")
			build, statusCode, err := getBuildFromGetterWithRetry(&SimpleMockBuildDataGetter{build: []*gojenkins.Build{nil}, err: []error{err2}}, int64(42), entry)
			Expect(build).To(BeNil())
			Expect(statusCode).To(BeEquivalentTo(http.StatusNotFound))
			Expect(err).To(BeIdenticalTo(err2))
		})

		It("should return build after one retry with gateway timeout", func() {
			expectedBuild := &gojenkins.Build{}
			build, statusCode, err := getBuildFromGetterWithRetry(&SimpleMockBuildDataGetter{build: []*gojenkins.Build{nil, expectedBuild}, err: []error{fmt.Errorf("504"), nil}}, int64(42), entry)
			Expect(build).To(BeIdenticalTo(expectedBuild))
			Expect(statusCode).To(BeEquivalentTo(http.StatusGatewayTimeout))
			Expect(err).To(BeNil())
		})

		It("should only call the service once after 504 happened, then once per each thread after service is available again", func() {
			buildDataGetter := &DurationBasedMockBuildDataGetter{start: time.Now(), durationIndex: []time.Duration{100 * time.Millisecond, 1000 * time.Millisecond}, build: []*gojenkins.Build{nil, {}}, err: []error{fmt.Errorf("504"), nil}}
			var wg sync.WaitGroup
			numberOfThreads := 5
			wg.Add(numberOfThreads)
			for i := 0; i < numberOfThreads; i++ {
				go func() {
					defer wg.Done()
					_, _, _ = getBuildFromGetterWithRetry(buildDataGetter, int64(42), entry)
				}()
			}
			wg.Wait()
			Expect(buildDataGetter.GetCallCounter()).To(BeNumerically("<=", uint32(numberOfThreads+1)))
		})

		It("each of the getters should not be called more than twice", func() {
			numberOfThreads := 5
			var wg sync.WaitGroup
			wg.Add(numberOfThreads)
			var buildDataGetters []*DurationBasedMockBuildDataGetter
			for i := 0; i < numberOfThreads; i++ {
				buildDataGetter := &DurationBasedMockBuildDataGetter{start: time.Now(), durationIndex: []time.Duration{100 * time.Millisecond, 1000 * time.Millisecond}, build: []*gojenkins.Build{nil, {}}, err: []error{fmt.Errorf("504"), nil}}
				go func() {
					defer wg.Done()
					_, _, _ = getBuildFromGetterWithRetry(buildDataGetter, int64(42), entry)
				}()
				buildDataGetters = append(buildDataGetters, buildDataGetter)
			}
			wg.Wait()
			for _, b := range buildDataGetters {
				Expect(b.GetCallCounter()).To(BeNumerically("<=", 2))
			}
		})

	})

})
