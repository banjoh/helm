/*
Copyright The Helm Authors.

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

package release

import (
	"time"

	"helm.sh/helm/v4/pkg/chart"
)

type Releaser interface{}

type Hook interface{}

type Accessor interface {
	Name() string
	Namespace() string
	Version() int
	Hooks() []Hook
	Manifest() string
	Notes() string
	Labels() map[string]string
	Chart() chart.Charter
	Status() string
	ApplyMethod() string
	DeployedAt() time.Time
}

type HookAccessor interface {
	Path() string
	Manifest() string
	Name() string
	Kind() string
	Weight() int

	// HasEvent checks if the hook has the given event (as string for cross-version compatibility)
	HasEvent(event string) bool

	// HasDeletePolicy checks if the hook has the given delete policy (as string)
	HasDeletePolicy(policy string) bool
	SetDefaultDeletePolicy()

	// HasOutputLogPolicy checks if the hook has the given output log policy (as string)
	HasOutputLogPolicy(policy string) bool

	// LastRun management
	SetLastRunStarted()
	SetLastRunPhase(phase string)
	SetLastRunCompleted()
}

// HookDeletePolicyBeforeCreation is the policy string for before-hook-creation
const HookDeletePolicyBeforeCreation = "before-hook-creation"

// HookDeletePolicySucceeded is the policy string for hook-succeeded
const HookDeletePolicySucceeded = "hook-succeeded"

// HookDeletePolicyFailed is the policy string for hook-failed
const HookDeletePolicyFailed = "hook-failed"

// HookOutputPolicySucceeded is the output log policy string for hook-succeeded
const HookOutputPolicySucceeded = "hook-succeeded"

// HookOutputPolicyFailed is the output log policy string for hook-failed
const HookOutputPolicyFailed = "hook-failed"

// HookPhase constants for cross-version compatibility
const (
	HookPhaseUnknown   = "Unknown"
	HookPhaseRunning   = "Running"
	HookPhaseSucceeded = "Succeeded"
	HookPhaseFailed    = "Failed"
)
