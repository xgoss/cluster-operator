// +build !ignore_autogenerated

/*
Copyright 2017 The Kubernetes Authors.

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

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package clusteroperator

import (
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	reflect "reflect"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
//
// Deprecated: deepcopy registration will go away when static deepcopy is fully implemented.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Cluster).DeepCopyInto(out.(*Cluster))
			return nil
		}, InType: reflect.TypeOf(&Cluster{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterList).DeepCopyInto(out.(*ClusterList))
			return nil
		}, InType: reflect.TypeOf(&ClusterList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterNodeGroup).DeepCopyInto(out.(*ClusterNodeGroup))
			return nil
		}, InType: reflect.TypeOf(&ClusterNodeGroup{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterSpec).DeepCopyInto(out.(*ClusterSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterStatus).DeepCopyInto(out.(*ClusterStatus))
			return nil
		}, InType: reflect.TypeOf(&ClusterStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Node).DeepCopyInto(out.(*Node))
			return nil
		}, InType: reflect.TypeOf(&Node{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeGroup).DeepCopyInto(out.(*NodeGroup))
			return nil
		}, InType: reflect.TypeOf(&NodeGroup{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeGroupList).DeepCopyInto(out.(*NodeGroupList))
			return nil
		}, InType: reflect.TypeOf(&NodeGroupList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeGroupSpec).DeepCopyInto(out.(*NodeGroupSpec))
			return nil
		}, InType: reflect.TypeOf(&NodeGroupSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeGroupStatus).DeepCopyInto(out.(*NodeGroupStatus))
			return nil
		}, InType: reflect.TypeOf(&NodeGroupStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeList).DeepCopyInto(out.(*NodeList))
			return nil
		}, InType: reflect.TypeOf(&NodeList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeSpec).DeepCopyInto(out.(*NodeSpec))
			return nil
		}, InType: reflect.TypeOf(&NodeSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeStatus).DeepCopyInto(out.(*NodeStatus))
			return nil
		}, InType: reflect.TypeOf(&NodeStatus{})},
	)
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Cluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterList) DeepCopyInto(out *ClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Cluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterList.
func (in *ClusterList) DeepCopy() *ClusterList {
	if in == nil {
		return nil
	}
	out := new(ClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterNodeGroup) DeepCopyInto(out *ClusterNodeGroup) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterNodeGroup.
func (in *ClusterNodeGroup) DeepCopy() *ClusterNodeGroup {
	if in == nil {
		return nil
	}
	out := new(ClusterNodeGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	out.MasterNodes = in.MasterNodes
	if in.ComputeNodeGroups != nil {
		in, out := &in.ComputeNodeGroups, &out.ComputeNodeGroups
		*out = make([]ClusterNodeGroup, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStatus) DeepCopyInto(out *ClusterStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStatus.
func (in *ClusterStatus) DeepCopy() *ClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Node) DeepCopyInto(out *Node) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Node.
func (in *Node) DeepCopy() *Node {
	if in == nil {
		return nil
	}
	out := new(Node)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Node) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeGroup) DeepCopyInto(out *NodeGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeGroup.
func (in *NodeGroup) DeepCopy() *NodeGroup {
	if in == nil {
		return nil
	}
	out := new(NodeGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeGroup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeGroupList) DeepCopyInto(out *NodeGroupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeGroup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeGroupList.
func (in *NodeGroupList) DeepCopy() *NodeGroupList {
	if in == nil {
		return nil
	}
	out := new(NodeGroupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeGroupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeGroupSpec) DeepCopyInto(out *NodeGroupSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeGroupSpec.
func (in *NodeGroupSpec) DeepCopy() *NodeGroupSpec {
	if in == nil {
		return nil
	}
	out := new(NodeGroupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeGroupStatus) DeepCopyInto(out *NodeGroupStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeGroupStatus.
func (in *NodeGroupStatus) DeepCopy() *NodeGroupStatus {
	if in == nil {
		return nil
	}
	out := new(NodeGroupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeList) DeepCopyInto(out *NodeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Node, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeList.
func (in *NodeList) DeepCopy() *NodeList {
	if in == nil {
		return nil
	}
	out := new(NodeList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeSpec) DeepCopyInto(out *NodeSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeSpec.
func (in *NodeSpec) DeepCopy() *NodeSpec {
	if in == nil {
		return nil
	}
	out := new(NodeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeStatus) DeepCopyInto(out *NodeStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeStatus.
func (in *NodeStatus) DeepCopy() *NodeStatus {
	if in == nil {
		return nil
	}
	out := new(NodeStatus)
	in.DeepCopyInto(out)
	return out
}