// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.4
// 	protoc        v5.29.3
// source: rtpproxy/rtpproxy.proto

package rtpproxy

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SdpOffer struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RoomId        string                 `protobuf:"bytes,1,opt,name=room_id,json=roomId,proto3" json:"room_id,omitempty"`
	Sdp           string                 `protobuf:"bytes,2,opt,name=sdp,proto3" json:"sdp,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SdpOffer) Reset() {
	*x = SdpOffer{}
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SdpOffer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SdpOffer) ProtoMessage() {}

func (x *SdpOffer) ProtoReflect() protoreflect.Message {
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SdpOffer.ProtoReflect.Descriptor instead.
func (*SdpOffer) Descriptor() ([]byte, []int) {
	return file_rtpproxy_rtpproxy_proto_rawDescGZIP(), []int{0}
}

func (x *SdpOffer) GetRoomId() string {
	if x != nil {
		return x.RoomId
	}
	return ""
}

func (x *SdpOffer) GetSdp() string {
	if x != nil {
		return x.Sdp
	}
	return ""
}

type SdpAnswer struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Sdp           string                 `protobuf:"bytes,1,opt,name=sdp,proto3" json:"sdp,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SdpAnswer) Reset() {
	*x = SdpAnswer{}
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SdpAnswer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SdpAnswer) ProtoMessage() {}

func (x *SdpAnswer) ProtoReflect() protoreflect.Message {
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SdpAnswer.ProtoReflect.Descriptor instead.
func (*SdpAnswer) Descriptor() ([]byte, []int) {
	return file_rtpproxy_rtpproxy_proto_rawDescGZIP(), []int{1}
}

func (x *SdpAnswer) GetSdp() string {
	if x != nil {
		return x.Sdp
	}
	return ""
}

type RoomIdRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RoomId        string                 `protobuf:"bytes,1,opt,name=room_id,json=roomId,proto3" json:"room_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RoomIdRequest) Reset() {
	*x = RoomIdRequest{}
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RoomIdRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RoomIdRequest) ProtoMessage() {}

func (x *RoomIdRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RoomIdRequest.ProtoReflect.Descriptor instead.
func (*RoomIdRequest) Descriptor() ([]byte, []int) {
	return file_rtpproxy_rtpproxy_proto_rawDescGZIP(), []int{2}
}

func (x *RoomIdRequest) GetRoomId() string {
	if x != nil {
		return x.RoomId
	}
	return ""
}

type StopStreamStatus struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        bool                   `protobuf:"varint,1,opt,name=status,proto3" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StopStreamStatus) Reset() {
	*x = StopStreamStatus{}
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StopStreamStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StopStreamStatus) ProtoMessage() {}

func (x *StopStreamStatus) ProtoReflect() protoreflect.Message {
	mi := &file_rtpproxy_rtpproxy_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StopStreamStatus.ProtoReflect.Descriptor instead.
func (*StopStreamStatus) Descriptor() ([]byte, []int) {
	return file_rtpproxy_rtpproxy_proto_rawDescGZIP(), []int{3}
}

func (x *StopStreamStatus) GetStatus() bool {
	if x != nil {
		return x.Status
	}
	return false
}

var File_rtpproxy_rtpproxy_proto protoreflect.FileDescriptor

var file_rtpproxy_rtpproxy_proto_rawDesc = string([]byte{
	0x0a, 0x17, 0x72, 0x74, 0x70, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2f, 0x72, 0x74, 0x70, 0x70, 0x72,
	0x6f, 0x78, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x72, 0x74, 0x70, 0x70, 0x72,
	0x6f, 0x78, 0x79, 0x22, 0x35, 0x0a, 0x08, 0x53, 0x64, 0x70, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x12,
	0x17, 0x0a, 0x07, 0x72, 0x6f, 0x6f, 0x6d, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x72, 0x6f, 0x6f, 0x6d, 0x49, 0x64, 0x12, 0x10, 0x0a, 0x03, 0x73, 0x64, 0x70, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x73, 0x64, 0x70, 0x22, 0x1d, 0x0a, 0x09, 0x53, 0x64,
	0x70, 0x41, 0x6e, 0x73, 0x77, 0x65, 0x72, 0x12, 0x10, 0x0a, 0x03, 0x73, 0x64, 0x70, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x73, 0x64, 0x70, 0x22, 0x28, 0x0a, 0x0d, 0x52, 0x6f, 0x6f,
	0x6d, 0x49, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x17, 0x0a, 0x07, 0x72, 0x6f,
	0x6f, 0x6d, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x6f, 0x6f,
	0x6d, 0x49, 0x64, 0x22, 0x2a, 0x0a, 0x10, 0x53, 0x74, 0x6f, 0x70, 0x53, 0x74, 0x72, 0x65, 0x61,
	0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x32,
	0x8c, 0x01, 0x0a, 0x0f, 0x52, 0x74, 0x70, 0x50, 0x72, 0x6f, 0x78, 0x79, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x12, 0x36, 0x0a, 0x0b, 0x45, 0x78, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x53,
	0x64, 0x70, 0x12, 0x12, 0x2e, 0x72, 0x74, 0x70, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x53, 0x64,
	0x70, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x1a, 0x13, 0x2e, 0x72, 0x74, 0x70, 0x70, 0x72, 0x6f, 0x78,
	0x79, 0x2e, 0x53, 0x64, 0x70, 0x41, 0x6e, 0x73, 0x77, 0x65, 0x72, 0x12, 0x41, 0x0a, 0x0a, 0x53,
	0x74, 0x6f, 0x70, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x17, 0x2e, 0x72, 0x74, 0x70, 0x70,
	0x72, 0x6f, 0x78, 0x79, 0x2e, 0x52, 0x6f, 0x6f, 0x6d, 0x49, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x72, 0x74, 0x70, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x53, 0x74,
	0x6f, 0x70, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x42, 0x37,
	0x5a, 0x35, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x69, 0x67,
	0x68, 0x74, 0x6c, 0x69, 0x6e, 0x6b, 0x2f, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x2d, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x67, 0x65, 0x6e, 0x2f, 0x72,
	0x74, 0x70, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_rtpproxy_rtpproxy_proto_rawDescOnce sync.Once
	file_rtpproxy_rtpproxy_proto_rawDescData []byte
)

func file_rtpproxy_rtpproxy_proto_rawDescGZIP() []byte {
	file_rtpproxy_rtpproxy_proto_rawDescOnce.Do(func() {
		file_rtpproxy_rtpproxy_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_rtpproxy_rtpproxy_proto_rawDesc), len(file_rtpproxy_rtpproxy_proto_rawDesc)))
	})
	return file_rtpproxy_rtpproxy_proto_rawDescData
}

var file_rtpproxy_rtpproxy_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_rtpproxy_rtpproxy_proto_goTypes = []any{
	(*SdpOffer)(nil),         // 0: rtpproxy.SdpOffer
	(*SdpAnswer)(nil),        // 1: rtpproxy.SdpAnswer
	(*RoomIdRequest)(nil),    // 2: rtpproxy.RoomIdRequest
	(*StopStreamStatus)(nil), // 3: rtpproxy.StopStreamStatus
}
var file_rtpproxy_rtpproxy_proto_depIdxs = []int32{
	0, // 0: rtpproxy.RtpProxyService.ExchangeSdp:input_type -> rtpproxy.SdpOffer
	2, // 1: rtpproxy.RtpProxyService.StopStream:input_type -> rtpproxy.RoomIdRequest
	1, // 2: rtpproxy.RtpProxyService.ExchangeSdp:output_type -> rtpproxy.SdpAnswer
	3, // 3: rtpproxy.RtpProxyService.StopStream:output_type -> rtpproxy.StopStreamStatus
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_rtpproxy_rtpproxy_proto_init() }
func file_rtpproxy_rtpproxy_proto_init() {
	if File_rtpproxy_rtpproxy_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_rtpproxy_rtpproxy_proto_rawDesc), len(file_rtpproxy_rtpproxy_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_rtpproxy_rtpproxy_proto_goTypes,
		DependencyIndexes: file_rtpproxy_rtpproxy_proto_depIdxs,
		MessageInfos:      file_rtpproxy_rtpproxy_proto_msgTypes,
	}.Build()
	File_rtpproxy_rtpproxy_proto = out.File
	file_rtpproxy_rtpproxy_proto_goTypes = nil
	file_rtpproxy_rtpproxy_proto_depIdxs = nil
}
