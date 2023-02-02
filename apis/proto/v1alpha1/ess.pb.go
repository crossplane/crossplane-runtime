//
//Copyright 2023 The Crossplane Authors.
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1-devel
// 	protoc        (unknown)
// source: proto/v1alpha1/ess.proto

package ess

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ConfigReference struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ApiVersion string `protobuf:"bytes,1,opt,name=api_version,json=apiVersion,proto3" json:"api_version,omitempty"`
	Kind       string `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind,omitempty"`
	Name       string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *ConfigReference) Reset() {
	*x = ConfigReference{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ConfigReference) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConfigReference) ProtoMessage() {}

func (x *ConfigReference) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConfigReference.ProtoReflect.Descriptor instead.
func (*ConfigReference) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{0}
}

func (x *ConfigReference) GetApiVersion() string {
	if x != nil {
		return x.ApiVersion
	}
	return ""
}

func (x *ConfigReference) GetKind() string {
	if x != nil {
		return x.Kind
	}
	return ""
}

func (x *ConfigReference) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type Secret struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ScopedName string            `protobuf:"bytes,1,opt,name=scoped_name,json=scopedName,proto3" json:"scoped_name,omitempty"`
	Metadata   map[string]string `protobuf:"bytes,2,rep,name=metadata,proto3" json:"metadata,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Data       map[string][]byte `protobuf:"bytes,3,rep,name=data,proto3" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *Secret) Reset() {
	*x = Secret{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Secret) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Secret) ProtoMessage() {}

func (x *Secret) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Secret.ProtoReflect.Descriptor instead.
func (*Secret) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{1}
}

func (x *Secret) GetScopedName() string {
	if x != nil {
		return x.ScopedName
	}
	return ""
}

func (x *Secret) GetMetadata() map[string]string {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *Secret) GetData() map[string][]byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type GetSecretRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Config  *ConfigReference `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
	Secret  *Secret          `protobuf:"bytes,2,opt,name=secret,proto3" json:"secret,omitempty"`
	Options *GetOptions      `protobuf:"bytes,3,opt,name=options,proto3" json:"options,omitempty"`
}

func (x *GetSecretRequest) Reset() {
	*x = GetSecretRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSecretRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSecretRequest) ProtoMessage() {}

func (x *GetSecretRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSecretRequest.ProtoReflect.Descriptor instead.
func (*GetSecretRequest) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{2}
}

func (x *GetSecretRequest) GetConfig() *ConfigReference {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *GetSecretRequest) GetSecret() *Secret {
	if x != nil {
		return x.Secret
	}
	return nil
}

func (x *GetSecretRequest) GetOptions() *GetOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

type GetSecretResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Secret *Secret `protobuf:"bytes,1,opt,name=secret,proto3" json:"secret,omitempty"`
}

func (x *GetSecretResponse) Reset() {
	*x = GetSecretResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSecretResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSecretResponse) ProtoMessage() {}

func (x *GetSecretResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSecretResponse.ProtoReflect.Descriptor instead.
func (*GetSecretResponse) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{3}
}

func (x *GetSecretResponse) GetSecret() *Secret {
	if x != nil {
		return x.Secret
	}
	return nil
}

type ApplySecretRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Config  *ConfigReference `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
	Secret  *Secret          `protobuf:"bytes,2,opt,name=secret,proto3" json:"secret,omitempty"`
	Options *ApplyOptions    `protobuf:"bytes,3,opt,name=options,proto3" json:"options,omitempty"`
}

func (x *ApplySecretRequest) Reset() {
	*x = ApplySecretRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplySecretRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplySecretRequest) ProtoMessage() {}

func (x *ApplySecretRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplySecretRequest.ProtoReflect.Descriptor instead.
func (*ApplySecretRequest) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{4}
}

func (x *ApplySecretRequest) GetConfig() *ConfigReference {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *ApplySecretRequest) GetSecret() *Secret {
	if x != nil {
		return x.Secret
	}
	return nil
}

func (x *ApplySecretRequest) GetOptions() *ApplyOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

type ApplySecretResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Changed bool `protobuf:"varint,1,opt,name=changed,proto3" json:"changed,omitempty"`
}

func (x *ApplySecretResponse) Reset() {
	*x = ApplySecretResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplySecretResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplySecretResponse) ProtoMessage() {}

func (x *ApplySecretResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplySecretResponse.ProtoReflect.Descriptor instead.
func (*ApplySecretResponse) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{5}
}

func (x *ApplySecretResponse) GetChanged() bool {
	if x != nil {
		return x.Changed
	}
	return false
}

type DeleteKeysRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Config  *ConfigReference `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
	Secret  *Secret          `protobuf:"bytes,2,opt,name=secret,proto3" json:"secret,omitempty"`
	Options *DeleteOptions   `protobuf:"bytes,3,opt,name=options,proto3" json:"options,omitempty"`
}

func (x *DeleteKeysRequest) Reset() {
	*x = DeleteKeysRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteKeysRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteKeysRequest) ProtoMessage() {}

func (x *DeleteKeysRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteKeysRequest.ProtoReflect.Descriptor instead.
func (*DeleteKeysRequest) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{6}
}

func (x *DeleteKeysRequest) GetConfig() *ConfigReference {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *DeleteKeysRequest) GetSecret() *Secret {
	if x != nil {
		return x.Secret
	}
	return nil
}

func (x *DeleteKeysRequest) GetOptions() *DeleteOptions {
	if x != nil {
		return x.Options
	}
	return nil
}

type DeleteKeysResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DeleteKeysResponse) Reset() {
	*x = DeleteKeysResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteKeysResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteKeysResponse) ProtoMessage() {}

func (x *DeleteKeysResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteKeysResponse.ProtoReflect.Descriptor instead.
func (*DeleteKeysResponse) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{7}
}

type GetOptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetOptions) Reset() {
	*x = GetOptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetOptions) ProtoMessage() {}

func (x *GetOptions) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetOptions.ProtoReflect.Descriptor instead.
func (*GetOptions) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{8}
}

type ApplyOptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ApplyOptions) Reset() {
	*x = ApplyOptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplyOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyOptions) ProtoMessage() {}

func (x *ApplyOptions) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyOptions.ProtoReflect.Descriptor instead.
func (*ApplyOptions) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{9}
}

type DeleteOptions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KeepEmptySecret bool `protobuf:"varint,1,opt,name=keep_empty_secret,json=keepEmptySecret,proto3" json:"keep_empty_secret,omitempty"`
}

func (x *DeleteOptions) Reset() {
	*x = DeleteOptions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_v1alpha1_ess_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteOptions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteOptions) ProtoMessage() {}

func (x *DeleteOptions) ProtoReflect() protoreflect.Message {
	mi := &file_proto_v1alpha1_ess_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteOptions.ProtoReflect.Descriptor instead.
func (*DeleteOptions) Descriptor() ([]byte, []int) {
	return file_proto_v1alpha1_ess_proto_rawDescGZIP(), []int{10}
}

func (x *DeleteOptions) GetKeepEmptySecret() bool {
	if x != nil {
		return x.KeepEmptySecret
	}
	return false
}

var File_proto_v1alpha1_ess_proto protoreflect.FileDescriptor

var file_proto_v1alpha1_ess_proto_rawDesc = []byte{
	0x0a, 0x18, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x2f, 0x65, 0x73, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5a, 0x0a, 0x0f, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x1f, 0x0a,
	0x0b, 0x61, 0x70, 0x69, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0a, 0x61, 0x70, 0x69, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x12,
	0x0a, 0x04, 0x6b, 0x69, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6b, 0x69,
	0x6e, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0xf9, 0x01, 0x0a, 0x06, 0x53, 0x65, 0x63, 0x72, 0x65,
	0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x73, 0x63, 0x6f, 0x70, 0x65, 0x64, 0x5f, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73, 0x63, 0x6f, 0x70, 0x65, 0x64, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x31, 0x0a, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x2e, 0x4d, 0x65,
	0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x6d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x25, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x2e, 0x44, 0x61, 0x74,
	0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x1a, 0x3b, 0x0a, 0x0d,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a,
	0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12,
	0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x1a, 0x37, 0x0a, 0x09, 0x44, 0x61, 0x74,
	0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02,
	0x38, 0x01, 0x22, 0x84, 0x01, 0x0a, 0x10, 0x47, 0x65, 0x74, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x28, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x12, 0x1f, 0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x07, 0x2e, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x06, 0x73, 0x65, 0x63, 0x72,
	0x65, 0x74, 0x12, 0x25, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x47, 0x65, 0x74, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x52, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x34, 0x0a, 0x11, 0x47, 0x65, 0x74,
	0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1f,
	0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07,
	0x2e, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x22,
	0x88, 0x01, 0x0a, 0x12, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x28, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52,
	0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x12, 0x1f, 0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x07, 0x2e, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65,
	0x74, 0x12, 0x27, 0x0a, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x52, 0x07, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x2f, 0x0a, 0x13, 0x41, 0x70,
	0x70, 0x6c, 0x79, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x07, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x64, 0x22, 0x88, 0x01, 0x0a, 0x11,
	0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4b, 0x65, 0x79, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x28, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x10, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65,
	0x6e, 0x63, 0x65, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x1f, 0x0a, 0x06, 0x73,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x53, 0x65,
	0x63, 0x72, 0x65, 0x74, 0x52, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x28, 0x0a, 0x07,
	0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e,
	0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x07, 0x6f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x14, 0x0a, 0x12, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x4b, 0x65, 0x79, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x0c, 0x0a, 0x0a,
	0x47, 0x65, 0x74, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x0e, 0x0a, 0x0c, 0x41, 0x70,
	0x70, 0x6c, 0x79, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x3b, 0x0a, 0x0d, 0x44, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x2a, 0x0a, 0x11, 0x6b,
	0x65, 0x65, 0x70, 0x5f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x5f, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x6b, 0x65, 0x65, 0x70, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x32, 0xc7, 0x01, 0x0a, 0x1a, 0x45, 0x78, 0x74, 0x65,
	0x72, 0x6e, 0x61, 0x6c, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x53,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x34, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x53, 0x65, 0x63,
	0x72, 0x65, 0x74, 0x12, 0x11, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x65, 0x63, 0x72,
	0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3a, 0x0a, 0x0b,
	0x41, 0x70, 0x70, 0x6c, 0x79, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x13, 0x2e, 0x41, 0x70,
	0x70, 0x6c, 0x79, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x14, 0x2e, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x37, 0x0a, 0x0a, 0x44, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x4b, 0x65, 0x79, 0x73, 0x12, 0x12, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4b,
	0x65, 0x79, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x13, 0x2e, 0x44, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x4b, 0x65, 0x79, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x42, 0x16, 0x5a, 0x14, 0x65, 0x73, 0x73, 0x2d, 0x67, 0x72, 0x70, 0x63, 0x2d, 0x32, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x73, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_proto_v1alpha1_ess_proto_rawDescOnce sync.Once
	file_proto_v1alpha1_ess_proto_rawDescData = file_proto_v1alpha1_ess_proto_rawDesc
)

func file_proto_v1alpha1_ess_proto_rawDescGZIP() []byte {
	file_proto_v1alpha1_ess_proto_rawDescOnce.Do(func() {
		file_proto_v1alpha1_ess_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_v1alpha1_ess_proto_rawDescData)
	})
	return file_proto_v1alpha1_ess_proto_rawDescData
}

var file_proto_v1alpha1_ess_proto_msgTypes = make([]protoimpl.MessageInfo, 13)
var file_proto_v1alpha1_ess_proto_goTypes = []interface{}{
	(*ConfigReference)(nil),     // 0: ConfigReference
	(*Secret)(nil),              // 1: Secret
	(*GetSecretRequest)(nil),    // 2: GetSecretRequest
	(*GetSecretResponse)(nil),   // 3: GetSecretResponse
	(*ApplySecretRequest)(nil),  // 4: ApplySecretRequest
	(*ApplySecretResponse)(nil), // 5: ApplySecretResponse
	(*DeleteKeysRequest)(nil),   // 6: DeleteKeysRequest
	(*DeleteKeysResponse)(nil),  // 7: DeleteKeysResponse
	(*GetOptions)(nil),          // 8: GetOptions
	(*ApplyOptions)(nil),        // 9: ApplyOptions
	(*DeleteOptions)(nil),       // 10: DeleteOptions
	nil,                         // 11: Secret.MetadataEntry
	nil,                         // 12: Secret.DataEntry
}
var file_proto_v1alpha1_ess_proto_depIdxs = []int32{
	11, // 0: Secret.metadata:type_name -> Secret.MetadataEntry
	12, // 1: Secret.data:type_name -> Secret.DataEntry
	0,  // 2: GetSecretRequest.config:type_name -> ConfigReference
	1,  // 3: GetSecretRequest.secret:type_name -> Secret
	8,  // 4: GetSecretRequest.options:type_name -> GetOptions
	1,  // 5: GetSecretResponse.secret:type_name -> Secret
	0,  // 6: ApplySecretRequest.config:type_name -> ConfigReference
	1,  // 7: ApplySecretRequest.secret:type_name -> Secret
	9,  // 8: ApplySecretRequest.options:type_name -> ApplyOptions
	0,  // 9: DeleteKeysRequest.config:type_name -> ConfigReference
	1,  // 10: DeleteKeysRequest.secret:type_name -> Secret
	10, // 11: DeleteKeysRequest.options:type_name -> DeleteOptions
	2,  // 12: ExternalSecretStoreService.GetSecret:input_type -> GetSecretRequest
	4,  // 13: ExternalSecretStoreService.ApplySecret:input_type -> ApplySecretRequest
	6,  // 14: ExternalSecretStoreService.DeleteKeys:input_type -> DeleteKeysRequest
	3,  // 15: ExternalSecretStoreService.GetSecret:output_type -> GetSecretResponse
	5,  // 16: ExternalSecretStoreService.ApplySecret:output_type -> ApplySecretResponse
	7,  // 17: ExternalSecretStoreService.DeleteKeys:output_type -> DeleteKeysResponse
	15, // [15:18] is the sub-list for method output_type
	12, // [12:15] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_proto_v1alpha1_ess_proto_init() }
func file_proto_v1alpha1_ess_proto_init() {
	if File_proto_v1alpha1_ess_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_v1alpha1_ess_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ConfigReference); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Secret); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSecretRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSecretResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplySecretRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplySecretResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteKeysRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteKeysResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetOptions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplyOptions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_v1alpha1_ess_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteOptions); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_v1alpha1_ess_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   13,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_v1alpha1_ess_proto_goTypes,
		DependencyIndexes: file_proto_v1alpha1_ess_proto_depIdxs,
		MessageInfos:      file_proto_v1alpha1_ess_proto_msgTypes,
	}.Build()
	File_proto_v1alpha1_ess_proto = out.File
	file_proto_v1alpha1_ess_proto_rawDesc = nil
	file_proto_v1alpha1_ess_proto_goTypes = nil
	file_proto_v1alpha1_ess_proto_depIdxs = nil
}
