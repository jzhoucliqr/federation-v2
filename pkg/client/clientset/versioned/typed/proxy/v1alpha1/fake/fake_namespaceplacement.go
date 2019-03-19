/*
Copyright 2018 The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/proxy/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNamespacePlacements implements NamespacePlacementInterface
type FakeNamespacePlacements struct {
	Fake *FakeProxyV1alpha1
	ns   string
}

var namespaceplacementsResource = schema.GroupVersionResource{Group: "proxy.federation.k8s.io", Version: "v1alpha1", Resource: "namespaceplacements"}

var namespaceplacementsKind = schema.GroupVersionKind{Group: "proxy.federation.k8s.io", Version: "v1alpha1", Kind: "NamespacePlacement"}

// Get takes name of the namespacePlacement, and returns the corresponding namespacePlacement object, and an error if there is any.
func (c *FakeNamespacePlacements) Get(name string, options v1.GetOptions) (result *v1alpha1.NamespacePlacement, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(namespaceplacementsResource, c.ns, name), &v1alpha1.NamespacePlacement{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NamespacePlacement), err
}

// List takes label and field selectors, and returns the list of NamespacePlacements that match those selectors.
func (c *FakeNamespacePlacements) List(opts v1.ListOptions) (result *v1alpha1.NamespacePlacementList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(namespaceplacementsResource, namespaceplacementsKind, c.ns, opts), &v1alpha1.NamespacePlacementList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.NamespacePlacementList{ListMeta: obj.(*v1alpha1.NamespacePlacementList).ListMeta}
	for _, item := range obj.(*v1alpha1.NamespacePlacementList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested namespacePlacements.
func (c *FakeNamespacePlacements) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(namespaceplacementsResource, c.ns, opts))

}

// Create takes the representation of a namespacePlacement and creates it.  Returns the server's representation of the namespacePlacement, and an error, if there is any.
func (c *FakeNamespacePlacements) Create(namespacePlacement *v1alpha1.NamespacePlacement) (result *v1alpha1.NamespacePlacement, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(namespaceplacementsResource, c.ns, namespacePlacement), &v1alpha1.NamespacePlacement{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NamespacePlacement), err
}

// Update takes the representation of a namespacePlacement and updates it. Returns the server's representation of the namespacePlacement, and an error, if there is any.
func (c *FakeNamespacePlacements) Update(namespacePlacement *v1alpha1.NamespacePlacement) (result *v1alpha1.NamespacePlacement, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(namespaceplacementsResource, c.ns, namespacePlacement), &v1alpha1.NamespacePlacement{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NamespacePlacement), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNamespacePlacements) UpdateStatus(namespacePlacement *v1alpha1.NamespacePlacement) (*v1alpha1.NamespacePlacement, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(namespaceplacementsResource, "status", c.ns, namespacePlacement), &v1alpha1.NamespacePlacement{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NamespacePlacement), err
}

// Delete takes name of the namespacePlacement and deletes it. Returns an error if one occurs.
func (c *FakeNamespacePlacements) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(namespaceplacementsResource, c.ns, name), &v1alpha1.NamespacePlacement{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNamespacePlacements) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(namespaceplacementsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.NamespacePlacementList{})
	return err
}

// Patch applies the patch and returns the patched namespacePlacement.
func (c *FakeNamespacePlacements) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.NamespacePlacement, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(namespaceplacementsResource, c.ns, name, data, subresources...), &v1alpha1.NamespacePlacement{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NamespacePlacement), err
}
