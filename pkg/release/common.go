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
	"errors"
	"fmt"
	"time"

	v2release "helm.sh/helm/v4/internal/release/v2"
	"helm.sh/helm/v4/pkg/chart"
	v1release "helm.sh/helm/v4/pkg/release/v1"
)

var NewAccessor func(rel Releaser) (Accessor, error) = newDefaultAccessor //nolint:revive

var NewHookAccessor func(rel Hook) (HookAccessor, error) = newDefaultHookAccessor //nolint:revive

func newDefaultAccessor(rel Releaser) (Accessor, error) {
	switch v := rel.(type) {
	case v1release.Release:
		return &v1Accessor{&v}, nil
	case *v1release.Release:
		return &v1Accessor{v}, nil
	case v2release.Release:
		return &v2Accessor{&v}, nil
	case *v2release.Release:
		return &v2Accessor{v}, nil
	default:
		return nil, fmt.Errorf("unsupported release type: %T", rel)
	}
}

func newDefaultHookAccessor(hook Hook) (HookAccessor, error) {
	switch h := hook.(type) {
	case v1release.Hook:
		return &v1HookAccessor{&h}, nil
	case *v1release.Hook:
		return &v1HookAccessor{h}, nil
	case v2release.Hook:
		return &v2HookAccessor{&h}, nil
	case *v2release.Hook:
		return &v2HookAccessor{h}, nil
	default:
		return nil, errors.New("unsupported release hook type")
	}
}

type v1Accessor struct {
	rel *v1release.Release
}

func (a *v1Accessor) Name() string {
	return a.rel.Name
}

func (a *v1Accessor) Namespace() string {
	return a.rel.Namespace
}

func (a *v1Accessor) Version() int {
	return a.rel.Version
}

func (a *v1Accessor) Hooks() []Hook {
	var hooks = make([]Hook, len(a.rel.Hooks))
	for i, h := range a.rel.Hooks {
		hooks[i] = h
	}
	return hooks
}

func (a *v1Accessor) Manifest() string {
	return a.rel.Manifest
}

func (a *v1Accessor) Notes() string {
	return a.rel.Info.Notes
}

func (a *v1Accessor) Labels() map[string]string {
	return a.rel.Labels
}

func (a *v1Accessor) Chart() chart.Charter {
	return a.rel.Chart
}

func (a *v1Accessor) Status() string {
	return a.rel.Info.Status.String()
}

func (a *v1Accessor) ApplyMethod() string {
	return a.rel.ApplyMethod
}

func (a *v1Accessor) DeployedAt() time.Time {
	return a.rel.Info.LastDeployed
}

type v1HookAccessor struct {
	hook *v1release.Hook
}

func (a *v1HookAccessor) Path() string {
	return a.hook.Path
}

func (a *v1HookAccessor) Manifest() string {
	return a.hook.Manifest
}

func (a *v1HookAccessor) Name() string {
	return a.hook.Name
}

func (a *v1HookAccessor) Kind() string {
	return a.hook.Kind
}

func (a *v1HookAccessor) Weight() int {
	return a.hook.Weight
}

func (a *v1HookAccessor) HasEvent(event string) bool {
	for _, e := range a.hook.Events {
		if string(e) == event {
			return true
		}
	}
	return false
}

func (a *v1HookAccessor) HasDeletePolicy(policy string) bool {
	for _, p := range a.hook.DeletePolicies {
		if string(p) == policy {
			return true
		}
	}
	return false
}

func (a *v1HookAccessor) SetDefaultDeletePolicy() {
	if len(a.hook.DeletePolicies) == 0 {
		a.hook.DeletePolicies = []v1release.HookDeletePolicy{v1release.HookBeforeHookCreation}
	}
}

func (a *v1HookAccessor) HasOutputLogPolicy(policy string) bool {
	for _, p := range a.hook.OutputLogPolicies {
		if string(p) == policy {
			return true
		}
	}
	return false
}

func (a *v1HookAccessor) SetLastRunStarted() {
	a.hook.LastRun = v1release.HookExecution{
		StartedAt: time.Now(),
		Phase:     v1release.HookPhaseRunning,
	}
}

func (a *v1HookAccessor) SetLastRunPhase(phase string) {
	a.hook.LastRun.Phase = v1release.HookPhase(phase)
}

func (a *v1HookAccessor) SetLastRunCompleted() {
	a.hook.LastRun.CompletedAt = time.Now()
}

type v2Accessor struct {
	rel *v2release.Release
}

func (a *v2Accessor) Name() string {
	return a.rel.Name
}

func (a *v2Accessor) Namespace() string {
	return a.rel.Namespace
}

func (a *v2Accessor) Version() int {
	return a.rel.Version
}

func (a *v2Accessor) Hooks() []Hook {
	var hooks = make([]Hook, len(a.rel.Hooks))
	for i, h := range a.rel.Hooks {
		hooks[i] = h
	}
	return hooks
}

func (a *v2Accessor) Manifest() string {
	return a.rel.Manifest
}

func (a *v2Accessor) Notes() string {
	return a.rel.Info.Notes
}

func (a *v2Accessor) Labels() map[string]string {
	return a.rel.Labels
}

func (a *v2Accessor) Chart() chart.Charter {
	return a.rel.Chart
}

func (a *v2Accessor) Status() string {
	return a.rel.Info.Status.String()
}

func (a *v2Accessor) ApplyMethod() string {
	return a.rel.ApplyMethod
}

func (a *v2Accessor) DeployedAt() time.Time {
	return a.rel.Info.LastDeployed
}

type v2HookAccessor struct {
	hook *v2release.Hook
}

func (a *v2HookAccessor) Path() string {
	return a.hook.Path
}

func (a *v2HookAccessor) Manifest() string {
	return a.hook.Manifest
}

func (a *v2HookAccessor) Name() string {
	return a.hook.Name
}

func (a *v2HookAccessor) Kind() string {
	return a.hook.Kind
}

func (a *v2HookAccessor) Weight() int {
	return a.hook.Weight
}

func (a *v2HookAccessor) HasEvent(event string) bool {
	for _, e := range a.hook.Events {
		if string(e) == event {
			return true
		}
	}
	return false
}

func (a *v2HookAccessor) HasDeletePolicy(policy string) bool {
	for _, p := range a.hook.DeletePolicies {
		if string(p) == policy {
			return true
		}
	}
	return false
}

func (a *v2HookAccessor) SetDefaultDeletePolicy() {
	if len(a.hook.DeletePolicies) == 0 {
		a.hook.DeletePolicies = []v2release.HookDeletePolicy{v2release.HookBeforeHookCreation}
	}
}

func (a *v2HookAccessor) HasOutputLogPolicy(policy string) bool {
	for _, p := range a.hook.OutputLogPolicies {
		if string(p) == policy {
			return true
		}
	}
	return false
}

func (a *v2HookAccessor) SetLastRunStarted() {
	a.hook.LastRun = v2release.HookExecution{
		StartedAt: time.Now(),
		Phase:     v2release.HookPhaseRunning,
	}
}

func (a *v2HookAccessor) SetLastRunPhase(phase string) {
	a.hook.LastRun.Phase = v2release.HookPhase(phase)
}

func (a *v2HookAccessor) SetLastRunCompleted() {
	a.hook.LastRun.CompletedAt = time.Now()
}
