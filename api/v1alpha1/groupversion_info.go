/*
Copyright (C) 2025 github.com/bovf

This program is free software: it can be redistributed and/or modified under the terms of the GNU Affero General Public License as published by the Free Software Foundation, either version 3 of the License, or (at the option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

A copy of the GNU Affero General Public License should be included with this program. If not, see https://www.gnu.org/licenses/.

Third‑party code bundled in this repository may be licensed under different terms (for example, Apache‑2.0 for Kubernetes libraries). Such components retain their original licenses; see the corresponding LICENSE/NOTICE files in their source directories.
*/

// Package v1alpha1 contains API Schema definitions for the tunnel v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=tunnel.pangolin.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "tunnel.pangolin.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
