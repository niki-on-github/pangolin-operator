/*
Copyright (C) 2025 github.com/bovf

This program is free software: it can be redistributed and/or modified under the terms of the GNU Affero General Public License as published by the Free Software Foundation, either version 3 of the License, or (at the option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more details.

A copy of the GNU Affero General Public License should be included with this program. If not, see https://www.gnu.org/licenses/.

Third‑party code bundled in this repository may be licensed under different terms (for example, Apache‑2.0 for Kubernetes libraries). Such components retain their original licenses; see the corresponding LICENSE/NOTICE files in their source directories.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tunnelv1alpha1 "github.com/bovf/pangolin-operator/api/v1alpha1"
)

var _ = Describe("PangolinResource Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		pangolinresource := &tunnelv1alpha1.PangolinResource{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind PangolinResource")
			err := k8sClient.Get(ctx, typeNamespacedName, pangolinresource)
			if err != nil && errors.IsNotFound(err) {
				resource := &tunnelv1alpha1.PangolinResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &tunnelv1alpha1.PangolinResource{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PangolinResource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &PangolinResourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
