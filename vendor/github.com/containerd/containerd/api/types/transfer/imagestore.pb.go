//
//Copyright The containerd Authors.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.20.1
// source: github.com/containerd/containerd/api/types/transfer/imagestore.proto

package transfer

import (
	types "github.com/containerd/containerd/api/types"
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

type ImageStore struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name          string            `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Labels        map[string]string `protobuf:"bytes,2,rep,name=labels,proto3" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Platforms     []*types.Platform `protobuf:"bytes,3,rep,name=platforms,proto3" json:"platforms,omitempty"`
	AllMetadata   bool              `protobuf:"varint,4,opt,name=all_metadata,json=allMetadata,proto3" json:"all_metadata,omitempty"`
	ManifestLimit uint32            `protobuf:"varint,5,opt,name=manifest_limit,json=manifestLimit,proto3" json:"manifest_limit,omitempty"`
	// prefix is the intended image name prefix for imported images
	Prefix string `protobuf:"bytes,6,opt,name=prefix,proto3" json:"prefix,omitempty"`
	// check_prefix only stores images with the prefix
	CheckPrefix bool `protobuf:"varint,7,opt,name=check_prefix,json=checkPrefix,proto3" json:"check_prefix,omitempty"`
	// digest_refs adds digest references for images using prefix
	DigestRefs bool `protobuf:"varint,8,opt,name=digest_refs,json=digestRefs,proto3" json:"digest_refs,omitempty"`
	// always_digest includes a digest image even when a non-digest image is stored
	AlwaysDigest bool                   `protobuf:"varint,9,opt,name=always_digest,json=alwaysDigest,proto3" json:"always_digest,omitempty"`
	Unpacks      []*UnpackConfiguration `protobuf:"bytes,10,rep,name=unpacks,proto3" json:"unpacks,omitempty"`
}

func (x *ImageStore) Reset() {
	*x = ImageStore{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ImageStore) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ImageStore) ProtoMessage() {}

func (x *ImageStore) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ImageStore.ProtoReflect.Descriptor instead.
func (*ImageStore) Descriptor() ([]byte, []int) {
	return file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescGZIP(), []int{0}
}

func (x *ImageStore) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ImageStore) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

func (x *ImageStore) GetPlatforms() []*types.Platform {
	if x != nil {
		return x.Platforms
	}
	return nil
}

func (x *ImageStore) GetAllMetadata() bool {
	if x != nil {
		return x.AllMetadata
	}
	return false
}

func (x *ImageStore) GetManifestLimit() uint32 {
	if x != nil {
		return x.ManifestLimit
	}
	return 0
}

func (x *ImageStore) GetPrefix() string {
	if x != nil {
		return x.Prefix
	}
	return ""
}

func (x *ImageStore) GetCheckPrefix() bool {
	if x != nil {
		return x.CheckPrefix
	}
	return false
}

func (x *ImageStore) GetDigestRefs() bool {
	if x != nil {
		return x.DigestRefs
	}
	return false
}

func (x *ImageStore) GetAlwaysDigest() bool {
	if x != nil {
		return x.AlwaysDigest
	}
	return false
}

func (x *ImageStore) GetUnpacks() []*UnpackConfiguration {
	if x != nil {
		return x.Unpacks
	}
	return nil
}

type UnpackConfiguration struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// platform is the platform to unpack for, used for resolving manifest and snapshotter
	// if not provided
	Platform *types.Platform `protobuf:"bytes,1,opt,name=platform,proto3" json:"platform,omitempty"`
	// snapshotter to unpack to, if not provided default for platform shoudl be used
	Snapshotter string `protobuf:"bytes,2,opt,name=snapshotter,proto3" json:"snapshotter,omitempty"`
}

func (x *UnpackConfiguration) Reset() {
	*x = UnpackConfiguration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnpackConfiguration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnpackConfiguration) ProtoMessage() {}

func (x *UnpackConfiguration) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnpackConfiguration.ProtoReflect.Descriptor instead.
func (*UnpackConfiguration) Descriptor() ([]byte, []int) {
	return file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescGZIP(), []int{1}
}

func (x *UnpackConfiguration) GetPlatform() *types.Platform {
	if x != nil {
		return x.Platform
	}
	return nil
}

func (x *UnpackConfiguration) GetSnapshotter() string {
	if x != nil {
		return x.Snapshotter
	}
	return ""
}

var File_github_com_containerd_containerd_api_types_transfer_imagestore_proto protoreflect.FileDescriptor

var file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDesc = []byte{
	0x0a, 0x44, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x6e,
	0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65,
	0x72, 0x64, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f, 0x74, 0x72, 0x61,
	0x6e, 0x73, 0x66, 0x65, 0x72, 0x2f, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x73, 0x74, 0x6f, 0x72, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x19, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65,
	0x72, 0x64, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65,
	0x72, 0x1a, 0x39, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f,
	0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e,
	0x65, 0x72, 0x64, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f, 0x70, 0x6c,
	0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xf5, 0x03, 0x0a,
	0x0a, 0x49, 0x6d, 0x61, 0x67, 0x65, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12,
	0x49, 0x0a, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x31, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x74, 0x79, 0x70,
	0x65, 0x73, 0x2e, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x2e, 0x49, 0x6d, 0x61, 0x67,
	0x65, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x2e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x38, 0x0a, 0x09, 0x70, 0x6c,
	0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x2e, 0x50, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x52, 0x09, 0x70, 0x6c, 0x61, 0x74, 0x66,
	0x6f, 0x72, 0x6d, 0x73, 0x12, 0x21, 0x0a, 0x0c, 0x61, 0x6c, 0x6c, 0x5f, 0x6d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0b, 0x61, 0x6c, 0x6c, 0x4d,
	0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x25, 0x0a, 0x0e, 0x6d, 0x61, 0x6e, 0x69, 0x66,
	0x65, 0x73, 0x74, 0x5f, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x0d, 0x6d, 0x61, 0x6e, 0x69, 0x66, 0x65, 0x73, 0x74, 0x4c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x16,
	0x0a, 0x06, 0x70, 0x72, 0x65, 0x66, 0x69, 0x78, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x70, 0x72, 0x65, 0x66, 0x69, 0x78, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x68, 0x65, 0x63, 0x6b, 0x5f,
	0x70, 0x72, 0x65, 0x66, 0x69, 0x78, 0x18, 0x07, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0b, 0x63, 0x68,
	0x65, 0x63, 0x6b, 0x50, 0x72, 0x65, 0x66, 0x69, 0x78, 0x12, 0x1f, 0x0a, 0x0b, 0x64, 0x69, 0x67,
	0x65, 0x73, 0x74, 0x5f, 0x72, 0x65, 0x66, 0x73, 0x18, 0x08, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a,
	0x64, 0x69, 0x67, 0x65, 0x73, 0x74, 0x52, 0x65, 0x66, 0x73, 0x12, 0x23, 0x0a, 0x0d, 0x61, 0x6c,
	0x77, 0x61, 0x79, 0x73, 0x5f, 0x64, 0x69, 0x67, 0x65, 0x73, 0x74, 0x18, 0x09, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x0c, 0x61, 0x6c, 0x77, 0x61, 0x79, 0x73, 0x44, 0x69, 0x67, 0x65, 0x73, 0x74, 0x12,
	0x48, 0x0a, 0x07, 0x75, 0x6e, 0x70, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x0a, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x2e, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x74, 0x79,
	0x70, 0x65, 0x73, 0x2e, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x2e, 0x55, 0x6e, 0x70,
	0x61, 0x63, 0x6b, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x07, 0x75, 0x6e, 0x70, 0x61, 0x63, 0x6b, 0x73, 0x1a, 0x39, 0x0a, 0x0b, 0x4c, 0x61, 0x62,
	0x65, 0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x3a, 0x02, 0x38, 0x01, 0x22, 0x6f, 0x0a, 0x13, 0x55, 0x6e, 0x70, 0x61, 0x63, 0x6b, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x36, 0x0a, 0x08, 0x70,
	0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x2e, 0x50, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x74, 0x66,
	0x6f, 0x72, 0x6d, 0x12, 0x20, 0x0a, 0x0b, 0x73, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x74,
	0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x6e, 0x61, 0x70, 0x73, 0x68,
	0x6f, 0x74, 0x74, 0x65, 0x72, 0x42, 0x35, 0x5a, 0x33, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2f, 0x63,
	0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79,
	0x70, 0x65, 0x73, 0x2f, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescOnce sync.Once
	file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescData = file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDesc
)

func file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescGZIP() []byte {
	file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescOnce.Do(func() {
		file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescData = protoimpl.X.CompressGZIP(file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescData)
	})
	return file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDescData
}

var file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_goTypes = []interface{}{
	(*ImageStore)(nil),          // 0: containerd.types.transfer.ImageStore
	(*UnpackConfiguration)(nil), // 1: containerd.types.transfer.UnpackConfiguration
	nil,                         // 2: containerd.types.transfer.ImageStore.LabelsEntry
	(*types.Platform)(nil),      // 3: containerd.types.Platform
}
var file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_depIdxs = []int32{
	2, // 0: containerd.types.transfer.ImageStore.labels:type_name -> containerd.types.transfer.ImageStore.LabelsEntry
	3, // 1: containerd.types.transfer.ImageStore.platforms:type_name -> containerd.types.Platform
	1, // 2: containerd.types.transfer.ImageStore.unpacks:type_name -> containerd.types.transfer.UnpackConfiguration
	3, // 3: containerd.types.transfer.UnpackConfiguration.platform:type_name -> containerd.types.Platform
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_init() }
func file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_init() {
	if File_github_com_containerd_containerd_api_types_transfer_imagestore_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ImageStore); i {
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
		file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnpackConfiguration); i {
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
			RawDescriptor: file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_goTypes,
		DependencyIndexes: file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_depIdxs,
		MessageInfos:      file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_msgTypes,
	}.Build()
	File_github_com_containerd_containerd_api_types_transfer_imagestore_proto = out.File
	file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_rawDesc = nil
	file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_goTypes = nil
	file_github_com_containerd_containerd_api_types_transfer_imagestore_proto_depIdxs = nil
}
