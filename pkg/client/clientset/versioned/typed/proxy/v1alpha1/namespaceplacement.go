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

package v1alpha1

import (
	v1alpha1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/proxy/v1alpha1"
	scheme "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// NamespacePlacementsGetter has a method to return a NamespacePlacementInterface.
// A group's client should implement this interface.
type NamespacePlacementsGetter interface {
	NamespacePlacements(namespace string) NamespacePlacementInterface
}

// NamespacePlacementInterface has methods to work with NamespacePlacement resources.
type NamespacePlacementInterface interface {
	Create(*v1alpha1.NamespacePlacement) (*v1alpha1.NamespacePlacement, error)
	Update(*v1alpha1.NamespacePlacement) (*v1alpha1.NamespacePlacement, error)
	UpdateStatus(*v1alpha1.NamespacePlacement) (*v1alpha1.NamespacePlacement, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.NamespacePlacement, error)
	List(opts v1.ListOptions) (*v1alpha1.NamespacePlacementList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.NamespacePlacement, err error)
	NamespacePlacementExpansion
}

// namespacePlacements implements NamespacePlacementInterface
type namespacePlacements struct {
	client rest.Interface
	ns     string
}

// newNamespacePlacements returns a NamespacePlacements
func newNamespacePlacements(c *ProxyV1alpha1Client, namespace string) *namespacePlacements {
	return &namespacePlacements{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the namespacePlacement, and returns the corresponding namespacePlacement object, and an error if there is any.
func (c *namespacePlacements) Get(name string, options v1.GetOptions) (result *v1alpha1.NamespacePlacement, err error) {
	result = &v1alpha1.NamespacePlacement{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("namespaceplacements").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of NamespacePlacements that match those selectors.
func (c *namespacePlacements) List(opts v1.ListOptions) (result *v1alpha1.NamespacePlacementList, err error) {
	result = &v1alpha1.NamespacePlacementList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("namespaceplacements").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested namespacePlacements.
func (c *namespacePlacements) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("namespaceplacements").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a namespacePlacement and creates it.  Returns the server's representation of the namespacePlacement, and an error, if there is any.
func (c *namespacePlacements) Create(namespacePlacement *v1alpha1.NamespacePlacement) (result *v1alpha1.NamespacePlacement, err error) {
	result = &v1alpha1.NamespacePlacement{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("namespaceplacements").
		Body(namespacePlacement).
		Do().
		Into(result)
	return
}

// Update takes the representation of a namespacePlacement and updates it. Returns the server's representation of the namespacePlacement, and an error, if there is any.
func (c *namespacePlacements) Update(namespacePlacement *v1alpha1.NamespacePlacement) (result *v1alpha1.NamespacePlacement, err error) {
	result = &v1alpha1.NamespacePlacement{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("namespaceplacements").
		Name(namespacePlacement.Name).
		Body(namespacePlacement).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *namespacePlacements) UpdateStatus(namespacePlacement *v1alpha1.NamespacePlacement) (result *v1alpha1.NamespacePlacement, err error) {
	result = &v1alpha1.NamespacePlacement{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("namespaceplacements").
		Name(namespacePlacement.Name).
		SubResource("status").
		Body(namespacePlacement).
		Do().
		Into(result)
	return
}

// Delete takes name of the namespacePlacement and deletes it. Returns an error if one occurs.
func (c *namespacePlacements) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("namespaceplacements").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *namespacePlacements) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("namespaceplacements").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched namespacePlacement.
func (c *namespacePlacements) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.NamespacePlacement, err error) {
	result = &v1alpha1.NamespacePlacement{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("namespaceplacements").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}