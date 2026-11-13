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

package chart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v3chart "helm.sh/helm/v4/internal/chart/v3"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestNewDefaultAccessorUnsupported(t *testing.T) {
	_, err := NewDefaultAccessor("not a chart")
	assert.Error(t, err)
}

func TestAccessorMetaDependencies(t *testing.T) {
	// Metadata declares two dependencies but none are loaded as subcharts,
	// as is the case before `helm dependency build`.
	tests := []struct {
		name  string
		chrt  Charter
		names []string
	}{
		{
			name: "v2",
			chrt: &v2chart.Chart{Metadata: &v2chart.Metadata{
				Name: "parent",
				Dependencies: []*v2chart.Dependency{
					{Name: "a"},
					{Name: "b"},
				},
			}},
			names: []string{"a", "b"},
		},
		{
			name: "v3",
			chrt: &v3chart.Chart{Metadata: &v3chart.Metadata{
				Name: "parent",
				Dependencies: []*v3chart.Dependency{
					{Name: "a"},
					{Name: "b"},
				},
			}},
			names: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, err := NewDefaultAccessor(tt.chrt)
			require.NoError(t, err)

			assert.Equal(t, "parent", acc.Name())
			assert.Empty(t, acc.Dependencies())

			deps := acc.MetaDependencies()
			require.Len(t, deps, len(tt.names))
			for i, dep := range deps {
				dacc, err := NewDependencyAccessor(dep)
				require.NoError(t, err)
				assert.Equal(t, tt.names[i], dacc.Name())
			}
		})
	}
}
