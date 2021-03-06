/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pjutil

import (
	"regexp"

	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/config"
)

var TestAllRe = regexp.MustCompile(`(?m)^/test all,?($|\s.*)`)

// Filter digests a presubmit config to determine if:
//  - we the presubmit matched the filter
//  - we know that the presubmit is forced to run
//  - what the default behavior should be if the presubmit
//    runs conditionally and does not match trigger conditions
type Filter func(p config.Presubmit) (shouldRun bool, forcedToRun bool, defaultBehavior bool)

// CommandFilter builds a filter for `/test foo`
func CommandFilter(body string) Filter {
	return func(p config.Presubmit) (bool, bool, bool) {
		return p.TriggerMatches(body), p.TriggerMatches(body), true
	}
}

// TestAllFilter builds a filter for the automatic behavior of `/test all`.
// Jobs that explicitly match `/test all` in their trigger regex will be
// handled by a commandFilter for the comment in question.
func TestAllFilter() Filter {
	return func(p config.Presubmit) (bool, bool, bool) {
		return !p.NeedsExplicitTrigger(), false, false
	}
}

// AggregateFilter builds a filter that evaluates the child filters in order
// and returns the first match
func AggregateFilter(filters []Filter) Filter {
	return func(presubmit config.Presubmit) (bool, bool, bool) {
		for _, filter := range filters {
			if shouldRun, forced, defaults := filter(presubmit); shouldRun {
				return shouldRun, forced, defaults
			}
		}
		return false, false, false
	}
}

// FilterPresubmits determines which presubmits should run and which should be skipped
// by evaluating the user-provided filter.
func FilterPresubmits(filter Filter, changes config.ChangedFilesProvider, branch string, presubmits []config.Presubmit, logger *logrus.Entry) ([]config.Presubmit, []config.Presubmit, error) {

	var toTrigger []config.Presubmit
	var namesToTrigger []string
	var toSkip []config.Presubmit
	var namesToSkip []string
	for _, presubmit := range presubmits {
		matches, forced, defaults := filter(presubmit)
		if !matches {
			continue
		}
		shouldRun, err := presubmit.ShouldRun(branch, changes, forced, defaults)
		if err != nil {
			return nil, nil, err
		}
		if shouldRun {
			toTrigger = append(toTrigger, presubmit)
			namesToTrigger = append(namesToTrigger, presubmit.Name)
		} else {
			toSkip = append(toSkip, presubmit)
			namesToSkip = append(namesToSkip, presubmit.Name)
		}
	}

	logger.WithFields(logrus.Fields{"to-trigger": namesToTrigger, "to-skip": namesToSkip}).Debugf("Filtered %d jobs, found %d to trigger and %d to skip.", len(presubmits), len(toTrigger), len(toSkip))
	return toTrigger, toSkip, nil
}
