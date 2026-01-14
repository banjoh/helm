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

package action

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"time"

	"helm.sh/helm/v4/pkg/kube"

	"go.yaml.in/yaml/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2release "helm.sh/helm/v4/internal/release/v2"
	ri "helm.sh/helm/v4/pkg/release"
	release "helm.sh/helm/v4/pkg/release/v1"
)

// execHook executes all of the hooks for the given hook event.
func (cfg *Configuration) execHook(rl *release.Release, hook release.HookEvent, waitStrategy kube.WaitStrategy, timeout time.Duration, serverSideApply bool) error {
	shutdown, err := cfg.execHookWithDelayedShutdown(rl, hook, waitStrategy, timeout, serverSideApply)
	if shutdown == nil {
		return err
	}
	if err != nil {
		if err := shutdown(); err != nil {
			return err
		}
		return err
	}
	return shutdown()
}

type ExecuteShutdownFunc = func() error

func shutdownNoOp() error {
	return nil
}

// hookExecutionCallbacks holds the callbacks needed for hook execution.
// This allows the core hook logic to work with both v1 and v2 releases.
type hookExecutionCallbacks struct {
	recordRelease      func()
	deleteByPolicy     func(hook ri.HookAccessor, policy string) error
	outputLogsByPolicy func(hook ri.HookAccessor, policy string) error
}

// execHookCore is the core hook execution logic that works with HookAccessor interface.
// It takes hooks as accessors and uses callbacks for release-type-specific operations.
func (cfg *Configuration) execHookCore(executingHooks []ri.HookAccessor, hookEvent string, _ string, waitStrategy kube.WaitStrategy, timeout time.Duration, serverSideApply bool, callbacks hookExecutionCallbacks) (ExecuteShutdownFunc, error) {
	// Sort by weight (hooks are pre-ordered by kind, so keep order stable)
	sort.SliceStable(executingHooks, func(i, j int) bool {
		if executingHooks[i].Weight() == executingHooks[j].Weight() {
			return executingHooks[i].Name() < executingHooks[j].Name()
		}
		return executingHooks[i].Weight() < executingHooks[j].Weight()
	})

	for i, h := range executingHooks {
		// Set default delete policy to before-hook-creation
		h.SetDefaultDeletePolicy()

		if err := callbacks.deleteByPolicy(h, ri.HookDeletePolicyBeforeCreation); err != nil {
			return shutdownNoOp, err
		}

		resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest()), true)
		if err != nil {
			return shutdownNoOp, fmt.Errorf("unable to build kubernetes object for %s hook %s: %w", hookEvent, h.Path(), err)
		}

		// Record the time at which the hook was applied to the cluster
		h.SetLastRunStarted()
		callbacks.recordRelease()

		// As long as the implementation of WatchUntilReady does not panic, HookPhaseFailed or HookPhaseSucceeded
		// should always be set by this function. If we fail to do that for any reason, then HookPhaseUnknown is
		// the most appropriate value to surface.
		h.SetLastRunPhase(ri.HookPhaseUnknown)

		// Create hook resources
		if _, err := cfg.KubeClient.Create(
			resources,
			kube.ClientCreateOptionServerSideApply(serverSideApply, false)); err != nil {
			h.SetLastRunCompleted()
			h.SetLastRunPhase(ri.HookPhaseFailed)
			return shutdownNoOp, fmt.Errorf("warning: Hook %s %s failed: %w", hookEvent, h.Path(), err)
		}

		waiter, err := cfg.KubeClient.GetWaiter(waitStrategy)
		if err != nil {
			return shutdownNoOp, fmt.Errorf("unable to get waiter: %w", err)
		}
		// Watch hook resources until they have completed
		err = waiter.WatchUntilReady(resources, timeout)
		// Note the time of success/failure
		h.SetLastRunCompleted()
		// Mark hook as succeeded or failed
		if err != nil {
			h.SetLastRunPhase(ri.HookPhaseFailed)
			// If a hook is failed, check the annotation of the hook to determine if we should copy the logs client side
			if errOutputting := callbacks.outputLogsByPolicy(h, ri.HookOutputPolicyFailed); errOutputting != nil {
				log.Printf("error outputting logs for hook failure: %v", errOutputting)
			}
			// Return a function to clean up on failure
			return func() error {
				if errDeleting := callbacks.deleteByPolicy(h, ri.HookDeletePolicyFailed); errDeleting != nil {
					log.Printf("error deleting the hook resource on hook failure: %v", errDeleting)
				}
				// Delete previous successful hooks
				for j := 0; j < i; j++ {
					if err := callbacks.deleteByPolicy(executingHooks[j], ri.HookDeletePolicySucceeded); err != nil {
						return err
					}
				}
				return err
			}, err
		}
		h.SetLastRunPhase(ri.HookPhaseSucceeded)
	}

	return func() error {
		// If all hooks are successful, clean up and output logs
		for i := len(executingHooks) - 1; i >= 0; i-- {
			h := executingHooks[i]
			if err := callbacks.outputLogsByPolicy(h, ri.HookOutputPolicySucceeded); err != nil {
				log.Printf("error outputting logs for hook success: %v", err)
			}
			if err := callbacks.deleteByPolicy(h, ri.HookDeletePolicySucceeded); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// execHookWithDelayedShutdown executes all of the hooks for the given hook event and returns a shutdownHook function to trigger deletions after doing other things like e.g. retrieving logs.
func (cfg *Configuration) execHookWithDelayedShutdown(rl *release.Release, hook release.HookEvent, waitStrategy kube.WaitStrategy, timeout time.Duration, serverSideApply bool) (ExecuteShutdownFunc, error) {
	// Build list of hooks matching this event as accessors
	var executingHooks []ri.HookAccessor
	for _, h := range rl.Hooks {
		for _, e := range h.Events {
			if e == hook {
				// NewHookAccessor cannot fail for known hook types (*release.Hook)
				acc, _ := ri.NewHookAccessor(h)
				executingHooks = append(executingHooks, acc)
			}
		}
	}

	callbacks := hookExecutionCallbacks{
		recordRelease: func() { cfg.recordRelease(rl) },
		deleteByPolicy: func(h ri.HookAccessor, policy string) error {
			return cfg.deleteHookByPolicyGeneric(h, policy, waitStrategy, timeout)
		},
		outputLogsByPolicy: func(h ri.HookAccessor, policy string) error {
			return cfg.outputLogsByPolicyGeneric(h, rl.Namespace, policy)
		},
	}

	return cfg.execHookCore(executingHooks, string(hook), rl.Namespace, waitStrategy, timeout, serverSideApply, callbacks)
}

// hookByWeight is a sorter for hooks
type hookByWeight []*release.Hook

func (x hookByWeight) Len() int      { return len(x) }
func (x hookByWeight) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x hookByWeight) Less(i, j int) bool {
	if x[i].Weight == x[j].Weight {
		return x[i].Name < x[j].Name
	}
	return x[i].Weight < x[j].Weight
}

// hookSetDeletePolicy sets the default delete policy on a hook if none is set.
func (cfg *Configuration) hookSetDeletePolicy(h *release.Hook) {
	cfg.mutex.Lock()
	defer cfg.mutex.Unlock()
	if len(h.DeletePolicies) == 0 {
		h.DeletePolicies = []release.HookDeletePolicy{release.HookBeforeHookCreation}
	}
}

func (cfg *Configuration) outputContainerLogsForListOptions(namespace string, listOptions metav1.ListOptions) error {
	podList, err := cfg.KubeClient.GetPodList(namespace, listOptions)
	if err != nil {
		return err
	}

	return cfg.KubeClient.OutputContainerLogsForPodList(podList, namespace, cfg.HookOutputFunc)
}

// deleteHookByPolicyGeneric deletes a hook using the HookAccessor interface.
func (cfg *Configuration) deleteHookByPolicyGeneric(h ri.HookAccessor, policy string, waitStrategy kube.WaitStrategy, timeout time.Duration) error {
	// Never delete CustomResourceDefinitions; this could cause lots of
	// cascading garbage collection.
	if h.Kind() == "CustomResourceDefinition" {
		return nil
	}
	if h.HasDeletePolicy(policy) {
		resources, err := cfg.KubeClient.Build(bytes.NewBufferString(h.Manifest()), false)
		if err != nil {
			return fmt.Errorf("unable to build kubernetes object for deleting hook %s: %w", h.Path(), err)
		}
		_, errs := cfg.KubeClient.Delete(resources, metav1.DeletePropagationBackground)
		if len(errs) > 0 {
			return joinErrors(errs, "; ")
		}

		waiter, err := cfg.KubeClient.GetWaiter(waitStrategy)
		if err != nil {
			return err
		}
		if err := waiter.WaitForDelete(resources, timeout); err != nil {
			return err
		}
	}
	return nil
}

// outputLogsByPolicyGeneric outputs a pod's logs using the HookAccessor interface.
func (cfg *Configuration) outputLogsByPolicyGeneric(h ri.HookAccessor, releaseNamespace string, policy string) error {
	if !h.HasOutputLogPolicy(policy) {
		return nil
	}
	namespace, err := cfg.deriveNamespaceGeneric(h, releaseNamespace)
	if err != nil {
		return err
	}
	switch h.Kind() {
	case "Job":
		return cfg.outputContainerLogsForListOptions(namespace, metav1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", h.Name())})
	case "Pod":
		return cfg.outputContainerLogsForListOptions(namespace, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", h.Name())})
	default:
		return nil
	}
}

// deriveNamespaceGeneric extracts namespace from hook manifest using the HookAccessor interface.
func (cfg *Configuration) deriveNamespaceGeneric(h ri.HookAccessor, namespace string) (string, error) {
	tmp := struct {
		Metadata struct {
			Namespace string
		}
	}{}
	err := yaml.Unmarshal([]byte(h.Manifest()), &tmp)
	if err != nil {
		return "", fmt.Errorf("unable to parse metadata.namespace from kubernetes manifest for output logs hook %s: %w", h.Path(), err)
	}
	if tmp.Metadata.Namespace == "" {
		return namespace, nil
	}
	return tmp.Metadata.Namespace, nil
}

// V3 versions of hook functions

// execHookV3 executes all of the hooks for the given hook event on a v3 release.
func (cfg *Configuration) execHookV3(rl *v2release.Release, hook v2release.HookEvent, waitStrategy kube.WaitStrategy, timeout time.Duration, serverSideApply bool) error {
	shutdown, err := cfg.execHookWithDelayedShutdownV3(rl, hook, waitStrategy, timeout, serverSideApply)
	if shutdown == nil {
		return err
	}
	if err != nil {
		if err := shutdown(); err != nil {
			return err
		}
		return err
	}
	return shutdown()
}

// execHookWithDelayedShutdownV3 executes all of the hooks for the given hook event for v3 charts.
// It uses the shared execHookCore function to avoid code duplication.
func (cfg *Configuration) execHookWithDelayedShutdownV3(rl *v2release.Release, hook v2release.HookEvent, waitStrategy kube.WaitStrategy, timeout time.Duration, serverSideApply bool) (ExecuteShutdownFunc, error) {
	// Build list of hooks matching this event as accessors
	var executingHooks []ri.HookAccessor
	for _, h := range rl.Hooks {
		for _, e := range h.Events {
			if e == hook {
				// NewHookAccessor cannot fail for known hook types (*v2release.Hook)
				acc, _ := ri.NewHookAccessor(h)
				executingHooks = append(executingHooks, acc)
			}
		}
	}

	callbacks := hookExecutionCallbacks{
		recordRelease: func() { cfg.Releases.Update(rl) },
		deleteByPolicy: func(h ri.HookAccessor, policy string) error {
			return cfg.deleteHookByPolicyGeneric(h, policy, waitStrategy, timeout)
		},
		outputLogsByPolicy: func(h ri.HookAccessor, policy string) error {
			return cfg.outputLogsByPolicyGeneric(h, rl.Namespace, policy)
		},
	}

	return cfg.execHookCore(executingHooks, string(hook), rl.Namespace, waitStrategy, timeout, serverSideApply, callbacks)
}

// recordReleaseV3 records a v2 release with an update operation.
func (cfg *Configuration) recordReleaseV3(rl *v2release.Release) error {
	return cfg.Releases.Update(rl)
}
