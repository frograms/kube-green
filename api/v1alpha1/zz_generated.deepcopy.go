//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2021.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExcludeRef) DeepCopyInto(out *ExcludeRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExcludeRef.
func (in *ExcludeRef) DeepCopy() *ExcludeRef {
	if in == nil {
		return nil
	}
	out := new(ExcludeRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SleepInfo) DeepCopyInto(out *SleepInfo) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SleepInfo.
func (in *SleepInfo) DeepCopy() *SleepInfo {
	if in == nil {
		return nil
	}
	out := new(SleepInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SleepInfo) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SleepInfoList) DeepCopyInto(out *SleepInfoList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SleepInfo, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SleepInfoList.
func (in *SleepInfoList) DeepCopy() *SleepInfoList {
	if in == nil {
		return nil
	}
	out := new(SleepInfoList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SleepInfoList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SleepInfoSpec) DeepCopyInto(out *SleepInfoSpec) {
	*out = *in
	if in.ExcludeRef != nil {
		in, out := &in.ExcludeRef, &out.ExcludeRef
		*out = make([]ExcludeRef, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SleepInfoSpec.
func (in *SleepInfoSpec) DeepCopy() *SleepInfoSpec {
	if in == nil {
		return nil
	}
	out := new(SleepInfoSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SleepInfoStatus) DeepCopyInto(out *SleepInfoStatus) {
	*out = *in
	in.LastScheduleTime.DeepCopyInto(&out.LastScheduleTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SleepInfoStatus.
func (in *SleepInfoStatus) DeepCopy() *SleepInfoStatus {
	if in == nil {
		return nil
	}
	out := new(SleepInfoStatus)
	in.DeepCopyInto(out)
	return out
}
